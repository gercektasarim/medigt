// Package enabiz adapts MediGt to e-Nabız (Sağlık Bakanlığı's national
// PHR). e-Nabız ingests FHIR R4 resources over HTTPS — Encounter,
// Observation, Condition, MedicationRequest — and links them to the
// patient's TC. Every Turkish hospital is legally required to push
// clinical events here (mevzuat 2023+ kademeli).
//
// Today this is a mock with deterministic behaviour for development +
// pilot testing:
//   - Patient TC ending in '0' → REJECTED (simulates uppermost fail rate)
//   - Otherwise SUCCESS with a deterministic 12-char receipt id
//
// Real client will go via the Bakanlık's REST FHIR endpoint with the
// hospital's CLIENT_ID/SECRET OAuth flow + per-doctor e-imza on the
// resource body. Only this file changes when production credentials
// arrive. The outbox pattern (see repo.EnabizMessageRepo) guarantees
// at-least-once delivery; the worker drains pending rows with retries.
package enabiz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ResourceKind mirrors the FHIR resource types we push to e-Nabız.
type ResourceKind string

const (
	KindEncounter         ResourceKind = "Encounter"
	KindObservation       ResourceKind = "Observation"
	KindCondition         ResourceKind = "Condition"        // ICD-10 tanı
	KindMedicationRequest ResourceKind = "MedicationRequest"
	KindDiagnosticReport  ResourceKind = "DiagnosticReport" // lab + radyoloji raporu
)

type Client interface {
	// SubmitResource pushes a single FHIR resource. Caller provides the
	// already-serialized JSON body so this layer stays transport-only.
	SubmitResource(ctx context.Context, in SubmitInput) (*SubmitResponse, error)
	// QueryPatient looks up what e-Nabız already has on a TC. Useful for
	// reconciliation reports — "did our last 50 visits all land?"
	QueryPatient(ctx context.Context, tc string) (*PatientSummary, error)
}

type SubmitInput struct {
	MessageID    uuid.UUID    // outbox row id — used as Idempotency-Key
	PatientTC    string       // 11 digits; SDK side validates checksum
	Kind         ResourceKind // Encounter / Observation / ...
	ResourceJSON []byte       // FHIR R4 resource serialized
}

type SubmitResponse struct {
	Success      bool
	ReceiptID    string         // Bakanlık tarafı id (verildiğinde)
	ResponseCode string         // BAKANLIK_OK / BAKANLIK_REJECTED / ...
	Raw          map[string]any // ham JSON gövdesi (audit için)
}

type PatientSummary struct {
	TC               string         `json:"tc"`
	EncountersCount  int            `json:"encounters_count"`
	LastEncounterAt  string         `json:"last_encounter_at,omitempty"`
	ObservationCount int            `json:"observation_count"`
	Raw              map[string]any `json:"raw"`
}

var ErrInvalidInput = errors.New("e-Nabız girdisi geçersiz")

// ---------- Mock ----------

type MockClient struct{}

func NewMockClient() *MockClient { return &MockClient{} }

func (m *MockClient) SubmitResource(ctx context.Context, in SubmitInput) (*SubmitResponse, error) {
	select {
	case <-time.After(180 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if in.PatientTC == "" || len(in.ResourceJSON) == 0 || in.Kind == "" {
		return nil, ErrInvalidInput
	}
	// Deterministic fail-path for tests / pilot QA: any TC ending in '0'
	// gets rejected. Mirrors the eRecete mock's nudge for negative cases.
	if strings.HasSuffix(in.PatientTC, "0") {
		return &SubmitResponse{
			Success:      false,
			ResponseCode: "BAKANLIK_REJECTED_SIM",
			Raw: map[string]any{
				"hata":     "imza doğrulanamadı (mock)",
				"kind":     string(in.Kind),
				"patient":  maskTCLast4(in.PatientTC),
				"receivedAt": time.Now().UTC().Format(time.RFC3339),
			},
		}, nil
	}
	// Deterministic receipt id derived from the outbox row's UUID — same
	// input ⇒ same receipt, which makes retries idempotent on our side.
	sum := sha256.Sum256([]byte(in.MessageID.String()))
	receipt := "ENB-" + strings.ToUpper(hex.EncodeToString(sum[:])[:8])
	return &SubmitResponse{
		Success:      true,
		ReceiptID:    receipt,
		ResponseCode: "BAKANLIK_OK_SIM",
		Raw: map[string]any{
			"receiptId":     receipt,
			"kind":          string(in.Kind),
			"acceptedAtSim": time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}

func (m *MockClient) QueryPatient(ctx context.Context, tc string) (*PatientSummary, error) {
	select {
	case <-time.After(120 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if len(tc) != 11 {
		return nil, ErrInvalidInput
	}
	// Mock summary — deterministic numbers tied to TC tail so tests can
	// assert specific counts without seeding the table.
	tail := int(tc[len(tc)-1] - '0')
	return &PatientSummary{
		TC:               tc,
		EncountersCount:  tail * 3,
		ObservationCount: tail * 7,
		LastEncounterAt:  time.Now().Add(-time.Duration(tail) * 24 * time.Hour).UTC().Format(time.RFC3339),
		Raw:              map[string]any{"source": "mock"},
	}, nil
}

// maskTCLast4 keeps only the last 4 digits, mirroring util.MaskTC. We
// duplicate the rule locally to keep this package self-contained.
func maskTCLast4(tc string) string {
	if len(tc) < 4 {
		return "****"
	}
	return "*******" + tc[len(tc)-4:]
}
