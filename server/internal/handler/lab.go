package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/middleware"
	"github.com/medigt/medigt/server/internal/util"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

// ---------- Catalog payload ----------

type labTestPayload struct {
	ID             string  `json:"id"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	SampleType     string  `json:"sample_type"`
	Unit           *string `json:"unit,omitempty"`
	ReferenceRange *string `json:"reference_range,omitempty"`
	LoincCode      *string `json:"loinc_code,omitempty"`
	SutCode        *string `json:"sut_code,omitempty"`
	IsSystem       bool    `json:"is_system"`
}

func toLabTestPayload(t *repo.LabTest) labTestPayload {
	return labTestPayload{
		ID: t.ID.String(), Code: t.Code, Name: t.Name,
		SampleType: t.SampleType, Unit: t.Unit, ReferenceRange: t.ReferenceRange,
		LoincCode: t.LoincCode, SutCode: t.SutCode, IsSystem: t.IsSystem,
	}
}

func (h *Handler) searchLabTests(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	q := r.URL.Query().Get("q")
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			limit = v
		}
	}
	items, err := h.deps.Lab.SearchTests(r.Context(), orgID, q, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]labTestPayload, 0, len(items))
	for i := range items {
		out = append(out, toLabTestPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- Order payload ----------

type labOrderItemPayload struct {
	ID             string     `json:"id"`
	TestCode       string     `json:"test_code"`
	TestName       string     `json:"test_name"`
	SampleType     string     `json:"sample_type"`
	Unit           *string    `json:"unit,omitempty"`
	ReferenceRange *string    `json:"reference_range,omitempty"`
	Status         string     `json:"status"`
	SortOrder      int        `json:"sort_order"`
	ValueNumeric   *float64   `json:"value_numeric,omitempty"`
	ValueText      *string    `json:"value_text,omitempty"`
	Flag           *string    `json:"flag,omitempty"`
	ResultedAt     *time.Time `json:"resulted_at,omitempty"`
	Notes          *string    `json:"notes,omitempty"`
}

type labOrderPayload struct {
	ID                  string                `json:"id"`
	OrderNo             string                `json:"order_no"`
	Status              string                `json:"status"`
	Priority            string                `json:"priority"`
	VisitID             *string               `json:"visit_id,omitempty"`
	PatientID           string                `json:"patient_id"`
	PatientMRN          string                `json:"patient_mrn"`
	PatientFirstName    string                `json:"patient_first_name"`
	PatientLastName     string                `json:"patient_last_name"`
	DoctorFirstName     *string               `json:"doctor_first_name,omitempty"`
	DoctorLastName      *string               `json:"doctor_last_name,omitempty"`
	DoctorTitle         *string               `json:"doctor_title,omitempty"`
	ClinicalIndication  *string               `json:"clinical_indication,omitempty"`
	Notes               *string               `json:"notes,omitempty"`
	OrderedAt           time.Time             `json:"ordered_at"`
	SampledAt           *time.Time            `json:"sampled_at,omitempty"`
	CompletedAt         *time.Time            `json:"completed_at,omitempty"`
	Items               []labOrderItemPayload `json:"items"`
	CreatedAt           time.Time             `json:"created_at"`
	UpdatedAt           time.Time             `json:"updated_at"`
}

func toLabOrderItemPayload(it *repo.LabOrderItem) labOrderItemPayload {
	return labOrderItemPayload{
		ID: it.ID.String(), TestCode: it.TestCode, TestName: it.TestName,
		SampleType: it.SampleType, Unit: it.Unit, ReferenceRange: it.ReferenceRange,
		Status: it.Status, SortOrder: it.SortOrder,
		ValueNumeric: it.ValueNumeric, ValueText: it.ValueText,
		Flag: it.Flag, ResultedAt: it.ResultedAt, Notes: it.Notes,
	}
}

func toLabOrderPayload(w *repo.LabOrderWithJoins) labOrderPayload {
	o := &w.Order
	p := labOrderPayload{
		ID: o.ID.String(), OrderNo: o.OrderNo, Status: o.Status, Priority: o.Priority,
		PatientID: o.PatientID.String(),
		PatientMRN: w.PatientMRN, PatientFirstName: w.PatientFirstName,
		PatientLastName: w.PatientLastName,
		DoctorFirstName: w.DoctorFirstName, DoctorLastName: w.DoctorLastName,
		DoctorTitle: w.DoctorTitle,
		ClinicalIndication: o.ClinicalIndication, Notes: o.Notes,
		OrderedAt: o.OrderedAt, SampledAt: o.SampledAt, CompletedAt: o.CompletedAt,
		CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
		Items: make([]labOrderItemPayload, 0, len(w.Items)),
	}
	if o.VisitID != nil {
		s := o.VisitID.String()
		p.VisitID = &s
	}
	for i := range w.Items {
		p.Items = append(p.Items, toLabOrderItemPayload(&w.Items[i]))
	}
	return p
}

func (h *Handler) listLabOrders(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	f := repo.ListLabOrderFilter{}
	if s := r.URL.Query().Get("status"); s != "" {
		f.Status = s
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
	if v := r.URL.Query().Get("visit_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.VisitID = &id
		}
	}

	items, err := h.deps.Lab.ListOrders(r.Context(), branchID, f)
	if err != nil {
		h.deps.Log.Error("list lab orders failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]labOrderPayload, 0, len(items))
	for i := range items {
		out = append(out, toLabOrderPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) getLabOrder(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	order, err := h.deps.Lab.GetOrderWithItems(r.Context(), branchID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "lab istek bulunamadı")
		return
	}
	writeJSON(w, http.StatusOK, toLabOrderPayload(order))
}

type createLabOrderReq struct {
	VisitID            string   `json:"visit_id"`             // optional — direct walk-in supported later
	PatientID          string   `json:"patient_id"`           // required when no visit
	OrderingDoctorID   string   `json:"ordering_doctor_id"`
	Priority           string   `json:"priority"`
	ClinicalIndication string   `json:"clinical_indication"`
	Notes              string   `json:"notes"`
	TestIDs            []string `json:"test_ids"`
}

func (h *Handler) createLabOrder(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req createLabOrderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if len(req.TestIDs) == 0 {
		writeError(w, http.StatusBadRequest, "missing_tests", "en az 1 test seçilmeli")
		return
	}

	in := repo.CreateLabOrderInput{
		OrganizationID:     orgID,
		BranchID:           branchID,
		Priority:           req.Priority,
		ClinicalIndication: emptyToNil(&req.ClinicalIndication),
		Notes:              emptyToNil(&req.Notes),
	}

	// Resolve patient (either directly, or from the visit).
	if req.VisitID != "" {
		vid, err := uuid.Parse(req.VisitID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_visit", "visit_id geçersiz")
			return
		}
		visit, err := h.deps.Visits.GetByID(r.Context(), branchID, vid)
		if err != nil {
			writeError(w, http.StatusNotFound, "visit_not_found", "muayene bulunamadı")
			return
		}
		in.VisitID = &visit.ID
		in.PatientID = visit.PatientID
		if visit.DoctorID != nil {
			in.OrderingDoctorID = visit.DoctorID
		}
	} else {
		pid, err := uuid.Parse(req.PatientID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_patient", "patient_id veya visit_id zorunlu")
			return
		}
		in.PatientID = pid
	}
	if req.OrderingDoctorID != "" {
		id, err := uuid.Parse(req.OrderingDoctorID)
		if err == nil {
			in.OrderingDoctorID = &id
		}
	}

	for _, s := range req.TestIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_test", "test_id geçersiz")
			return
		}
		in.TestCatalogIDs = append(in.TestCatalogIDs, id)
	}

	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.OrderedByUserID = &uid
		}
	}

	nextNo, err := h.deps.Lab.NextOrderNo(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "seq_failed", "istek numarası alınamadı")
		return
	}
	in.OrderNo = util.FormatMRN(nextNo)

	order, err := h.deps.Lab.Create(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("create lab order failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, toLabOrderPayload(order))
}

type updateLabOrderStatusReq struct {
	Status string `json:"status"`
}

var validLabOrderStatuses = map[string]bool{
	"sampled": true, "in_progress": true, "verified": true, "cancelled": true,
}

func (h *Handler) updateLabOrderStatus(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req updateLabOrderStatusReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if !validLabOrderStatuses[req.Status] {
		writeError(w, http.StatusBadRequest, "bad_status", "geçersiz durum")
		return
	}
	var byUser *uuid.UUID
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			byUser = &uid
		}
	}
	if err := h.deps.Lab.UpdateOrderStatus(r.Context(), branchID, id, req.Status, byUser); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "lab istek bulunamadı")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_failed", "güncelleme başarısız")
		return
	}
	order, err := h.deps.Lab.GetOrderWithItems(r.Context(), branchID, id)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, toLabOrderPayload(order))
}

type updateLabItemReq struct {
	ValueNumeric *float64 `json:"value_numeric"`
	ValueText    *string  `json:"value_text"`
	Flag         *string  `json:"flag"`
	Notes        *string  `json:"notes"`
}

var validLabFlags = map[string]bool{
	"normal": true, "low": true, "high": true,
	"critical_low": true, "critical_high": true, "abnormal": true,
}

func (h *Handler) updateLabItemResult(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "itemId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "itemId geçersiz")
		return
	}
	var req updateLabItemReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if req.Flag != nil {
		f := strings.TrimSpace(*req.Flag)
		if f == "" {
			req.Flag = nil
		} else if !validLabFlags[f] {
			writeError(w, http.StatusBadRequest, "bad_flag", "geçersiz flag")
			return
		}
	}
	var byUser *uuid.UUID
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			byUser = &uid
		}
	}
	it, err := h.deps.Lab.UpdateItemResult(r.Context(), id, repo.UpdateItemResultInput{
		ValueNumeric:     req.ValueNumeric,
		ValueText:        emptyToNil(req.ValueText),
		Flag:             req.Flag,
		Notes:            emptyToNil(req.Notes),
		ResultedByUserID: byUser,
	})
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "test satırı bulunamadı")
			return
		}
		h.deps.Log.Error("update lab item failed", "err", err)
		writeError(w, http.StatusInternalServerError, "update_failed", "güncelleme başarısız")
		return
	}
	// Lab sonucu yayını klinik+adli değeri yüksek — flag (critical) içerse
	// audit'te belirgin tut.
	details := map[string]any{}
	if req.Flag != nil {
		details["flag"] = *req.Flag
	}
	h.auditAccess(r.Context(), r, "lab.result.publish", "lab_order_item", id.String(), details)
	writeJSON(w, http.StatusOK, toLabOrderItemPayload(it))
}
