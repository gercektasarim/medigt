package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/medigt/medigt/server/pkg/db/repo"
)

// ---------- Date-range helpers ----------

func parseDateRange(r *http.Request) (time.Time, time.Time, bool) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		// Default to current month.
		now := time.Now()
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		end := start.AddDate(0, 1, 0)
		return start, end, true
	}
	f, err := time.Parse("2006-01-02", from)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", to)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	fStart := time.Date(f.Year(), f.Month(), f.Day(), 0, 0, 0, 0, time.Local)
	tEnd := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local).Add(24 * time.Hour)
	return fStart, tEnd, true
}

// ---------- Summary ----------

type hakedisSummaryPayload struct {
	DoctorID       string  `json:"doctor_id"`
	FirstName      string  `json:"first_name"`
	LastName       string  `json:"last_name"`
	Title          *string `json:"title,omitempty"`
	ItemCount      int     `json:"item_count"`
	GrossRevenue   float64 `json:"gross_revenue"`
	EarningTotal   float64 `json:"earning_total"`
}

func (h *Handler) listHakedisSummary(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	from, to, ok := parseDateRange(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_date", "from / to YYYY-MM-DD olmalı")
		return
	}
	items, err := h.deps.Hakedis.SummaryByDoctor(r.Context(), branchID, from, to)
	if err != nil {
		h.deps.Log.Error("hakedis summary failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]hakedisSummaryPayload, 0, len(items))
	for _, s := range items {
		out = append(out, hakedisSummaryPayload{
			DoctorID: s.DoctorID.String(),
			FirstName: s.StaffFirstName, LastName: s.StaffLastName, Title: s.StaffTitle,
			ItemCount: s.ItemCount, GrossRevenue: s.GrossRevenue, EarningTotal: s.EarningTotal,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- Per-doctor items ----------

type hakedisItemPayload struct {
	InvoiceItemID    string    `json:"invoice_item_id"`
	InvoiceID        string    `json:"invoice_id"`
	InvoiceNo        string    `json:"invoice_no"`
	IssuedAt         time.Time `json:"issued_at"`
	PatientMRN       string    `json:"patient_mrn"`
	PatientFirstName string    `json:"patient_first_name"`
	PatientLastName  string    `json:"patient_last_name"`
	Code             string    `json:"code"`
	Name             string    `json:"name"`
	Category         *string   `json:"category,omitempty"`
	LineTotal        float64   `json:"line_total"`
	CommissionPct    float64   `json:"commission_pct"`
	Earning          float64   `json:"earning"`
}

func (h *Handler) listHakedisItems(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	doctorID, err := uuid.Parse(chi.URLParam(r, "doctorId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_doctor", "doctorId geçersiz")
		return
	}
	from, to, ok := parseDateRange(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_date", "from / to YYYY-MM-DD olmalı")
		return
	}
	items, err := h.deps.Hakedis.ItemsForDoctor(r.Context(), branchID, doctorID, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]hakedisItemPayload, 0, len(items))
	for _, x := range items {
		out = append(out, hakedisItemPayload{
			InvoiceItemID: x.InvoiceItemID.String(),
			InvoiceID:     x.InvoiceID.String(),
			InvoiceNo:     x.InvoiceNo,
			IssuedAt:      x.IssuedAt,
			PatientMRN:    x.PatientMRN, PatientFirstName: x.PatientFirstName, PatientLastName: x.PatientLastName,
			Code:          x.Code, Name: x.Name, Category: x.Category,
			LineTotal:     x.LineTotal, CommissionPct: x.CommissionPct, Earning: x.Earning,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- Commission rules ----------

type commissionRulePayload struct {
	ID            string  `json:"id"`
	DoctorID      string  `json:"doctor_id"`
	Category      *string `json:"category,omitempty"`
	CommissionPct float64 `json:"commission_pct"`
	ValidFrom     string  `json:"valid_from"`
	ValidTo       *string `json:"valid_to,omitempty"`
	Notes         *string `json:"notes,omitempty"`
}

func toRulePayload(r *repo.CommissionRule) commissionRulePayload {
	p := commissionRulePayload{
		ID: r.ID.String(), DoctorID: r.DoctorID.String(),
		Category: r.Category, CommissionPct: r.CommissionPct,
		ValidFrom: r.ValidFrom.Format("2006-01-02"),
		Notes:     r.Notes,
	}
	if r.ValidTo != nil {
		v := r.ValidTo.Format("2006-01-02")
		p.ValidTo = &v
	}
	return p
}

func (h *Handler) listCommissionRules(w http.ResponseWriter, r *http.Request) {
	doctorID, err := uuid.Parse(chi.URLParam(r, "doctorId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_doctor", "doctorId geçersiz")
		return
	}
	items, err := h.deps.Hakedis.ListRulesForDoctor(r.Context(), doctorID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]commissionRulePayload, 0, len(items))
	for i := range items {
		out = append(out, toRulePayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- Bulk commission rules ----------

type bulkCommissionReq struct {
	DoctorIDs           []string `json:"doctor_ids"`
	SpecializationCodes []string `json:"specialization_codes"`
	Category            string   `json:"category"`
	CommissionPct       float64  `json:"commission_pct"`
	ValidFrom           string   `json:"valid_from"`
	Notes               string   `json:"notes"`
}

func parseBulkInput(orgID uuid.UUID, req bulkCommissionReq) (repo.BulkCreateRulesInput, error) {
	in := repo.BulkCreateRulesInput{
		OrganizationID:      orgID,
		SpecializationCodes: req.SpecializationCodes,
		CommissionPct:       req.CommissionPct,
		ValidFrom:           time.Now(),
		Notes:               emptyToNil(&req.Notes),
	}
	if req.Category != "" {
		in.Category = &req.Category
	}
	if req.ValidFrom != "" {
		t, err := time.Parse("2006-01-02", req.ValidFrom)
		if err != nil {
			return in, err
		}
		in.ValidFrom = t
	}
	if len(req.DoctorIDs) > 0 {
		ids := make([]uuid.UUID, 0, len(req.DoctorIDs))
		for _, s := range req.DoctorIDs {
			id, err := uuid.Parse(s)
			if err != nil {
				return in, err
			}
			ids = append(ids, id)
		}
		in.DoctorIDs = ids
	}
	return in, nil
}

// previewBulkRules returns how many doctors a given filter targets, without
// inserting anything. Lets the UI show "X doctors will be affected" before
// the user clicks Apply.
func (h *Handler) previewBulkRules(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	var req bulkCommissionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	in, err := parseBulkInput(orgID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_input", err.Error())
		return
	}
	if len(in.DoctorIDs) == 0 && len(in.SpecializationCodes) == 0 {
		writeError(w, http.StatusBadRequest, "missing_filter", "en az bir doktor veya branş seçilmeli")
		return
	}
	ids, err := h.deps.Hakedis.ResolveTargetDoctors(r.Context(), orgID, in.DoctorIDs, in.SpecializationCodes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"targeted_doctors": len(ids),
		"doctor_ids":       ids,
	})
}

func (h *Handler) bulkCreateRules(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	var req bulkCommissionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if req.CommissionPct < 0 || req.CommissionPct > 100 {
		writeError(w, http.StatusBadRequest, "bad_pct", "yüzde 0-100 arası olmalı")
		return
	}
	in, err := parseBulkInput(orgID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_input", err.Error())
		return
	}
	if len(in.DoctorIDs) == 0 && len(in.SpecializationCodes) == 0 {
		writeError(w, http.StatusBadRequest, "missing_filter", "en az bir doktor veya branş seçilmeli")
		return
	}
	res, err := h.deps.Hakedis.BulkCreate(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("bulk commission rule failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "kayıt başarısız")
		return
	}
	// Bulk financial config change — audit it.
	h.auditAccess(r.Context(), r, "hakedis.bulk_rule", "doctor_commission_rule", "", map[string]any{
		"targeted":         res.TargetedDoctors,
		"added":            res.RulesAdded,
		"skipped":          res.Skipped,
		"category":         req.Category,
		"commission_pct":   req.CommissionPct,
		"valid_from":       req.ValidFrom,
		"specializations":  req.SpecializationCodes,
		"explicit_doctors": len(req.DoctorIDs),
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"targeted_doctors": res.TargetedDoctors,
		"rules_added":      res.RulesAdded,
		"skipped":          res.Skipped,
		"errors":           res.Errors,
	})
}

type createCommissionRuleReq struct {
	Category      string  `json:"category"`
	CommissionPct float64 `json:"commission_pct"`
	ValidFrom     string  `json:"valid_from"`
	Notes         string  `json:"notes"`
}

func (h *Handler) createCommissionRule(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	doctorID, err := uuid.Parse(chi.URLParam(r, "doctorId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_doctor", "doctorId geçersiz")
		return
	}
	var req createCommissionRuleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if req.CommissionPct < 0 || req.CommissionPct > 100 {
		writeError(w, http.StatusBadRequest, "bad_pct", "yüzde 0-100 arası olmalı")
		return
	}
	validFrom := time.Now()
	if req.ValidFrom != "" {
		t, err := time.Parse("2006-01-02", req.ValidFrom)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_date", "valid_from YYYY-MM-DD olmalı")
			return
		}
		validFrom = t
	}
	in := repo.CreateCommissionRuleInput{
		OrganizationID: orgID, DoctorID: doctorID,
		CommissionPct: req.CommissionPct,
		ValidFrom:     validFrom,
		Notes:         emptyToNil(&req.Notes),
	}
	if req.Category != "" {
		in.Category = &req.Category
	}
	rule, err := h.deps.Hakedis.CreateRule(r.Context(), in)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "rule_exists", "bu tarihte kural zaten var")
			return
		}
		h.deps.Log.Error("create commission rule failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "kayıt başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, toRulePayload(rule))
}
