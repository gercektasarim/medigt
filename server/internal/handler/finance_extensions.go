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
	"github.com/medigt/medigt/server/internal/service"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

// ============================================================================
//  Patient cari hesap (avans)
// ============================================================================

type cariSummaryPayload struct {
	PatientID string                       `json:"patient_id"`
	Balance   float64                      `json:"balance"`
	Entries   []patientAccountEntryPayload `json:"entries"`
}

type patientAccountEntryPayload struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Amount      float64   `json:"amount"`
	Direction   int       `json:"direction"`
	Signed      float64   `json:"signed_amount"`
	PaymentID   *string   `json:"payment_id,omitempty"`
	InvoiceID   *string   `json:"invoice_id,omitempty"`
	Notes       *string   `json:"notes,omitempty"`
	PerformedAt time.Time `json:"performed_at"`
}

func (h *Handler) getPatientAccount(w http.ResponseWriter, r *http.Request) {
	patientID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	balance, err := h.deps.Cari.BalanceFor(r.Context(), patientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	entries, err := h.deps.Cari.EntriesFor(r.Context(), patientID, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "kayıtlar alınamadı")
		return
	}
	out := cariSummaryPayload{PatientID: patientID.String(), Balance: balance, Entries: make([]patientAccountEntryPayload, 0, len(entries))}
	for _, e := range entries {
		p := patientAccountEntryPayload{
			ID: e.ID.String(), Kind: e.Kind, Amount: e.Amount, Direction: e.Direction,
			Signed: float64(e.Direction) * e.Amount,
			Notes: e.Notes, PerformedAt: e.PerformedAt,
		}
		if e.PaymentID != nil {
			s := e.PaymentID.String()
			p.PaymentID = &s
		}
		if e.InvoiceID != nil {
			s := e.InvoiceID.String()
			p.InvoiceID = &s
		}
		out.Entries = append(out.Entries, p)
	}
	writeJSON(w, http.StatusOK, out)
}

type receiveAdvanceReq struct {
	Amount         float64 `json:"amount"`
	Method         string  `json:"method"`
	CashRegisterID string  `json:"cash_register_id"`
	Notes          string  `json:"notes"`
}

func (h *Handler) receiveAdvance(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	patientID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req receiveAdvanceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if !validPaymentMethods[req.Method] {
		writeError(w, http.StatusBadRequest, "bad_method", "geçersiz ödeme yöntemi")
		return
	}
	in := service.ReceiveAdvanceInput{
		OrganizationID: orgID, BranchID: branchID, PatientID: patientID,
		Amount: req.Amount, Method: req.Method,
		CashRegisterID: parseUUIDPtr(req.CashRegisterID),
		Notes:          emptyToNil(&req.Notes),
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.PerformedByUserID = &uid
		}
	}
	entryID, err := h.deps.FinanceExt.ReceiveAdvance(r.Context(), in)
	if err != nil {
		if errors.Is(err, service.ErrCashRegisterMissing) {
			writeError(w, http.StatusConflict, "kasa_required", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "advance_failed", err.Error())
		return
	}
	h.auditAccess(r.Context(), r, "cari.advance.receive", "patient_account_entry", entryID.String(), map[string]any{
		"patient_id": patientID.String(),
		"amount":     req.Amount,
		"method":     req.Method,
	})
	writeJSON(w, http.StatusCreated, map[string]any{"entry_id": entryID.String()})
}

type applyAdvanceReq struct {
	PatientID string  `json:"patient_id"`
	InvoiceID string  `json:"invoice_id"`
	Amount    float64 `json:"amount"`
	Notes     string  `json:"notes"`
}

