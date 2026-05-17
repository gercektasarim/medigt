package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/integration/hl7"
	"github.com/medigt/medigt/server/internal/middleware"
	"github.com/medigt/medigt/server/internal/service"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

// ---------- Ward ----------

type wardPayload struct {
	ID        string  `json:"id"`
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Floor     *string `json:"floor,omitempty"`
	Capacity  *int    `json:"capacity,omitempty"`
	IsActive  bool    `json:"is_active"`
	Notes     *string `json:"notes,omitempty"`
}

func toWardPayload(w *repo.Ward) wardPayload {
	return wardPayload{
		ID: w.ID.String(), Code: w.Code, Name: w.Name, Kind: w.Kind,
		Floor: w.Floor, Capacity: w.Capacity, IsActive: w.IsActive, Notes: w.Notes,
	}
}

func (h *Handler) listWards(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	activeOnly := r.URL.Query().Get("active") == "true"
	items, err := h.deps.Wards.List(r.Context(), branchID, activeOnly)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]wardPayload, 0, len(items))
	for i := range items {
		out = append(out, toWardPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type createWardReq struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Kind     string  `json:"kind"`
	Floor    string  `json:"floor"`
	Capacity *int    `json:"capacity"`
	Notes    string  `json:"notes"`
}

var validWardKinds = map[string]bool{
	"general": true, "icu": true, "ccu": true, "pediatrics": true,
	"maternity": true, "surgical": true, "isolation": true, "observation": true,
}

func (h *Handler) createWard(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req createWardReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	code := strings.TrimSpace(req.Code)
	name := strings.TrimSpace(req.Name)
	if code == "" || name == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "code ve name zorunlu")
		return
	}
	kind := req.Kind
	if kind == "" {
		kind = "general"
	}
	if !validWardKinds[kind] {
		writeError(w, http.StatusBadRequest, "bad_kind", "geçersiz servis türü")
		return
	}

	ward, err := h.deps.Wards.Create(r.Context(), repo.CreateWardInput{
		OrganizationID: orgID, BranchID: branchID,
		Code: strings.ToUpper(strings.ReplaceAll(code, " ", "_")),
		Name: name, Kind: kind,
		Floor:    emptyToNil(&req.Floor),
		Capacity: req.Capacity,
		Notes:    emptyToNil(&req.Notes),
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "code_taken", "bu kod zaten kayıtlı")
			return
		}
		h.deps.Log.Error("create ward failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, toWardPayload(ward))
}

// ---------- Bed ----------

type bedPayload struct {
	ID       string  `json:"id"`
	WardID   string  `json:"ward_id"`
	Code     string  `json:"code"`
	Kind     string  `json:"kind"`
	Status   string  `json:"status"`
	IsActive bool    `json:"is_active"`
	Notes    *string `json:"notes,omitempty"`
}

type bedMapPayload struct {
	Bed              bedPayload `json:"bed"`
	WardID           string     `json:"ward_id"`
	WardName         string     `json:"ward_name"`
	WardKind         string     `json:"ward_kind"`
	AdmissionID      *string    `json:"admission_id,omitempty"`
	AdmissionNo      *string    `json:"admission_no,omitempty"`
	PatientID        *string    `json:"patient_id,omitempty"`
	PatientFirstName *string    `json:"patient_first_name,omitempty"`
	PatientLastName  *string    `json:"patient_last_name,omitempty"`
	PatientMRN       *string    `json:"patient_mrn,omitempty"`
	AdmittedAt       *time.Time `json:"admitted_at,omitempty"`
}

func toBedPayload(b *repo.Bed) bedPayload {
	return bedPayload{
		ID: b.ID.String(), WardID: b.WardID.String(), Code: b.Code,
		Kind: b.Kind, Status: b.Status, IsActive: b.IsActive, Notes: b.Notes,
	}
}

func toBedMapPayload(j *repo.BedWithJoins) bedMapPayload {
	p := bedMapPayload{
		Bed: toBedPayload(&j.Bed), WardID: j.Bed.WardID.String(),
		WardName: j.WardName, WardKind: j.WardKind,
		PatientFirstName: j.PatientFirstName, PatientLastName: j.PatientLastName,
		PatientMRN: j.PatientMRN, AdmittedAt: j.AdmittedAt,
		AdmissionNo: j.AdmissionNo,
	}
	if j.AdmissionID != nil {
		s := j.AdmissionID.String()
		p.AdmissionID = &s
	}
	if j.PatientID != nil {
		s := j.PatientID.String()
		p.PatientID = &s
	}
	return p
}

