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

// inpatientBoardRow is a flat payload the nursing dashboard renders directly —
// admission + patient + ward + bed + most-recent vital sample in one shot.
type inpatientBoardRow struct {
	AdmissionID         string     `json:"admission_id"`
	AdmissionNo         string     `json:"admission_no"`
	PatientID           string     `json:"patient_id"`
	PatientFirstName    string     `json:"patient_first_name"`
	PatientLastName     string     `json:"patient_last_name"`
	PatientMRN          string     `json:"patient_mrn"`
	WardID              string     `json:"ward_id"`
	WardName            string     `json:"ward_name"`
	WardKind            string     `json:"ward_kind"`
	BedCode             *string    `json:"bed_code,omitempty"`
	Kind                string     `json:"kind"`
	ChiefComplaint      *string    `json:"chief_complaint,omitempty"`
	AdmissionDiagnosis  *string    `json:"admission_diagnosis,omitempty"`
	AdmittedAt          time.Time  `json:"admitted_at"`

	VitalsMeasuredAt *time.Time `json:"vitals_measured_at,omitempty"`
	SystolicBP       *int       `json:"systolic_bp,omitempty"`
	DiastolicBP      *int       `json:"diastolic_bp,omitempty"`
	Pulse            *int       `json:"pulse,omitempty"`
	TemperatureC     *float64   `json:"temperature_c,omitempty"`
	Spo2Percent      *int       `json:"spo2_percent,omitempty"`
	Respiration      *int       `json:"respiration,omitempty"`
	PainScore        *int       `json:"pain_score,omitempty"`
}

func toBoardRow(j *repo.InpatientBoardRow) inpatientBoardRow {
	return inpatientBoardRow{
		AdmissionID: j.Admission.ID.String(), AdmissionNo: j.Admission.AdmissionNo,
		PatientID: j.Admission.PatientID.String(),
		PatientFirstName: j.PatientFirstName, PatientLastName: j.PatientLastName,
		PatientMRN: j.PatientMRN,
		WardID: j.Admission.WardID.String(),
		WardName: j.WardName, WardKind: j.WardKind, BedCode: j.BedCode,
		Kind: j.Admission.Kind,
		ChiefComplaint: j.Admission.ChiefComplaint,
		AdmissionDiagnosis: j.Admission.AdmissionDiagnosis,
		AdmittedAt: j.Admission.AdmittedAt,
		VitalsMeasuredAt: j.VitalsMeasuredAt,
		SystolicBP: j.SystolicBP, DiastolicBP: j.DiastolicBP,
		Pulse: j.Pulse, TemperatureC: j.TemperatureC,
		Spo2Percent: j.Spo2Percent, Respiration: j.Respiration,
		PainScore: j.PainScore,
	}
}

func (h *Handler) getInpatientBoard(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var wardID *uuid.UUID
	if s := r.URL.Query().Get("ward_id"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			wardID = &id
		}
	}
	items, err := h.deps.Admissions.ListInpatientBoard(r.Context(), branchID, wardID)
	if err != nil {
		h.deps.Log.Error("inpatient board failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]inpatientBoardRow, 0, len(items))
	for i := range items {
		out = append(out, toBoardRow(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

// addPatientVitals lets the nursing dashboard record a vital sample directly
// against a patient (without a visit_id). Reuses the existing VitalSignsRepo.
type addPatientVitalsReq struct {
	SystolicBP   *int     `json:"systolic_bp"`
	DiastolicBP  *int     `json:"diastolic_bp"`
	Pulse        *int     `json:"pulse"`
	TemperatureC *float64 `json:"temperature_c"`
	Spo2Percent  *int     `json:"spo2_percent"`
	Respiration  *int     `json:"respiration"`
	WeightKg     *float64 `json:"weight_kg"`
	HeightCm     *float64 `json:"height_cm"`
	PainScore    *int     `json:"pain_score"`
	Notes        *string  `json:"notes"`
}

func (h *Handler) addPatientVitals(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	patientID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req addPatientVitalsReq
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
		PatientID:        patientID,
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
		Notes:            emptyToNil(req.Notes),
	})
	if err != nil {
		h.deps.Log.Error("add patient vitals failed", "err", err)
		writeError(w, http.StatusInternalServerError, "add_failed", "ölçüm eklenemedi")
		return
	}
	writeJSON(w, http.StatusCreated, toVitalsPayload(v))
}
