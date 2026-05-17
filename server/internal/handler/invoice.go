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

// ---------- Invoice payloads ----------

type invoiceItemPayload struct {
	ID               string   `json:"id"`
	ServiceID        *string  `json:"service_id,omitempty"`
	Code             string   `json:"code"`
	Name             string   `json:"name"`
	VisitID          *string  `json:"visit_id,omitempty"`
	LabOrderID       *string  `json:"lab_order_id,omitempty"`
	RadiologyOrderID *string  `json:"radiology_order_id,omitempty"`
	SurgeryID        *string  `json:"surgery_id,omitempty"`
	DoctorID         *string  `json:"doctor_id,omitempty"`
	Quantity         float64  `json:"quantity"`
	UnitPrice        float64  `json:"unit_price"`
	DiscountPct      float64  `json:"discount_pct"`
	VatRate          float64  `json:"vat_rate"`
	LineSubtotal     float64  `json:"line_subtotal"`
	LineTax          float64  `json:"line_tax"`
	LineTotal        float64  `json:"line_total"`
	SortOrder        int      `json:"sort_order"`
	Notes            *string  `json:"notes,omitempty"`
}

func uuidPtrToString(u *uuid.UUID) *string {
	if u == nil {
		return nil
	}
	s := u.String()
	return &s
}

func toInvoiceItemPayload(it *repo.InvoiceItem) invoiceItemPayload {
	return invoiceItemPayload{
		ID: it.ID.String(), ServiceID: uuidPtrToString(it.ServiceID),
		Code: it.Code, Name: it.Name,
		VisitID: uuidPtrToString(it.VisitID),
		LabOrderID: uuidPtrToString(it.LabOrderID),
		RadiologyOrderID: uuidPtrToString(it.RadiologyOrderID),
		SurgeryID: uuidPtrToString(it.SurgeryID),
		DoctorID: uuidPtrToString(it.DoctorID),
		Quantity: it.Quantity, UnitPrice: it.UnitPrice,
		DiscountPct: it.DiscountPct, VatRate: it.VatRate,
		LineSubtotal: it.LineSubtotal, LineTax: it.LineTax, LineTotal: it.LineTotal,
		SortOrder: it.SortOrder, Notes: it.Notes,
	}
}