func (h *Handler) getBedMap(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	items, err := h.deps.Beds.BedMap(r.Context(), branchID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]bedMapPayload, 0, len(items))
	for i := range items {
		out = append(out, toBedMapPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type createBedReq struct {
	Code  string  `json:"code"`
	Kind  string  `json:"kind"`
	Notes string  `json:"notes"`
}

var validBedKinds = map[string]bool{
	"standard": true, "icu": true, "isolation": true,
	"pediatric": true, "vip": true, "observation": true,
}

func (h *Handler) createBed(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	wardID, err := uuid.Parse(chi.URLParam(r, "wardId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_ward", "wardId geçersiz")
		return
	}
	// Verify ward belongs to this branch.
	if _, err := h.deps.Wards.GetByID(r.Context(), branchID, wardID); err != nil {
		writeError(w, http.StatusNotFound, "ward_not_found", "servis bulunamadı")
		return
	}
	var req createBedReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "code zorunlu")
		return
	}
	if req.Kind != "" && !validBedKinds[req.Kind] {
		writeError(w, http.StatusBadRequest, "bad_kind", "geçersiz yatak türü")
		return
	}
	bed, err := h.deps.Beds.Create(r.Context(), repo.CreateBedInput{
		WardID: wardID, Code: code, Kind: req.Kind, Notes: emptyToNil(&req.Notes),
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "code_taken", "bu yatak kodu zaten var")
			return
		}
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, toBedPayload(bed))
}

type updateBedStatusReq struct {
	Status string `json:"status"`
}

var validBedStatuses = map[string]bool{
	"free": true, "occupied": true, "reserved": true,
	"cleaning": true, "blocked": true,
}

func (h *Handler) updateBedStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req updateBedStatusReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if !validBedStatuses[req.Status] {
		writeError(w, http.StatusBadRequest, "bad_status", "geçersiz yatak durumu")
		return
	}
	if err := h.deps.Beds.SetStatus(r.Context(), id, req.Status); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "yatak bulunamadı")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_failed", "güncelleme başarısız")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- Admission ----------

