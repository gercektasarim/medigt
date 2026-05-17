package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/middleware"
	"github.com/medigt/medigt/server/internal/service"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

// Intake endpoint — drives the video assistant + walk-in kiosk flow.
//
// Single POST handles the full "hasta gelir, ben kabul ederim" chain:
//
//   1) Find patient by TC (organization-scoped). Insert if missing.
//   2) Pick a doctor in the chosen specialization (first one we find).
//      Calling app may also explicitly pass a doctor_id.
//   3) Create appointment scheduled now+15min, status='arrived' so it
//      lands directly in the poliklinik queue.
//   4) Return enough info to render the slip (MRN + appointment_no +
//      doctor name).
//
// The chain is intentionally lenient on edge cases (e.g. no doctor in
// branch) — those return 409 with a friendly message the assistant can
// read out loud. KVKK note: only structural IDs land in audit_log; the
// assistant logs `intake.create` not the TC.

type intakeReq struct {
	TCKimlikNo       string `json:"tc_kimlik_no"`
	FirstName        string `json:"first_name"`
	LastName         string `json:"last_name"`
	BirthYear        int    `json:"birth_year"`
	Gender           string `json:"gender"`
	Phone            string `json:"phone"`
	Complaint        string `json:"complaint"`
	SpecializationID string `json:"specialization_id"`
	DoctorID         string `json:"doctor_id"`
}

type intakeResp struct {
	PatientID       string  `json:"patient_id"`
	PatientMRN      string  `json:"patient_mrn"`
	PatientCreated  bool    `json:"patient_created"`
	AppointmentID   string  `json:"appointment_id"`
	AppointmentNo   string  `json:"appointment_no"`
	ScheduledAt     string  `json:"scheduled_at"`
	DoctorID        *string `json:"doctor_id,omitempty"`
	DoctorFullName  *string `json:"doctor_full_name,omitempty"`
	SpecializationName *string `json:"specialization_name,omitempty"`
}

// intake is the single endpoint the assistant calls after confirmation.
func (h *Handler) intake(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req intakeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	req.TCKimlikNo = strings.TrimSpace(req.TCKimlikNo)
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)

	if !tcChecksum(req.TCKimlikNo) {
		writeError(w, http.StatusBadRequest, "bad_tc",
			"TC kimlik numarası geçersiz")
		return
	}
	if req.FirstName == "" || req.LastName == "" {
		writeError(w, http.StatusBadRequest, "missing_name",
			"ad ve soyad zorunlu")
		return
	}
	if strings.TrimSpace(req.SpecializationID) == "" && strings.TrimSpace(req.DoctorID) == "" {
		writeError(w, http.StatusBadRequest, "missing_specialization",
			"branş veya doktor seçilmeli")
		return
	}

	// 1) Patient lookup or create.
	patientID, mrn, created, err := h.intakeFindOrCreatePatient(r, orgID, &req)
	if err != nil {
		h.deps.Log.Error("intake: patient resolve failed", "err", err)
		writeError(w, http.StatusInternalServerError, "patient_failed", err.Error())
		return
	}

	// 2) Doctor lookup. Caller can pin a doctor, otherwise we pick the
	//    first one in the specialization.
	doctorID, doctorName, specName, err := h.intakePickDoctor(r, orgID, &req)
	if err != nil {
		writeError(w, http.StatusConflict, "no_doctor", err.Error())
		return
	}

	// 3) Appointment now+15min, status='arrived'.
	apptID, apptNo, scheduledAt, err := h.intakeCreateArrivedAppointment(
		r, orgID, branchID, patientID, doctorID, &req,
	)
	if err != nil {
		h.deps.Log.Error("intake: appointment failed", "err", err)
		writeError(w, http.StatusInternalServerError, "appt_failed", err.Error())
		return
	}

	// 4) Audit. KVKK: only structural ids.
	h.auditAccess(r.Context(), r, "intake.create", "appointment", apptID.String(), map[string]any{
		"patient_id":     patientID.String(),
		"patient_created": created,
		"doctor_id":      strDeref(doctorID),
		"complaint":      req.Complaint != "",
	})

	resp := intakeResp{
		PatientID:      patientID.String(),
		PatientMRN:     mrn,
		PatientCreated: created,
		AppointmentID:  apptID.String(),
		AppointmentNo:  apptNo,
		ScheduledAt:    scheduledAt.Format(time.RFC3339),
		SpecializationName: specName,
	}
	if doctorID != nil {
		s := doctorID.String()
		resp.DoctorID = &s
	}
	if doctorName != "" {
		resp.DoctorFullName = &doctorName
	}
	writeJSON(w, http.StatusCreated, resp)
}

