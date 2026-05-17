package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/middleware"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

type specializationPayload struct {
	ID             string    `json:"id"`
	OrganizationID *string   `json:"organization_id,omitempty"`
	Code           string    `json:"code"`
	Name           string    `json:"name"`
	ParentID       *string   `json:"parent_id,omitempty"`
	IsSystem       bool      `json:"is_system"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func toSpecPayload(s *repo.Specialization) specializationPayload {
	out := specializationPayload{
		ID: s.ID.String(), Code: s.Code, Name: s.Name,
		IsSystem:  s.IsSystem,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
	if s.OrganizationID != nil {
		v := s.OrganizationID.String()
		out.OrganizationID = &v
	}
	if s.ParentID != nil {
		v := s.ParentID.String()
		out.ParentID = &v
	}
	return out
}

func (h *Handler) listSpecializations(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	items, err := h.deps.Specializations.List(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]specializationPayload, 0, len(items))
	for i := range items {
		out = append(out, toSpecPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type createSpecReq struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id"`
}

func (h *Handler) createSpecialization(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	var req createSpecReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "code ve name zorunlu")
		return
	}
	var parentID *uuid.UUID
	if req.ParentID != nil && *req.ParentID != "" {
		id, err := uuid.Parse(*req.ParentID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_parent", "parent_id geçersiz")
			return
		}
		parentID = &id
	}
	spec, err := h.deps.Specializations.Create(r.Context(), repo.CreateSpecializationInput{
		OrganizationID: &orgID, Code: strings.ToUpper(strings.ReplaceAll(req.Code, " ", "_")),
		Name: req.Name, ParentID: parentID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "code_taken", "bu kod zaten kayıtlı")
			return
		}
		h.deps.Log.Error("create specialization failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, toSpecPayload(spec))
}

func (h *Handler) deleteSpecialization(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	if err := h.deps.Specializations.Delete(r.Context(), id, orgID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "branş bulunamadı veya sistem kaydı")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// mustOrgID extracts the resolved org UUID from the tenant middleware.
func mustOrgID(w http.ResponseWriter, r *http.Request) uuid.UUID {
	s := middleware.OrgIDFromContext(r.Context())
	if s == "" {
		writeError(w, http.StatusBadRequest, "missing_org", "X-Organization-ID gerekli")
		return uuid.Nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_org", "X-Organization-ID geçersiz")
		return uuid.Nil
	}
	return id
}