type admissionPayload struct {
	ID                  string     `json:"id"`
	AdmissionNo         string     `json:"admission_no"`
	PatientID           string     `json:"patient_id"`
	PatientMRN          string     `json:"patient_mrn"`
	PatientFirstName    string     `json:"patient_first_name"`
	PatientLastName     string     `json:"patient_last_name"`
	PatientPhone        *string    `json:"patient_phone,omitempty"`
	WardID              string     `json:"ward_id"`
	WardCode            string     `json:"ward_code"`
	WardName            string     `json:"ward_name"`
	BedID               *string    `json:"bed_id,omitempty"`
	BedCode             *string    `json:"bed_code,omitempty"`
	AdmittingDoctorID   *string    `json:"admitting_doctor_id,omitempty"`
	DoctorFirstName     *string    `json:"doctor_first_name,omitempty"`
	DoctorLastName      *string    `json:"doctor_last_name,omitempty"`
	DoctorTitle         *string    `json:"doctor_title,omitempty"`
	Kind                string     `json:"kind"`
	Status              string     `json:"status"`
	ChiefComplaint      *string    `json:"chief_complaint,omitempty"`
	AdmissionDiagnosis  *string    `json:"admission_diagnosis,omitempty"`
	Notes               *string    `json:"notes,omitempty"`
	AdmittedAt          time.Time  `json:"admitted_at"`
	DischargedAt        *time.Time `json:"discharged_at,omitempty"`
	DischargeKind       *string    `json:"discharge_kind,omitempty"`
	DischargeSummary    *string    `json:"discharge_summary,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func baseAdmission(a *repo.Admission) admissionPayload {
	p := admissionPayload{
		ID: a.ID.String(), AdmissionNo: a.AdmissionNo,
		PatientID: a.PatientID.String(), WardID: a.WardID.String(),
		Kind: a.Kind, Status: a.Status,
		ChiefComplaint: a.ChiefComplaint, AdmissionDiagnosis: a.AdmissionDiagnosis,
		Notes: a.Notes, AdmittedAt: a.AdmittedAt,
		DischargedAt: a.DischargedAt, DischargeKind: a.DischargeKind,
		DischargeSummary: a.DischargeSummary,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
	if a.AdmittingDoctorID != nil {
		s := a.AdmittingDoctorID.String()
		p.AdmittingDoctorID = &s
	}
	if a.BedID != nil {
		s := a.BedID.String()
		p.BedID = &s
	}
	return p
}

func joinedAdmission(j *repo.AdmissionWithJoins) admissionPayload {
	p := baseAdmission(&j.Admission)
	p.PatientMRN = j.PatientMRN
	p.PatientFirstName = j.PatientFirstName
	p.PatientLastName = j.PatientLastName
	p.PatientPhone = j.PatientPhone
	p.WardCode = j.WardCode
	p.WardName = j.WardName
	p.BedCode = j.BedCode
	p.DoctorFirstName = j.DoctorFirstName
	p.DoctorLastName = j.DoctorLastName
	p.DoctorTitle = j.DoctorTitle
	return p
}

func (h *Handler) listAdmissions(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	f := repo.ListAdmissionFilter{
		Status: r.URL.Query().Get("status"),
	}
	if v := r.URL.Query().Get("ward_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.WardID = &id
		}
	}
	items, err := h.deps.Admissions.List(r.Context(), branchID, f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]admissionPayload, 0, len(items))
	for i := range items {
		out = append(out, joinedAdmission(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) getAdmission(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	a, err := h.deps.Admissions.GetByID(r.Context(), branchID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "yatış bulunamadı")
		return
	}
	writeJSON(w, http.StatusOK, joinedAdmission(a))
}

type admitReq struct {
	PatientID          string `json:"patient_id"`
	WardID             string `json:"ward_id"`
	BedID              string `json:"bed_id"`
	AdmittingDoctorID  string `json:"admitting_doctor_id"`
	Kind               string `json:"kind"`
	ChiefComplaint     string `json:"chief_complaint"`
	AdmissionDiagnosis string `json:"admission_diagnosis"`
	Notes              string `json:"notes"`
}

var validAdmissionKinds = map[string]bool{
	"planned": true, "emergency": true, "transfer_in": true, "newborn": true,
}

func (h *Handler) admit(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req admitReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_patient", "patient_id zorunlu")
		return
	}
	wardID, err := uuid.Parse(req.WardID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_ward", "ward_id zorunlu")
		return
	}
	in := service.AdmitInput{
		OrganizationID: orgID, BranchID: branchID,
		PatientID: patientID, WardID: wardID,
		Kind: req.Kind,
		ChiefComplaint:     emptyToNil(&req.ChiefComplaint),
		AdmissionDiagnosis: emptyToNil(&req.AdmissionDiagnosis),
		Notes:              emptyToNil(&req.Notes),
	}
	if in.Kind == "" {
		in.Kind = "planned"
	}
	if !validAdmissionKinds[in.Kind] {
		writeError(w, http.StatusBadRequest, "bad_kind", "geçersiz yatış türü")
		return
	}
	if req.BedID != "" {
		bid, err := uuid.Parse(req.BedID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_bed", "bed_id geçersiz")
			return
		}
		in.BedID = &bid
	}
	if req.AdmittingDoctorID != "" {
		if did, err := uuid.Parse(req.AdmittingDoctorID); err == nil {
			in.AdmittingDoctorID = &did
		}
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.AdmittedByUserID = &uid
		}
	}

	a, err := h.deps.AdmissionSvc.Admit(r.Context(), in)
	switch {
	case errors.Is(err, service.ErrBedUnavailable):
		writeError(w, http.StatusConflict, "bed_unavailable", err.Error())
		return
	case errors.Is(err, service.ErrBedWrongWard):
		writeError(w, http.StatusBadRequest, "wrong_ward", err.Error())
		return
	case errors.Is(err, service.ErrPatientAlreadyAdmitted):
		writeError(w, http.StatusConflict, "already_admitted", err.Error())
		return
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "bed_not_found", "yatak bulunamadı")
		return
	case err != nil:
		h.deps.Log.Error("admit failed", "err", err)
		writeError(w, http.StatusInternalServerError, "admit_failed", "yatış oluşturulamadı")
		return
	}
	// HL7 ADT^A01 — downstream consumers (PACS / LIS / HIE) öğrensin.
	// Emit is best-effort; outbox sürdürürse retry yapar.
	if h.deps.ADT != nil {
		h.deps.ADT.Emit(r.Context(), hl7.EventAdmit, a.ID)
		h.auditAccess(r.Context(), r, "hl7.adt.emit", "admission", a.ID.String(), map[string]any{
			"event": "A01",
		})
	}
	writeJSON(w, http.StatusCreated, baseAdmission(a))
}

type transferReq struct {
	ToBedID string `json:"to_bed_id"`
	Reason  string `json:"reason"`
}

func (h *Handler) transferAdmission(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req transferReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	toBedID, err := uuid.Parse(req.ToBedID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_bed", "to_bed_id zorunlu")
		return
	}
	in := service.TransferInput{
		BranchID: branchID, AdmissionID: id, ToBedID: toBedID,
		Reason: emptyToNil(&req.Reason),
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.TransferredByUserID = &uid
		}
	}
	a, err := h.deps.AdmissionSvc.Transfer(r.Context(), in)
	switch {
	case errors.Is(err, service.ErrAdmissionMissing):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	case errors.Is(err, service.ErrAdmissionClosed):
		writeError(w, http.StatusConflict, "closed", err.Error())
		return
	case errors.Is(err, service.ErrBedUnavailable):
		writeError(w, http.StatusConflict, "bed_unavailable", err.Error())
		return
	case err != nil:
		h.deps.Log.Error("transfer failed", "err", err)
		writeError(w, http.StatusInternalServerError, "transfer_failed", "transfer başarısız")
		return
	}
	if h.deps.ADT != nil {
		h.deps.ADT.Emit(r.Context(), hl7.EventTransfer, a.ID)
		h.auditAccess(r.Context(), r, "hl7.adt.emit", "admission", a.ID.String(), map[string]any{
			"event": "A02",
		})
	}
	writeJSON(w, http.StatusOK, baseAdmission(a))
}

type dischargeReq struct {
	Kind    string `json:"kind"`
	Summary string `json:"summary"`
}

var validDischargeKinds = map[string]bool{
	"home": true, "home_with_help": true, "referred": true,
	"against_advice": true, "left_without_notice": true,
	"transferred": true, "expired": true,
}

func (h *Handler) discharge(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req dischargeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	kind := req.Kind
	if kind == "" {
		kind = "home"
	}
	if !validDischargeKinds[kind] {
		writeError(w, http.StatusBadRequest, "bad_kind", "geçersiz taburcu türü")
		return
	}
	in := service.DischargeInput{
		BranchID: branchID, AdmissionID: id, Kind: kind,
		Summary: emptyToNil(&req.Summary),
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.DischargedByUserID = &uid
		}
	}
	a, err := h.deps.AdmissionSvc.Discharge(r.Context(), in)
	switch {
	case errors.Is(err, service.ErrAdmissionMissing):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	case errors.Is(err, service.ErrAdmissionClosed):
		writeError(w, http.StatusConflict, "closed", err.Error())
		return
	case err != nil:
		h.deps.Log.Error("discharge failed", "err", err)
		writeError(w, http.StatusInternalServerError, "discharge_failed", "taburcu başarısız")
		return
	}
	if h.deps.ADT != nil {
		h.deps.ADT.Emit(r.Context(), hl7.EventDischarge, a.ID)
		h.auditAccess(r.Context(), r, "hl7.adt.emit", "admission", a.ID.String(), map[string]any{
			"event": "A03",
		})
	}
	writeJSON(w, http.StatusOK, baseAdmission(a))
}

// ---------- HL7 ADT outbound messages for an admission ----------

type adtMessagePayload struct {
	ID               string     `json:"id"`
	MessageControlID string     `json:"message_control_id"`
	EventType        string     `json:"event_type"`
	Status           string     `json:"status"`
	RetryCount       int        `json:"retry_count"`
	NextRetryAt      time.Time  `json:"next_retry_at"`
	LastError        *string    `json:"last_error,omitempty"`
	SentAt           *time.Time `json:"sent_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	RawMessage       string     `json:"raw_message"`
	AckRaw           *string    `json:"ack_raw,omitempty"`
}