// intakeFindOrCreatePatient returns the existing patient if one matches
// the TC, otherwise inserts a fresh row. Always returns (id, mrn, created).
func (h *Handler) intakeFindOrCreatePatient(
	r *http.Request, orgID uuid.UUID, req *intakeReq,
) (uuid.UUID, string, bool, error) {
	var id uuid.UUID
	var mrn string
	err := h.deps.Pool.QueryRow(r.Context(),
		`SELECT id, mrn FROM patient
		 WHERE organization_id = $1 AND identifier_kind = 'tc' AND identifier_value = $2`,
		orgID, req.TCKimlikNo).Scan(&id, &mrn)
	if err == nil {
		return id, mrn, false, nil
	}
	// Anything other than no-rows is a real error.
	if !isNoRows(err) {
		return uuid.Nil, "", false, err
	}

	// Build a CreatePatientInput and let the service handle MRN + audit.
	in := service.CreatePatientInput{
		OrganizationID:  orgID,
		FirstName:       req.FirstName,
		LastName:        req.LastName,
		Gender:          req.Gender,
		IdentifierKind:  "tc",
		IdentifierValue: req.TCKimlikNo,
		Phone:           req.Phone,
	}
	// BirthYear → BirthDate (1 Jan of that year) so date arithmetic still
	// works. The assistant doesn't collect month/day to keep the dialog
	// short — staff can correct it from the patient detail page later.
	if req.BirthYear > 1900 && req.BirthYear < 2100 {
		t := time.Date(req.BirthYear, 1, 1, 0, 0, 0, 0, time.Local)
		in.BirthDate = &t
	}
	p, err := h.deps.PatientSvc.Create(r.Context(), in)
	if err != nil {
		return uuid.Nil, "", false, err
	}
	return p.ID, p.MRN, true, nil
}

// intakePickDoctor returns the doctor for the appointment + their
// specialization name, picking the first available doctor in the chosen
// specialization when one isn't explicitly named.
func (h *Handler) intakePickDoctor(
	r *http.Request, orgID uuid.UUID, req *intakeReq,
) (*uuid.UUID, string, *string, error) {
	if req.DoctorID != "" {
		id, err := uuid.Parse(req.DoctorID)
		if err != nil {
			return nil, "", nil, errors.New("doctor_id geçersiz")
		}
		fullName, specName, err := h.lookupDoctorName(r, orgID, id, req.SpecializationID)
		if err != nil {
			return nil, "", nil, err
		}
		return &id, fullName, specName, nil
	}

	// Specialization → first accepting doctor.
	specID, err := uuid.Parse(req.SpecializationID)
	if err != nil {
		return nil, "", nil, errors.New("specialization_id geçersiz")
	}
	var doctorID uuid.UUID
	var fullName string
	var specName string
	err = h.deps.Pool.QueryRow(r.Context(),
		`SELECT d.id,
		        TRIM(CONCAT_WS(' ', sm.title, sm.first_name, sm.last_name)),
		        s.name
		 FROM doctor d
		 JOIN staff_member sm ON sm.id = d.staff_member_id
		 JOIN doctor_specialization ds ON ds.doctor_id = d.id
		 JOIN specialization s ON s.id = ds.specialization_id
		 WHERE sm.organization_id = $1
		   AND ds.specialization_id = $2
		   AND d.is_accepting_patients = TRUE
		 ORDER BY ds.is_primary DESC, sm.last_name
		 LIMIT 1`,
		orgID, specID).Scan(&doctorID, &fullName, &specName)
	if err != nil {
		if isNoRows(err) {
			return nil, "", nil, errors.New("bu branşta hasta kabul eden doktor yok")
		}
		return nil, "", nil, err
	}
	return &doctorID, fullName, &specName, nil
}