func (h *Handler) applyAdvance(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req applyAdvanceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_patient", "patient_id zorunlu")
		return
	}
	invoiceID, err := uuid.Parse(req.InvoiceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_invoice", "invoice_id zorunlu")
		return
	}
	in := service.ApplyAdvanceInput{
		OrganizationID: orgID, BranchID: branchID, PatientID: patientID,
		InvoiceID: invoiceID, Amount: req.Amount,
		Notes: emptyToNil(&req.Notes),
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.PerformedByUserID = &uid
		}
	}
	paymentID, err := h.deps.FinanceExt.ApplyAdvance(r.Context(), in)
	if err != nil {
		if errors.Is(err, service.ErrInsufficientAdvance) {
			writeError(w, http.StatusConflict, "insufficient_advance", err.Error())
			return
		}
		if errors.Is(err, service.ErrInvoiceNotPayable) || errors.Is(err, service.ErrOverAllocate) {
			writeError(w, http.StatusConflict, "invalid_invoice", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "apply_failed", err.Error())
		return
	}
	h.auditAccess(r.Context(), r, "cari.advance.apply", "payment", paymentID.String(), map[string]any{
		"patient_id": patientID.String(),
		"invoice_id": invoiceID.String(),
		"amount":     req.Amount,
	})
	writeJSON(w, http.StatusCreated, map[string]any{"payment_id": paymentID.String()})
}

type refundAdvanceReq struct {
	Amount         float64 `json:"amount"`
	CashRegisterID string  `json:"cash_register_id"`
	Reason         string  `json:"reason"`
}

func (h *Handler) refundAdvance(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	patientID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req refundAdvanceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	in := service.RefundAdvanceInput{
		OrganizationID: orgID, BranchID: branchID, PatientID: patientID,
		Amount: req.Amount,
		CashRegisterID: parseUUIDPtr(req.CashRegisterID),
		Reason:         emptyToNil(&req.Reason),
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.PerformedByUserID = &uid
		}
	}
	entryID, err := h.deps.FinanceExt.RefundAdvance(r.Context(), in)
	if err != nil {
		if errors.Is(err, service.ErrInsufficientAdvance) {
			writeError(w, http.StatusConflict, "insufficient_advance", err.Error())
			return
		}
		if errors.Is(err, service.ErrCashRegisterMissing) {
			writeError(w, http.StatusConflict, "kasa_required", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "refund_failed", err.Error())
		return
	}
	h.auditAccess(r.Context(), r, "cari.advance.refund", "patient_account_entry", entryID.String(), map[string]any{
		"patient_id": patientID.String(),
		"amount":     req.Amount,
		"reason":     req.Reason,
	})
	writeJSON(w, http.StatusCreated, map[string]any{"entry_id": entryID.String()})
}

// ============================================================================
//  Refund (fatura iadesi)
// ============================================================================

type refundPayload struct {
	ID                string     `json:"id"`
	RefundNo          string     `json:"refund_no"`
	PatientID         string     `json:"patient_id"`
	PatientMRN        string     `json:"patient_mrn,omitempty"`
	PatientFirstName  string     `json:"patient_first_name,omitempty"`
	PatientLastName   string     `json:"patient_last_name,omitempty"`
	InvoiceID         *string    `json:"invoice_id,omitempty"`
	InvoiceNo         *string    `json:"invoice_no,omitempty"`
	PaymentID         *string    `json:"payment_id,omitempty"`
	Amount            float64    `json:"amount"`
	Method            string     `json:"method"`
	CashRegisterID    *string    `json:"cash_register_id,omitempty"`
	ToAdvance         bool       `json:"to_advance"`
	Reason            *string    `json:"reason,omitempty"`
	PerformedAt       time.Time  `json:"performed_at"`
}

func toRefundPayload(r *repo.Refund, invoiceNo *string, patientMRN, patientFn, patientLn string) refundPayload {
	p := refundPayload{
		ID: r.ID.String(), RefundNo: r.RefundNo, PatientID: r.PatientID.String(),
		PatientMRN: patientMRN, PatientFirstName: patientFn, PatientLastName: patientLn,
		Amount: r.Amount, Method: r.Method, ToAdvance: r.ToAdvance, Reason: r.Reason,
		PerformedAt: r.PerformedAt, InvoiceNo: invoiceNo,
	}
	if r.InvoiceID != nil {
		s := r.InvoiceID.String()
		p.InvoiceID = &s
	}
	if r.PaymentID != nil {
		s := r.PaymentID.String()
		p.PaymentID = &s
	}
	if r.CashRegisterID != nil {
		s := r.CashRegisterID.String()
		p.CashRegisterID = &s
	}
	return p
}

