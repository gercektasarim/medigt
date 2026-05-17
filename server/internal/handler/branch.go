package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/medigt/medigt/server/internal/service"
)

type createBranchReq struct {
	Slug            string `json:"slug"`
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	SGKFacilityCode string `json:"sgk_facility_code"`
}

func (h *Handler) listBranches(w http.ResponseWriter, r *http.Request) {
	uid := mustUserID(w, r)
	if uid.String() == "00000000-0000-0000-0000-000000000000" {
		return
	}

	orgSlug := chi.URLParam(r, "orgSlug")
	org, err := h.deps.Orgs.GetBySlug(r.Context(), orgSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "org_not_found", "hastane bulunamadı")
		return
	}

	ok, err := h.deps.Memberships.HasMembership(r.Context(), uid, org.ID)
	if err != nil || !ok {
		writeError(w, http.StatusForbidden, "not_member", "bu hastaneye erişiminiz yok")
		return
	}

	branches, err := h.deps.Branches.ListAccessibleForUser(r.Context(), uid, org.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]branchPayload, 0, len(branches))
	for i := range branches {
		out = append(out, toBranchPayload(&branches[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createBranch(w http.ResponseWriter, r *http.Request) {
	uid := mustUserID(w, r)
	if uid.String() == "00000000-0000-0000-0000-000000000000" {
		return
	}

	orgSlug := chi.URLParam(r, "orgSlug")
	org, err := h.deps.Orgs.GetBySlug(r.Context(), orgSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "org_not_found", "hastane bulunamadı")
		return
	}

	ok, err := h.deps.Memberships.HasMembership(r.Context(), uid, org.ID)
	if err != nil || !ok {
		writeError(w, http.StatusForbidden, "not_member", "bu hastaneye erişiminiz yok")
		return
	}

	var req createBranchReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Slug) == "" || strings.TrimSpace(req.Kind) == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "slug, name ve kind zorunlu")
		return
	}

	branch, err := h.deps.Tenant.CreateBranch(r.Context(), service.CreateBranchInput{
		OrganizationID:  org.ID,
		Slug:            strings.ToLower(req.Slug),
		Name:            req.Name,
		Kind:            req.Kind,
		SGKFacilityCode: req.SGKFacilityCode,
	})
	switch {
	case errors.Is(err, service.ErrInvalidSlug):
		writeError(w, http.StatusBadRequest, "invalid_slug", "geçersiz şube slug")
		return
	case err != nil:
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "slug_taken", "bu şube slug'ı zaten kullanımda")
			return
		}
		h.deps.Log.Error("create branch failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, toBranchPayload(branch))
}
