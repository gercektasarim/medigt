package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/middleware"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

// ============================================================================
//  Provision: cancel + close-takip
// ============================================================================

type cancelProvisionReq struct {
	Reason string `json:"reason"`
}

func (h *Handler) cancelMedulaProvision(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req cancelProvisionReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.deps.Medula.QueueProvisionCancel(r.Context(), branchID, id, strings.TrimSpace(req.Reason)); err != nil {
		writeError(w, http.StatusConflict, "cancel_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

func (h *Handler) closeMedulaTakip(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	if err := h.deps.Medula.QueueTakipClose(r.Context(), branchID, id); err != nil {
		writeError(w, http.StatusConflict, "close_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

// ============================================================================
//  Invoice submission
// ============================================================================

type invoiceSubmissionPayload struct {
	ID                 string         `json:"id"`
	InvoiceID          string         `json:"invoice_id"`
	InvoiceNo          string         `json:"invoice_no,omitempty"`
	Total              float64        `json:"total,omitempty"`
	PatientFirstName   string         `json:"patient_first_name,omitempty"`
	PatientLastName    string         `json:"patient_last_name,omitempty"`
	PatientMRN         string         `json:"patient_mrn,omitempty"`
	ProvisionID        *string        `json:"provision_id,omitempty"`
	BatchNo            *string        `json:"batch_no,omitempty"`
	SGKInvoiceNo       *string        `json:"sgk_invoice_no,omitempty"`
	Status             string         `json:"status"`
	ResponseCode       *string        `json:"response_code,omitempty"`
	ErrorMessage       *string        `json:"error_message,omitempty"`
	CancelledAt        *time.Time     `json:"cancelled_at,omitempty"`
	CancellationReason *string        `json:"cancellation_reason,omitempty"`
	ResponsePayload    map[string]any `json:"response_payload,omitempty"`
	RequestedAt        time.Time      `json:"requested_at"`
	CompletedAt        *time.Time     `json:"completed_at,omitempty"`
}

func toInvoiceSubmissionPayload(w *repo.InvoiceSubmissionWithJoins) invoiceSubmissionPayload {
	s := &w.Submission
	p := invoiceSubmissionPayload{
		ID: s.ID.String(), InvoiceID: s.InvoiceID.String(),
		InvoiceNo: w.InvoiceNo, Total: w.Total,
		PatientFirstName: w.PatientFirstName, PatientLastName: w.PatientLastName, PatientMRN: w.PatientMRN,
		BatchNo: s.BatchNo, SGKInvoiceNo: s.SGKInvoiceNo, Status: s.Status,
		ResponseCode: s.ResponseCode, ErrorMessage: s.ErrorMessage,
		CancelledAt: s.CancelledAt, CancellationReason: s.CancellationReason,
		ResponsePayload: s.ResponsePayload,
		RequestedAt: s.RequestedAt, CompletedAt: s.CompletedAt,
	}
	if s.ProvisionID != nil {
		v := s.ProvisionID.String()
		p.ProvisionID = &v
	}
	return p
}

type createSubmissionReq struct {
	InvoiceID   string `json:"invoice_id"`
	ProvisionID string `json:"provision_id"`
}

func (h *Handler) listInvoiceSubmissions(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	items, err := h.deps.Medula.ListInvoiceSubmissions(r.Context(), branchID, r.URL.Query().Get("status"), 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]invoiceSubmissionPayload, 0, len(items))
	for i := range items {
		out = append(out, toInvoiceSubmissionPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createInvoiceSubmission(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req createSubmissionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	invoiceID, err := uuid.Parse(req.InvoiceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_invoice", "invoice_id zorunlu")
		return
	}
	in := repo.CreateInvoiceSubmissionInput{
		OrganizationID: orgID, BranchID: branchID, InvoiceID: invoiceID,
		ProvisionID: parseUUIDPtr(req.ProvisionID),
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.RequestedByUserID = &uid
		}
	}
	sub, err := h.deps.Medula.QueueInvoiceSubmission(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("queue invoice submission failed", "err", err)
		writeError(w, http.StatusConflict, "submit_failed", err.Error())
		return
	}
	h.auditAccess(r.Context(), r, "medula.invoice.submit", "medula_invoice_submission", sub.ID.String(), map[string]any{
		"invoice_id":   invoiceID.String(),
		"provision_id": req.ProvisionID,
	})
	w_ := repo.InvoiceSubmissionWithJoins{Submission: *sub}
	writeJSON(w, http.StatusAccepted, toInvoiceSubmissionPayload(&w_))
}

func (h *Handler) cancelInvoiceSubmission(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req cancelProvisionReq // re-use {reason}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.deps.Medula.QueueInvoiceCancel(r.Context(), branchID, id, strings.TrimSpace(req.Reason)); err != nil {
		writeError(w, http.StatusConflict, "cancel_failed", err.Error())
		return
	}
	h.auditAccess(r.Context(), r, "medula.invoice.cancel", "medula_invoice_submission", id.String(), map[string]any{
		"reason": req.Reason,
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

// ============================================================================
//  Referral
// ============================================================================

type referralPayload struct {
	ID                 string         `json:"id"`
	PatientID          string         `json:"patient_id"`
	PatientMRN         string         `json:"patient_mrn,omitempty"`
	PatientFirstName   string         `json:"patient_first_name,omitempty"`
	PatientLastName    string         `json:"patient_last_name,omitempty"`
	ReferringDoctorID  *string        `json:"referring_doctor_id,omitempty"`
	TargetProviderCode string         `json:"target_provider_code"`
	TargetProviderName *string        `json:"target_provider_name,omitempty"`
	TargetBranchCode   *string        `json:"target_branch_code,omitempty"`
	Reason             string         `json:"reason"`
	DiagnosisICD10     *string        `json:"diagnosis_icd10,omitempty"`
	ReferralType       string         `json:"referral_type"`
	Status             string         `json:"status"`
	SevkNo             *string        `json:"sevk_no,omitempty"`
	ResponseCode       *string        `json:"response_code,omitempty"`
	ErrorMessage       *string        `json:"error_message,omitempty"`
	ResponsePayload    map[string]any `json:"response_payload,omitempty"`
	RequestedAt        time.Time      `json:"requested_at"`
	CompletedAt        *time.Time     `json:"completed_at,omitempty"`
	CancelledAt        *time.Time     `json:"cancelled_at,omitempty"`
}

func toReferralPayload(w *repo.ReferralWithJoins) referralPayload {
	r := &w.Referral
	p := referralPayload{
		ID: r.ID.String(), PatientID: r.PatientID.String(),
		PatientMRN: w.PatientMRN, PatientFirstName: w.PatientFirstName, PatientLastName: w.PatientLastName,
		TargetProviderCode: r.TargetProviderCode, TargetProviderName: r.TargetProviderName,
		TargetBranchCode: r.TargetBranchCode,
		Reason: r.Reason, DiagnosisICD10: r.DiagnosisICD10,
		ReferralType: r.ReferralType, Status: r.Status, SevkNo: r.SevkNo,
		ResponseCode: r.ResponseCode, ErrorMessage: r.ErrorMessage,
		ResponsePayload: r.ResponsePayload,
		RequestedAt: r.RequestedAt, CompletedAt: r.CompletedAt, CancelledAt: r.CancelledAt,
	}
	if r.ReferringDoctorID != nil {
		s := r.ReferringDoctorID.String()
		p.ReferringDoctorID = &s
	}
	return p
}

type createReferralReq struct {
	PatientID          string `json:"patient_id"`
	ReferringDoctorID  string `json:"referring_doctor_id"`
	TargetProviderCode string `json:"target_provider_code"`
	TargetProviderName string `json:"target_provider_name"`
	TargetBranchCode   string `json:"target_branch_code"`
	Reason             string `json:"reason"`
	DiagnosisICD10     string `json:"diagnosis_icd10"`
	ReferralType       string `json:"referral_type"`
}

var validReferralTypes = map[string]bool{"normal": true, "acil": true, "kontrol": true}

func (h *Handler) listMedulaReferrals(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	items, err := h.deps.Medula.ListReferrals(r.Context(), branchID, r.URL.Query().Get("status"), 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]referralPayload, 0, len(items))
	for i := range items {
		out = append(out, toReferralPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createMedulaReferral(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req createReferralReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_patient", "patient_id zorunlu")
		return
	}
	target := strings.TrimSpace(req.TargetProviderCode)
	reason := strings.TrimSpace(req.Reason)
	if target == "" || reason == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "target_provider_code ve reason zorunlu")
		return
	}
	refType := req.ReferralType
	if refType == "" {
		refType = "normal"
	}
	if !validReferralTypes[refType] {
		writeError(w, http.StatusBadRequest, "bad_type", "geçersiz sevk türü")
		return
	}
	in := repo.CreateReferralInput{
		OrganizationID: orgID, BranchID: branchID, PatientID: patientID,
		ReferringDoctorID:  parseUUIDPtr(req.ReferringDoctorID),
		TargetProviderCode: target,
		TargetProviderName: emptyToNil(&req.TargetProviderName),
		TargetBranchCode:   emptyToNil(&req.TargetBranchCode),
		Reason:             reason,
		DiagnosisICD10:     emptyToNil(&req.DiagnosisICD10),
		ReferralType:       refType,
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.RequestedByUserID = &uid
		}
	}
	ref, err := h.deps.Medula.QueueReferral(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("queue referral failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	w_ := repo.ReferralWithJoins{Referral: *ref}
	writeJSON(w, http.StatusAccepted, toReferralPayload(&w_))
}

// ============================================================================
//  e-Rapor
// ============================================================================

type eraportPayload struct {
	ID               string         `json:"id"`
	PatientID        string         `json:"patient_id"`
	PatientMRN       string         `json:"patient_mrn,omitempty"`
	PatientFirstName string         `json:"patient_first_name,omitempty"`
	PatientLastName  string         `json:"patient_last_name,omitempty"`
	DoctorID         *string        `json:"doctor_id,omitempty"`
	Kind             string         `json:"kind"`
	DiagnosesICD10   []string       `json:"diagnoses_icd10"`
	DrugCodes        []string       `json:"drug_codes"`
	ValidFrom        string         `json:"valid_from"`
	ValidTo          *string        `json:"valid_to,omitempty"`
	ReportText       *string        `json:"report_text,omitempty"`
	Status           string         `json:"status"`
	EraportNo        *string        `json:"eraport_no,omitempty"`
	ResponseCode     *string        `json:"response_code,omitempty"`
	ErrorMessage     *string        `json:"error_message,omitempty"`
	ResponsePayload  map[string]any `json:"response_payload,omitempty"`
	RequestedAt      time.Time      `json:"requested_at"`
	CompletedAt      *time.Time     `json:"completed_at,omitempty"`
	CancelledAt      *time.Time     `json:"cancelled_at,omitempty"`
}

func toEraportPayload(w *repo.EraportWithJoins) eraportPayload {
	e := &w.Eraport
	p := eraportPayload{
		ID: e.ID.String(), PatientID: e.PatientID.String(),
		PatientMRN: w.PatientMRN, PatientFirstName: w.PatientFirstName, PatientLastName: w.PatientLastName,
		Kind: e.Kind, DiagnosesICD10: e.DiagnosesICD10, DrugCodes: e.DrugCodes,
		ValidFrom: e.ValidFrom.Format("2006-01-02"),
		ReportText: e.ReportText, Status: e.Status, EraportNo: e.EraportNo,
		ResponseCode: e.ResponseCode, ErrorMessage: e.ErrorMessage,
		ResponsePayload: e.ResponsePayload,
		RequestedAt: e.RequestedAt, CompletedAt: e.CompletedAt, CancelledAt: e.CancelledAt,
	}
	if e.DoctorID != nil {
		s := e.DoctorID.String()
		p.DoctorID = &s
	}
	if e.ValidTo != nil {
		v := e.ValidTo.Format("2006-01-02")
		p.ValidTo = &v
	}
	if p.DiagnosesICD10 == nil {
		p.DiagnosesICD10 = []string{}
	}
	if p.DrugCodes == nil {
		p.DrugCodes = []string{}
	}
	return p
}

type createEraportReq struct {
	PatientID      string   `json:"patient_id"`
	DoctorID       string   `json:"doctor_id"`
	Kind           string   `json:"kind"`
	DiagnosesICD10 []string `json:"diagnoses_icd10"`
	DrugCodes      []string `json:"drug_codes"`
	ValidFrom      string   `json:"valid_from"`
	ValidTo        string   `json:"valid_to"`
	ReportText     string   `json:"report_text"`
}

var validEraportKinds = map[string]bool{
	"chronic_drug": true, "inpatient": true, "work_incapacity": true, "special_procedure": true,
}

func (h *Handler) listMedulaEraports(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	items, err := h.deps.Medula.ListEraports(r.Context(), branchID, r.URL.Query().Get("status"), 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]eraportPayload, 0, len(items))
	for i := range items {
		out = append(out, toEraportPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createMedulaEraport(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req createEraportReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_patient", "patient_id zorunlu")
		return
	}
	kind := req.Kind
	if kind == "" {
		kind = "chronic_drug"
	}
	if !validEraportKinds[kind] {
		writeError(w, http.StatusBadRequest, "bad_kind", "geçersiz rapor türü")
		return
	}
	if req.ValidFrom == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "valid_from zorunlu (YYYY-MM-DD)")
		return
	}
	validFrom, err := time.Parse("2006-01-02", req.ValidFrom)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_date", "valid_from YYYY-MM-DD olmalı")
		return
	}
	var validTo *time.Time
	if req.ValidTo != "" {
		t, err := time.Parse("2006-01-02", req.ValidTo)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_date", "valid_to YYYY-MM-DD olmalı")
			return
		}
		validTo = &t
	}
	in := repo.CreateEraportInput{
		OrganizationID: orgID, BranchID: branchID, PatientID: patientID,
		DoctorID:       parseUUIDPtr(req.DoctorID),
		Kind:           kind,
		DiagnosesICD10: req.DiagnosesICD10,
		DrugCodes:      req.DrugCodes,
		ValidFrom:      validFrom,
		ValidTo:        validTo,
		ReportText:     emptyToNil(&req.ReportText),
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.RequestedByUserID = &uid
		}
	}
	ep, err := h.deps.Medula.QueueEraport(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("queue eraport failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	w_ := repo.EraportWithJoins{Eraport: *ep}
	writeJSON(w, http.StatusAccepted, toEraportPayload(&w_))
}

func (h *Handler) cancelMedulaEraport(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req cancelProvisionReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.deps.Medula.QueueEraportCancel(r.Context(), branchID, id, strings.TrimSpace(req.Reason)); err != nil {
		writeError(w, http.StatusConflict, "cancel_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

// ============================================================================
//  Sync queries (SGK okuma operasyonları)
// ============================================================================

func (h *Handler) queryTakip(w http.ResponseWriter, r *http.Request) {
	takipNo := strings.TrimSpace(chi.URLParam(r, "takipNo"))
	if takipNo == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "takipNo zorunlu")
		return
	}
	res, err := h.deps.MedulaClient.QueryTakip(r.Context(), takipNo)
	if err != nil {
		writeError(w, http.StatusBadGateway, "soap_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) queryEraport(w http.ResponseWriter, r *http.Request) {
	no := strings.TrimSpace(chi.URLParam(r, "eraportNo"))
	if no == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "eraportNo zorunlu")
		return
	}
	res, err := h.deps.MedulaClient.QueryEraport(r.Context(), no)
	if err != nil {
		writeError(w, http.StatusBadGateway, "soap_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) queryDoctor(w http.ResponseWriter, r *http.Request) {
	tc := strings.TrimSpace(chi.URLParam(r, "tc"))
	if tc == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "tc zorunlu")
		return
	}
	res, err := h.deps.MedulaClient.QueryDoctor(r.Context(), tc)
	if err != nil {
		writeError(w, http.StatusBadGateway, "soap_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) queryBranches(w http.ResponseWriter, r *http.Request) {
	res, err := h.deps.MedulaClient.QueryBranches(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "soap_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) queryTreatmentTypes(w http.ResponseWriter, r *http.Request) {
	res, err := h.deps.MedulaClient.QueryTreatmentTypes(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "soap_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) queryDrugPayment(w http.ResponseWriter, r *http.Request) {
	barcode := strings.TrimSpace(chi.URLParam(r, "barcode"))
	if barcode == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "barcode zorunlu")
		return
	}
	res, err := h.deps.MedulaClient.QueryDrugPayment(r.Context(), barcode)
	if err != nil {
		writeError(w, http.StatusBadGateway, "soap_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}
