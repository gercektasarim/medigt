package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/service"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

type patientPayload struct {
	ID                string     `json:"id"`
	OrganizationID    string     `json:"organization_id"`
	MRN               string     `json:"mrn"`
	FirstName         string     `json:"first_name"`
	LastName          string     `json:"last_name"`
	BirthDate         *time.Time `json:"birth_date,omitempty"`
	Gender            string     `json:"gender"`
	BloodType         string     `json:"blood_type"`
	IdentifierKind    *string    `json:"identifier_kind,omitempty"`
	IdentifierValue   *string    `json:"identifier_value,omitempty"`
	IdentifierMasked  *string    `json:"identifier_masked,omitempty"`
	MernisVerifiedAt  *time.Time `json:"mernis_verified_at,omitempty"`
	Phone             *string    `json:"phone,omitempty"`
	Email             *string    `json:"email,omitempty"`
	Address           *string    `json:"address,omitempty"`
	NextOfKinName     *string    `json:"next_of_kin_name,omitempty"`
	NextOfKinPhone    *string    `json:"next_of_kin_phone,omitempty"`
	Notes             *string    `json:"notes,omitempty"`
	IsDeceased        bool       `json:"is_deceased"`
	DeceasedAt        *time.Time `json:"deceased_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func toPatientPayload(p *repo.Patient) patientPayload {
	out := patientPayload{
		ID: p.ID.String(), OrganizationID: p.OrganizationID.String(),
		MRN: p.MRN, FirstName: p.FirstName, LastName: p.LastName,
		BirthDate: p.BirthDate, Gender: p.Gender, BloodType: p.BloodType,
		IdentifierKind: p.IdentifierKind, IdentifierValue: p.IdentifierValue,
		MernisVerifiedAt: p.MernisVerifiedAt,
		Phone: p.Phone, Email: p.Email, Address: p.Address,
		NextOfKinName: p.NextOfKinName, NextOfKinPhone: p.NextOfKinPhone,
		Notes: p.Notes,
		IsDeceased: p.IsDeceased, DeceasedAt: p.DeceasedAt,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
	if p.IdentifierValue != nil && len(*p.IdentifierValue) >= 4 {
		m := strings.Repeat("*", len(*p.IdentifierValue)-4) + (*p.IdentifierValue)[len(*p.IdentifierValue)-4:]
		out.IdentifierMasked = &m
	}
	return out
}

func (h *Handler) listPatients(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	q := r.URL.Query().Get("q")
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			limit = v
		}
	}
	items, err := h.deps.Patients.List(r.Context(), orgID, repo.ListPatientFilter{
		Search: q, Limit: limit,
	})
	if err != nil {
		h.deps.Log.Error("list patients failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]patientPayload, 0, len(items))
	for i := range items {
		out = append(out, toPatientPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) getPatient(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	p, err := h.deps.Patients.GetByID(r.Context(), orgID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "hasta bulunamadı")
		return
	}
	// KVKK — record patient-record access. Only the ID is stored;
	// names/TC stay out of audit details.
	h.auditAccess(r.Context(), r, "patient.read", "patient", id.String(), nil)
	writeJSON(w, http.StatusOK, toPatientPayload(p))
}

type createPatientReq struct {
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	BirthDate       string `json:"birth_date"` // YYYY-MM-DD
	Gender          string `json:"gender"`
	BloodType       string `json:"blood_type"`
	IdentifierKind  string `json:"identifier_kind"`
	IdentifierValue string `json:"identifier_value"`
	Phone           string `json:"phone"`
	Email           string `json:"email"`
	Address         string `json:"address"`
	NextOfKinName   string `json:"next_of_kin_name"`
	NextOfKinPhone  string `json:"next_of_kin_phone"`
	Notes           string `json:"notes"`
}

func (h *Handler) createPatient(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	var req createPatientReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}

	in := service.CreatePatientInput{
		OrganizationID:  orgID,
		FirstName:       req.FirstName,
		LastName:        req.LastName,
		Gender:          req.Gender,
		BloodType:       req.BloodType,
		IdentifierKind:  req.IdentifierKind,
		IdentifierValue: req.IdentifierValue,
		Phone:           req.Phone,
		Email:           req.Email,
		Address:         req.Address,
		NextOfKinName:   req.NextOfKinName,
		NextOfKinPhone:  req.NextOfKinPhone,
		Notes:           req.Notes,
	}
	if req.BirthDate != "" {
		t, err := time.Parse("2006-01-02", req.BirthDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_date", "birth_date YYYY-MM-DD olmalı")
			return
		}
		in.BirthDate = &t
	}

	p, err := h.deps.PatientSvc.Create(r.Context(), in)
	switch {
	case errors.Is(err, service.ErrInvalidTC):
		writeError(w, http.StatusBadRequest, "invalid_tc", err.Error())
		return
	case errors.Is(err, service.ErrPatientExists):
		writeError(w, http.StatusConflict, "patient_exists", err.Error())
		return
	case errors.Is(err, service.ErrMissingPatientName):
		writeError(w, http.StatusBadRequest, "missing_fields", err.Error())
		return
	case err != nil:
		h.deps.Log.Error("create patient failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	// KVKK — yeni hasta kaydı kişisel veri toplama. Details'a kimlik
	// kırma yapmıyoruz; sadece ID + identifier_kind iz bırak.
	h.auditAccess(r.Context(), r, "patient.create", "patient", p.ID.String(), map[string]any{
		"identifier_kind": req.IdentifierKind,
	})
	writeJSON(w, http.StatusCreated, toPatientPayload(p))
}

// Lightweight TC validator endpoint — UI can call this on blur for instant
// feedback before submitting the create form.
type tcValidateReq struct {
	Tc string `json:"tc"`
}
type tcValidateResp struct {
	Valid bool `json:"valid"`
}

func (h *Handler) validateTC(w http.ResponseWriter, r *http.Request) {
	var req tcValidateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	writeJSON(w, http.StatusOK, tcValidateResp{Valid: validateTCFromHandler(req.Tc)})
}
