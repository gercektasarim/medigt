// Package erecete is the adapter for Sağlık Bakanlığı's e-Reçete system.
//
// Today this is a mock — deterministic behaviour:
//   - prescription_no'nun son hanesi '0' ise rejected
//   - aksi halde "ER" + son 6 karakterli rapor numarası ile submitted
//
// Real client will go via the Bakanlık's SOAP endpoint with TURKKEP /
// TURKTRUST e-imza header; only this file changes when certification
// arrives.
package erecete

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Client interface {
	Submit(ctx context.Context, in SubmitInput) (*SubmitResponse, error)
	Cancel(ctx context.Context, in CancelInput) (*CancelResponse, error)
	Query(ctx context.Context, eRxNo string) (*QueryResponse, error)
}

type SubmitInput struct {
	PrescriptionID  uuid.UUID
	PrescriptionNo  string
	DoctorTC        string
	PatientTC       string
	DiagnosesICD10  []string
	DrugATCCodes    []string
}

type SubmitResponse struct {
	Success         bool
	EPrescriptionNo string
	ResponseCode    string
	Raw             map[string]any
}

type CancelInput struct {
	PrescriptionID  uuid.UUID
	EPrescriptionNo string
	Reason          string
}

type CancelResponse struct {
	Success      bool
	ResponseCode string
	Raw          map[string]any
}

type QueryResponse struct {
	EPrescriptionNo string         `json:"e_prescription_no"`
	Status          string         `json:"status"`
	DispensedAt     string         `json:"dispensed_at,omitempty"`
	Raw             map[string]any `json:"raw"`
}

var ErrInvalidInput = errors.New("e-reçete girdisi geçersiz")

type MockClient struct{}

func NewMockClient() *MockClient { return &MockClient{} }

func (m *MockClient) Submit(ctx context.Context, in SubmitInput) (*SubmitResponse, error) {
	select {
	case <-time.After(150 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if in.PrescriptionNo == "" || in.PatientTC == "" || in.DoctorTC == "" {
		return nil, ErrInvalidInput
	}
	if strings.HasSuffix(in.PrescriptionNo, "0") {
		return &SubmitResponse{
			Success: false, ResponseCode: "BAKANLIK_REJECTED",
			Raw: map[string]any{"hata": "imza doğrulanamadı"},
		}, nil
	}
	idStr := in.PrescriptionID.String()
	no := "ER" + strings.ToUpper(idStr[len(idStr)-6:])
	return &SubmitResponse{
		Success:         true,
		EPrescriptionNo: no,
		ResponseCode:    "BAKANLIK_OK_SIM",
		Raw: map[string]any{
			"ePrescriptionNo": no,
			"diagnoses":       in.DiagnosesICD10,
			"drugs":           in.DrugATCCodes,
			"submittedAtSim":  time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}

func (m *MockClient) Cancel(ctx context.Context, in CancelInput) (*CancelResponse, error) {
	select {
	case <-time.After(100 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if in.EPrescriptionNo == "" {
		return &CancelResponse{Success: false, ResponseCode: "BAKANLIK_BAD_NO",
			Raw: map[string]any{"hata": "ePrescriptionNo zorunlu"}}, nil
	}
	return &CancelResponse{
		Success: true, ResponseCode: "BAKANLIK_CANCELLED_SIM",
		Raw: map[string]any{"cancelledAt": time.Now().UTC().Format(time.RFC3339), "reason": in.Reason},
	}, nil
}

func (m *MockClient) Query(ctx context.Context, eRxNo string) (*QueryResponse, error) {
	select {
	case <-time.After(100 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &QueryResponse{
		EPrescriptionNo: eRxNo,
		Status:          "DISPENSED",
		DispensedAt:     time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339),
		Raw:             map[string]any{"source": "mock"},
	}, nil
}