// lookupDoctorName resolves the doctor's display name + spec name when
// the caller pre-selected a doctor. The spec name comes from either the
// requested specialization (when provided) or the doctor's primary spec.
func (h *Handler) lookupDoctorName(
	r *http.Request, orgID, doctorID uuid.UUID, specHint string,
) (string, *string, error) {
	var fullName, specName string
	var specNamePtr *string
	q := `SELECT TRIM(CONCAT_WS(' ', sm.title, sm.first_name, sm.last_name)),
	            COALESCE((
	              SELECT s.name FROM doctor_specialization ds
	              JOIN specialization s ON s.id = ds.specialization_id
	              WHERE ds.doctor_id = d.id
	              ORDER BY ds.is_primary DESC LIMIT 1
	            ), '')
	      FROM doctor d
	      JOIN staff_member sm ON sm.id = d.staff_member_id
	      WHERE d.id = $1 AND sm.organization_id = $2`
	if err := h.deps.Pool.QueryRow(r.Context(), q, doctorID, orgID).Scan(&fullName, &specName); err != nil {
		if isNoRows(err) {
			return "", nil, errors.New("doktor bulunamadı")
		}
		return "", nil, err
	}
	if specName != "" {
		specNamePtr = &specName
	}
	_ = specHint // currently unused; placeholder if we want to validate doctor∈spec later
	return fullName, specNamePtr, nil
}

// intakeCreateArrivedAppointment inserts an appointment scheduled now+15
// minutes with status='arrived' so the row is immediately visible in the
// poliklinik queue.
func (h *Handler) intakeCreateArrivedAppointment(
	r *http.Request,
	orgID, branchID, patientID uuid.UUID,
	doctorID *uuid.UUID,
	req *intakeReq,
) (uuid.UUID, string, time.Time, error) {
	scheduledAt := time.Now().Add(15 * time.Minute)
	in := repo.CreateAppointmentInput{
		OrganizationID:  orgID,
		BranchID:        branchID,
		PatientID:       patientID,
		DoctorID:        doctorID,
		ScheduledAt:     scheduledAt,
		DurationMinutes: 20,
		Kind:            "outpatient",
	}
	if req.Complaint != "" {
		in.Reason = &req.Complaint
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.CreatedByUserID = &uid
		}
	}
	appt, err := h.deps.Appointments.Create(r.Context(), in)
	if err != nil {
		return uuid.Nil, "", time.Time{}, err
	}
	// Flip to 'arrived' so the appointment shows up in the poliklinik
	// queue board immediately — the kiosk just printed a slip, the
	// patient is in the lobby.
	if err := h.deps.Appointments.UpdateStatus(r.Context(), branchID, appt.ID, "arrived"); err != nil {
		// Non-fatal — appointment still exists; just won't auto-appear
		// in the "arrived" filter. Log + move on.
		h.deps.Log.Warn("intake: arrived flip failed", "err", err, "appt", appt.ID)
	}
	// Appointment table has no separate sequence — use the first 8 hex
	// chars of the UUID as a short slip number. Uppercased for legibility.
	apptNo := strings.ToUpper(appt.ID.String()[:8])
	return appt.ID, apptNo, scheduledAt, nil
}

// ---------- small helpers ----------

func isNoRows(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "no rows")
}

func strDeref(p *uuid.UUID) string {
	if p == nil {
		return ""
	}
	return p.String()
}
