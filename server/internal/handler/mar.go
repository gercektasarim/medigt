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

// Medication Administration Record (MAR) handlers.
//
// Doktor → `POST /api/admissions/:id/medication-orders` ile reçete açar.
// Hemşire → `GET /api/admissions/:id/medication-orders` ile listeyi alır,
// `POST /api/medication-orders/:id/administrations` ile her doz için kayıt
// atar. 5 doğru kontrolü (doğru hasta, ilaç, doz, yol, zaman) verme
// anında işaretlenir; barkod taranan değerler iz amacıyla saklanır.

type medicationOrderPayload struct {
	ID               string     `json:"id"`
	OrderNo          string     `json:"order_no"`
	AdmissionID      string     `json:"admission_id"`
	PatientID        string     `json:"patient_id"`
	MedicationID     string     `json:"medication_id"`
	MedicationName   string     `json:"medication_name,omitempty"`
	MedicationCode   *string    `json:"medication_code,omitempty"`
	OrderingDoctorID *string    `json:"ordering_doctor_id,omitempty"`
	DoctorFirstName  *string    `json:"doctor_first_name,omitempty"`
	DoctorLastName   *string    `json:"doctor_last_name,omitempty"`
	DoseAmount       float64    `json:"dose_amount"`
	DoseUnit         string     `json:"dose_unit"`
	Route            string     `json:"route"`
	Frequency        string     `json:"frequency"`
	ScheduledTimes   []string   `json:"scheduled_times"`
	IsPRN            bool       `json:"is_prn"`
	PRNReason        *string    `json:"prn_reason,omitempty"`
	StartsAt         time.Time  `json:"starts_at"`
	EndsAt           *time.Time `json:"ends_at,omitempty"`
	Instructions     *string    `json:"instructions,omitempty"`
	Status           string     `json:"status"`
	CreatedAt        time.Time  `json:"created_at"`
}

func toMedicationOrderPayload(o *repo.MedicationOrder) medicationOrderPayload {
	p := medicationOrderPayload{
		ID: o.ID.String(), OrderNo: o.OrderNo,
		AdmissionID:    o.AdmissionID.String(),
		PatientID:      o.PatientID.String(),
		MedicationID:   o.MedicationID.String(),
		MedicationName: o.MedicationName,
		MedicationCode: o.MedicationCode,
		DoseAmount:     o.DoseAmount, DoseUnit: o.DoseUnit,
		Route: o.Route, Frequency: o.Frequency,
		ScheduledTimes: o.ScheduledTimes,
		IsPRN:          o.IsPRN, PRNReason: o.PRNReason,
		StartsAt:     o.StartsAt, EndsAt: o.EndsAt,
		Instructions: o.Instructions,
		Status:       o.Status, CreatedAt: o.CreatedAt,
		DoctorFirstName: o.DoctorFirstName,
		DoctorLastName:  o.DoctorLastName,
	}
	if o.OrderingDoctorID != nil {
		s := o.OrderingDoctorID.String()
		p.OrderingDoctorID = &s
	}
	if p.ScheduledTimes == nil {
		p.ScheduledTimes = []string{}
	}
	return p
}