func (h *Handler) listRefunds(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	items, err := h.deps.Refunds.ListForBranch(r.Context(), branchID, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]refundPayload, 0, len(items))
	for _, w_ := range items {
		out = append(out, toRefundPayload(&w_.Refund, w_.InvoiceNo, w_.PatientMRN, w_.PatientFirstName, w_.PatientLastName))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) listInvoiceRefunds(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	if _, err := h.deps.Invoices.GetByID(r.Context(), branchID, id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "fatura bulunamadı")
		return
	}
	items, err := h.deps.Refunds.ListForInvoice(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]refundPayload, 0, len(items))
	for i := range items {
		out = append(out, toRefundPayload(&items[i], nil, "", "", ""))
	}
	writeJSON(w, http.StatusOK, out)
}

type processRefundReq struct {
	PatientID      string  `json:"patient_id"`
	InvoiceID      string  `json:"invoice_id"`
	PaymentID      string  `json:"payment_id"`
	Amount         float64 `json:"amount"`
	Method         string  `json:"method"`
	CashRegisterID string  `json:"cash_register_id"`
	ToAdvance      bool    `json:"to_advance"`
	Reason         string  `json:"reason"`
}

func (h *Handler) processRefund(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req processRefundReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_patient", "patient_id zorunlu")
		return
	}
	method := req.Method
	if method == "" {
		method = "cash"
	}
	if !validPaymentMethods[method] {
		writeError(w, http.StatusBadRequest, "bad_method", "geçersiz ödeme yöntemi")
		return
	}
	in := service.ProcessRefundInput{
		OrganizationID: orgID, BranchID: branchID, PatientID: patientID,
		PaymentID: parseUUIDPtr(req.PaymentID),
		InvoiceID: parseUUIDPtr(req.InvoiceID),
		Amount:    req.Amount,
		Method:    method,
		CashRegisterID: parseUUIDPtr(req.CashRegisterID),
		ToAdvance: req.ToAdvance,
		Reason:    emptyToNil(&req.Reason),
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.PerformedByUserID = &uid
		}
	}
	res, err := h.deps.FinanceExt.ProcessRefund(r.Context(), in)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrOverRefund):
			writeError(w, http.StatusConflict, "over_refund", err.Error())
		case errors.Is(err, service.ErrCashRegisterMissing):
			writeError(w, http.StatusConflict, "kasa_required", err.Error())
		case errors.Is(err, service.ErrNothingToRefund):
			writeError(w, http.StatusBadRequest, "no_source", err.Error())
		default:
			writeError(w, http.StatusBadRequest, "refund_failed", err.Error())
		}
		return
	}
	out := map[string]any{"refund_id": res.RefundID.String(), "refund_no": res.RefundNo}
	if res.CashMovementID != nil {
		out["cash_movement_id"] = res.CashMovementID.String()
	}
	h.auditAccess(r.Context(), r, "payment.refund", "refund", res.RefundID.String(), map[string]any{
		"refund_no":  res.RefundNo,
		"patient_id": patientID.String(),
		"amount":     req.Amount,
		"method":     method,
		"to_advance": req.ToAdvance,
		"reason":     req.Reason,
	})
	writeJSON(w, http.StatusCreated, out)
}

// ============================================================================
//  Installment plan
// ============================================================================

type installmentPayload struct {
	ID         string     `json:"id"`
	Seq        int        `json:"seq"`
	DueDate    string     `json:"due_date"`
	Amount     float64    `json:"amount"`
	PaidAmount float64    `json:"paid_amount"`
	Status     string     `json:"status"`
	PaidAt     *time.Time `json:"paid_at,omitempty"`
	PaymentID  *string    `json:"payment_id,omitempty"`
	Notes      *string    `json:"notes,omitempty"`
}

type installmentPlanPayload struct {
	ID              string               `json:"id"`
	InvoiceID       string               `json:"invoice_id"`
	TotalAmount     float64              `json:"total_amount"`
	NumInstallments int                  `json:"num_installments"`
	Status          string               `json:"status"`
	Notes           *string              `json:"notes,omitempty"`
	CreatedAt       time.Time            `json:"created_at"`
	Installments    []installmentPayload `json:"installments"`
}

