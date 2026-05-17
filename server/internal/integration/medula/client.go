// Package medula is the adapter for the full SGK Medula SOAP service
// surface (14 operations as of plan).
//
// Today every method is mocked deterministically so the rest of the system
// (handlers, outbox worker, UI) can develop end-to-end without SGK
// credentials. When the production certificate arrives in Sprint 1
// (~3-6 ay), only this file is replaced — every Client signature stays
// intact.
//
// Behavioural conventions for the mock:
//   - TC kimlik number ending in '0' → simulated rejection (404 / kişi
//     bulunamadı / not_found code).
//   - Anything else passes with a deterministic ID derived from the
//     request UUID so test assertions stay stable.
//   - Read-only queries return a small canned dataset; never error in
//     the absence of an error condition (write methods do, to exercise
//     the outbox retry path).
package medula

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Client is the single dispatch point for all 14 Medula operations.
// Write operations are async via the outbox; reads are sync RPC.
type Client interface {
	// --- Write (outbox-driven) ---
	RequestProvision(ctx context.Context, in ProvisionInput) (*ProvisionResponse, error)
	CancelProvision(ctx context.Context, in CancelProvisionInput) (*SimpleResponse, error)
	CloseTakip(ctx context.Context, in CloseTakipInput) (*SimpleResponse, error)
	SubmitInvoice(ctx context.Context, in InvoiceSubmitInput) (*InvoiceSubmitResponse, error)
	CancelInvoice(ctx context.Context, in CancelInvoiceInput) (*SimpleResponse, error)
	CreateReferral(ctx context.Context, in ReferralInput) (*ReferralResponse, error)
	SubmitEraport(ctx context.Context, in EraportInput) (*EraportResponse, error)
	CancelEraport(ctx context.Context, in CancelEraportInput) (*SimpleResponse, error)

	// --- Read (sync) ---
	QueryTakip(ctx context.Context, takipNo string) (*TakipDetail, error)
	QueryEraport(ctx context.Context, eraportNo string) (*EraportDetail, error)
	QueryDoctor(ctx context.Context, tc string) (*DoctorDetail, error)
	QueryBranches(ctx context.Context) ([]CodeName, error)
	QueryTreatmentTypes(ctx context.Context) ([]CodeName, error)
	QueryDrugPayment(ctx context.Context, barcode string) (*DrugPaymentDetail, error)
}

// ---------- Shared types ----------

type SimpleResponse struct {
	Success      bool
	ResponseCode string
	ResponseRaw  map[string]any
}

type CodeName struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// ---------- Provision (existing) ----------

type ProvisionInput struct {
	ProvisionID   uuid.UUID
	PatientTC     string
	ProvisionType string
	BranchCode    string
	InstitutionID *uuid.UUID
}

type ProvisionResponse struct {
	Success      bool
	TakipNo      string
	ResponseCode string
	ResponseRaw  map[string]any
}

type CancelProvisionInput struct {
	ProvisionID uuid.UUID
	TakipNo     string
	Reason      string
}

type CloseTakipInput struct {
	ProvisionID uuid.UUID
	TakipNo     string
}

// ---------- Invoice ----------

type InvoiceSubmitInput struct {
	SubmissionID uuid.UUID
	InvoiceID    uuid.UUID
	TakipNo      string
	Total        float64
	LineCount    int
}

type InvoiceSubmitResponse struct {
	Success       bool
	BatchNo       string
	SGKInvoiceNo  string
	ResponseCode  string
	ResponseRaw   map[string]any
}

type CancelInvoiceInput struct {
	SubmissionID uuid.UUID
	SGKInvoiceNo string
	Reason       string
}

// ---------- Referral ----------

type ReferralInput struct {
	ReferralID         uuid.UUID
	PatientTC          string
	TargetProviderCode string
	TargetBranchCode   string
	ReferralType       string
	Reason             string
	DiagnosisICD10     string
}

type ReferralResponse struct {
	Success      bool
	SevkNo       string
	ResponseCode string
	ResponseRaw  map[string]any
}

