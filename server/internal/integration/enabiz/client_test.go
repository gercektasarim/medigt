package enabiz

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestMockClient_SubmitResource_Success(t *testing.T) {
	c := NewMockClient()
	id := uuid.New()
	res, err := c.SubmitResource(context.Background(), SubmitInput{
		MessageID:    id,
		PatientTC:    "10000000146", // ends in 6 → success path
		Kind:         KindEncounter,
		ResourceJSON: []byte(`{"resourceType":"Encounter"}`),
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got %v", res)
	}
	if !strings.HasPrefix(res.ReceiptID, "ENB-") {
		t.Fatalf("receipt id must start with ENB-, got %q", res.ReceiptID)
	}

	// Determinism — same MessageID ⇒ same receipt id. Tests + retries
	// rely on this to be idempotent on our side.
	res2, _ := c.SubmitResource(context.Background(), SubmitInput{
		MessageID:    id,
		PatientTC:    "10000000146",
		Kind:         KindEncounter,
		ResourceJSON: []byte(`{"resourceType":"Encounter"}`),
	})
	if res.ReceiptID != res2.ReceiptID {
		t.Fatalf("non-deterministic receipt id: %q vs %q", res.ReceiptID, res2.ReceiptID)
	}
}

func TestMockClient_SubmitResource_RejectsTCEndingInZero(t *testing.T) {
	c := NewMockClient()
	res, err := c.SubmitResource(context.Background(), SubmitInput{
		MessageID:    uuid.New(),
		PatientTC:    "12345678900",
		Kind:         KindObservation,
		ResourceJSON: []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if res.Success {
		t.Fatal("expected rejection for TC ending in 0")
	}
	if res.ResponseCode != "BAKANLIK_REJECTED_SIM" {
		t.Fatalf("unexpected response code %q", res.ResponseCode)
	}
}

func TestMockClient_SubmitResource_InvalidInput(t *testing.T) {
	c := NewMockClient()
	cases := []struct {
		name string
		in   SubmitInput
	}{
		{"empty TC", SubmitInput{MessageID: uuid.New(), Kind: KindEncounter, ResourceJSON: []byte(`{}`)}},
		{"empty body", SubmitInput{MessageID: uuid.New(), PatientTC: "10000000146", Kind: KindEncounter}},
		{"empty kind", SubmitInput{MessageID: uuid.New(), PatientTC: "10000000146", ResourceJSON: []byte(`{}`)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := c.SubmitResource(context.Background(), tc.in)
			if err != ErrInvalidInput {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestMockClient_QueryPatient_ReturnsDeterministicCounts(t *testing.T) {
	c := NewMockClient()
	sum, err := c.QueryPatient(context.Background(), "10000000146")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// Tail = 6 → encounters = 18, observations = 42.
	if sum.EncountersCount != 18 || sum.ObservationCount != 42 {
		// Reverify via JSON round-trip in case ints leak.
		b, _ := json.Marshal(sum)
		t.Fatalf("unexpected mock counts: %s", b)
	}
}

func TestMockClient_QueryPatient_RejectsBadTCLength(t *testing.T) {
	c := NewMockClient()
	_, err := c.QueryPatient(context.Background(), "12345")
	if err != ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}
