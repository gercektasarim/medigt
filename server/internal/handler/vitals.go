package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/middleware"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

type vitalsPayload struct {
	ID            string    `json:"id"`
	PatientID     string    `json:"patient_id"`
	VisitID       *string   `json:"visit_id,omitempty"`
	MeasuredAt    time.Time `json:"measured_at"`
	SystolicBP    *int      `json:"systolic_bp,omitempty"`
	DiastolicBP   *int      `json:"diastolic_bp,omitempty"`
	Pulse         *int      `json:"pulse,omitempty"`
	TemperatureC  *float64  `json:"temperature_c,omitempty"`
	Spo2Percent   *int      `json:"spo2_percent,omitempty"`
	Respiration   *int      `json:"respiration,omitempty"`
	WeightKg      *float64  `json:"weight_kg,omitempty"`
	HeightCm      *float64  `json:"height_cm,omitempty"`
	PainScore     *int      `json:"pain_score,omitempty"`
	Notes         *string   `json:"notes,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

func toVitalsPayload(v *repo.VitalSigns) vitalsPayload {
	p := vitalsPayload{
		ID: v.ID.String(), PatientID: v.PatientID.String(),
		MeasuredAt: v.MeasuredAt,
		SystolicBP: v.SystolicBP, DiastolicBP: v.DiastolicBP, Pulse: v.Pulse,
		TemperatureC: v.TemperatureC, Spo2Percent: v.Spo2Percent,
		Respiration: v.Respiration, WeightKg: v.WeightKg, HeightCm: v.HeightCm,
		PainScore: v.PainScore, Notes: v.Notes, CreatedAt: v.CreatedAt,
	}
	if v.VisitID != nil {
		s := v.VisitID.String()
		p.VisitID = &s
	}
	return p
}

func (h *Handler) listVisitVitals(w http.ResponseWriter, r *http.Request) {
	visitID, err := uuid.Parse(chi.URLParam(r, "visitId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_visit", "visitId geçersiz")
		return
	}
	items, err := h.deps.Vitals.ListForVisit(r.Context(), visitID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]vitalsPayload, 0, len(items))
	for i := range items {
		out = append(out, toVitalsPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type addVitalsReq struct {
	SystolicBP    *int     `json:"systolic_bp"`
	DiastolicBP   *int     `json:"diastolic_bp"`
	Pulse         *int     `json:"pulse"`
	TemperatureC  *float64 `json:"temperature_c"`
	Spo2Percent   *int     `json:"spo2_percent"`
	Respiration   *int     `json:"respiration"`
	WeightKg      *float64 `json:"weight_kg"`
	HeightCm      *float64 `json:"height_cm"`
	PainScore     *int     `json:"pain_score"`
	Notes         *string  `json:"notes"`
}

func (h *Handler) addVisitVitals(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	visitID, err := uuid.Parse(chi.URLParam(r, "visitId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_visit", "visitId geçersiz")
		return
	}
	visit, err := h.deps.Visits.GetByID(r.Context(), branchID, visitID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "muayene bulunamadı")
		return
	}

	var req addVitalsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}

	var byUser *uuid.UUID
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			byUser = &uid
		}
	}
	v, err := h.deps.Vitals.Add(r.Context(), repo.CreateVitalsInput{
		OrganizationID:   orgID,
		PatientID:        visit.PatientID,
		VisitID:          &visitID,
		MeasuredByUserID: byUser,
		SystolicBP:       req.SystolicBP,
		DiastolicBP:      req.DiastolicBP,
		Pulse:            req.Pulse,
		TemperatureC:     req.TemperatureC,
		Spo2Percent:      req.Spo2Percent,
		Respiration:      req.Respiration,
		WeightKg:         req.WeightKg,
		HeightCm:         req.HeightCm,
		PainScore:        req.PainScore,
		Notes:            req.Notes,
	})
	if err != nil {
		h.deps.Log.Error("add vitals failed", "err", err)
		writeError(w, http.StatusInternalServerError, "add_failed", "ölçüm eklenemedi")
		return
	}
	writeJSON(w, http.StatusCreated, toVitalsPayload(v))
}
