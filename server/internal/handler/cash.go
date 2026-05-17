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

// ---------- Cash register ----------

type cashRegisterPayload struct {
	ID              string     `json:"id"`
	RegisterNo      string     `json:"register_no"`
	CashierUserID   string     `json:"cashier_user_id"`
	CashierName     string     `json:"cashier_name"`
	Status          string     `json:"status"`
	OpeningBalance  float64    `json:"opening_balance"`
	DeclaredBalance *float64   `json:"declared_balance,omitempty"`
	Notes           *string    `json:"notes,omitempty"`
	OpenedAt        time.Time  `json:"opened_at"`
	ClosedAt        *time.Time `json:"closed_at,omitempty"`
}

func toCashRegisterPayload(c *repo.CashRegister) cashRegisterPayload {
	return cashRegisterPayload{
		ID: c.ID.String(), RegisterNo: c.RegisterNo,
		CashierUserID: c.CashierUserID.String(), CashierName: c.CashierName,
		Status: c.Status, OpeningBalance: c.OpeningBalance,
		DeclaredBalance: c.DeclaredBalance, Notes: c.Notes,
		OpenedAt: c.OpenedAt, ClosedAt: c.ClosedAt,
	}
}

func (h *Handler) myRegister(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	if uid == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "kullanıcı yok")
		return
	}
	userID, err := uuid.Parse(uid)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_user", "kullanıcı geçersiz")
		return
	}
	reg, err := h.deps.CashRegisters.FindOpenForUser(r.Context(), userID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeJSON(w, http.StatusOK, nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	writeJSON(w, http.StatusOK, toCashRegisterPayload(reg))
}

func (h *Handler) listRegisters(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	f := repo.ListRegisterFilter{Status: r.URL.Query().Get("status")}
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
	items, err := h.deps.CashRegisters.List(r.Context(), branchID, f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]cashRegisterPayload, 0, len(items))
	for i := range items {
		out = append(out, toCashRegisterPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type openRegisterReq struct {
	OpeningBalance float64 `json:"opening_balance"`
	Notes          string  `json:"notes"`
}

func (h *Handler) openRegister(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	uid := middleware.UserIDFromContext(r.Context())
	if uid == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "kullanıcı yok")
		return
	}
	userID, err := uuid.Parse(uid)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_user", "kullanıcı geçersiz")
		return
	}
	// Snapshot cashier name from users table.
	var cashierName string
	if err := h.deps.Pool.QueryRow(r.Context(),
		`SELECT COALESCE(NULLIF(name, ''), email) FROM app_user WHERE id = $1`, userID).
		Scan(&cashierName); err != nil {
		cashierName = "Kasiyer"
	}

	var req openRegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if req.OpeningBalance < 0 {
		writeError(w, http.StatusBadRequest, "bad_balance", "açılış bakiyesi negatif olamaz")
		return
	}
	in := service.OpenRegisterInput{
		OrganizationID: orgID, BranchID: branchID,
		CashierUserID: userID, CashierName: cashierName,
		OpeningBalance: req.OpeningBalance, Notes: emptyToNil(&req.Notes),
	}
	regID, registerNo, err := h.deps.CashSvc.Open(r.Context(), in)
	if err != nil {
		if errors.Is(err, service.ErrRegisterAlreadyOpen) {
			writeError(w, http.StatusConflict, "already_open", "zaten açık bir kasanız var")
			return
		}
		h.deps.Log.Error("open register failed", "err", err)
		writeError(w, http.StatusInternalServerError, "open_failed", "kasa açma başarısız")
		return
	}
	h.auditAccess(r.Context(), r, "kasa.open", "cash_register", regID.String(), map[string]any{
		"register_no":     registerNo,
		"opening_balance": req.OpeningBalance,
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":           regID.String(),
		"register_no":  registerNo,
	})
}

type closeRegisterReq struct {
	DeclaredBalance float64 `json:"declared_balance"`
	Notes           string  `json:"notes"`
}

