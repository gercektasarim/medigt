package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/pkg/db/repo"
)

type doctorPayload struct {
	ID                    string                  `json:"id"`
	StaffMemberID         string                  `json:"staff_member_id"`
	Staff                 staffPayload            `json:"staff"`
	DiplomaNo             *string                 `json:"diploma_no,omitempty"`
	MedulaDoctorCode      *string                 `json:"medula_doctor_code,omitempty"`
	LicenseExpiresAt      *time.Time              `json:"license_expires_at,omitempty"`
	IsAcceptingPatients   bool                    `json:"is_accepting_patients"`
	Specializations       []specializationPayload `json:"specializations"`
	CreatedAt             time.Time               `json:"created_at"`
	UpdatedAt             time.Time               `json:"updated_at"`
}

func toDoctorPayload(dp *repo.DoctorWithProfile) doctorPayload {
	specs := make([]specializationPayload, 0, len(dp.Specializations))
	for i := range dp.Specializations {
		specs = append(specs, toSpecPayload(&dp.Specializations[i]))
	}
	return doctorPayload{
		ID:                  dp.Doctor.ID.String(),
		StaffMemberID:       dp.Doctor.StaffMemberID.String(),
		Staff:               toStaffPayload(&dp.Staff),
		DiplomaNo:           dp.Doctor.DiplomaNo,
		MedulaDoctorCode:    dp.Doctor.MedulaDoctorCode,
		LicenseExpiresAt:    dp.Doctor.LicenseExpiresAt,
		IsAcceptingPatients: dp.Doctor.IsAcceptingPatients,
		Specializations:     specs,
		CreatedAt:           dp.Doctor.CreatedAt,
		UpdatedAt:           dp.Doctor.UpdatedAt,
	}
}

func (h *Handler) listDoctors(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	items, err := h.deps.Doctors.ListWithProfiles(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]doctorPayload, 0, len(items))
	for i := range items {
		out = append(out, toDoctorPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type createDoctorReq struct {
	// Doctor can be created two ways: against an existing staff_member, or by
	// creating both at once. Embedded staff fields are used when StaffMemberID
	// is empty.
	StaffMemberID          string          `json:"staff_member_id,omitempty"`
	StaffData              *createStaffReq `json:"staff,omitempty"`
	DiplomaNo              *string         `json:"diploma_no"`
	MedulaDoctorCode       *string         `json:"medula_doctor_code"`
	IsAcceptingPatients    bool            `json:"is_accepting_patients"`
	SpecializationIDs      []string        `json:"specialization_ids"`
	PrimarySpecializationID *string        `json:"primary_specialization_id"`
}

func (h *Handler) createDoctor(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}

	var req createDoctorReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}

	var staffID uuid.UUID
	if req.StaffMemberID != "" {
		id, err := uuid.Parse(req.StaffMemberID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_staff_id", "staff_member_id geçersiz")
			return
		}
		staffID = id
	} else if req.StaffData != nil {
		// Create the staff member first.
		if req.StaffData.FirstName == "" || req.StaffData.LastName == "" {
			writeError(w, http.StatusBadRequest, "missing_fields", "doktor için staff.first_name + last_name zorunlu")
			return
		}
		in := repo.CreateStaffInput{
			OrganizationID: orgID,
			EmployeeNo:     emptyToNil(req.StaffData.EmployeeNo),
			FirstName:      req.StaffData.FirstName,
			LastName:       req.StaffData.LastName,
			Title:          emptyToNil(req.StaffData.Title),
			EmploymentType: req.StaffData.EmploymentType,
			Phone:          emptyToNil(req.StaffData.Phone),
			Email:          emptyToNil(req.StaffData.Email),
			Notes:          emptyToNil(req.StaffData.Notes),
		}
		s, err := h.deps.Staff.Create(r.Context(), in)
		if err != nil {
			h.deps.Log.Error("create staff for doctor failed", "err", err)
			writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
			return
		}
		staffID = s.ID
	} else {
		writeError(w, http.StatusBadRequest, "missing_fields", "staff_member_id veya staff alanı gerekli")
		return
	}

	specIDs := []uuid.UUID{}
	for _, sid := range req.SpecializationIDs {
		id, err := uuid.Parse(sid)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_specialization", "branş id geçersiz")
			return
		}
		specIDs = append(specIDs, id)
	}
	var primary *uuid.UUID
	if req.PrimarySpecializationID != nil && *req.PrimarySpecializationID != "" {
		id, err := uuid.Parse(*req.PrimarySpecializationID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_specialization", "primary branş id geçersiz")
			return
		}
		primary = &id
	}

	if _, err := h.deps.Doctors.Create(r.Context(), repo.CreateDoctorInput{
		StaffMemberID:           staffID,
		DiplomaNo:               emptyToNil(req.DiplomaNo),
		MedulaDoctorCode:        emptyToNil(req.MedulaDoctorCode),
		IsAcceptingPatients:     req.IsAcceptingPatients,
		SpecializationIDs:       specIDs,
		PrimarySpecializationID: primary,
	}); err != nil {
		h.deps.Log.Error("create doctor failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}

	// Re-fetch to return the joined view.
	items, err := h.deps.Doctors.ListWithProfiles(r.Context(), orgID)
	if err != nil {
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
		return
	}
	for i := range items {
		if items[i].Doctor.StaffMemberID == staffID {
			writeJSON(w, http.StatusCreated, toDoctorPayload(&items[i]))
			return
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}
