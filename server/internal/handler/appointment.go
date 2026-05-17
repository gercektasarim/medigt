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
	"github.com/medigt/medigt/server/pkg/db/repo"
)

type appointmentPayload struct {
	ID                 string     `json:"id"`
	OrganizationID     string     `json:"organization_id"`
	BranchID           string     `json:"branch_id"`
	PatientID          string     `json:"patient_id"`
	DoctorID           *string    `json:"doctor_id,omitempty"`
	DepartmentID       *string    `json:"department_id,omitempty"`
	ScheduledAt        time.Time  `json:"scheduled_at"`
	DurationMinutes    int        `json:"duration_minutes"`
	Status             string     `json:"status"`
	Kind               string     `json:"kind"`
	Reason             *string    `json:"reason,omitempty"`
	Notes              *string    `json:"notes,omitempty"`
	ArrivedAt          *time.Time `json:"arrived_at,omitempty"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	CancelledAt        *time.Time `json:"cancelled_at,omitempty"`
	CancellationReason *string    `json:"cancellation_reason,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`

	// Joined display fields (populated by list endpoint only).
	PatientMRN       string  `json:"patient_mrn,omitempty"`
	PatientFirstName string  `json:"patient_first_name,omitempty"`
	PatientLastName  string  `json:"patient_last_name,omitempty"`
	PatientPhone     *string `json:"patient_phone,omitempty"`
	DoctorFirstName  *string `json:"doctor_first_name,omitempty"`
	DoctorLastName   *string `json:"doctor_last_name,omitempty"`
	DoctorTitle      *string `json:"doctor_title,omitempty"`
}

func basePayload(a *repo.Appointment) appointmentPayload {
	p := appointmentPayload{
		ID: a.ID.String(), OrganizationID: a.OrganizationID.String(),
		BranchID: a.BranchID.String(), PatientID: a.PatientID.String(),
		ScheduledAt: a.ScheduledAt, DurationMinutes: a.DurationMinutes,
		Status: a.Status, Kind: a.Kind, Reason: a.Reason, Notes: a.Notes,
		ArrivedAt: a.ArrivedAt, StartedAt: a.StartedAt, CompletedAt: a.CompletedAt,
		CancelledAt: a.CancelledAt, CancellationReason: a.CancellationReason,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
	if a.DoctorID != nil {
		v := a.DoctorID.String()
		p.DoctorID = &v
	}
	if a.DepartmentID != nil {
		v := a.DepartmentID.String()
		p.DepartmentID = &v
	}
	return p
}

func joinedPayload(w *repo.AppointmentWithJoins) appointmentPayload {
	p := basePayload(&w.Appointment)
	p.PatientMRN = w.PatientMRN
	p.PatientFirstName = w.PatientFirstName
	p.PatientLastName = w.PatientLastName
	p.PatientPhone = w.PatientPhone
	p.DoctorFirstName = w.DoctorFirstName
	p.DoctorLastName = w.DoctorLastName
	p.DoctorTitle = w.DoctorTitle
	return p
}

func mustBranchID(w http.ResponseWriter, r *http.Request) uuid.UUID {
	s := middleware.BranchIDFromContext(r.Context())
	if s == "" {
		writeError(w, http.StatusBadRequest, "missing_branch", "X-Branch-ID gerekli")
		return uuid.Nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_branch", "X-Branch-ID geçersiz")
		return uuid.Nil
	}
	return id
}

func (h *Handler) listAppointments(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}

	// Default: today (local Istanbul time, but we just use server time).
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	to := from.Add(24 * time.Hour)

	if v := r.URL.Query().Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			// Allow YYYY-MM-DD shorthand — implies start of that local day.
			t, err = time.Parse("2006-01-02", v)
			if err != nil {
				writeError(w, http.StatusBadRequest, "bad_date", "from RFC3339 veya YYYY-MM-DD olmalı")
				return
			}
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
		}
		from = t
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			t, err = time.Parse("2006-01-02", v)
			if err != nil {
				writeError(w, http.StatusBadRequest, "bad_date", "to RFC3339 veya YYYY-MM-DD olmalı")
				return
			}
			// Treat YYYY-MM-DD `to` as end-of-day exclusive (start of next day).
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local).Add(24 * time.Hour)
		}
		to = t
	}

	var doctorID *uuid.UUID
	if s := r.URL.Query().Get("doctor_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_doctor", "doctor_id geçersiz")
			return
		}
		doctorID = &id
	}

	items, err := h.deps.Appointments.ListByDay(r.Context(), branchID, from, to, doctorID)
	if err != nil {
		h.deps.Log.Error("list appointments failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]appointmentPayload, 0, len(items))
	for i := range items {
		out = append(out, joinedPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type createApptReq struct {
	PatientID       string `json:"patient_id"`
	DoctorID        string `json:"doctor_id"`
	DepartmentID    string `json:"department_id"`
	ScheduledAt     string `json:"scheduled_at"` // RFC3339
	DurationMinutes int    `json:"duration_minutes"`
	Kind            string `json:"kind"`
	Reason          string `json:"reason"`
	Notes           string `json:"notes"`
}

func (h *Handler) createAppointment(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req createApptReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_patient", "patient_id geçersiz")
		return
	}
	scheduled, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_date", "scheduled_at RFC3339 olmalı")
		return
	}
	in := repo.CreateAppointmentInput{
		OrganizationID:  orgID,
		BranchID:        branchID,
		PatientID:       patientID,
		ScheduledAt:     scheduled,
		DurationMinutes: req.DurationMinutes,
		Kind:            req.Kind,
		Reason:          emptyToNil(&req.Reason),
		Notes:           emptyToNil(&req.Notes),
	}
	if req.DoctorID != "" {
		id, err := uuid.Parse(req.DoctorID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_doctor", "doctor_id geçersiz")
			return
		}
		in.DoctorID = &id
	}
	if req.DepartmentID != "" {
		id, err := uuid.Parse(req.DepartmentID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_department", "department_id geçersiz")
			return
		}
		in.DepartmentID = &id
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.CreatedByUserID = &uid
		}
	}

	a, err := h.deps.Appointments.Create(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("create appointment failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, basePayload(a))
}

var validApptStatuses = map[string]bool{
	"arrived": true, "in_progress": true, "completed": true, "no_show": true,
}

type updateStatusReq struct {
	Status string `json:"status"`
}

func (h *Handler) updateAppointmentStatus(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req updateStatusReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if !validApptStatuses[req.Status] {
		writeError(w, http.StatusBadRequest, "bad_status",
			"geçerli durumlar: arrived, in_progress, completed, no_show")
		return
	}
	if err := h.deps.Appointments.UpdateStatus(r.Context(), branchID, id, req.Status); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "randevu bulunamadı")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_failed", "güncelleme başarısız")
		return
	}

	// Return the fresh row.
	a, err := h.deps.Appointments.GetByID(r.Context(), branchID, id)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, basePayload(a))
}

type cancelApptReq struct {
	Reason string `json:"reason"`
}

func (h *Handler) cancelAppointment(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req cancelApptReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	uid := uuid.Nil
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if v, err := uuid.Parse(s); err == nil {
			uid = v
		}
	}
	if err := h.deps.Appointments.Cancel(r.Context(), branchID, id, uid, strings.TrimSpace(req.Reason)); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "randevu bulunamadı veya zaten kapalı")
			return
		}
		writeError(w, http.StatusInternalServerError, "cancel_failed", "iptal başarısız")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