func (h *Handler) closeRegister(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req closeRegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	in := service.CloseRegisterInput{
		OrganizationID: orgID, BranchID: branchID, RegisterID: id,
		DeclaredBalance: req.DeclaredBalance, Notes: emptyToNil(&req.Notes),
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.PerformedByUserID = &uid
		}
	}
	if err := h.deps.CashSvc.Close(r.Context(), in); err != nil {
		if errors.Is(err, service.ErrRegisterNotOpen) {
			writeError(w, http.StatusConflict, "not_open", "kasa zaten kapalı")
			return
		}
		h.deps.Log.Error("close register failed", "err", err)
		writeError(w, http.StatusInternalServerError, "close_failed", "kapanış başarısız")
		return
	}
	h.auditAccess(r.Context(), r, "kasa.close", "cash_register", id.String(), map[string]any{
		"declared_balance": req.DeclaredBalance,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- Cash movements ----------

type cashMovementPayload struct {
	ID            string    `json:"id"`
	MovementNo    string    `json:"movement_no"`
	Kind          string    `json:"kind"`
	Method        string    `json:"method"`
	Amount        float64   `json:"amount"`
	ReferenceType *string   `json:"reference_type,omitempty"`
	ReferenceID   *string   `json:"reference_id,omitempty"`
	Counterparty  *string   `json:"counterparty,omitempty"`
	Description   *string   `json:"description,omitempty"`
	PerformedAt   time.Time `json:"performed_at"`
}

func toCashMovementPayload(m *repo.CashMovement) cashMovementPayload {
	p := cashMovementPayload{
		ID: m.ID.String(), MovementNo: m.MovementNo,
		Kind: m.Kind, Method: m.Method, Amount: m.Amount,
		ReferenceType: m.ReferenceType, Counterparty: m.Counterparty,
		Description: m.Description, PerformedAt: m.PerformedAt,
	}
	if m.ReferenceID != nil {
		s := m.ReferenceID.String()
		p.ReferenceID = &s
	}
	return p
}

func (h *Handler) listRegisterMovements(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	// Ensure the register belongs to the branch.
	if _, err := h.deps.CashRegisters.GetByID(r.Context(), branchID, id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "kasa bulunamadı")
		return
	}
	items, err := h.deps.CashMovements.ListForRegister(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]cashMovementPayload, 0, len(items))
	for i := range items {
		out = append(out, toCashMovementPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type recordMovementReq struct {
	Kind         string  `json:"kind"`
	Method       string  `json:"method"`
	Amount       float64 `json:"amount"`
	Counterparty string  `json:"counterparty"`
	Description  string  `json:"description"`
}

var validCashMovementKinds = map[string]bool{
	"income": true, "expense": true, "refund": true,
	"transfer_in": true, "transfer_out": true,
}

var validPaymentMethods = map[string]bool{
	"cash": true, "card": true, "transfer": true, "mobile": true, "other": true,
}

func (h *Handler) recordRegisterMovement(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req recordMovementReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if !validCashMovementKinds[req.Kind] {
		writeError(w, http.StatusBadRequest, "bad_kind", "geçersiz hareket türü")
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
	if req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "bad_amount", "tutar pozitif olmalı")
		return
	}
	in := service.MovementInput{
		OrganizationID: orgID, BranchID: branchID,
		RegisterID: id, Kind: req.Kind, Method: method, Amount: req.Amount,
		Counterparty: emptyToNil(&req.Counterparty),
		Description:  emptyToNil(&req.Description),
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.PerformedByUserID = &uid
		}
	}
	mvtNo, err := h.deps.CashSvc.RecordMovement(r.Context(), in)
	if err != nil {
		if errors.Is(err, service.ErrRegisterClosed) || errors.Is(err, service.ErrRegisterNotOpen) {
			writeError(w, http.StatusConflict, "register_closed", "kasa kapalı")
			return
		}
		writeError(w, http.StatusInternalServerError, "record_failed", "kayıt başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"movement_no": mvtNo})
}

// ---------- Z report ----------

type zReportRowPayload struct {
	Kind   string  `json:"kind"`
	Method string  `json:"method"`
	Total  float64 `json:"total"`
	Count  int     `json:"count"`
}

type zReportPayload struct {
	Register      cashRegisterPayload   `json:"register"`
	Movements     []cashMovementPayload `json:"movements"`
	ByKindMethod  []zReportRowPayload   `json:"by_kind_method"`
	TotalIncome   float64               `json:"total_income"`
	TotalExpense  float64               `json:"total_expense"`
	TotalRefund   float64               `json:"total_refund"`
	ExpectedClose float64               `json:"expected_close"`
	Variance      *float64              `json:"variance,omitempty"`
}

func (h *Handler) zReport(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	reg, err := h.deps.CashRegisters.GetByID(r.Context(), branchID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "kasa bulunamadı")
		return
	}
	z, err := h.deps.CashMovements.ZReport(r.Context(), reg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := zReportPayload{
		Register:      toCashRegisterPayload(&z.Register),
		Movements:     make([]cashMovementPayload, 0, len(z.Movements)),
		ByKindMethod:  make([]zReportRowPayload, 0, len(z.ByKindMethod)),
		TotalIncome:   z.TotalIncome,
		TotalExpense:  z.TotalExpense,
		TotalRefund:   z.TotalRefund,
		ExpectedClose: z.ExpectedClose,
		Variance:      z.Variance,
	}
	for i := range z.Movements {
		out.Movements = append(out.Movements, toCashMovementPayload(&z.Movements[i]))
	}
	for _, s := range z.ByKindMethod {
		out.ByKindMethod = append(out.ByKindMethod, zReportRowPayload{
			Kind: s.Kind, Method: s.Method, Total: s.Total, Count: s.Count,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