func (h *Handler) getInstallmentPlan(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	invoiceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	if _, err := h.deps.Invoices.GetByID(r.Context(), branchID, invoiceID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "fatura bulunamadı")
		return
	}
	plan, items, err := h.deps.Installments.GetForInvoice(r.Context(), invoiceID)
	if err != nil {
		writeJSON(w, http.StatusOK, nil) // no plan yet
		return
	}
	out := installmentPlanPayload{
		ID: plan.ID.String(), InvoiceID: plan.InvoiceID.String(),
		TotalAmount: plan.TotalAmount, NumInstallments: plan.NumInstallments,
		Status: plan.Status, Notes: plan.Notes, CreatedAt: plan.CreatedAt,
		Installments: make([]installmentPayload, 0, len(items)),
	}
	for _, i := range items {
		p := installmentPayload{
			ID: i.ID.String(), Seq: i.Seq, DueDate: i.DueDate.Format("2006-01-02"),
			Amount: i.Amount, PaidAmount: i.PaidAmount, Status: i.Status,
			PaidAt: i.PaidAt, Notes: i.Notes,
		}
		if i.PaymentID != nil {
			s := i.PaymentID.String()
			p.PaymentID = &s
		}
		out.Installments = append(out.Installments, p)
	}
	writeJSON(w, http.StatusOK, out)
}

type createInstallmentPlanReq struct {
	NumInstallments int    `json:"num_installments"`
	FirstDueDate    string `json:"first_due_date"` // YYYY-MM-DD
	IntervalDays    int    `json:"interval_days"`
	Notes           string `json:"notes"`
}

func (h *Handler) createInstallmentPlan(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	invoiceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req createInstallmentPlanReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if req.FirstDueDate == "" {
		req.FirstDueDate = time.Now().Format("2006-01-02")
	}
	firstDue, err := time.Parse("2006-01-02", strings.TrimSpace(req.FirstDueDate))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_date", "first_due_date YYYY-MM-DD olmalı")
		return
	}
	in := service.CreateInstallmentPlanInput{
		OrganizationID: orgID, BranchID: branchID, InvoiceID: invoiceID,
		NumInstallments: req.NumInstallments,
		FirstDueDate:    firstDue,
		IntervalDays:    req.IntervalDays,
		Notes:           emptyToNil(&req.Notes),
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.CreatedByUserID = &uid
		}
	}
	planID, err := h.deps.FinanceExt.CreatePlan(r.Context(), in)
	if err != nil {
		if errors.Is(err, service.ErrInstallmentDuplicate) {
			writeError(w, http.StatusConflict, "duplicate", err.Error())
			return
		}
		if errors.Is(err, service.ErrInvoiceNotPayable) || errors.Is(err, service.ErrInstallmentInvalid) {
			writeError(w, http.StatusBadRequest, "invalid", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"plan_id": planID.String()})
}

type upcomingInstallmentPayload struct {
	InstallmentID    string  `json:"installment_id"`
	PlanID           string  `json:"plan_id"`
	Seq              int     `json:"seq"`
	DueDate          string  `json:"due_date"`
	Amount           float64 `json:"amount"`
	PaidAmount       float64 `json:"paid_amount"`
	Status           string  `json:"status"`
	InvoiceID        string  `json:"invoice_id"`
	InvoiceNo        string  `json:"invoice_no"`
	PatientID        string  `json:"patient_id"`
	PatientMRN       string  `json:"patient_mrn"`
	PatientFirstName string  `json:"patient_first_name"`
	PatientLastName  string  `json:"patient_last_name"`
}

func (h *Handler) listUpcomingInstallments(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	through := time.Now().AddDate(0, 0, 30) // 30 gün sonrasına kadar
	if v := r.URL.Query().Get("through"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			through = t
		}
	}
	items, err := h.deps.Installments.UpcomingForBranch(r.Context(), branchID, through, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]upcomingInstallmentPayload, 0, len(items))
	for _, u := range items {
		out = append(out, upcomingInstallmentPayload{
			InstallmentID:    u.Installment.ID.String(),
			PlanID:           u.Installment.PlanID.String(),
			Seq:              u.Installment.Seq,
			DueDate:          u.Installment.DueDate.Format("2006-01-02"),
			Amount:           u.Installment.Amount,
			PaidAmount:       u.Installment.PaidAmount,
			Status:           u.Installment.Status,
			InvoiceID:        u.InvoiceID.String(),
			InvoiceNo:        u.InvoiceNo,
			PatientID:        u.PatientID.String(),
			PatientMRN:       u.PatientMRN,
			PatientFirstName: u.PatientFirstName,
			PatientLastName:  u.PatientLastName,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