type invoicePayload struct {
	ID               string     `json:"id"`
	InvoiceNo        string     `json:"invoice_no"`
	Status           string     `json:"status"`
	PatientID        string     `json:"patient_id"`
	PatientMRN       string     `json:"patient_mrn,omitempty"`
	PatientFirstName string     `json:"patient_first_name,omitempty"`
	PatientLastName  string     `json:"patient_last_name,omitempty"`
	InstitutionID    *string    `json:"institution_id,omitempty"`
	InstitutionName  *string    `json:"institution_name,omitempty"`
	VisitID          *string    `json:"visit_id,omitempty"`
	AdmissionID      *string    `json:"admission_id,omitempty"`
	Subtotal         float64    `json:"subtotal"`
	DiscountTotal    float64    `json:"discount_total"`
	TaxTotal         float64    `json:"tax_total"`
	Total            float64    `json:"total"`
	PaidTotal        float64    `json:"paid_total"`
	BalanceDue       float64    `json:"balance_due"`
	IssuedAt         *time.Time `json:"issued_at,omitempty"`
	CancelledAt      *time.Time `json:"cancelled_at,omitempty"`
	Notes            *string    `json:"notes,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func baseInvoice(inv *repo.Invoice) invoicePayload {
	p := invoicePayload{
		ID: inv.ID.String(), InvoiceNo: inv.InvoiceNo, Status: inv.Status,
		PatientID: inv.PatientID.String(),
		InstitutionID: uuidPtrToString(inv.InstitutionID),
		VisitID: uuidPtrToString(inv.VisitID),
		AdmissionID: uuidPtrToString(inv.AdmissionID),
		Subtotal: inv.Subtotal, DiscountTotal: inv.DiscountTotal,
		TaxTotal: inv.TaxTotal, Total: inv.Total,
		PaidTotal: inv.PaidTotal, BalanceDue: inv.BalanceDue,
		IssuedAt: inv.IssuedAt, CancelledAt: inv.CancelledAt,
		Notes: inv.Notes,
		CreatedAt: inv.CreatedAt, UpdatedAt: inv.UpdatedAt,
	}
	return p
}

func joinedInvoice(w *repo.InvoiceWithJoins) invoicePayload {
	p := baseInvoice(&w.Invoice)
	p.PatientMRN = w.PatientMRN
	p.PatientFirstName = w.PatientFirstName
	p.PatientLastName = w.PatientLastName
	p.InstitutionName = w.InstitutionName
	return p
}

type invoiceDetailPayload struct {
	Invoice invoicePayload      `json:"invoice"`
	Items   []invoiceItemPayload `json:"items"`
}

// ---------- List / detail ----------

func (h *Handler) listInvoices(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	f := repo.ListInvoiceFilter{
		Status:     r.URL.Query().Get("status"),
		OnlyUnpaid: r.URL.Query().Get("unpaid") == "true",
	}
	if v := r.URL.Query().Get("patient_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.PatientID = &id
		}
	}
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
			f.From = &t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local).Add(24 * time.Hour)
			f.To = &t
		}
	}
	items, err := h.deps.Invoices.List(r.Context(), branchID, f)
	if err != nil {
		h.deps.Log.Error("list invoices failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]invoicePayload, 0, len(items))
	for i := range items {
		out = append(out, joinedInvoice(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) getInvoice(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	inv, err := h.deps.Invoices.GetByID(r.Context(), branchID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "fatura bulunamadı")
		return
	}
	items, err := h.deps.Invoices.ListItems(r.Context(), inv.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "kalemler yüklenemedi")
		return
	}
	out := invoiceDetailPayload{Invoice: baseInvoice(inv), Items: make([]invoiceItemPayload, 0, len(items))}
	for i := range items {
		out.Items = append(out.Items, toInvoiceItemPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- Create / finalize / cancel ----------

type createInvoiceItemReq struct {
	ServiceID        string   `json:"service_id"`
	Code             string   `json:"code"`
	Name             string   `json:"name"`
	VisitID          string   `json:"visit_id"`
	LabOrderID       string   `json:"lab_order_id"`
	RadiologyOrderID string   `json:"radiology_order_id"`
	SurgeryID        string   `json:"surgery_id"`
	DoctorID         string   `json:"doctor_id"`
	Quantity         float64  `json:"quantity"`
	UnitPrice        float64  `json:"unit_price"`
	DiscountPct      float64  `json:"discount_pct"`
	VatRate          float64  `json:"vat_rate"`
	Notes            string   `json:"notes"`
}

type createInvoiceReq struct {
	PatientID     string                 `json:"patient_id"`
	InstitutionID string                 `json:"institution_id"`
	VisitID       string                 `json:"visit_id"`
	AdmissionID   string                 `json:"admission_id"`
	Notes         string                 `json:"notes"`
	Items         []createInvoiceItemReq `json:"items"`
	Finalize      bool                   `json:"finalize"`
}

func parseUUIDPtr(s string) *uuid.UUID {
	if s == "" {
		return nil
	}
	if id, err := uuid.Parse(s); err == nil {
		return &id
	}
	return nil
}

func (h *Handler) createInvoice(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req createInvoiceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_patient", "patient_id zorunlu")
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "no_items", "en az bir kalem gerekli")
		return
	}
	in := service.CreateInvoiceInput{
		OrganizationID: orgID, BranchID: branchID, PatientID: patientID,
		InstitutionID: parseUUIDPtr(req.InstitutionID),
		VisitID:       parseUUIDPtr(req.VisitID),
		AdmissionID:   parseUUIDPtr(req.AdmissionID),
		Notes:         emptyToNil(&req.Notes),
		Finalize:      req.Finalize,
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.CreatedByUserID = &uid
		}
	}
	for _, it := range req.Items {
		code := strings.TrimSpace(it.Code)
		name := strings.TrimSpace(it.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "bad_item", "her kalem için ad zorunlu")
			return
		}
		if code == "" {
			code = "CUSTOM"
		}
		in.Items = append(in.Items, service.CreateInvoiceItemInput{
			ServiceID: parseUUIDPtr(it.ServiceID),
			Code: code, Name: name,
			VisitID: parseUUIDPtr(it.VisitID),
			LabOrderID: parseUUIDPtr(it.LabOrderID),
			RadiologyOrderID: parseUUIDPtr(it.RadiologyOrderID),
			SurgeryID: parseUUIDPtr(it.SurgeryID),
			DoctorID: parseUUIDPtr(it.DoctorID),
			Quantity: it.Quantity, UnitPrice: it.UnitPrice,
			DiscountPct: it.DiscountPct, VatRate: it.VatRate,
			Notes: emptyToNil(&it.Notes),
		})
	}
	invID, invNo, err := h.deps.InvoiceSvc.Create(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("create invoice failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	h.auditAccess(r.Context(), r, "invoice.create", "invoice", invID.String(), map[string]any{
		"invoice_no":  invNo,
		"item_count":  len(in.Items),
		"patient_id":  patientID.String(),
		"finalize":    in.Finalize,
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         invID.String(),
		"invoice_no": invNo,
	})
}

func (h *Handler) finalizeInvoice(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	if err := h.deps.InvoiceSvc.Finalize(r.Context(), branchID, id); err != nil {
		if errors.Is(err, service.ErrInvoiceNotOpen) {
			writeError(w, http.StatusConflict, "not_open", "fatura zaten kapalı veya iptal edilmiş")
			return
		}
		writeError(w, http.StatusInternalServerError, "finalize_failed", err.Error())
		return
	}
	h.auditAccess(r.Context(), r, "invoice.finalize", "invoice", id.String(), nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type cancelInvoiceReq struct {
	Reason string `json:"reason"`
}

func (h *Handler) cancelInvoice(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req cancelInvoiceReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.deps.InvoiceSvc.Cancel(r.Context(), branchID, id, emptyToNil(&req.Reason)); err != nil {
		writeError(w, http.StatusConflict, "cancel_failed", err.Error())
		return
	}
	h.auditAccess(r.Context(), r, "invoice.cancel", "invoice", id.String(), map[string]any{
		"reason": req.Reason,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- Record payment ----------

type paymentAllocationReq struct {
	InvoiceID string  `json:"invoice_id"`
	Amount    float64 `json:"amount"`
}

type recordPaymentReq struct {
	PatientID      string                 `json:"patient_id"`
	Method         string                 `json:"method"`
	Amount         float64                `json:"amount"`
	Reference      string                 `json:"reference"`
	Notes          string                 `json:"notes"`
	CashRegisterID string                 `json:"cash_register_id"`
	Allocations    []paymentAllocationReq `json:"allocations"`
}

func (h *Handler) recordPayment(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req recordPaymentReq
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
	if !validPaymentMethods[method] {
		writeError(w, http.StatusBadRequest, "bad_method", "geçersiz ödeme yöntemi")
		return
	}
	allocs := make([]service.PaymentAllocationInput, 0, len(req.Allocations))
	for _, a := range req.Allocations {
		invID, err := uuid.Parse(a.InvoiceID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_alloc", "tahsis fatura id geçersiz")
			return
		}
		allocs = append(allocs, service.PaymentAllocationInput{InvoiceID: invID, Amount: a.Amount})
	}
	in := service.RecordPaymentInput{
		OrganizationID: orgID, BranchID: branchID, PatientID: patientID,
		Method: method, Amount: req.Amount,
		Reference: emptyToNil(&req.Reference), Notes: emptyToNil(&req.Notes),
		CashRegisterID: parseUUIDPtr(req.CashRegisterID),
		Allocations:    allocs,
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.ReceivedByUserID = &uid
		}
	}
	res, err := h.deps.InvoiceSvc.RecordPayment(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("record payment failed", "err", err)
		writeError(w, http.StatusConflict, "payment_failed", err.Error())
		return
	}
	// If any allocation lands on an invoice with a plan, apply against
	// installments in seq order. Errors here are non-fatal — payment
	// already committed; we log and move on.
	for _, a := range allocs {
		if err := h.deps.FinanceExt.MarkInstallmentPayment(r.Context(), branchID, a.InvoiceID, res.PaymentID, a.Amount); err != nil {
			h.deps.Log.Warn("installment update failed", "invoice", a.InvoiceID, "err", err)
		}
	}
	resp := map[string]any{
		"payment_id": res.PaymentID.String(),
		"payment_no": res.PaymentNo,
	}
	if res.MovementNo != nil {
		resp["cash_movement_no"] = *res.MovementNo
	}
	h.auditAccess(r.Context(), r, "payment.create", "payment", res.PaymentID.String(), map[string]any{
		"payment_no":   res.PaymentNo,
		"method":       method,
		"amount":       req.Amount,
		"patient_id":   patientID.String(),
		"allocations":  len(allocs),
	})
	writeJSON(w, http.StatusCreated, resp)
}

// ---------- List payments for an invoice ----------

type paymentSummaryPayload struct {
	ID          string    `json:"id"`
	PaymentNo   string    `json:"payment_no"`
	Method      string    `json:"method"`
	Amount      float64   `json:"amount"`
	Allocated   float64   `json:"allocated_to_this_invoice"`
	Reference   *string   `json:"reference,omitempty"`
	ReceivedAt  time.Time `json:"received_at"`
}

func (h *Handler) listInvoicePayments(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	// Verify invoice belongs to branch (best-effort scope check).
	if _, err := h.deps.Invoices.GetByID(r.Context(), branchID, id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "fatura bulunamadı")
		return
	}
	payments, allocs, err := h.deps.Payments.ListForInvoice(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "ödemeler yüklenemedi")
		return
	}
	allocByPayment := map[uuid.UUID]float64{}
	for _, a := range allocs {
		allocByPayment[a.PaymentID] = a.Amount
	}
	out := make([]paymentSummaryPayload, 0, len(payments))
	for _, p := range payments {
		out = append(out, paymentSummaryPayload{
			ID: p.ID.String(), PaymentNo: p.PaymentNo,
			Method: p.Method, Amount: p.Amount,
			Allocated: allocByPayment[p.ID],
			Reference: p.Reference, ReceivedAt: p.ReceivedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
