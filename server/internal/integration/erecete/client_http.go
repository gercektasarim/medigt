package erecete

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// HTTPClient calls the real e-Reçete endpoint. Bakanlık serves a JSON
// surface (some servisler hâlâ SOAP üzerinden; cert geldiğinde gerçek
// kontrat yenilenir) so we keep the shape JSON for simplicity and
// translate as needed in the body builders.
//
// Auth: Sağlık Bakanlığı kabul/test ortamında OAuth2 client_credentials
// akışı kullanılır. Token uzun ömürlüdür (~12 sa) — burada in-memory
// cache + refresh-when-near-expiry yapısı var. Production cert geldiğinde
// muhtemelen yenileme sıklığı + scope listesi netleşir; sadece bu dosya
// değişir. Interface (`erecete.Client`) sabit kalıyor.
type HTTPClient struct {
	baseURL      string
	clientID     string
	clientSecret string
	doctorTCFn   func() string // optional override; production hospital may run multiple doctors per process
	httpClient   *http.Client
	log          *slog.Logger

	tokenMu     sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

type HTTPConfig struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	// Optional — defaults to 15s. Production cert sürecinde Bakanlık
	// uzun cevap verebilir (~30s); call site override edebilir.
	Timeout time.Duration
	Logger  *slog.Logger
}

// NewHTTPClient constructs a real client. Returns nil + error if cred set
// is incomplete — the caller (main) is expected to fall back to mock in
// that case.
func NewHTTPClient(cfg HTTPConfig) (*HTTPClient, error) {
	if cfg.BaseURL == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("e-reçete real client: baseURL + clientID + clientSecret zorunlu")
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	return &HTTPClient{
		baseURL:      strings.TrimRight(cfg.BaseURL, "/"),
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		httpClient:   &http.Client{Timeout: timeout},
		log:          log,
	}, nil
}

// ensureToken gets a fresh OAuth2 access token if the cached one is gone
// or within 60 seconds of expiry. Wrapped under tokenMu so a flood of
// outbox messages doesn't trigger N parallel /token calls.
func (c *HTTPClient) ensureToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.cachedToken != "" && time.Until(c.tokenExpiry) > 60*time.Second {
		return c.cachedToken, nil
	}

	form := strings.NewReader(
		"grant_type=client_credentials" +
			"&client_id=" + c.clientID +
			"&client_secret=" + c.clientSecret +
			"&scope=erecete",
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/oauth/token", form)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("oauth/token %d: %s", resp.StatusCode, body)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", err
	}
	c.cachedToken = tok.AccessToken
	// Fall back to 1h if Bakanlık omits expires_in.
	ttl := time.Duration(tok.ExpiresIn) * time.Second
	if ttl == 0 {
		ttl = time.Hour
	}
	c.tokenExpiry = time.Now().Add(ttl)
	return c.cachedToken, nil
}

// doJSON runs an authed POST with a JSON body, returning the raw decoded
// map plus the HTTP status. We keep the response as a generic map so the
// outbox worker can JSONB-persist it for audit + reconciliation reports.
func (c *HTTPClient) doJSON(ctx context.Context, path string, body any) (map[string]any, int, error) {
	tok, err := c.ensureToken(ctx)
	if err != nil {
		return nil, 0, err
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	if len(respBody) > 0 {
		_ = json.Unmarshal(respBody, &out)
	}
	return out, resp.StatusCode, nil
}

// ---------- Client interface impl ----------

func (c *HTTPClient) Submit(ctx context.Context, in SubmitInput) (*SubmitResponse, error) {
	if in.PrescriptionNo == "" || in.PatientTC == "" || in.DoctorTC == "" {
		return nil, ErrInvalidInput
	}
	payload := map[string]any{
		"prescriptionNo": in.PrescriptionNo,
		"doctorTC":       in.DoctorTC,
		"patientTC":      in.PatientTC,
		"diagnoses":      in.DiagnosesICD10,
		"drugs":          in.DrugATCCodes,
		"prescriptionId": in.PrescriptionID.String(),
	}
	raw, status, err := c.doJSON(ctx, "/api/prescriptions", payload)
	if err != nil {
		return nil, err
	}
	if status >= 500 {
		// Transient — let the outbox retry.
		return nil, fmt.Errorf("e-reçete 5xx: status=%d", status)
	}
	if status >= 400 {
		code, _ := raw["responseCode"].(string)
		if code == "" {
			code = fmt.Sprintf("HTTP_%d", status)
		}
		return &SubmitResponse{Success: false, ResponseCode: code, Raw: raw}, nil
	}
	rxNo, _ := raw["ePrescriptionNo"].(string)
	code, _ := raw["responseCode"].(string)
	if code == "" {
		code = "BAKANLIK_OK"
	}
	if rxNo == "" {
		return &SubmitResponse{
			Success: false, ResponseCode: "MISSING_RX_NO", Raw: raw,
		}, nil
	}
	return &SubmitResponse{
		Success:         true,
		EPrescriptionNo: rxNo,
		ResponseCode:    code,
		Raw:             raw,
	}, nil
}

func (c *HTTPClient) Cancel(ctx context.Context, in CancelInput) (*CancelResponse, error) {
	if in.EPrescriptionNo == "" {
		return &CancelResponse{
			Success: false, ResponseCode: "BAKANLIK_BAD_NO",
			Raw: map[string]any{"hata": "ePrescriptionNo zorunlu"},
		}, nil
	}
	payload := map[string]any{
		"ePrescriptionNo": in.EPrescriptionNo,
		"reason":          in.Reason,
		"prescriptionId":  in.PrescriptionID.String(),
	}
	raw, status, err := c.doJSON(ctx, "/api/prescriptions/cancel", payload)
	if err != nil {
		return nil, err
	}
	if status >= 500 {
		return nil, fmt.Errorf("e-reçete cancel 5xx: status=%d", status)
	}
	if status >= 400 {
		code, _ := raw["responseCode"].(string)
		if code == "" {
			code = fmt.Sprintf("HTTP_%d", status)
		}
		return &CancelResponse{Success: false, ResponseCode: code, Raw: raw}, nil
	}
	code, _ := raw["responseCode"].(string)
	if code == "" {
		code = "BAKANLIK_CANCELLED"
	}
	return &CancelResponse{Success: true, ResponseCode: code, Raw: raw}, nil
}

func (c *HTTPClient) Query(ctx context.Context, eRxNo string) (*QueryResponse, error) {
	raw, status, err := c.doJSON(ctx, "/api/prescriptions/query", map[string]any{
		"ePrescriptionNo": eRxNo,
	})
	if err != nil {
		return nil, err
	}
	if status >= 500 {
		return nil, fmt.Errorf("e-reçete query 5xx: status=%d", status)
	}
	if status >= 400 {
		return nil, fmt.Errorf("e-reçete query %d: %v", status, raw["error"])
	}
	out := &QueryResponse{
		EPrescriptionNo: eRxNo,
		Raw:             raw,
	}
	if v, ok := raw["status"].(string); ok {
		out.Status = v
	}
	if v, ok := raw["dispensedAt"].(string); ok {
		out.DispensedAt = v
	}
	return out, nil
}
