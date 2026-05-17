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
)

// ---------- Eczane queue ----------

type pendingItemPayload struct {
	ItemID           string   `json:"item_id"`
	MedicationName   string   `json:"medication_name"`
	MedicationID     *string  `json:"medication_id,omitempty"`
	Dosage           *string  `json:"dosage,omitempty"`
	Frequency        *string  `json:"frequency,omitempty"`
	Quantity         *string  `json:"quantity,omitempty"`
	DispenseQuantity *float64 `json:"dispense_quantity,omitempty"`
	DispensedTotal   float64  `json:"dispensed_total"`
	Instructions     *string  `json:"instructions,omitempty"`
}

type pendingPrescriptionPayload struct {
	ID               string     `json:"id"`
	PrescriptionNo   string     `json:"prescription_no"`
	Status           string     `json:"status"`
	SignedAt         *time.Time `json:"signed_at,omitempty"`
	PatientID        string     `json:"patient_id"`
	PatientMRN       string     `json:"patient_mrn"`
	PatientFirstName string     `json:"patient_first_name"`
	PatientLastName  string     `json:"patient_last_name"`
	DoctorFirstName  *string    `json:"doctor_first_name,omitempty"`
	DoctorLastName   *string    `json:"doctor_last_name,omitempty"`
	DoctorTitle      *string    `json:"doctor_title,omitempty"`
	Items            []pendingItemPayload `json:"items"`
}