type medicationAdministrationPayload struct {
	ID                       string     `json:"id"`
	MedicationOrderID        string     `json:"medication_order_id"`
	AdmissionID              string     `json:"admission_id"`
	PatientID                string     `json:"patient_id"`
	ScheduledAt              *time.Time `json:"scheduled_at,omitempty"`
	AdministeredAt           time.Time  `json:"administered_at"`
	Status                   string     `json:"status"`
	FiveRightsChecked        bool       `json:"five_rights_checked"`
	PatientBarcodeScanned    *string    `json:"patient_barcode_scanned,omitempty"`
	MedicationBarcodeScanned *string    `json:"medication_barcode_scanned,omitempty"`
	DoseAmount               *float64   `json:"dose_amount,omitempty"`
	DoseUnit                 *string    `json:"dose_unit,omitempty"`
	Route                    *string    `json:"route,omitempty"`
	Notes                    *string    `json:"notes,omitempty"`
	PerformedByUserID        *string    `json:"performed_by_user_id,omitempty"`
	WitnessedByUserID        *string    `json:"witnessed_by_user_id,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
}

func toAdminPayload(a *repo.MedicationAdministration) medicationAdministrationPayload {
	p := medicationAdministrationPayload{
		ID:                       a.ID.String(),
		MedicationOrderID:        a.MedicationOrderID.String(),
		AdmissionID:              a.AdmissionID.String(),
		PatientID:                a.PatientID.String(),
		ScheduledAt:              a.ScheduledAt,
		AdministeredAt:           a.AdministeredAt,
		Status:                   a.Status,
		FiveRightsChecked:        a.FiveRightsChecked,
		PatientBarcodeScanned:    a.PatientBarcodeScanned,
		MedicationBarcodeScanned: a.MedicationBarcodeScanned,
		DoseAmount:               a.DoseAmount,
		DoseUnit:                 a.DoseUnit,
		Route:                    a.Route,
		Notes:                    a.Notes,
		CreatedAt:                a.CreatedAt,
	}
	if a.PerformedByUserID != nil {
		s := a.PerformedByUserID.String()
		p.PerformedByUserID = &s
	}
	if a.WitnessedByUserID != nil {
		s := a.WitnessedByUserID.String()
		p.WitnessedByUserID = &s
	}
	return p
}

var validRoutes = map[string]bool{
	"oral": true, "iv": true, "im": true, "sc": true, "topical": true,
	"inhalation": true, "rectal": true, "sublingual": true, "intranasal": true,
	"ophthalmic": true, "otic": true, "other": true,
}

var validAdminStatuses = map[string]bool{
	"given": true, "refused": true, "withheld": true, "missed": true, "wrong_time": true,
}

var validOrderStatuses = map[string]bool{
	"active": true, "on_hold": true, "completed": true, "cancelled": true, "expired": true,
}

// ---------- Order list/create/update ----------

type createMedicationOrderReq struct {
	MedicationID     string   `json:"medication_id"`
	OrderingDoctorID string   `json:"ordering_doctor_id"`
	DoseAmount       float64  `json:"dose_amount"`
	DoseUnit         string   `json:"dose_unit"`
	Route            string   `json:"route"`
	Frequency        string   `json:"frequency"`
	ScheduledTimes   []string `json:"scheduled_times"`
	IsPRN            bool     `json:"is_prn"`
	PRNReason        string   `json:"prn_reason"`
	StartsAt         string   `json:"starts_at"`
	EndsAt           string   `json:"ends_at"`
	Instructions     string   `json:"instructions"`
}

func (h *Handler) listMedicationOrders(w http.ResponseWriter, r *http.Request) {
	admID, err := uuid.Parse(chi.URLParam(r, "admissionId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "admissionId geçersiz")
		return
	}
	items, err := h.deps.MAR.ListForAdmission(r.Context(), admID)
	if err != nil {
		h.deps.Log.Error("list mar orders failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]medicationOrderPayload, 0, len(items))
	for i := range items {
		out = append(out, toMedicationOrderPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createMedicationOrder(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	admID, err := uuid.Parse(chi.URLParam(r, "admissionId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "admissionId geçersiz")
		return
	}
	var req createMedicationOrderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	medID, err := uuid.Parse(req.MedicationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_medication", "medication_id zorunlu")
		return
	}
	if req.DoseAmount <= 0 {
		writeError(w, http.StatusBadRequest, "bad_dose", "dose_amount pozitif olmalı")
		return
	}
	if strings.TrimSpace(req.DoseUnit) == "" {
		writeError(w, http.StatusBadRequest, "bad_unit", "dose_unit zorunlu")
		return
	}
	if !validRoutes[req.Route] {
		writeError(w, http.StatusBadRequest, "bad_route", "geçersiz uygulama yolu")
		return
	}
	if strings.TrimSpace(req.Frequency) == "" {
		writeError(w, http.StatusBadRequest, "bad_freq", "frequency zorunlu")
		return
	}

	// Look up patient_id from admission so the caller cannot mismatch.
	var patientID uuid.UUID
	if err := h.deps.Pool.QueryRow(r.Context(),
		`SELECT patient_id FROM admission WHERE id = $1 AND branch_id = $2`,
		admID, branchID).Scan(&patientID); err != nil {
		writeError(w, http.StatusNotFound, "admission_not_found", "yatış bulunamadı")
		return
	}

	in := repo.CreateMedicationOrderInput{
		OrganizationID: orgID, BranchID: branchID,
		AdmissionID: admID, PatientID: patientID,
		MedicationID:   medID,
		DoseAmount:     req.DoseAmount, DoseUnit: req.DoseUnit,
		Route: req.Route, Frequency: req.Frequency,
		ScheduledTimes: req.ScheduledTimes,
		IsPRN:          req.IsPRN, PRNReason: emptyToNil(&req.PRNReason),
		Instructions: emptyToNil(&req.Instructions),
	}
	if req.OrderingDoctorID != "" {
		if id, err := uuid.Parse(req.OrderingDoctorID); err == nil {
			in.OrderingDoctorID = &id
		}
	}
	in.StartsAt = time.Now()
	if req.StartsAt != "" {
		if t, err := time.Parse(time.RFC3339, req.StartsAt); err == nil {
			in.StartsAt = t
		}
	}
	if req.EndsAt != "" {
		if t, err := time.Parse(time.RFC3339, req.EndsAt); err == nil {
			in.EndsAt = &t
		}
	}

	o, err := h.deps.MAR.CreateOrder(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("create mar order failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	h.auditAccess(r.Context(), r, "mar.order.create", "medication_order", o.ID.String(), map[string]any{
		"admission_id":  admID.String(),
		"medication_id": medID.String(),
		"dose":          req.DoseAmount,
		"unit":          req.DoseUnit,
		"route":         req.Route,
		"frequency":     req.Frequency,
		"is_prn":        req.IsPRN,
	})
	writeJSON(w, http.StatusCreated, toMedicationOrderPayload(o))
}

type updateOrderStatusReq struct {
	Status string `json:"status"`
}

func (h *Handler) updateMedicationOrderStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req updateOrderStatusReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if !validOrderStatuses[req.Status] {
		writeError(w, http.StatusBadRequest, "bad_status", "geçersiz durum")
		return
	}
	if err := h.deps.MAR.UpdateOrderStatus(r.Context(), id, req.Status); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "kayıt bulunamadı")
		return
	}
	h.auditAccess(r.Context(), r, "mar.order.status_change", "medication_order", id.String(), map[string]any{
		"new_status": req.Status,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- Administration record ----------

type recordAdministrationReq struct {
	ScheduledAt              string  `json:"scheduled_at"`
	Status                   string  `json:"status"`
	FiveRightsChecked        bool    `json:"five_rights_checked"`
	PatientBarcodeScanned    string  `json:"patient_barcode_scanned"`
	MedicationBarcodeScanned string  `json:"medication_barcode_scanned"`
	DoseAmount               float64 `json:"dose_amount"`
	DoseUnit                 string  `json:"dose_unit"`
	Route                    string  `json:"route"`
	Notes                    string  `json:"notes"`
	WitnessedByUserID        string  `json:"witnessed_by_user_id"`
}

// recordAdministration creates a single dose record for a medication
// order. status='given' must accompany a confirmed 5-rights check;
// barcode fields are recommended but optional for sites that haven't
// rolled out scan-at-bedside yet.
func (h *Handler) recordAdministration(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req recordAdministrationReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	status := req.Status
	if status == "" {
		status = "given"
	}
	if !validAdminStatuses[status] {
		writeError(w, http.StatusBadRequest, "bad_status", "geçersiz durum")
		return
	}
	if status == "given" && !req.FiveRightsChecked {
		writeError(w, http.StatusBadRequest, "five_rights_required",
			"5 doğru kontrolü işaretlenmeden ilaç verildi olarak kaydedilemez")
		return
	}
	if req.Route != "" && !validRoutes[req.Route] {
		writeError(w, http.StatusBadRequest, "bad_route", "geçersiz uygulama yolu")
		return
	}

	in := repo.RecordAdministrationInput{
		OrganizationID:    orgID,
		BranchID:          branchID,
		MedicationOrderID: orderID,
		AdministeredAt:    time.Now(),
		Status:            status,
		FiveRightsChecked: req.FiveRightsChecked,
		Notes:             emptyToNil(&req.Notes),
	}
	if req.ScheduledAt != "" {
		if t, err := time.Parse(time.RFC3339, req.ScheduledAt); err == nil {
			in.ScheduledAt = &t
		}
	}
	if s := strings.TrimSpace(req.PatientBarcodeScanned); s != "" {
		in.PatientBarcodeScanned = &s
	}
	if s := strings.TrimSpace(req.MedicationBarcodeScanned); s != "" {
		in.MedicationBarcodeScanned = &s
	}
	if req.DoseAmount > 0 {
		in.DoseAmount = &req.DoseAmount
	}
	if s := strings.TrimSpace(req.DoseUnit); s != "" {
		in.DoseUnit = &s
	}
	if s := strings.TrimSpace(req.Route); s != "" {
		in.Route = &s
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.PerformedByUserID = &uid
		}
	}
	if s := strings.TrimSpace(req.WitnessedByUserID); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.WitnessedByUserID = &uid
		}
	}

	a, err := h.deps.MAR.RecordAdministration(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("record administration failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	h.auditAccess(r.Context(), r, "mar.administer", "medication_administration", a.ID.String(), map[string]any{
		"medication_order_id":  orderID.String(),
		"status":               status,
		"five_rights_checked":  req.FiveRightsChecked,
		"barcode_patient":      req.PatientBarcodeScanned != "",
		"barcode_medication":   req.MedicationBarcodeScanned != "",
		"witnessed":            req.WitnessedByUserID != "",
	})
	writeJSON(w, http.StatusCreated, toAdminPayload(a))
}

func (h *Handler) listAdministrationsForOrder(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	items, err := h.deps.MAR.ListAdministrationsForOrder(r.Context(), orderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]medicationAdministrationPayload, 0, len(items))
	for i := range items {
		out = append(out, toAdminPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) listAdministrationsForAdmission(w http.ResponseWriter, r *http.Request) {
	admID, err := uuid.Parse(chi.URLParam(r, "admissionId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "admissionId geçersiz")
		return
	}
	items, err := h.deps.MAR.ListAdministrationsForAdmission(r.Context(), admID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]medicationAdministrationPayload, 0, len(items))
	for i := range items {
		out = append(out, toAdminPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}
