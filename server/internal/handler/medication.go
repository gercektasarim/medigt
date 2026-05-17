package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/medigt/medigt/server/pkg/db/repo"
)

type medicationPayload struct {
	ID                string   `json:"id"`
	AtcCode           *string  `json:"atc_code,omitempty"`
	Barcode           *string  `json:"barcode,omitempty"`
	Name              string   `json:"name"`
	GenericName       *string  `json:"generic_name,omitempty"`
	Form              string   `json:"form"`
	Strength          *string  `json:"strength,omitempty"`
	PackSize          *string  `json:"pack_size,omitempty"`
	PrescriptionClass string   `json:"prescription_class"`
	RequiresColdChain bool     `json:"requires_cold_chain"`
	IsControlled      bool     `json:"is_controlled"`
	Manufacturer      *string  `json:"manufacturer,omitempty"`
	ListPrice         *float64 `json:"list_price,omitempty"`
	Notes             *string  `json:"notes,omitempty"`
	IsActive          bool     `json:"is_active"`
}

func toMedicationPayload(m *repo.Medication) medicationPayload {
	return medicationPayload{
		ID: m.ID.String(), AtcCode: m.AtcCode, Barcode: m.Barcode,
		Name: m.Name, GenericName: m.GenericName, Form: m.Form,
		Strength: m.Strength, PackSize: m.PackSize,
		PrescriptionClass: m.PrescriptionClass,
		RequiresColdChain: m.RequiresColdChain, IsControlled: m.IsControlled,
		Manufacturer: m.Manufacturer, ListPrice: m.ListPrice, Notes: m.Notes,
		IsActive: m.IsActive,
	}
}

var validMedicationForms = map[string]bool{
	"tablet": true, "capsule": true, "syrup": true, "injection": true,
	"ampoule": true, "cream": true, "ointment": true, "drops": true,
	"spray": true, "patch": true, "suppository": true, "solution": true,
	"powder": true, "other": true,
}

var validPrescriptionClasses = map[string]bool{
	"otc": true, "normal": true, "green": true,
	"red": true, "orange": true, "purple": true,
}

func (h *Handler) listMedications(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	f := repo.ListMedicationFilter{
		Search:            strings.TrimSpace(r.URL.Query().Get("q")),
		Form:              r.URL.Query().Get("form"),
		PrescriptionClass: r.URL.Query().Get("class"),
		ActiveOnly:        r.URL.Query().Get("active") != "false",
	}
	items, err := h.deps.Medications.List(r.Context(), orgID, f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]medicationPayload, 0, len(items))
	for i := range items {
		out = append(out, toMedicationPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) getMedication(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	m, err := h.deps.Medications.GetByID(r.Context(), orgID, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "ilaç bulunamadı")
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	writeJSON(w, http.StatusOK, toMedicationPayload(m))
}

type createMedicationReq struct {
	AtcCode           string   `json:"atc_code"`
	Barcode           string   `json:"barcode"`
	Name              string   `json:"name"`
	GenericName       string   `json:"generic_name"`
	Form              string   `json:"form"`
	Strength          string   `json:"strength"`
	PackSize          string   `json:"pack_size"`
	PrescriptionClass string   `json:"prescription_class"`
	RequiresColdChain bool     `json:"requires_cold_chain"`
	IsControlled      bool     `json:"is_controlled"`
	Manufacturer      string   `json:"manufacturer"`
	ListPrice         *float64 `json:"list_price"`
	Notes             string   `json:"notes"`
}

func (h *Handler) createMedication(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	var req createMedicationReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "name zorunlu")
		return
	}
	form := req.Form
	if form == "" {
		form = "tablet"
	}
	if !validMedicationForms[form] {
		writeError(w, http.StatusBadRequest, "bad_form", "geçersiz form")
		return
	}
	pclass := req.PrescriptionClass
	if pclass == "" {
		pclass = "normal"
	}
	if !validPrescriptionClasses[pclass] {
		writeError(w, http.StatusBadRequest, "bad_class", "geçersiz reçete sınıfı")
		return
	}

	in := repo.CreateMedicationInput{
		OrganizationID:    orgID,
		AtcCode:           emptyToNil(&req.AtcCode),
		Barcode:           emptyToNil(&req.Barcode),
		Name:              name,
		GenericName:       emptyToNil(&req.GenericName),
		Form:              form,
		Strength:          emptyToNil(&req.Strength),
		PackSize:          emptyToNil(&req.PackSize),
		PrescriptionClass: pclass,
		RequiresColdChain: req.RequiresColdChain,
		IsControlled:      req.IsControlled,
		Manufacturer:      emptyToNil(&req.Manufacturer),
		ListPrice:         req.ListPrice,
		Notes:             emptyToNil(&req.Notes),
	}

	m, err := h.deps.Medications.Create(r.Context(), in)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "name_taken", "bu ad zaten kayıtlı")
			return
		}
		h.deps.Log.Error("create medication failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, toMedicationPayload(m))
}
