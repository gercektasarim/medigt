package its

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func fakeITS(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "tok-its",
			"expires_in":   3600,
		})
	})
	mux.HandleFunc("/api/dispense", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			w.WriteHeader(401)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		karekod, _ := body["karekod"].(string)
		if strings.HasSuffix(karekod, "X") {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"responseCode": "ITS_BARCODE_UNKNOWN",
				"hata":         "karekod bulunamadı",
			})
			return
		}
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]any{"responseCode": "ITS_OK"})
	})
	return httptest.NewServer(mux)
}

func TestITS_HTTPClient_Success(t *testing.T) {
	srv := fakeITS(t)
	defer srv.Close()
	c, err := NewHTTPClient(HTTPConfig{
		BaseURL: srv.URL, ClientID: "cid", ClientSecret: "csec",
	})
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	res, err := c.Notify(context.Background(), NotifyInput{
		DispenseID:   uuid.New(),
		Karekod:      "0809421036123456789",
		PatientTC:    "10000000146",
		PharmacistTC: "19283746506",
		Quantity:     1,
		DispensedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("notify: %v", err)
	}
	if !res.Success || res.ResponseCode != "ITS_OK" {
		t.Fatalf("unexpected: %+v", res)
	}
}

func TestITS_HTTPClient_Rejected(t *testing.T) {
	srv := fakeITS(t)
	defer srv.Close()
	c, _ := NewHTTPClient(HTTPConfig{
		BaseURL: srv.URL, ClientID: "cid", ClientSecret: "csec",
	})
	res, err := c.Notify(context.Background(), NotifyInput{
		DispenseID:   uuid.New(),
		Karekod:      "ABCDEFGHIJX", // fake server rejects suffix 'X'
		PatientTC:    "10000000146",
		PharmacistTC: "19283746506",
		Quantity:     1,
		DispensedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("notify: %v", err)
	}
	if res.Success {
		t.Fatal("expected rejection")
	}
}

func TestITS_FactoryFallsBack(t *testing.T) {
	c := NewFromConfig(HTTPConfig{})
	if _, ok := c.(*MockClient); !ok {
		t.Fatalf("expected MockClient on missing creds, got %T", c)
	}
}
