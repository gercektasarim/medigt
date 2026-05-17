// Package its is the adapter for Sağlık Bakanlığı's İlaç Takip Sistemi (İTS).
//
// Eczane dispense ettikten sonra ilacın karekodunu (GTIN+lot+SKT+seri) hangi
// hastaya verildiğini İTS'e bildirmek zorunda. Bu mock geliştirme zamanında
// devre dışı kalmasın diye deterministik cevap döner.
package its

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Client interface {
	Notify(ctx context.Context, in NotifyInput) (*NotifyResponse, error)
}

type NotifyInput struct {
	DispenseID    uuid.UUID
	Karekod       string
	PatientTC     string
	DispensedAt   time.Time
	PharmacistTC  string
	Quantity      float64
}

type NotifyResponse struct {
	Success      bool
	ResponseCode string
	Raw          map[string]any
}

var ErrInvalidInput = errors.New("İTS bildirim girdisi geçersiz")

type MockClient struct{}

func NewMockClient() *MockClient { return &MockClient{} }

func (m *MockClient) Notify(ctx context.Context, in NotifyInput) (*NotifyResponse, error) {
	select {
	case <-time.After(120 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if in.Karekod == "" {
		return nil, ErrInvalidInput
	}
	if in.PatientTC == "" {
		return &NotifyResponse{Success: false, ResponseCode: "ITS_PATIENT_TC_REQUIRED",
			Raw: map[string]any{"hata": "hasta TC zorunlu"}}, nil
	}
	// Simülasyon: karekod son hanesi '0' ise red.
	if strings.HasSuffix(in.Karekod, "0") {
		return &NotifyResponse{
			Success: false, ResponseCode: "ITS_BARCODE_UNKNOWN",
			Raw: map[string]any{"hata": "karekod bulunamadı"},
		}, nil
	}
	return &NotifyResponse{
		Success: true, ResponseCode: "ITS_OK_SIM",
		Raw: map[string]any{
			"karekod":     in.Karekod,
			"notifiedAt":  time.Now().UTC().Format(time.RFC3339),
			"patientHash": "***" + in.PatientTC[len(in.PatientTC)-4:],
		},
	}, nil
}
