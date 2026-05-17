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

type medulaProvisionPayload struct {
	ID               string         `json:"id"`
	PatientID        string         `json:"patient_id"`
	PatientMRN       string         `json:"patient_mrn,omitempty"`
	PatientFirstName string         `json:"patient_first_name,omitempty"`
	PatientLastName  string         `json:"patient_last_name,omitempty"`
	InstitutionID    *string        `json:"institution_id,omitempty"`
	InstitutionName  *string        `json:"institution_name,omitempty"`
	TakipNo          *string        `json:"takip_no,omitempty"`
	ProvisionType    string         `json:"provision_type"`
	BranchCode       *string        `json:"branch_code,omitempty"`
	Status           string         `json:"status"`
	ResponseCode     *string        `json:"response_code,omitempty"`
	ErrorMessage     *string        `json:"error_message,omitempty"`
	RequestedAt      time.Time      `json:"requested_at"`
	CompletedAt      *time.Time     `json:"completed_at,omitempty"`
	ResponsePayload  map[string]any `json:"response_payload,omitempty"`
}

func toMedulaProvisionPayload(p *repo.MedulaProvision) medulaProvisionPayload {
	pp := medulaProvisionPayload{
		ID: p.ID.String(), PatientID: p.PatientID.String(),
		TakipNo: p.TakipNo, ProvisionType: p.ProvisionType,
		BranchCode: p.BranchCode, Status: p.Status,
		ResponseCode: p.ResponseCode, ErrorMessage: p.ErrorMessage,
		RequestedAt: p.RequestedAt, CompletedAt: p.CompletedAt,
		ResponsePayload: p.ResponsePayload,
	}
	if p.InstitutionID != nil {
		s := p.InstitutionID.String()
		pp.InstitutionID = &s
	}
	return pp
}

func toJoinedMedulaPayload(w *repo.MedulaProvisionWithJoins) medulaProvisionPayload {
	p := toMedulaProvisionPayload(&w.Provision)
	p.PatientMRN = w.PatientMRN
	p.PatientFirstName = w.PatientFirstName
	p.PatientLastName = w.PatientLastName
	p.InstitutionName = w.InstitutionName
	return p
}

func (h *Handler) listMedulaProvisions(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	items, err := h.deps.Medula.List(r.Context(), branchID, r.URL.Query().Get("status"), 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]medulaProvisionPayload, 0, len(items))
	for i := range items {
		out = append(out, toJoinedMedulaPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) getMedulaProvision(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	p, err := h.deps.Medula.GetByID(r.Context(), branchID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "provizyon bulunamadı")
		return
	}
	writeJSON(w, http.StatusOK, toMedulaProvisionPayload(p))
}

type createProvisionReq struct {
	PatientID     string `json:"patient_id"`
	InstitutionID string `json:"institution_id"`
	ProvisionType string `json:"provision_type"`
	BranchCode    string `json:"branch_code"`
}

var validProvisionTypes = map[string]bool{"normal": true, "acil": true, "yatis": true}

func (h *Handler) createMedulaProvision(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req createProvisionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_patient", "patient_id zorunlu")
		return
	}
	provType := req.ProvisionType
	if provType == "" {
		provType = "normal"
	}
	if !validProvisionTypes[provType] {
		writeError(w, http.StatusBadRequest, "bad_type", "geçersiz provizyon türü")
		return
	}
	in := repo.CreateProvisionInput{
		OrganizationID: orgID, BranchID: branchID, PatientID: patientID,
		ProvisionType: provType,
		BranchCode:    emptyToNil(&req.BranchCode),
	}
	if req.InstitutionID != "" {
		if id, err := uuid.Parse(req.InstitutionID); err == nil {
			in.InstitutionID = &id
		}
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.RequestedByUserID = &uid
		}
	}
	p, err := h.deps.Medula.Create(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("create medula provision failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	h.auditAccess(r.Context(), r, "medula.provision.request", "medula_provision", p.ID.String(), map[string]any{
		"patient_id":     patientID.String(),
		"provision_type": provType,
		"branch_code":    req.BranchCode,
	})
	// 202 Accepted — worker tarafından SGK'ya gönderilecek.
	writeJSON(w, http.StatusAccepted, toMedulaProvisionPayload(p))
}
