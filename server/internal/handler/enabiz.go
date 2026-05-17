package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/pkg/db/repo"
)

// e-Nabız outbox visibility — status panel + ad-hoc enqueue.
//
// Operational reality: most outbox writes happen automatically when a
// clinical event lands (visit complete → Encounter, prescription sign
// → MedicationRequest, lab result publish → Observation). Those wires
// are intentionally NOT in place yet — sertifikasyon süreci ve policy
// kararları beklenir. For now this endpoint set lets ops manually
// queue a resource and watch the worker drain it.

type enabizMessagePayload struct {
	ID            string          `json:"id"`
	PatientID     string          `json:"patient_id"`
	PatientTCLast string          `json:"patient_tc_last4"`
	Kind          string          `json:"kind"`
	Status        string          `json:"status"`
	RetryCount    int             `json:"retry_count"`
	NextRetryAt   time.Time       `json:"next_retry_at"`
	LastError     *string         `json:"last_error,omitempty"`
	ReceiptID     *string         `json:"receipt_id,omitempty"`
	SourceTable   *string         `json:"source_table,omitempty"`
	SourceID      *string         `json:"source_id,omitempty"`
	QueuedAt      time.Time       `json:"queued_at"`
	SentAt        *time.Time      `json:"sent_at,omitempty"`
	Resource      json.RawMessage `json:"resource"`
	LastResponse  json.RawMessage `json:"last_response,omitempty"`
}

func toEnabizPayload(m *repo.EnabizMessage) enabizMessagePayload {
	p := enabizMessagePayload{
		ID:            m.ID.String(),
		PatientID:     m.PatientID.String(),
		PatientTCLast: maskTC(m.PatientTC),
		Kind:          m.Kind,
		Status:        m.Status,
		RetryCount:    m.RetryCount,
		NextRetryAt:   m.NextRetryAt,
		LastError:     m.LastError,
		ReceiptID:     m.ReceiptID,
		SourceTable:   m.SourceTable,
		QueuedAt:      m.QueuedAt,
		SentAt:        m.SentAt,
		Resource:      m.ResourceJSON,
		LastResponse:  m.LastResponse,
	}
	if m.SourceID != nil {
		s := m.SourceID.String()
		p.SourceID = &s
	}
	if len(p.LastResponse) == 0 {
		p.LastResponse = nil
	}
	return p
}

// maskTC returns "*******XXXX" — KVKK rule, last 4 only at the API surface.
func maskTC(tc string) string {
	if len(tc) < 4 {
		return "****"
	}
	return "*******" + tc[len(tc)-4:]
}

func (h *Handler) listEnabizMessages(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	limit := 100
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			limit = n
		}
	}
	items, err := h.deps.Enabiz.ListForBranch(r.Context(), repo.EnabizListFilter{
		BranchID: branchID,
		Status:   r.URL.Query().Get("status"),
		Limit:    limit,
	})
	if err != nil {
		h.deps.Log.Error("enabiz list failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]enabizMessagePayload, 0, len(items))
	for i := range items {
		out = append(out, toEnabizPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type enqueueEnabizReq struct {
	PatientID   string         `json:"patient_id"`
	Kind        string         `json:"kind"`
	Resource    map[string]any `json:"resource"`
	SourceTable string         `json:"source_table"`
	SourceID    string         `json:"source_id"`
}

var validEnabizKinds = map[string]bool{
	"Encounter": true, "Observation": true, "Condition": true,
	"MedicationRequest": true, "DiagnosticReport": true,
}

func (h *Handler) enqueueEnabiz(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req enqueueEnabizReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_patient", "patient_id geçersiz")
		return
	}
	if !validEnabizKinds[req.Kind] {
		writeError(w, http.StatusBadRequest, "bad_kind", "geçersiz FHIR resource kind")
		return
	}

	// Patient TC needed to address Bakanlık. We trust the patient row.
	var tc string
	if err := h.deps.Pool.QueryRow(r.Context(),
		`SELECT COALESCE(identifier_value, '')
		 FROM patient
		 WHERE id = $1 AND organization_id = $2`, patientID, orgID).Scan(&tc); err != nil {
		writeError(w, http.StatusNotFound, "patient_not_found", "hasta bulunamadı")
		return
	}
	if tc == "" {
		writeError(w, http.StatusConflict, "no_tc",
			"hastanın TC kimlik no'su yok; e-Nabız'a gönderilemez")
		return
	}

	in := repo.EnqueueEnabizInput{
		OrganizationID: orgID, BranchID: branchID,
		PatientID: patientID, PatientTC: tc,
		Kind:     req.Kind,
		Resource: req.Resource,
		SourceTable: req.SourceTable,
	}
	if req.SourceID != "" {
		if id, err := uuid.Parse(req.SourceID); err == nil {
			in.SourceID = &id
		}
	}
	id, err := h.deps.Enabiz.Enqueue(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("enabiz enqueue failed", "err", err)
		writeError(w, http.StatusInternalServerError, "queue_failed", err.Error())
		return
	}
	h.auditAccess(r.Context(), r, "enabiz.enqueue", "enabiz_message", id.String(), map[string]any{
		"kind":       req.Kind,
		"patient_id": patientID.String(),
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"id": id.String()})
}