func (h *Handler) listAdmissionADTMessages(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	items, err := h.deps.HL7Outbound.ListForAdmission(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]adtMessagePayload, 0, len(items))
	for _, m := range items {
		out = append(out, adtMessagePayload{
			ID: m.ID.String(), MessageControlID: m.MessageControlID,
			EventType: m.EventType, Status: m.Status,
			RetryCount: m.RetryCount, NextRetryAt: m.NextRetryAt,
			LastError: m.LastError, SentAt: m.SentAt, CreatedAt: m.CreatedAt,
			RawMessage: m.RawMessage, AckRaw: m.AckRaw,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- Bed transfer audit ----------

type bedTransferPayload struct {
	ID            string    `json:"id"`
	FromBedCode   *string   `json:"from_bed_code,omitempty"`
	ToBedCode     string    `json:"to_bed_code"`
	FromWardName  *string   `json:"from_ward_name,omitempty"`
	ToWardName    string    `json:"to_ward_name"`
	Reason        *string   `json:"reason,omitempty"`
	TransferredAt time.Time `json:"transferred_at"`
}

func (h *Handler) listAdmissionTransfers(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	items, err := h.deps.Admissions.ListTransfers(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]bedTransferPayload, 0, len(items))
	for _, t := range items {
		out = append(out, bedTransferPayload{
			ID: t.ID.String(), FromBedCode: t.FromBedCode, ToBedCode: t.ToBedCode,
			FromWardName: t.FromWardName, ToWardName: t.ToWardName,
			Reason: t.Reason, TransferredAt: t.TransferredAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