func (h *Handler) listEczanePending(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	limit := 100
	items, err := h.deps.Eczane.ListPending(r.Context(), orgID, limit)
	if err != nil {
		h.deps.Log.Error("list eczane pending failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]pendingPrescriptionPayload, 0, len(items))
	for _, p := range items {
		row := pendingPrescriptionPayload{
			ID: p.ID.String(), PrescriptionNo: p.PrescriptionNo,
			Status: p.Status, SignedAt: p.SignedAt,
			PatientID: p.PatientID.String(), PatientMRN: p.PatientMRN,
			PatientFirstName: p.PatientFirstName, PatientLastName: p.PatientLastName,
			DoctorFirstName: p.DoctorFirstName, DoctorLastName: p.DoctorLastName,
			DoctorTitle: p.DoctorTitle,
			Items: make([]pendingItemPayload, 0, len(p.Items)),
		}
		for _, it := range p.Items {
			ip := pendingItemPayload{
				ItemID: it.ItemID.String(), MedicationName: it.MedicationName,
				Dosage: it.Dosage, Frequency: it.Frequency,
				Quantity: it.Quantity, DispenseQuantity: it.DispenseQuantity,
				DispensedTotal: it.DispensedTotal, Instructions: it.Instructions,
			}
			if it.MedicationID != nil {
				s := it.MedicationID.String()
				ip.MedicationID = &s
			}
			row.Items = append(row.Items, ip)
		}
		out = append(out, row)
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- FEFO lot lookup ----------

type lotPayload struct {
	StockID    string  `json:"stock_id"`
	LotNo      string  `json:"lot_no"`
	ExpiryDate *string `json:"expiry_date,omitempty"`
	Quantity   float64 `json:"quantity"`
}

func (h *Handler) listFEFOLots(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	whID, err := uuid.Parse(r.URL.Query().Get("warehouse_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_warehouse", "warehouse_id zorunlu")
		return
	}
	medID, err := uuid.Parse(r.URL.Query().Get("medication_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_medication", "medication_id zorunlu")
		return
	}
	items, err := h.deps.Eczane.FEFOLots(r.Context(), branchID, whID, medID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]lotPayload, 0, len(items))
	for _, l := range items {
		p := lotPayload{StockID: l.StockID.String(), LotNo: l.LotNo, Quantity: l.Quantity}
		if l.ExpiryDate != nil {
			d := l.ExpiryDate.Format("2006-01-02")
			p.ExpiryDate = &d
		}
		out = append(out, p)
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- Dispense ----------

type dispenseReq struct {
	MedicationID string  `json:"medication_id"`
	WarehouseID  string  `json:"warehouse_id"`
	LotNo        string  `json:"lot_no"`
	ExpiryDate   string  `json:"expiry_date"`
	Quantity     float64 `json:"quantity"`
	Counterparty string  `json:"counterparty"`
}

func (h *Handler) dispensePrescriptionItem(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	itemID, err := uuid.Parse(chi.URLParam(r, "itemId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_item", "itemId geçersiz")
		return
	}
	var req dispenseReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	medID, err := uuid.Parse(req.MedicationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_medication", "medication_id zorunlu")
		return
	}
	whID, err := uuid.Parse(req.WarehouseID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_warehouse", "warehouse_id zorunlu")
		return
	}
	if req.Quantity <= 0 {
		writeError(w, http.StatusBadRequest, "bad_quantity", "miktar pozitif olmalı")
		return
	}
	in := service.DispenseInput{
		OrganizationID: orgID, BranchID: branchID,
		PrescriptionItemID: itemID, MedicationID: medID,
		WarehouseID: whID, LotNo: req.LotNo, Quantity: req.Quantity,
		Counterparty: emptyToNil(&req.Counterparty),
	}
	if req.ExpiryDate != "" {
		if t, err := time.Parse("2006-01-02", req.ExpiryDate); err == nil {
			in.ExpiryDate = &t
		}
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.DispensedByUserID = &uid
		}
	}
	res, err := h.deps.StockSvc.Dispense(r.Context(), in)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrLotNotFound):
			writeError(w, http.StatusNotFound, "lot_not_found", "lot bulunamadı")
		case errors.Is(err, service.ErrInsufficientStock):
			writeError(w, http.StatusConflict, "insufficient_stock", "yetersiz stok")
		default:
			h.deps.Log.Error("dispense failed", "err", err)
			writeError(w, http.StatusInternalServerError, "dispense_failed", "dispense başarısız")
		}
		return
	}
	h.auditAccess(r.Context(), r, "prescription.dispense", "prescription_item", itemID.String(), map[string]any{
		"medication_id": medID.String(),
		"warehouse_id":  whID.String(),
		"lot_no":        req.LotNo,
		"quantity":      req.Quantity,
		"movement_no":   res.MovementNo,
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"dispense_id": res.DispenseID.String(),
		"movement_no": res.MovementNo,
	})
}

// ---------- Dispense history ----------

type historyPayload struct {
	ID               string     `json:"id"`
	PrescriptionNo   string     `json:"prescription_no"`
	PatientMRN       string     `json:"patient_mrn"`
	PatientFirstName string     `json:"patient_first_name"`
	PatientLastName  string     `json:"patient_last_name"`
	MedicationName   string     `json:"medication_name"`
	WarehouseName    string     `json:"warehouse_name"`
	LotNo            string     `json:"lot_no"`
	ExpiryDate       *string    `json:"expiry_date,omitempty"`
	Quantity         float64    `json:"quantity"`
	MovementNo       string     `json:"movement_no"`
	DispensedAt      time.Time  `json:"dispensed_at"`
	ItsStatus        string     `json:"its_status"`
	ItsNotifiedAt    *time.Time `json:"its_notified_at,omitempty"`
	ItsError         *string    `json:"its_error,omitempty"`
}

func (h *Handler) listDispenseHistory(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	items, err := h.deps.Eczane.DispenseHistory(r.Context(), branchID, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]historyPayload, 0, len(items))
	for _, x := range items {
		p := historyPayload{
			ID: x.ID.String(), PrescriptionNo: x.PrescriptionNo,
			PatientMRN: x.PatientMRN, PatientFirstName: x.PatientFirstName, PatientLastName: x.PatientLastName,
			MedicationName: x.MedicationName, WarehouseName: x.WarehouseName,
			LotNo: x.LotNo, Quantity: x.Quantity,
			MovementNo: x.MovementNo, DispensedAt: x.DispensedAt,
			ItsStatus: x.ItsStatus, ItsNotifiedAt: x.ItsNotifiedAt, ItsError: x.ItsError,
		}
		if x.ExpiryDate != nil {
			d := x.ExpiryDate.Format("2006-01-02")
			p.ExpiryDate = &d
		}
		out = append(out, p)
	}
	writeJSON(w, http.StatusOK, out)
}