// ---------- e-Report ----------

type EraportInput struct {
	EraportID      uuid.UUID
	PatientTC      string
	DoctorTC       string
	Kind           string   // chronic_drug / inpatient / work_incapacity / special_procedure
	DiagnosesICD10 []string
	DrugCodes      []string
	ValidFrom      string   // YYYY-MM-DD
	ValidTo        string   // YYYY-MM-DD (optional, empty = open-ended)
}

type EraportResponse struct {
	Success      bool
	EraportNo    string
	ResponseCode string
	ResponseRaw  map[string]any
}

type CancelEraportInput struct {
	EraportID uuid.UUID
	EraportNo string
	Reason    string
}

// ---------- Read responses ----------

type TakipDetail struct {
	TakipNo       string         `json:"takip_no"`
	Status        string         `json:"status"`
	ProvisionType string         `json:"provision_type"`
	OpenedAt      string         `json:"opened_at"`
	ClosedAt      string         `json:"closed_at,omitempty"`
	Patient       map[string]any `json:"patient"`
	Raw           map[string]any `json:"raw"`
}

type EraportDetail struct {
	EraportNo   string         `json:"eraport_no"`
	Status      string         `json:"status"`
	Kind        string         `json:"kind"`
	ValidFrom   string         `json:"valid_from"`
	ValidTo     string         `json:"valid_to,omitempty"`
	Diagnoses   []string       `json:"diagnoses"`
	DrugCodes   []string       `json:"drug_codes"`
	Raw         map[string]any `json:"raw"`
}

type DoctorDetail struct {
	MedulaDoctorCode string `json:"medula_doctor_code"`
	FullName         string `json:"full_name"`
	BranchCode       string `json:"branch_code"`
	BranchName       string `json:"branch_name"`
	IsActive         bool   `json:"is_active"`
}

type DrugPaymentDetail struct {
	Barcode       string  `json:"barcode"`
	DrugName      string  `json:"drug_name"`
	IsReimbursed  bool    `json:"is_reimbursed"`
	PatientShare  float64 `json:"patient_share_pct"`
	Notes         string  `json:"notes,omitempty"`
}

// ---------- Errors ----------

var (
	ErrInvalidProvisionInput = errors.New("provizyon girdisi geçersiz")
	ErrInvalidInvoiceInput   = errors.New("fatura girdisi geçersiz")
	ErrInvalidReferralInput  = errors.New("sevk girdisi geçersiz")
	ErrInvalidEraportInput   = errors.New("e-rapor girdisi geçersiz")
)

// ============================================================================
//  MockClient — deterministic simulation
// ============================================================================

type MockClient struct{}

func NewMockClient() *MockClient { return &MockClient{} }

