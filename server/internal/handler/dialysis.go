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
	"github.com/medigt/medigt/server/internal/util"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

// ---------- Dialysis machine ----------

type dialysisMachinePayload struct {
	ID           string  `json:"id"`
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Manufacturer *string `json:"manufacturer,omitempty"`
	Model        *string `json:"model,omitempty"`
	Location     *string `json:"location,omitempty"`
}

func toDialysisMachinePayload(m *repo.DialysisMachine) dialysisMachinePayload {
	return dialysisMachinePayload{
		ID: m.ID.String(), Code: m.Code, Name: m.Name,
		Manufacturer: m.Manufacturer, Model: m.Model, Location: m.Location,
	}
}

func (h *Handler) listDialysisMachines(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	items, err := h.deps.DialysisMachines.List(r.Context(), branchID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]dialysisMachinePayload, 0, len(items))
	for i := range items {
		out = append(out, toDialysisMachinePayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type createDialysisMachineReq struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Location     string `json:"location"`
	Notes        string `json:"notes"`
}

func (h *Handler) createDialysisMachine(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req createDialysisMachineReq
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
	m, err := h.deps.DialysisMachines.Create(r.Context(), repo.CreateDialysisMachineInput{
		OrganizationID: orgID, BranchID: branchID,
		Code: strings.ToUpper(strings.ReplaceAll(code, " ", "_")),
		Name: name,
		Manufacturer: emptyToNil(&req.Manufacturer),
		Model:        emptyToNil(&req.Model),
		Location:     emptyToNil(&req.Location),
		Notes:        emptyToNil(&req.Notes),
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "code_taken", "bu kod zaten kayıtlı")
			return
		}
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, toDialysisMachinePayload(m))
}

// ---------- Dialysis session ----------

