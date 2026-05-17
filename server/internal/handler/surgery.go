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

// ---------- Operating room ----------

type operatingRoomPayload struct {
	ID    string  `json:"id"`
	Code  string  `json:"code"`
	Name  string  `json:"name"`
	Floor *string `json:"floor,omitempty"`
}

func toORPayload(o *repo.OperatingRoom) operatingRoomPayload {
	return operatingRoomPayload{ID: o.ID.String(), Code: o.Code, Name: o.Name, Floor: o.Floor}
}

func (h *Handler) listOperatingRooms(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	items, err := h.deps.OperatingRooms.List(r.Context(), branchID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]operatingRoomPayload, 0, len(items))
	for i := range items {
		out = append(out, toORPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type createORReq struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Floor string `json:"floor"`
	Notes string `json:"notes"`
}

func (h *Handler) createOperatingRoom(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req createORReq
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
	or, err := h.deps.OperatingRooms.Create(r.Context(), repo.CreateORInput{
		OrganizationID: orgID, BranchID: branchID,
		Code: strings.ToUpper(strings.ReplaceAll(code, " ", "_")),
		Name: name, Floor: emptyToNil(&req.Floor), Notes: emptyToNil(&req.Notes),
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "code_taken", "bu kod zaten kayıtlı")
			return
		}
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, toORPayload(or))
}

// ---------- Surgery ----------

type surgeryTeamPayload struct {
	StaffMemberID *string `json:"staff_member_id,omitempty"`
	DoctorID      *string `json:"doctor_id,omitempty"`
	Role          string  `json:"role"`
	Name          string  `json:"name"`
}

type surgeryPayload struct {
	ID                 string                `json:"id"`
	SurgeryNo          string                `json:"surgery_no"`
	Status             string                `json:"status"`
	Priority           string                `json:"priority"`
	PatientID          string                `json:"patient_id"`
	PatientMRN         string                `json:"patient_mrn"`
	PatientFirstName   string                `json:"patient_first_name"`
	PatientLastName    string                `json:"patient_last_name"`
	OperatingRoomID    string                `json:"operating_room_id"`
	OperatingRoomCode  string                `json:"operating_room_code"`
	OperatingRoomName  string                `json:"operating_room_name"`
	PrimarySurgeonID   *string               `json:"primary_surgeon_id,omitempty"`
	SurgeonFirstName   *string               `json:"surgeon_first_name,omitempty"`
	SurgeonLastName    *string               `json:"surgeon_last_name,omitempty"`
	SurgeonTitle       *string               `json:"surgeon_title,omitempty"`
	AdmissionID        *string               `json:"admission_id,omitempty"`
	ProcedureName      string                `json:"procedure_name"`
	ProcedureCodes     []string              `json:"procedure_codes"`
	Indication         *string               `json:"indication,omitempty"`
	AnesthesiaType     string                `json:"anesthesia_type"`
	ScheduledAt        time.Time             `json:"scheduled_at"`
	EstimatedMinutes   int                   `json:"estimated_minutes"`
	Team               []surgeryTeamPayload  `json:"team"`
	StartedAt          *time.Time            `json:"started_at,omitempty"`
	EndedAt            *time.Time            `json:"ended_at,omitempty"`
	OpNote             *string               `json:"op_note,omitempty"`
	Complications      *string               `json:"complications,omitempty"`
	BloodLossML        *int                  `json:"blood_loss_ml,omitempty"`
	SpecimenSent       bool                  `json:"specimen_sent"`
	CancelledAt        *time.Time            `json:"cancelled_at,omitempty"`
	CancellationReason *string               `json:"cancellation_reason,omitempty"`
	CreatedAt          time.Time             `json:"created_at"`
	UpdatedAt          time.Time             `json:"updated_at"`
}

func teamToPayload(team []repo.SurgeryTeamMember) []surgeryTeamPayload {
	out := make([]surgeryTeamPayload, 0, len(team))
	for _, m := range team {
		t := surgeryTeamPayload{Role: m.Role, Name: m.Name}
		if m.StaffMemberID != nil {
			s := m.StaffMemberID.String()
			t.StaffMemberID = &s
		}
		if m.DoctorID != nil {
			s := m.DoctorID.String()
			t.DoctorID = &s
		}
		out = append(out, t)
	}
	return out
}