// simSleep is the canonical mock latency. ctx cancellation honoured.
func simSleep(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// idTag returns last 6 upper-case characters of a UUID, used to fabricate
// deterministic SGK identifiers.
func idTag(id uuid.UUID) string {
	s := id.String()
	return strings.ToUpper(s[len(s)-6:])
}

// rejectIfTCZero is the canonical "this would fail at SGK" simulation.
func rejectIfTCZero(tc string) (bool, *SimpleResponse) {
	if len(tc) != 11 {
		return true, &SimpleResponse{Success: false, ResponseCode: "SGK_BAD_TC",
			ResponseRaw: map[string]any{"hata": "TC formatı geçersiz"}}
	}
	if strings.HasSuffix(tc, "0") {
		return true, &SimpleResponse{Success: false, ResponseCode: "SGK_TARGET_NOT_FOUND",
			ResponseRaw: map[string]any{"hata": "kişi bulunamadı"}}
	}
	return false, nil
}

// ---------- Provision ops ----------

func (m *MockClient) RequestProvision(ctx context.Context, in ProvisionInput) (*ProvisionResponse, error) {
	if err := simSleep(ctx, 200*time.Millisecond); err != nil {
		return nil, err
	}
	if len(in.PatientTC) != 11 {
		return nil, ErrInvalidProvisionInput
	}
	if strings.HasSuffix(in.PatientTC, "0") {
		return &ProvisionResponse{
			Success:      false,
			ResponseCode: "SGK_TARGET_NOT_FOUND",
			ResponseRaw:  map[string]any{"hata": "kişi bulunamadı", "code": "404"},
		}, nil
	}
	takip := "TKP" + idTag(in.ProvisionID)
	return &ProvisionResponse{
		Success:      true,
		TakipNo:      takip,
		ResponseCode: "SGK_OK_SIM",
		ResponseRaw: map[string]any{
			"takipNo":       takip,
			"provisionType": in.ProvisionType,
			"branchCode":    in.BranchCode,
			"insertedAtSim": time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}

func (m *MockClient) CancelProvision(ctx context.Context, in CancelProvisionInput) (*SimpleResponse, error) {
	if err := simSleep(ctx, 150*time.Millisecond); err != nil {
		return nil, err
	}
	if in.TakipNo == "" {
		return &SimpleResponse{Success: false, ResponseCode: "SGK_BAD_TAKIP",
			ResponseRaw: map[string]any{"hata": "takipNo zorunlu"}}, nil
	}
	return &SimpleResponse{
		Success:      true,
		ResponseCode: "SGK_CANCELLED_SIM",
		ResponseRaw:  map[string]any{"cancelledAt": time.Now().UTC().Format(time.RFC3339), "reason": in.Reason},
	}, nil
}

func (m *MockClient) CloseTakip(ctx context.Context, in CloseTakipInput) (*SimpleResponse, error) {
	if err := simSleep(ctx, 150*time.Millisecond); err != nil {
		return nil, err
	}
	if in.TakipNo == "" {
		return &SimpleResponse{Success: false, ResponseCode: "SGK_BAD_TAKIP",
			ResponseRaw: map[string]any{"hata": "takipNo zorunlu"}}, nil
	}
	return &SimpleResponse{
		Success:      true,
		ResponseCode: "SGK_TAKIP_CLOSED_SIM",
		ResponseRaw:  map[string]any{"closedAt": time.Now().UTC().Format(time.RFC3339)},
	}, nil
}

// ---------- Invoice ops ----------

func (m *MockClient) SubmitInvoice(ctx context.Context, in InvoiceSubmitInput) (*InvoiceSubmitResponse, error) {
	if err := simSleep(ctx, 300*time.Millisecond); err != nil {
		return nil, err
	}
	if in.Total <= 0 || in.LineCount == 0 {
		return nil, ErrInvalidInvoiceInput
	}
	if in.TakipNo != "" && strings.HasSuffix(in.TakipNo, "0") {
		// Sertifika-öncesi bir red yolunu simüle etmek için.
		return &InvoiceSubmitResponse{
			Success:      false,
			ResponseCode: "SGK_INVOICE_REJECTED",
			ResponseRaw:  map[string]any{"hata": "takip eşleşmiyor"},
		}, nil
	}
	tag := idTag(in.SubmissionID)
	return &InvoiceSubmitResponse{
		Success:      true,
		BatchNo:      "BTC" + tag,
		SGKInvoiceNo: "SGK" + tag,
		ResponseCode: "SGK_INVOICE_OK_SIM",
		ResponseRaw: map[string]any{
			"batchNo":      "BTC" + tag,
			"sgkInvoiceNo": "SGK" + tag,
			"total":        in.Total,
			"lineCount":    in.LineCount,
			"submittedAt":  time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}

func (m *MockClient) CancelInvoice(ctx context.Context, in CancelInvoiceInput) (*SimpleResponse, error) {
	if err := simSleep(ctx, 150*time.Millisecond); err != nil {
		return nil, err
	}
	if in.SGKInvoiceNo == "" {
		return &SimpleResponse{Success: false, ResponseCode: "SGK_BAD_INVOICE_NO",
			ResponseRaw: map[string]any{"hata": "sgkInvoiceNo zorunlu"}}, nil
	}
	return &SimpleResponse{
		Success:      true,
		ResponseCode: "SGK_INVOICE_CANCELLED_SIM",
		ResponseRaw:  map[string]any{"cancelledAt": time.Now().UTC().Format(time.RFC3339), "reason": in.Reason},
	}, nil
}

// ---------- Referral ----------

func (m *MockClient) CreateReferral(ctx context.Context, in ReferralInput) (*ReferralResponse, error) {
	if err := simSleep(ctx, 200*time.Millisecond); err != nil {
		return nil, err
	}
	if rejected, simple := rejectIfTCZero(in.PatientTC); rejected {
		return &ReferralResponse{
			Success: false, ResponseCode: simple.ResponseCode, ResponseRaw: simple.ResponseRaw,
		}, nil
	}
	if in.TargetProviderCode == "" || in.Reason == "" {
		return nil, ErrInvalidReferralInput
	}
	sevk := "SVK" + idTag(in.ReferralID)
	return &ReferralResponse{
		Success:      true,
		SevkNo:       sevk,
		ResponseCode: "SGK_REFERRAL_OK_SIM",
		ResponseRaw: map[string]any{
			"sevkNo":             sevk,
			"targetProviderCode": in.TargetProviderCode,
			"targetBranchCode":   in.TargetBranchCode,
			"type":               in.ReferralType,
			"createdAtSim":       time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}

// ---------- e-Report ----------

func (m *MockClient) SubmitEraport(ctx context.Context, in EraportInput) (*EraportResponse, error) {
	if err := simSleep(ctx, 250*time.Millisecond); err != nil {
		return nil, err
	}
	if rejected, simple := rejectIfTCZero(in.PatientTC); rejected {
		return &EraportResponse{
			Success: false, ResponseCode: simple.ResponseCode, ResponseRaw: simple.ResponseRaw,
		}, nil
	}
	if in.Kind == "" || in.ValidFrom == "" {
		return nil, ErrInvalidEraportInput
	}
	if len(in.DiagnosesICD10) == 0 {
		return &EraportResponse{
			Success: false, ResponseCode: "SGK_ERAPORT_NO_DIAGNOSIS",
			ResponseRaw: map[string]any{"hata": "en az 1 tanı kodu gerekli"},
		}, nil
	}
	no := "RPR" + idTag(in.EraportID)
	return &EraportResponse{
		Success:      true,
		EraportNo:    no,
		ResponseCode: "SGK_ERAPORT_OK_SIM",
		ResponseRaw: map[string]any{
			"eraportNo": no,
			"kind":      in.Kind,
			"validFrom": in.ValidFrom,
			"validTo":   in.ValidTo,
			"createdAt": time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}

func (m *MockClient) CancelEraport(ctx context.Context, in CancelEraportInput) (*SimpleResponse, error) {
	if err := simSleep(ctx, 150*time.Millisecond); err != nil {
		return nil, err
	}
	if in.EraportNo == "" {
		return &SimpleResponse{Success: false, ResponseCode: "SGK_BAD_ERAPORT_NO",
			ResponseRaw: map[string]any{"hata": "eraportNo zorunlu"}}, nil
	}
	return &SimpleResponse{
		Success:      true,
		ResponseCode: "SGK_ERAPORT_CANCELLED_SIM",
		ResponseRaw:  map[string]any{"cancelledAt": time.Now().UTC().Format(time.RFC3339), "reason": in.Reason},
	}, nil
}

// ---------- Read queries ----------

func (m *MockClient) QueryTakip(ctx context.Context, takipNo string) (*TakipDetail, error) {
	if err := simSleep(ctx, 100*time.Millisecond); err != nil {
		return nil, err
	}
	if takipNo == "" {
		return nil, fmt.Errorf("takipNo zorunlu")
	}
	return &TakipDetail{
		TakipNo:       takipNo,
		Status:        "OPEN",
		ProvisionType: "normal",
		OpenedAt:      time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
		Patient:       map[string]any{"tcLast4": "****", "fullName": "Simülasyon Hasta"},
		Raw:           map[string]any{"source": "mock"},
	}, nil
}

func (m *MockClient) QueryEraport(ctx context.Context, eraportNo string) (*EraportDetail, error) {
	if err := simSleep(ctx, 100*time.Millisecond); err != nil {
		return nil, err
	}
	if eraportNo == "" {
		return nil, fmt.Errorf("eraportNo zorunlu")
	}
	return &EraportDetail{
		EraportNo: eraportNo,
		Status:    "APPROVED",
		Kind:      "chronic_drug",
		ValidFrom: time.Now().Add(-30 * 24 * time.Hour).UTC().Format("2006-01-02"),
		ValidTo:   time.Now().Add(335 * 24 * time.Hour).UTC().Format("2006-01-02"),
		Diagnoses: []string{"E11", "I10"},
		DrugCodes: []string{"A10BA02", "C09AA02"},
		Raw:       map[string]any{"source": "mock"},
	}, nil
}

func (m *MockClient) QueryDoctor(ctx context.Context, tc string) (*DoctorDetail, error) {
	if err := simSleep(ctx, 100*time.Millisecond); err != nil {
		return nil, err
	}
	if rejected, simple := rejectIfTCZero(tc); rejected {
		return nil, fmt.Errorf("%s", simple.ResponseCode)
	}
	return &DoctorDetail{
		MedulaDoctorCode: "DR" + tc[len(tc)-6:],
		FullName:         "Simülasyon Doktor",
		BranchCode:       "1900",
		BranchName:       "Genel Cerrahi",
		IsActive:         true,
	}, nil
}

// QueryBranches returns SGK branş kodları — small canned list, real SGK
// list has ~70 codes. Sufficient for mock testing.
func (m *MockClient) QueryBranches(ctx context.Context) ([]CodeName, error) {
	if err := simSleep(ctx, 80*time.Millisecond); err != nil {
		return nil, err
	}
	return []CodeName{
		{Code: "1000", Name: "Aile Hekimliği"},
		{Code: "1100", Name: "Dahiliye"},
		{Code: "1200", Name: "Genel Cerrahi"},
		{Code: "1300", Name: "Kardiyoloji"},
		{Code: "1400", Name: "Çocuk Sağlığı ve Hastalıkları"},
		{Code: "1500", Name: "Kadın Hastalıkları ve Doğum"},
		{Code: "1600", Name: "Göz Hastalıkları"},
		{Code: "1700", Name: "Kulak Burun Boğaz"},
		{Code: "1800", Name: "Ortopedi ve Travmatoloji"},
		{Code: "1900", Name: "Üroloji"},
		{Code: "2000", Name: "Nöroloji"},
		{Code: "2100", Name: "Ruh Sağlığı ve Hastalıkları"},
	}, nil
}

func (m *MockClient) QueryTreatmentTypes(ctx context.Context) ([]CodeName, error) {
	if err := simSleep(ctx, 80*time.Millisecond); err != nil {
		return nil, err
	}
	return []CodeName{
		{Code: "N", Name: "Normal Ayaktan"},
		{Code: "A", Name: "Acil"},
		{Code: "Y", Name: "Yatış"},
		{Code: "G", Name: "Günübirlik"},
		{Code: "K", Name: "Kontrol"},
		{Code: "T", Name: "Travma"},
	}, nil
}

func (m *MockClient) QueryDrugPayment(ctx context.Context, barcode string) (*DrugPaymentDetail, error) {
	if err := simSleep(ctx, 120*time.Millisecond); err != nil {
		return nil, err
	}
	if barcode == "" {
		return nil, fmt.Errorf("barcode zorunlu")
	}
	// Deterministic: barkodun son hanesi tek ise %20, çift ise %0 katılım.
	last := barcode[len(barcode)-1:]
	share := 0.0
	if last == "1" || last == "3" || last == "5" || last == "7" || last == "9" {
		share = 20.0
	}
	return &DrugPaymentDetail{
		Barcode:      barcode,
		DrugName:     "Simülasyon ilaç (" + barcode + ")",
		IsReimbursed: true,
		PatientShare: share,
		Notes:        "mock yanıtı",
	}, nil
}