type dialysisSessionPayload struct {
	ID                      string     `json:"id"`
	SessionNo               string     `json:"session_no"`
	Status                  string     `json:"status"`
	Modality                string     `json:"modality"`
	VascularAccess          string     `json:"vascular_access"`
	PatientID               string     `json:"patient_id"`
	PatientMRN              string     `json:"patient_mrn,omitempty"`
	PatientFirstName        string     `json:"patient_first_name,omitempty"`
	PatientLastName         string     `json:"patient_last_name,omitempty"`
	MachineID               *string    `json:"machine_id,omitempty"`
	MachineCode             *string    `json:"machine_code,omitempty"`
	MachineName             *string    `json:"machine_name,omitempty"`
	AdmissionID             *string    `json:"admission_id,omitempty"`
	PrimaryNurseID          *string    `json:"primary_nurse_id,omitempty"`
	SupervisorDoctorID      *string    `json:"supervisor_doctor_id,omitempty"`
	ScheduledAt             time.Time  `json:"scheduled_at"`
	DurationMinutes         int        `json:"duration_minutes"`
	PreWeightKg             *float64   `json:"pre_weight_kg,omitempty"`
	PreSystolicBP           *int       `json:"pre_systolic_bp,omitempty"`
	PreDiastolicBP          *int       `json:"pre_diastolic_bp,omitempty"`
	DryWeightKg             *float64   `json:"dry_weight_kg,omitempty"`
	DialyzerType            *string    `json:"dialyzer_type,omitempty"`
	Anticoagulant           *string    `json:"anticoagulant,omitempty"`
	UltrafiltrationTargetML *int       `json:"ultrafiltration_target_ml,omitempty"`
	BloodFlowRate           *int       `json:"blood_flow_rate,omitempty"`
	DialysateFlowRate       *int       `json:"dialysate_flow_rate,omitempty"`
	StartedAt               *time.Time `json:"started_at,omitempty"`
	EndedAt                 *time.Time `json:"ended_at,omitempty"`
	PostWeightKg            *float64   `json:"post_weight_kg,omitempty"`
	PostSystolicBP          *int       `json:"post_systolic_bp,omitempty"`
	PostDiastolicBP         *int       `json:"post_diastolic_bp,omitempty"`
	ActualUltrafiltrationML *int       `json:"actual_ultrafiltration_ml,omitempty"`
	Complications           *string    `json:"complications,omitempty"`
	SessionNotes            *string    `json:"session_notes,omitempty"`
	CancelledAt             *time.Time `json:"cancelled_at,omitempty"`
	CancellationReason      *string    `json:"cancellation_reason,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

func baseDialysisSession(s *repo.DialysisSession) dialysisSessionPayload {
	p := dialysisSessionPayload{
		ID: s.ID.String(), SessionNo: s.SessionNo,
		Status: s.Status, Modality: s.Modality, VascularAccess: s.VascularAccess,
		PatientID: s.PatientID.String(),
		ScheduledAt: s.ScheduledAt, DurationMinutes: s.DurationMinutes,
		PreWeightKg: s.PreWeightKg, PreSystolicBP: s.PreSystolicBP,
		PreDiastolicBP: s.PreDiastolicBP, DryWeightKg: s.DryWeightKg,
		DialyzerType: s.DialyzerType, Anticoagulant: s.Anticoagulant,
		UltrafiltrationTargetML: s.UltrafiltrationTargetML,
		BloodFlowRate: s.BloodFlowRate, DialysateFlowRate: s.DialysateFlowRate,
		StartedAt: s.StartedAt, EndedAt: s.EndedAt,
		PostWeightKg: s.PostWeightKg, PostSystolicBP: s.PostSystolicBP,
		PostDiastolicBP: s.PostDiastolicBP,
		ActualUltrafiltrationML: s.ActualUltrafiltrationML,
		Complications: s.Complications, SessionNotes: s.SessionNotes,
		CancelledAt: s.CancelledAt, CancellationReason: s.CancellationReason,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
	if s.MachineID != nil {
		v := s.MachineID.String()
		p.MachineID = &v
	}
	if s.AdmissionID != nil {
		v := s.AdmissionID.String()
		p.AdmissionID = &v
	}
	if s.PrimaryNurseID != nil {
		v := s.PrimaryNurseID.String()
		p.PrimaryNurseID = &v
	}
	if s.SupervisorDoctorID != nil {
		v := s.SupervisorDoctorID.String()
		p.SupervisorDoctorID = &v
	}
	return p
}

func joinedDialysisSession(j *repo.DialysisSessionWithJoins) dialysisSessionPayload {
	p := baseDialysisSession(&j.Session)
	p.PatientMRN = j.PatientMRN
	p.PatientFirstName = j.PatientFirstName
	p.PatientLastName = j.PatientLastName
	p.MachineCode = j.MachineCode
	p.MachineName = j.MachineName
	return p
}

func (h *Handler) listDialysisSessions(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	f := repo.ListDialysisSessionFilter{
		Status: r.URL.Query().Get("status"),
	}
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
			f.From = &t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local).Add(24 * time.Hour)
			f.To = &t
		}
	}
	if v := r.URL.Query().Get("patient_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.PatientID = &id
		}
	}
	if v := r.URL.Query().Get("machine_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.MachineID = &id
		}
	}

	items, err := h.deps.DialysisSessions.List(r.Context(), branchID, f)
	if err != nil {
		h.deps.Log.Error("list dialysis sessions failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]dialysisSessionPayload, 0, len(items))
	for i := range items {
		out = append(out, joinedDialysisSession(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) getDialysisSession(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	s, err := h.deps.DialysisSessions.GetByID(r.Context(), branchID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "diyaliz seansı bulunamadı")
		return
	}
	writeJSON(w, http.StatusOK, baseDialysisSession(s))
}

type createDialysisSessionReq struct {
	PatientID               string  `json:"patient_id"`
	MachineID               string  `json:"machine_id"`
	AdmissionID             string  `json:"admission_id"`
	PrimaryNurseID          string  `json:"primary_nurse_id"`
	SupervisorDoctorID      string  `json:"supervisor_doctor_id"`
	Modality                string  `json:"modality"`
	VascularAccess          string  `json:"vascular_access"`
	ScheduledAt             string  `json:"scheduled_at"` // RFC3339
	DurationMinutes         int     `json:"duration_minutes"`
	DryWeightKg             *float64 `json:"dry_weight_kg"`
	DialyzerType            string  `json:"dialyzer_type"`
	Anticoagulant           string  `json:"anticoagulant"`
	UltrafiltrationTargetML *int    `json:"ultrafiltration_target_ml"`
	BloodFlowRate           *int    `json:"blood_flow_rate"`
	DialysateFlowRate       *int    `json:"dialysate_flow_rate"`
}

var validDialysisModalities = map[string]bool{
	"hemodialysis": true, "hemodiafiltration": true, "peritoneal": true,
}
var validVascularAccess = map[string]bool{
	"av_fistula": true, "av_graft": true, "central_catheter": true,
	"peritoneal_catheter": true, "other": true,
}

func (h *Handler) createDialysisSession(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req createDialysisSessionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_patient", "patient_id zorunlu")
		return
	}
	scheduled, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_date", "scheduled_at RFC3339 olmalı")
		return
	}
	modality := req.Modality
	if modality == "" {
		modality = "hemodialysis"
	}
	if !validDialysisModalities[modality] {
		writeError(w, http.StatusBadRequest, "bad_modality", "geçersiz modalite")
		return
	}
	access := req.VascularAccess
	if access == "" {
		access = "av_fistula"
	}
	if !validVascularAccess[access] {
		writeError(w, http.StatusBadRequest, "bad_access", "geçersiz damar yolu türü")
		return
	}

	in := repo.CreateDialysisSessionInput{
		OrganizationID: orgID, BranchID: branchID,
		PatientID: patientID,
		Modality: modality, VascularAccess: access,
		ScheduledAt: scheduled, DurationMinutes: req.DurationMinutes,
		DryWeightKg: req.DryWeightKg,
		DialyzerType: emptyToNil(&req.DialyzerType),
		Anticoagulant: emptyToNil(&req.Anticoagulant),
		UltrafiltrationTargetML: req.UltrafiltrationTargetML,
		BloodFlowRate: req.BloodFlowRate,
		DialysateFlowRate: req.DialysateFlowRate,
	}
	if req.MachineID != "" {
		if id, err := uuid.Parse(req.MachineID); err == nil {
			in.MachineID = &id
		}
	}
	if req.AdmissionID != "" {
		if id, err := uuid.Parse(req.AdmissionID); err == nil {
			in.AdmissionID = &id
		}
	}
	if req.PrimaryNurseID != "" {
		if id, err := uuid.Parse(req.PrimaryNurseID); err == nil {
			in.PrimaryNurseID = &id
		}
	}
	if req.SupervisorDoctorID != "" {
		if id, err := uuid.Parse(req.SupervisorDoctorID); err == nil {
			in.SupervisorDoctorID = &id
		}
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.CreatedByUserID = &uid
		}
	}

	nextNo, err := h.deps.DialysisSessions.NextNo(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "seq_failed", "seans numarası alınamadı")
		return
	}
	in.SessionNo = util.FormatMRN(nextNo)

	s, err := h.deps.DialysisSessions.Create(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("create dialysis session failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, baseDialysisSession(s))
}

type updateDialysisStatusReq struct {
	Status string `json:"status"`
}

var validDialysisStatuses = map[string]bool{
	"in_progress": true, "completed": true, "cancelled": true,
}

func (h *Handler) updateDialysisStatus(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req updateDialysisStatusReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if !validDialysisStatuses[req.Status] {
		writeError(w, http.StatusBadRequest, "bad_status", "geçersiz durum")
		return
	}
	s, err := h.deps.DialysisSessions.UpdateStatus(r.Context(), branchID, id, req.Status)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "diyaliz seansı bulunamadı")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_failed", "güncelleme başarısız")
		return
	}
	writeJSON(w, http.StatusOK, baseDialysisSession(s))
}

type saveDialysisRecordReq struct {
	PreWeightKg             *float64 `json:"pre_weight_kg"`
	PreSystolicBP           *int     `json:"pre_systolic_bp"`
	PreDiastolicBP          *int     `json:"pre_diastolic_bp"`
	PostWeightKg            *float64 `json:"post_weight_kg"`
	PostSystolicBP          *int     `json:"post_systolic_bp"`
	PostDiastolicBP         *int     `json:"post_diastolic_bp"`
	ActualUltrafiltrationML *int     `json:"actual_ultrafiltration_ml"`
	Complications           *string  `json:"complications"`
	SessionNotes            *string  `json:"session_notes"`
}

func (h *Handler) saveDialysisRecord(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req saveDialysisRecordReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	s, err := h.deps.DialysisSessions.SaveRecord(r.Context(), branchID, id, repo.SaveDialysisRecordInput{
		PreWeightKg: req.PreWeightKg, PreSystolicBP: req.PreSystolicBP, PreDiastolicBP: req.PreDiastolicBP,
		PostWeightKg: req.PostWeightKg, PostSystolicBP: req.PostSystolicBP, PostDiastolicBP: req.PostDiastolicBP,
		ActualUltrafiltrationML: req.ActualUltrafiltrationML,
		Complications: emptyToNil(req.Complications),
		SessionNotes:  emptyToNil(req.SessionNotes),
	})
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "diyaliz seansı bulunamadı")
			return
		}
		writeError(w, http.StatusInternalServerError, "save_failed", "kaydedilemedi")
		return
	}
	writeJSON(w, http.StatusOK, baseDialysisSession(s))
}
