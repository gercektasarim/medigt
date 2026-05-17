package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/pkg/db/repo"
)

type institutionPayload struct {
	ID               string     `json:"id"`
	OrganizationID   string     `json:"organization_id"`
	Code             string     `json:"code"`
	Name             string     `json:"name"`
	Kind             string     `json:"kind"`
	TaxID            *string    `json:"tax_id,omitempty"`
	Address          *string    `json:"address,omitempty"`
	Phone            *string    `json:"phone,omitempty"`
	Email            *string    `json:"email,omitempty"`
	ContractNo       *string    `json:"contract_no,omitempty"`
	ContractStartsAt *time.Time `json:"contract_starts_at,omitempty"`
	ContractEndsAt   *time.Time `json:"contract_ends_at,omitempty"`
	IsActive         bool       `json:"is_active"`
	Notes            *string    `json:"notes,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func toInstPayload(i *repo.ExternalInstitution) institutionPayload {
	return institutionPayload{
		ID:               i.ID.String(),
		OrganizationID:   i.OrganizationID.String(),
		Code:             i.Code,
		Name:             i.Name,
		Kind:             i.Kind,
		TaxID:            i.TaxID,
		Address:          i.Address,
		Phone:            i.Phone,
		Email:            i.Email,
		ContractNo:       i.ContractNo,
		ContractStartsAt: i.ContractStartsAt,
		ContractEndsAt:   i.ContractEndsAt,
		IsActive:         i.IsActive,
		Notes:            i.Notes,
		CreatedAt:        i.CreatedAt,
		UpdatedAt:        i.UpdatedAt,
	}
}

func (h *Handler) listInstitutions(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	activeOnly := r.URL.Query().Get("active") == "true"
	items, err := h.deps.Institutions.List(r.Context(), orgID, activeOnly)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]institutionPayload, 0, len(items))
	for i := range items {
		out = append(out, toInstPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type createInstReq struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	TaxID      *string `json:"tax_id"`
	Phone      *string `json:"phone"`
	Email      *string `json:"email"`
	Address    *string `json:"address"`
	ContractNo *string `json:"contract_no"`
	Notes      *string `json:"notes"`
}

func (h *Handler) createInstitution(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	var req createInstReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	code := strings.TrimSpace(req.Code)
	name := strings.TrimSpace(req.Name)
	kind := strings.TrimSpace(req.Kind)
	if code == "" || name == "" || kind == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "code, name ve kind zorunlu")
		return
	}
	inst, err := h.deps.Institutions.Create(r.Context(), repo.CreateInstitutionInput{
		OrganizationID: orgID,
		Code:           strings.ToUpper(strings.ReplaceAll(code, " ", "_")),
		Name:           name,
		Kind:           kind,
		TaxID:          emptyToNil(req.TaxID),
		Phone:          emptyToNil(req.Phone),
		Email:          emptyToNil(req.Email),
		Address:        emptyToNil(req.Address),
		ContractNo:     emptyToNil(req.ContractNo),
		Notes:          emptyToNil(req.Notes),
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "code_taken", "bu kod zaten kayıtlı")
			return
		}
		h.deps.Log.Error("create institution failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, toInstPayload(inst))
}
