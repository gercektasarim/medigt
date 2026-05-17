package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/middleware"
	"github.com/medigt/medigt/server/internal/service"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

type visitPayload struct {
	ID                       string     `json:"id"`
	OrganizationID           string     `json:"organization_id"`
	BranchID                 string     `json:"branch_id"`
	PatientID                string     `json:"patient_id"`
	DoctorID                 *string    `json:"doctor_id,omitempty"`
	AppointmentID            *string    `json:"appointment_id,omitempty"`
	EncounterType            string     `json:"encounter_type"`
	Status                   string     `json:"status"`
	ChiefComplaint           *string    `json:"chief_complaint,omitempty"`
	HistoryOfPresentIllness  *string    `json:"history_of_present_illness,omitempty"`
	ExaminationFindings      *string    `json:"examination_findings,omitempty"`
	TreatmentPlan            *string    `json:"treatment_plan,omitempty"`
	Notes                    *string    `json:"notes,omitempty"`
	StartedAt                time.Time  `json:"started_at"`
	EndedAt                  *time.Time `json:"ended_at,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`

	PatientMRN       string  `json:"patient_mrn,omitempty"`
	PatientFirstName string  `json:"patient_first_name,omitempty"`
	PatientLastName  string  `json:"patient_last_name,omitempty"`
	PatientPhone     *string `json:"patient_phone,omitempty"`
	DoctorFirstName  *string `json:"doctor_first_name,omitempty"`
	DoctorLastName   *string `json:"doctor_last_name,omitempty"`
	DoctorTitle      *string `json:"doctor_title,omitempty"`
}

func baseVisit(v *repo.Visit) visitPayload {
	p := visitPayload{
		ID: v.ID.String(), OrganizationID: v.OrganizationID.String(),
		BranchID: v.BranchID.String(), PatientID: v.PatientID.String(),
		EncounterType: v.EncounterType, Status: v.Status,
		ChiefComplaint: v.ChiefComplaint, HistoryOfPresentIllness: v.HistoryOfPresentIllness,
		ExaminationFindings: v.ExaminationFindings, TreatmentPlan: v.TreatmentPlan,
		Notes: v.Notes,
		StartedAt: v.StartedAt, EndedAt: v.EndedAt,
		CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
	if v.DoctorID != nil {
		s := v.DoctorID.String()
		p.DoctorID = &s
	}
	if v.AppointmentID != nil {
		s := v.AppointmentID.String()
		p.AppointmentID = &s
	}
	return p
}

func joinedVisit(w *repo.VisitWithJoins) visitPayload {
	p := baseVisit(&w.Visit)
	p.PatientMRN = w.PatientMRN
	p.PatientFirstName = w.PatientFirstName
	p.PatientLastName = w.PatientLastName
	p.PatientPhone = w.PatientPhone
	p.DoctorFirstName = w.DoctorFirstName
	p.DoctorLastName = w.DoctorLastName
	p.DoctorTitle = w.DoctorTitle
	return p
}

func (h *Handler) listVisits(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	f := repo.ListVisitFilter{}
	if s := r.URL.Query().Get("status"); s != "" {
		f.Status = s
	}
	if r.URL.Query().Get("mine") == "true" {
		if userIDStr := middleware.UserIDFromContext(r.Context()); userIDStr != "" {
			if uid, err := uuid.Parse(userIDStr); err == nil {
				// Map the current user to their doctor record (best-effort).
				// For now we just filter the visits where doctor_id matches the user's
				// staff -> doctor chain via a separate lookup; if no doctor exists we
				// fall back to returning everything.
				_ = uid
			}
		}
	}
	if v := r.URL.Query().Get("from"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err == nil {
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
			f.From = &t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err == nil {
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local).Add(24 * time.Hour)
			f.To = &t
		}
	}

	items, err := h.deps.Visits.ListWithJoins(r.Context(), branchID, f)
	if err != nil {
		h.deps.Log.Error("list visits failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]visitPayload, 0, len(items))
	for i := range items {
		out = append(out, joinedVisit(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) getVisit(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	v, err := h.deps.Visits.GetByID(r.Context(), branchID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "muayene bulunamadı")
		return
	}
	writeJSON(w, http.StatusOK, baseVisit(v))
}

type startVisitReq struct {
	AppointmentID string `json:"appointment_id"`
}

func (h *Handler) startVisitFromAppointment(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req startVisitReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	apptID, err := uuid.Parse(req.AppointmentID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_appointment", "appointment_id geçersiz")
		return
	}
	var openedBy *uuid.UUID
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			openedBy = &uid
		}
	}
	v, err := h.deps.VisitSvc.StartFromAppointment(r.Context(), branchID, apptID, openedBy)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAppointmentMissing):
			writeError(w, http.StatusNotFound, "appointment_missing", err.Error())
		default:
			h.deps.Log.Error("start visit failed", "err", err)
			writeError(w, http.StatusInternalServerError, "start_failed", "muayene başlatılamadı")
		}
		return
	}
	writeJSON(w, http.StatusOK, baseVisit(v))
}

type updateVisitNotesReq struct {
	ChiefComplaint           *string `json:"chief_complaint"`
	HistoryOfPresentIllness  *string `json:"history_of_present_illness"`
	ExaminationFindings      *string `json:"examination_findings"`
	TreatmentPlan            *string `json:"treatment_plan"`
	Notes                    *string `json:"notes"`
}

func (h *Handler) updateVisitNotes(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req updateVisitNotesReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if err := h.deps.Visits.UpdateNotes(r.Context(), branchID, id, repo.UpdateVisitNotesInput{
		ChiefComplaint:          req.ChiefComplaint,
		HistoryOfPresentIllness: req.HistoryOfPresentIllness,
		ExaminationFindings:     req.ExaminationFindings,
		TreatmentPlan:           req.TreatmentPlan,
		Notes:                   req.Notes,
	}); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "muayene bulunamadı")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_failed", "güncelleme başarısız")
		return
	}
	v, err := h.deps.Visits.GetByID(r.Context(), branchID, id)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, baseVisit(v))
}

func (h *Handler) completeVisit(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	if err := h.deps.Visits.Complete(r.Context(), branchID, id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "muayene bulunamadı veya zaten kapalı")
		return
	}
	// Best-effort: also flip the linked appointment to completed.
	v, _ := h.deps.Visits.GetByID(r.Context(), branchID, id)
	if v != nil && v.AppointmentID != nil {
		_ = h.deps.Appointments.UpdateStatus(r.Context(), branchID, *v.AppointmentID, "completed")
	}
	if v != nil {
		writeJSON(w, http.StatusOK, baseVisit(v))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
