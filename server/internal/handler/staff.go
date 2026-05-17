package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/pkg/db/repo"
)

type staffPayload struct {
	ID              string     `json:"id"`
	OrganizationID  string     `json:"organization_id"`
	UserID          *string    `json:"user_id,omitempty"`
	EmployeeNo      *string    `json:"employee_no,omitempty"`
	FirstName       string     `json:"first_name"`
	LastName        string     `json:"last_name"`
	Title           *string    `json:"title,omitempty"`
	EmploymentType  string     `json:"employment_type"`
	HireDate        *time.Time `json:"hire_date,omitempty"`
	Phone           *string    `json:"phone,omitempty"`
	Email           *string    `json:"email,omitempty"`
	Notes           *string    `json:"notes,omitempty"`
	IsActive        bool       `json:"is_active"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func toStaffPayload(s *repo.StaffMember) staffPayload {
	out := staffPayload{
		ID:             s.ID.String(),
		OrganizationID: s.OrganizationID.String(),
		EmployeeNo:     s.EmployeeNo,
		FirstName:      s.FirstName,
		LastName:       s.LastName,
		Title:          s.Title,
		EmploymentType: s.EmploymentType,
		HireDate:       s.HireDate,
		Phone:          s.Phone,
		Email:          s.Email,
		Notes:          s.Notes,
		IsActive:       s.IsActive,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
	if s.UserID != nil {
		v := s.UserID.String()
		out.UserID = &v
	}
	return out
}

func (h *Handler) listStaff(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	activeOnly := r.URL.Query().Get("active") == "true"
	items, err := h.deps.Staff.List(r.Context(), orgID, activeOnly)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]staffPayload, 0, len(items))
	for i := range items {
		out = append(out, toStaffPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type createStaffReq struct {
	UserID         *string `json:"user_id"`
	EmployeeNo     *string `json:"employee_no"`
	FirstName      string  `json:"first_name"`
	LastName       string  `json:"last_name"`
	Title          *string `json:"title"`
	EmploymentType string  `json:"employment_type"`
	HireDate       *string `json:"hire_date"`
	Phone          *string `json:"phone"`
	Email          *string `json:"email"`
	Notes          *string `json:"notes"`
}

func (h *Handler) createStaff(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	var req createStaffReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if strings.TrimSpace(req.FirstName) == "" || strings.TrimSpace(req.LastName) == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "ad ve soyad zorunlu")
		return
	}

	in := repo.CreateStaffInput{
		OrganizationID: orgID,
		EmployeeNo:     emptyToNil(req.EmployeeNo),
		FirstName:      strings.TrimSpace(req.FirstName),
		LastName:       strings.TrimSpace(req.LastName),
		Title:          emptyToNil(req.Title),
		EmploymentType: req.EmploymentType,
		Phone:          emptyToNil(req.Phone),
		Email:          emptyToNil(req.Email),
		Notes:          emptyToNil(req.Notes),
	}
	if req.UserID != nil && *req.UserID != "" {
		id, err := uuid.Parse(*req.UserID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_user", "user_id geçersiz")
			return
		}
		in.UserID = &id
	}
	if req.HireDate != nil && *req.HireDate != "" {
		t, err := time.Parse("2006-01-02", *req.HireDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_date", "hire_date YYYY-MM-DD biçiminde olmalı")
			return
		}
		in.HireDate = &t
	}

	staff, err := h.deps.Staff.Create(r.Context(), in)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "employee_no_taken", "bu sicil no zaten kayıtlı")
			return
		}
		h.deps.Log.Error("create staff failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, toStaffPayload(staff))
}

func emptyToNil(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}
