package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/middleware"
	"github.com/medigt/medigt/server/internal/service"
)

type createOrgReq struct {
	Slug          string             `json:"slug"`
	Name          string             `json:"name"`
	Kind          string             `json:"kind"`
	TaxID         string             `json:"tax_id"`
	SGKEmployerNo string             `json:"sgk_employer_no"`
	InitialBranch *createBranchInput `json:"initial_branch,omitempty"`
}

type createBranchInput struct {
	Slug            string `json:"slug"`
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	SGKFacilityCode string `json:"sgk_facility_code"`
}

type createOrgResp struct {
	Organization orgPayload     `json:"organization"`
	Branch       *branchPayload `json:"branch,omitempty"`
}

func (h *Handler) listOrganizations(w http.ResponseWriter, r *http.Request) {
	uid := mustUserID(w, r)
	if uid == uuid.Nil {
		return
	}
	orgs, err := h.deps.Orgs.ListForUser(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]orgPayload, 0, len(orgs))
	for i := range orgs {
		out = append(out, toOrgPayload(&orgs[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createOrganization(w http.ResponseWriter, r *http.Request) {
	uid := mustUserID(w, r)
	if uid == uuid.Nil {
		return
	}

	var req createOrgReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Slug) == "" || strings.TrimSpace(req.Kind) == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "slug, name ve kind zorunlu")
		return
	}

	in := service.CreateOrgInput{
		OwnerUserID:   uid,
		Slug:          strings.ToLower(req.Slug),
		Name:          req.Name,
		Kind:          req.Kind,
		TaxID:         req.TaxID,
		SGKEmployerNo: req.SGKEmployerNo,
	}
	if req.InitialBranch != nil {
		in.InitialBranch = &service.CreateBranchInput{
			Slug:            strings.ToLower(req.InitialBranch.Slug),
			Name:            req.InitialBranch.Name,
			Kind:            req.InitialBranch.Kind,
			SGKFacilityCode: req.InitialBranch.SGKFacilityCode,
		}
	}

	org, branch, err := h.deps.Tenant.CreateOrganization(r.Context(), in)
	switch {
	case errors.Is(err, service.ErrInvalidSlug):
		writeError(w, http.StatusBadRequest, "invalid_slug",
			"slug 2-40 karakter, küçük harf/rakam/tire olmalı; rezerve kelimeler kullanılamaz")
		return
	case err != nil:
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "slug_taken", "bu slug zaten kullanımda")
			return
		}
		h.deps.Log.Error("create organization failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}

	resp := createOrgResp{Organization: toOrgPayload(org)}
	if branch != nil {
		p := toBranchPayload(branch)
		resp.Branch = &p
	}
	writeJSON(w, http.StatusCreated, resp)
}

func mustUserID(w http.ResponseWriter, r *http.Request) uuid.UUID {
	s := middleware.UserIDFromContext(r.Context())
	if s == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "oturum açın")
		return uuid.Nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "geçersiz oturum")
		return uuid.Nil
	}
	return id
}

func isUniqueViolation(err error) bool {
	// Postgres SQLSTATE 23505 — unique_violation
	return err != nil && strings.Contains(err.Error(), "23505")
}
