package its

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

// HTTPClient calls the real İTS endpoint. Same shape as the erecete real
// client — OAuth client_credentials, in-memory token cache, JSON body
// + JSON response. The exact path + payload schema will be refined
// during the cert process; this is the V1 placeholder that production
// ops can iterate on without touching the worker.
type HTTPClient struct {
	baseURL      string
	clientID     string
	clientSecret string
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
	Timeout      time.Duration
	Logger       *slog.Logger
}

func NewHTTPClient(cfg HTTPConfig) (*HTTPClient, error) {
	if cfg.BaseURL == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("İTS real client: baseURL + clientID + clientSecret zorunlu")
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
			"&scope=its",
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
	ttl := time.Duration(tok.ExpiresIn) * time.Second
	if ttl == 0 {
		ttl = time.Hour
	}
	c.tokenExpiry = time.Now().Add(ttl)
	return c.cachedToken, nil
}

func (c *HTTPClient) Notify(ctx context.Context, in NotifyInput) (*NotifyResponse, error) {
	if in.Karekod == "" {
		return nil, ErrInvalidInput
	}
	if in.PatientTC == "" {
		return &NotifyResponse{
			Success:      false,
			ResponseCode: "ITS_PATIENT_TC_REQUIRED",
			Raw:          map[string]any{"hata": "hasta TC zorunlu"},
		}, nil
	}
	tok, err := c.ensureToken(ctx)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"dispenseId":   in.DispenseID.String(),
		"karekod":      in.Karekod,
		"patientTC":    in.PatientTC,
		"pharmacistTC": in.PharmacistTC,
		"dispensedAt":  in.DispensedAt.UTC().Format(time.RFC3339),
		"quantity":     in.Quantity,
	}
	buf, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/dispense", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	raw := map[string]any{}
	if len(body) > 0 {
		_ = json.Unmarshal(body, &raw)
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("İTS 5xx: status=%d body=%s", resp.StatusCode, body)
	}
	if resp.StatusCode >= 400 {
		code, _ := raw["responseCode"].(string)
		if code == "" {
			code = fmt.Sprintf("HTTP_%d", resp.StatusCode)
		}
		return &NotifyResponse{Success: false, ResponseCode: code, Raw: raw}, nil
	}
	code, _ := raw["responseCode"].(string)
	if code == "" {
		code = "ITS_OK"
	}
	return &NotifyResponse{Success: true, ResponseCode: code, Raw: raw}, nil
}
