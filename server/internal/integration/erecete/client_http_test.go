package erecete

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

// fakeBakanlik spins up a tiny test server that mimics e-Reçete's
// OAuth + JSON surface so we can exercise the real HTTPClient without
// hitting a remote endpoint.
func fakeBakanlik(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var tokenCalls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		tokenCalls.Add(1)
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(400)
			return
		}
		if r.FormValue("grant_type") != "client_credentials" {
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "tok-xyz",
			"expires_in":   3600,
		})
	})
	mux.HandleFunc("/api/prescriptions", func(w http.ResponseWriter, r *http.Request) {
		// Auth header zorunlu.
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			w.WriteHeader(401)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		// "BAD"+prescriptionNo → 400 reject path.
		if no, _ := body["prescriptionNo"].(string); strings.HasPrefix(no, "BAD") {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"responseCode": "BAKANLIK_REJECTED",
				"hata":         "imza doğrulanamadı",
			})
			return
		}
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ePrescriptionNo": "ER-SIM-12345",
			"responseCode":    "BAKANLIK_OK",
		})
	})
	mux.HandleFunc("/api/prescriptions/cancel", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]any{"responseCode": "BAKANLIK_CANCELLED"})
	})
	return httptest.NewServer(mux), &tokenCalls
}

func TestHTTPClient_Submit_Success(t *testing.T) {
	srv, _ := fakeBakanlik(t)
	defer srv.Close()
	c, err := NewHTTPClient(HTTPConfig{
		BaseURL: srv.URL, ClientID: "cid", ClientSecret: "csec",
	})
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	res, err := c.Submit(context.Background(), SubmitInput{
		PrescriptionID: uuid.New(),
		PrescriptionNo: "RX-00000001",
		DoctorTC:       "10000000146",
		PatientTC:      "19283746506",
		DiagnosesICD10: []string{"J45"},
		DrugATCCodes:   []string{"R03AC02"},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
	if res.EPrescriptionNo != "ER-SIM-12345" {
		t.Fatalf("wrong rx no: %q", res.EPrescriptionNo)
	}
}

func TestHTTPClient_Submit_Rejected(t *testing.T) {
	srv, _ := fakeBakanlik(t)
	defer srv.Close()
	c, _ := NewHTTPClient(HTTPConfig{
		BaseURL: srv.URL, ClientID: "cid", ClientSecret: "csec",
	})
	res, err := c.Submit(context.Background(), SubmitInput{
		PrescriptionID: uuid.New(),
		PrescriptionNo: "BAD-001", // ← rejected by fake server
		DoctorTC:       "10000000146",
		PatientTC:      "19283746506",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if res.Success {
		t.Fatal("expected rejection")
	}
	if res.ResponseCode != "BAKANLIK_REJECTED" {
		t.Fatalf("unexpected code: %s", res.ResponseCode)
	}
}

func TestHTTPClient_TokenCaching(t *testing.T) {
	srv, tokenCalls := fakeBakanlik(t)
	defer srv.Close()
	c, _ := NewHTTPClient(HTTPConfig{
		BaseURL: srv.URL, ClientID: "cid", ClientSecret: "csec",
	})
	for i := 0; i < 5; i++ {
		_, err := c.Submit(context.Background(), SubmitInput{
			PrescriptionID: uuid.New(),
			PrescriptionNo: "RX-0000000" + string(rune('0'+i)),
			DoctorTC:       "10000000146",
			PatientTC:      "19283746506",
		})
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}
	// 5 calls should share a single token fetch.
	if got := tokenCalls.Load(); got != 1 {
		t.Fatalf("expected token endpoint called once, got %d", got)
	}
}

func TestNewFromConfig_FallsBackToMockWhenCredsMissing(t *testing.T) {
	c := NewFromConfig(HTTPConfig{})
	if _, ok := c.(*MockClient); !ok {
		t.Fatalf("expected MockClient when creds missing, got %T", c)
	}
}

func TestNewFromConfig_RealWhenCredsPresent(t *testing.T) {
	srv, _ := fakeBakanlik(t)
	defer srv.Close()
	c := NewFromConfig(HTTPConfig{
		BaseURL: srv.URL, ClientID: "cid", ClientSecret: "csec",
	})
	if _, ok := c.(*HTTPClient); !ok {
		t.Fatalf("expected HTTPClient when creds present, got %T", c)
	}
}