func baseSurgery(s *repo.Surgery) surgeryPayload {
	p := surgeryPayload{
		ID: s.ID.String(), SurgeryNo: s.SurgeryNo, Status: s.Status, Priority: s.Priority,
		PatientID: s.PatientID.String(),
		OperatingRoomID: s.OperatingRoomID.String(),
		ProcedureName: s.ProcedureName, ProcedureCodes: s.ProcedureCodes,
		Indication: s.Indication, AnesthesiaType: s.AnesthesiaType,
		ScheduledAt: s.ScheduledAt, EstimatedMinutes: s.EstimatedMinutes,
		Team:        teamToPayload(s.Team),
		StartedAt: s.StartedAt, EndedAt: s.EndedAt, OpNote: s.OpNote,
		Complications: s.Complications, BloodLossML: s.BloodLossML, SpecimenSent: s.SpecimenSent,
		CancelledAt: s.CancelledAt, CancellationReason: s.CancellationReason,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
	if s.PrimarySurgeonID != nil {
		v := s.PrimarySurgeonID.String()
		p.PrimarySurgeonID = &v
	}
	if s.AdmissionID != nil {
		v := s.AdmissionID.String()
		p.AdmissionID = &v
	}
	if p.ProcedureCodes == nil {
		p.ProcedureCodes = []string{}
	}
	return p
}

func joinedSurgery(w *repo.SurgeryWithJoins) surgeryPayload {
	p := baseSurgery(&w.Surgery)
	p.PatientMRN = w.PatientMRN
	p.PatientFirstName = w.PatientFirstName
	p.PatientLastName = w.PatientLastName
	p.OperatingRoomCode = w.OperatingRoomCode
	p.OperatingRoomName = w.OperatingRoomName
	p.SurgeonFirstName = w.SurgeonFirstName
	p.SurgeonLastName = w.SurgeonLastName
	p.SurgeonTitle = w.SurgeonTitle
	return p
}

func (h *Handler) listSurgeries(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	f := repo.ListSurgeryFilter{
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
	if v := r.URL.Query().Get("operating_room_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.ORID = &id
		}
	}

	items, err := h.deps.Surgeries.List(r.Context(), branchID, f)
	if err != nil {
		h.deps.Log.Error("list surgeries failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]surgeryPayload, 0, len(items))
	for i := range items {
		out = append(out, joinedSurgery(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) getSurgery(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	s, err := h.deps.Surgeries.GetByID(r.Context(), branchID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "ameliyat bulunamadı")
		return
	}
	writeJSON(w, http.StatusOK, baseSurgery(s))
}

type createSurgeryReq struct {
	PatientID         string   `json:"patient_id"`
	OperatingRoomID   string   `json:"operating_room_id"`
	PrimarySurgeonID  string   `json:"primary_surgeon_id"`
	AdmissionID       string   `json:"admission_id"`
	Priority          string   `json:"priority"`
	ProcedureName     string   `json:"procedure_name"`
	ProcedureCodes    []string `json:"procedure_codes"`
	Indication        string   `json:"indication"`
	AnesthesiaType    string   `json:"anesthesia_type"`
	ScheduledAt       string   `json:"scheduled_at"`     // RFC3339
	EstimatedMinutes  int      `json:"estimated_minutes"`
	Team              []surgeryTeamPayload `json:"team"`
}

var validSurgeryPriorities = map[string]bool{"elective": true, "urgent": true, "emergency": true}
var validAnesthesia = map[string]bool{
	"general": true, "regional": true, "spinal": true,
	"epidural": true, "local": true, "sedation": true, "none": true,
}
var validTeamRoles = map[string]bool{
	"primary_surgeon": true, "assistant": true, "anesthesiologist": true,
	"scrub_nurse": true, "circulating_nurse": true, "technician": true,
}

func (h *Handler) createSurgery(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req createSurgeryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_patient", "patient_id zorunlu")
		return
	}
	orID, err := uuid.Parse(req.OperatingRoomID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_or", "operating_room_id zorunlu")
		return
	}
	scheduled, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_date", "scheduled_at RFC3339 olmalı")
		return
	}
	priority := req.Priority
	if priority == "" {
		priority = "elective"
	}
	if !validSurgeryPriorities[priority] {
		writeError(w, http.StatusBadRequest, "bad_priority", "geçersiz öncelik")
		return
	}
	anesthesia := req.AnesthesiaType
	if anesthesia == "" {
		anesthesia = "general"
	}
	if !validAnesthesia[anesthesia] {
		writeError(w, http.StatusBadRequest, "bad_anesthesia", "geçersiz anestezi türü")
		return
	}
	if strings.TrimSpace(req.ProcedureName) == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "procedure_name zorunlu")
		return
	}
	team := make([]repo.SurgeryTeamMember, 0, len(req.Team))
	for _, m := range req.Team {
		if !validTeamRoles[m.Role] {
			writeError(w, http.StatusBadRequest, "bad_role", "geçersiz ekip rolü: "+m.Role)
			return
		}
		entry := repo.SurgeryTeamMember{Role: m.Role, Name: strings.TrimSpace(m.Name)}
		if m.StaffMemberID != nil && *m.StaffMemberID != "" {
			if sid, err := uuid.Parse(*m.StaffMemberID); err == nil {
				entry.StaffMemberID = &sid
			}
		}
		if m.DoctorID != nil && *m.DoctorID != "" {
			if did, err := uuid.Parse(*m.DoctorID); err == nil {
				entry.DoctorID = &did
			}
		}
		team = append(team, entry)
	}

	in := repo.CreateSurgeryInput{
		OrganizationID: orgID, BranchID: branchID,
		PatientID: patientID, OperatingRoomID: orID,
		Priority: priority, ProcedureName: strings.TrimSpace(req.ProcedureName),
		ProcedureCodes: req.ProcedureCodes,
		Indication: emptyToNil(&req.Indication),
		AnesthesiaType: anesthesia,
		ScheduledAt: scheduled, EstimatedMinutes: req.EstimatedMinutes,
		Team: team,
	}
	if req.PrimarySurgeonID != "" {
		if id, err := uuid.Parse(req.PrimarySurgeonID); err == nil {
			in.PrimarySurgeonID = &id
		}
	}
	if req.AdmissionID != "" {
		if id, err := uuid.Parse(req.AdmissionID); err == nil {
			in.AdmissionID = &id
		}
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.CreatedByUserID = &uid
		}
	}

	nextNo, err := h.deps.Surgeries.NextNo(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "seq_failed", "ameliyat numarası alınamadı")
		return
	}
	in.SurgeryNo = util.FormatMRN(nextNo)

	s, err := h.deps.Surgeries.Create(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("create surgery failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, baseSurgery(s))
}

type updateSurgeryStatusReq struct {
	Status string `json:"status"`
}

var validSurgeryStatuses = map[string]bool{
	"in_progress": true, "completed": true, "cancelled": true,
}

func (h *Handler) updateSurgeryStatus(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req updateSurgeryStatusReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if !validSurgeryStatuses[req.Status] {
		writeError(w, http.StatusBadRequest, "bad_status", "geçersiz durum")
		return
	}
	s, err := h.deps.Surgeries.UpdateStatus(r.Context(), branchID, id, req.Status)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "ameliyat bulunamadı")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_failed", "güncelleme başarısız")
		return
	}
	writeJSON(w, http.StatusOK, baseSurgery(s))
}

type saveOpNoteReq struct {
	OpNote        *string `json:"op_note"`
	Complications *string `json:"complications"`
	BloodLossML   *int    `json:"blood_loss_ml"`
	SpecimenSent  *bool   `json:"specimen_sent"`
}

func (h *Handler) saveSurgeryOpNote(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req saveOpNoteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	s, err := h.deps.Surgeries.SaveOpNote(r.Context(), branchID, id,
		emptyToNil(req.OpNote), emptyToNil(req.Complications), req.BloodLossML, req.SpecimenSent)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "ameliyat bulunamadı")
			return
		}
		writeError(w, http.StatusInternalServerError, "save_failed", "kaydedilemedi")
		return
	}
	writeJSON(w, http.StatusOK, baseSurgery(s))
}
