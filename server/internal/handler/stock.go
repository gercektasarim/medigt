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

// ---------- Warehouse ----------

type warehousePayload struct {
	ID       string  `json:"id"`
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Kind     string  `json:"kind"`
	Location *string `json:"location,omitempty"`
}

func toWarehousePayload(w *repo.Warehouse) warehousePayload {
	return warehousePayload{ID: w.ID.String(), Code: w.Code, Name: w.Name, Kind: w.Kind, Location: w.Location}
}

func (h *Handler) listWarehouses(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	items, err := h.deps.Warehouses.List(r.Context(), branchID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]warehousePayload, 0, len(items))
	for i := range items {
		out = append(out, toWarehousePayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type createWarehouseReq struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Location string `json:"location"`
	Notes    string `json:"notes"`
}

var validWarehouseKinds = map[string]bool{
	"pharmacy": true, "general": true, "central": true,
	"ward": true, "operating_room": true, "other": true,
}

func (h *Handler) createWarehouse(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req createWarehouseReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	code := strings.TrimSpace(req.Code)
	name := strings.TrimSpace(req.Name)
	if code == "" || name == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "code ve name zorunlu")
		return
	}
	kind := req.Kind
	if kind == "" {
		kind = "pharmacy"
	}
	if !validWarehouseKinds[kind] {
		writeError(w, http.StatusBadRequest, "bad_kind", "geçersiz depo türü")
		return
	}
	wh, err := h.deps.Warehouses.Create(r.Context(), repo.CreateWarehouseInput{
		OrganizationID: orgID, BranchID: branchID,
		Code:     strings.ToUpper(strings.ReplaceAll(code, " ", "_")),
		Name:     name,
		Kind:     kind,
		Location: emptyToNil(&req.Location),
		Notes:    emptyToNil(&req.Notes),
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "code_taken", "bu kod zaten kayıtlı")
			return
		}
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, toWarehousePayload(wh))
}

// ---------- Live stock ----------

type stockPayload struct {
	StockID         string    `json:"stock_id"`
	WarehouseID     string    `json:"warehouse_id"`
	WarehouseCode   string    `json:"warehouse_code"`
	WarehouseName   string    `json:"warehouse_name"`
	MedicationID    string    `json:"medication_id"`
	MedicationName  string    `json:"medication_name"`
	GenericName     *string   `json:"generic_name,omitempty"`
	Form            string    `json:"form"`
	Strength        *string   `json:"strength,omitempty"`
	LotNo           string    `json:"lot_no"`
	ExpiryDate      *string   `json:"expiry_date,omitempty"`
	Quantity        float64   `json:"quantity"`
	LastMovementAt  time.Time `json:"last_movement_at"`
}

func toStockPayload(s *repo.StockRow) stockPayload {
	p := stockPayload{
		StockID: s.StockID.String(),
		WarehouseID: s.WarehouseID.String(), WarehouseCode: s.WarehouseCode, WarehouseName: s.WarehouseName,
		MedicationID: s.MedicationID.String(), MedicationName: s.MedicationName,
		GenericName: s.GenericName, Form: s.Form, Strength: s.Strength,
		LotNo: s.LotNo, Quantity: s.Quantity, LastMovementAt: s.LastMovementAt,
	}
	if s.ExpiryDate != nil {
		d := s.ExpiryDate.Format("2006-01-02")
		p.ExpiryDate = &d
	}
	return p
}

