package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/middleware"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

type diagnosisPayload struct {
	ID         string    `json:"id"`
	VisitID    string    `json:"visit_id"`
	Icd10Code  string    `json:"icd10_code"`
	Icd10Title string    `json:"icd10_title"`
	Kind       string    `json:"kind"`
	Notes      *string   `json:"notes,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func toDxPayload(d *repo.Diagnosis) diagnosisPayload {
	return diagnosisPayload{
		ID: d.ID.String(), VisitID: d.VisitID.String(),
		Icd10Code: d.Icd10Code, Icd10Title: d.Icd10Title,
		Kind: d.Kind, Notes: d.Notes, CreatedAt: d.CreatedAt,
	}
}

var validDxKinds = map[string]bool{
	"primary": true, "secondary": true, "provisional": true,
	"differential": true, "ruled_out": true,
}

func (h *Handler) listDiagnoses(w http.ResponseWriter, r *http.Request) {
	visitID, err := uuid.Parse(chi.URLParam(r, "visitId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_visit", "visitId geçersiz")
		return
	}
	items, err := h.deps.Diagnoses.ListForVisit(r.Context(), visitID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]diagnosisPayload, 0, len(items))
	for i := range items {
		out = append(out, toDxPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type addDxReq struct {
	Icd10Code  string  `json:"icd10_code"`
	Icd10Title string  `json:"icd10_title"`
	Kind       string  `json:"kind"`
	Notes      *string `json:"notes"`
}

func (h *Handler) addDiagnosis(w http.ResponseWriter, r *http.Request) {
	visitID, err := uuid.Parse(chi.URLParam(r, "visitId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_visit", "visitId geçersiz")
		return
	}
	var req addDxReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	code := strings.TrimSpace(req.Icd10Code)
	title := strings.TrimSpace(req.Icd10Title)
	if code == "" || title == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "icd10_code ve icd10_title zorunlu")
		return
	}
	if req.Kind == "" {
		req.Kind = "primary"
	}
	if !validDxKinds[req.Kind] {
		writeError(w, http.StatusBadRequest, "bad_kind", "geçersiz tanı türü")
		return
	}

	var byUser *uuid.UUID
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			byUser = &uid
		}
	}
	d, err := h.deps.Diagnoses.Add(r.Context(), repo.AddDiagnosisInput{
		VisitID:         visitID,
		Icd10Code:       code,
		Icd10Title:      title,
		Kind:            req.Kind,
		Notes:           req.Notes,
		CreatedByUserID: byUser,
	})
	if err != nil {
		h.deps.Log.Error("add diagnosis failed", "err", err)
		writeError(w, http.StatusInternalServerError, "add_failed", "ekleme başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, toDxPayload(d))
}

func (h *Handler) deleteDiagnosis(w http.ResponseWriter, r *http.Request) {
	visitID, err := uuid.Parse(chi.URLParam(r, "visitId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_visit", "visitId geçersiz")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	if err := h.deps.Diagnoses.Delete(r.Context(), visitID, id); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "tanı bulunamadı")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed", "silme başarısız")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