func (h *Handler) listStock(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	f := repo.ListStockFilter{Search: strings.TrimSpace(r.URL.Query().Get("q"))}
	if v := r.URL.Query().Get("warehouse_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.WarehouseID = &id
		}
	}
	if v := r.URL.Query().Get("medication_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.MedicationID = &id
		}
	}
	if r.URL.Query().Get("with_zero") == "true" {
		f.WithZero = true
	}
	if v := r.URL.Query().Get("expiring_days"); v != "" {
		var d int
		_, _ = fmtSscanInt(v, &d)
		f.ExpiringDays = d
	}
	items, err := h.deps.Stock.List(r.Context(), branchID, f)
	if err != nil {
		h.deps.Log.Error("list stock failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]stockPayload, 0, len(items))
	for i := range items {
		out = append(out, toStockPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- Movements (audit log + receive/adjust mutations) ----------

type movementPayload struct {
	ID             string    `json:"id"`
	MovementNo     string    `json:"movement_no"`
	WarehouseID    string    `json:"warehouse_id"`
	WarehouseCode  string    `json:"warehouse_code"`
	WarehouseName  string    `json:"warehouse_name"`
	MedicationID   string    `json:"medication_id"`
	MedicationName string    `json:"medication_name"`
	LotNo          string    `json:"lot_no"`
	ExpiryDate     *string   `json:"expiry_date,omitempty"`
	Kind           string    `json:"kind"`
	Quantity       float64   `json:"quantity"`
	UnitPrice      *float64  `json:"unit_price,omitempty"`
	ReferenceType  *string   `json:"reference_type,omitempty"`
	ReferenceID    *string   `json:"reference_id,omitempty"`
	Counterparty   *string   `json:"counterparty,omitempty"`
	Notes          *string   `json:"notes,omitempty"`
	PerformedAt    time.Time `json:"performed_at"`
}

func toMovementPayload(w *repo.StockMovementWithJoins) movementPayload {
	m := &w.Movement
	p := movementPayload{
		ID: m.ID.String(), MovementNo: m.MovementNo,
		WarehouseID: m.WarehouseID.String(), WarehouseCode: w.WarehouseCode, WarehouseName: w.WarehouseName,
		MedicationID: m.MedicationID.String(), MedicationName: w.MedicationName,
		LotNo: m.LotNo, Kind: m.Kind, Quantity: m.Quantity,
		UnitPrice: m.UnitPrice, ReferenceType: m.ReferenceType,
		Counterparty: m.Counterparty, Notes: m.Notes, PerformedAt: m.PerformedAt,
	}
	if m.ExpiryDate != nil {
		d := m.ExpiryDate.Format("2006-01-02")
		p.ExpiryDate = &d
	}
	if m.ReferenceID != nil {
		s := m.ReferenceID.String()
		p.ReferenceID = &s
	}
	return p
}

var validMovementKinds = map[string]bool{
	"receive": true, "issue": true, "transfer_out": true, "transfer_in": true,
	"adjust": true, "expire": true, "return": true,
}

func (h *Handler) listMovements(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	f := repo.ListMovementFilter{Kind: r.URL.Query().Get("kind")}
	if v := r.URL.Query().Get("warehouse_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.WarehouseID = &id
		}
	}
	if v := r.URL.Query().Get("medication_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.MedicationID = &id
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
	items, err := h.deps.Movements.List(r.Context(), branchID, f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]movementPayload, 0, len(items))
	for i := range items {
		out = append(out, toMovementPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type receiveReq struct {
	WarehouseID  string   `json:"warehouse_id"`
	MedicationID string   `json:"medication_id"`
	LotNo        string   `json:"lot_no"`
	ExpiryDate   string   `json:"expiry_date"` // YYYY-MM-DD or empty
	Quantity     float64  `json:"quantity"`
	UnitPrice    *float64 `json:"unit_price"`
	Counterparty string   `json:"counterparty"`
	Notes        string   `json:"notes"`
}

func (h *Handler) receiveStock(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req receiveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	whID, err := uuid.Parse(req.WarehouseID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_warehouse", "warehouse_id geçersiz")
		return
	}
	medID, err := uuid.Parse(req.MedicationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_medication", "medication_id geçersiz")
		return
	}
	if req.Quantity <= 0 {
		writeError(w, http.StatusBadRequest, "bad_quantity", "miktar pozitif olmalı")
		return
	}
	in := service.ReceiveInput{
		OrganizationID: orgID, BranchID: branchID,
		WarehouseID: whID, MedicationID: medID,
		LotNo: strings.TrimSpace(req.LotNo), Quantity: req.Quantity,
		UnitPrice: req.UnitPrice,
		Counterparty: emptyToNil(&req.Counterparty),
		Notes:        emptyToNil(&req.Notes),
	}
	if req.ExpiryDate != "" {
		if t, err := time.Parse("2006-01-02", req.ExpiryDate); err == nil {
			in.ExpiryDate = &t
		}
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.PerformedByUserID = &uid
		}
	}
	movementNo, err := h.deps.StockSvc.Receive(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("receive stock failed", "err", err)
		writeError(w, http.StatusInternalServerError, "receive_failed", "mal alımı başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"movement_no": movementNo})
}

type adjustReq struct {
	WarehouseID  string  `json:"warehouse_id"`
	MedicationID string  `json:"medication_id"`
	LotNo        string  `json:"lot_no"`
	ExpiryDate   string  `json:"expiry_date"`
	NewQuantity  float64 `json:"new_quantity"`
	Notes        string  `json:"notes"`
}

func (h *Handler) adjustStock(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req adjustReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	whID, err := uuid.Parse(req.WarehouseID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_warehouse", "warehouse_id geçersiz")
		return
	}
	medID, err := uuid.Parse(req.MedicationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_medication", "medication_id geçersiz")
		return
	}
	if req.NewQuantity < 0 {
		writeError(w, http.StatusBadRequest, "bad_quantity", "yeni miktar negatif olamaz")
		return
	}
	in := service.AdjustInput{
		OrganizationID: orgID, BranchID: branchID,
		WarehouseID: whID, MedicationID: medID,
		LotNo: strings.TrimSpace(req.LotNo), NewQuantity: req.NewQuantity,
		Notes: emptyToNil(&req.Notes),
	}
	if req.ExpiryDate != "" {
		if t, err := time.Parse("2006-01-02", req.ExpiryDate); err == nil {
			in.ExpiryDate = &t
		}
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.PerformedByUserID = &uid
		}
	}
	movementNo, err := h.deps.StockSvc.Adjust(r.Context(), in)
	if err != nil {
		if errors.Is(err, service.ErrLotNotFound) {
			writeError(w, http.StatusNotFound, "lot_not_found", "lot bulunamadı; önce mal girişi yapın")
			return
		}
		writeError(w, http.StatusInternalServerError, "adjust_failed", "düzeltme başarısız: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"movement_no": movementNo})
}

// Tiny helper: parse an int from a string without bringing in strconv to the
// already-busy import surface of this file. Stops at first non-digit.
func fmtSscanInt(s string, out *int) (int, error) {
	v := 0
	read := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		v = v*10 + int(c-'0')
		read++
	}
	*out = v
	return read, nil
}
