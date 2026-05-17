package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/medigt/medigt/server/pkg/db/repo"
)

type servicePayload struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Code           string    `json:"code"`
	SutCode        *string   `json:"sut_code,omitempty"`
	Name           string    `json:"name"`
	Category       string    `json:"category"`
	Description    *string   `json:"description,omitempty"`
	Unit           string    `json:"unit"`
	VatRate        float64   `json:"vat_rate"`
	BasePrice      *float64  `json:"base_price,omitempty"`
	RequiresDoctor bool      `json:"requires_doctor"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func toServicePayload(s *repo.ServiceCatalog) servicePayload {
	return servicePayload{
		ID: s.ID.String(), OrganizationID: s.OrganizationID.String(),
		Code: s.Code, SutCode: s.SutCode, Name: s.Name, Category: s.Category,
		Description: s.Description, Unit: s.Unit, VatRate: s.VatRate,
		BasePrice: s.BasePrice, RequiresDoctor: s.RequiresDoctor, IsActive: s.IsActive,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

func (h *Handler) listServices(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	q := r.URL.Query()
	items, err := h.deps.Services.List(r.Context(), orgID, repo.ListServiceFilter{
		Category:   q.Get("category"),
		ActiveOnly: q.Get("active") == "true",
		Search:     q.Get("q"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]servicePayload, 0, len(items))
	for i := range items {
		out = append(out, toServicePayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type createServiceReq struct {
	Code           string   `json:"code"`
	SutCode        *string  `json:"sut_code"`
	Name           string   `json:"name"`
	Category       string   `json:"category"`
	Description    *string  `json:"description"`
	Unit           string   `json:"unit"`
	VatRate        float64  `json:"vat_rate"`
	BasePrice      *float64 `json:"base_price"`
	RequiresDoctor bool     `json:"requires_doctor"`
}

var validCategories = map[string]bool{
	"consultation": true, "lab": true, "imaging": true, "procedure": true,
	"surgery": true, "inpatient": true, "medication": true, "supply": true,
	"package": true, "other": true,
}

func (h *Handler) createService(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	var req createServiceReq
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
	if !validCategories[req.Category] {
		writeError(w, http.StatusBadRequest, "bad_category", "geçersiz kategori")
		return
	}
	svc, err := h.deps.Services.Create(r.Context(), repo.CreateServiceInput{
		OrganizationID: orgID,
		Code:           strings.ToUpper(strings.ReplaceAll(code, " ", "_")),
		SutCode:        emptyToNil(req.SutCode),
		Name:           name,
		Category:       req.Category,
		Description:    emptyToNil(req.Description),
		Unit:           req.Unit,
		VatRate:        req.VatRate,
		BasePrice:      req.BasePrice,
		RequiresDoctor: req.RequiresDoctor,
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "code_taken", "bu kod zaten kayıtlı")
			return
		}
		h.deps.Log.Error("create service failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, toServicePayload(svc))
}

// ---------- Service prices ----------

type pricePayload struct {
	ID                    string     `json:"id"`
	ServiceCatalogID      string     `json:"service_catalog_id"`
	ExternalInstitutionID *string    `json:"external_institution_id,omitempty"`
	Price                 float64    `json:"price"`
	Currency              string     `json:"currency"`
	ValidFrom             time.Time  `json:"valid_from"`
	ValidTo               *time.Time `json:"valid_to,omitempty"`
	Notes                 *string    `json:"notes,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
}

func toPricePayload(p *repo.ServicePrice) pricePayload {
	out := pricePayload{
		ID: p.ID.String(), ServiceCatalogID: p.ServiceCatalogID.String(),
		Price: p.Price, Currency: p.Currency,
		ValidFrom: p.ValidFrom, ValidTo: p.ValidTo, Notes: p.Notes,
		CreatedAt: p.CreatedAt,
	}
	if p.ExternalInstitutionID != nil {
		v := p.ExternalInstitutionID.String()
		out.ExternalInstitutionID = &v
	}
	return out
}

func (h *Handler) listServicePrices(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	svcID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	items, err := h.deps.ServicePrices.ListForService(r.Context(), orgID, svcID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]pricePayload, 0, len(items))
	for i := range items {
		out = append(out, toPricePayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type createPriceReq struct {
	ExternalInstitutionID *string `json:"external_institution_id"`
	Price                 float64 `json:"price"`
	Currency              string  `json:"currency"`
	ValidFrom             *string `json:"valid_from"` // YYYY-MM-DD
	ValidTo               *string `json:"valid_to"`
	Notes                 *string `json:"notes"`
}

func (h *Handler) createServicePrice(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	svcID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req createPriceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if req.Price <= 0 {
		writeError(w, http.StatusBadRequest, "bad_price", "fiyat 0'dan büyük olmalı")
		return
	}
	in := repo.CreatePriceInput{
		OrganizationID:   orgID,
		ServiceCatalogID: svcID,
		Price:            req.Price,
		Currency:         req.Currency,
		Notes:            emptyToNil(req.Notes),
	}
	if req.ExternalInstitutionID != nil && *req.ExternalInstitutionID != "" {
		id, err := uuid.Parse(*req.ExternalInstitutionID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_institution", "external_institution_id geçersiz")
			return
		}
		in.ExternalInstitutionID = &id
	}
	if req.ValidFrom != nil && *req.ValidFrom != "" {
		t, err := time.Parse("2006-01-02", *req.ValidFrom)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_date", "valid_from YYYY-MM-DD olmalı")
			return
		}
		in.ValidFrom = &t
	}
	if req.ValidTo != nil && *req.ValidTo != "" {
		t, err := time.Parse("2006-01-02", *req.ValidTo)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_date", "valid_to YYYY-MM-DD olmalı")
			return
		}
		in.ValidTo = &t
	}

	price, err := h.deps.ServicePrices.Create(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("create service price failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, toPricePayload(price))
}

// ---------- Bulk price update wizard ----------

type bulkPriceReq struct {
	ServiceIDs     []string `json:"service_ids"`
	Category       string   `json:"category"`
	InstitutionIDs []string `json:"institution_ids"`
	IncludeOOP     bool     `json:"include_oop"`

	Kind   string  `json:"kind"`   // percent | fixed | set
	Amount float64 `json:"amount"`

	ValidFrom string  `json:"valid_from"`
	Notes     string  `json:"notes"`
	MinPrice  float64 `json:"min_price"`
	MaxPrice  float64 `json:"max_price"`
}

var validAdjustmentKinds = map[string]repo.AdjustmentKind{
	"percent": repo.AdjPercent,
	"fixed":   repo.AdjFixedAdd,
	"set":     repo.AdjFixedSet,
}

func parseBulkPriceInput(orgID uuid.UUID, req bulkPriceReq) (repo.BulkPriceUpdateInput, error) {
	kind, ok := validAdjustmentKinds[req.Kind]
	if !ok {
		return repo.BulkPriceUpdateInput{}, fmt.Errorf("geçersiz adjustment kind: %s", req.Kind)
	}
	in := repo.BulkPriceUpdateInput{
		Filter: repo.BulkPriceUpdateFilter{
			OrganizationID: orgID,
			Category:       req.Category,
			IncludeOOP:     req.IncludeOOP,
		},
		Kind:     kind,
		Amount:   req.Amount,
		Notes:    emptyToNil(&req.Notes),
		MinPrice: req.MinPrice,
		MaxPrice: req.MaxPrice,
	}
	for _, s := range req.ServiceIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			return in, fmt.Errorf("service_id geçersiz: %s", s)
		}
		in.Filter.ServiceIDs = append(in.Filter.ServiceIDs, id)
	}
	for _, s := range req.InstitutionIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			return in, fmt.Errorf("institution_id geçersiz: %s", s)
		}
		in.Filter.InstitutionIDs = append(in.Filter.InstitutionIDs, id)
	}
	if req.ValidFrom != "" {
		t, err := time.Parse("2006-01-02", req.ValidFrom)
		if err != nil {
			return in, fmt.Errorf("valid_from YYYY-MM-DD olmalı")
		}
		in.ValidFrom = t
	} else {
		in.ValidFrom = time.Now()
	}
	return in, nil
}

type bulkPricePreviewRow struct {
	ServiceID       string  `json:"service_id"`
	ServiceCode     string  `json:"service_code"`
	ServiceName     string  `json:"service_name"`
	InstitutionID   *string `json:"institution_id,omitempty"`
	InstitutionName *string `json:"institution_name,omitempty"`
	OldPrice        float64 `json:"old_price"`
	NewPrice        float64 `json:"new_price"`
}

func toPreviewRow(r repo.BulkPriceUpdatePreviewRow) bulkPricePreviewRow {
	p := bulkPricePreviewRow{
		ServiceID: r.ServiceID.String(), ServiceCode: r.ServiceCode, ServiceName: r.ServiceName,
		InstitutionName: r.InstitutionName,
		OldPrice:        r.OldPrice, NewPrice: r.NewPrice,
	}
	if r.InstitutionID != nil {
		s := r.InstitutionID.String()
		p.InstitutionID = &s
	}
	return p
}

// previewBulkPriceUpdate returns the rows that WOULD change plus the
// computed new prices, without touching the DB. Drives the wizard's
// "X fiyat etkilenecek, ortalama Y% / ₺Z" summary screen.
func (h *Handler) previewBulkPriceUpdate(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	var req bulkPriceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	in, err := parseBulkPriceInput(orgID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_input", err.Error())
		return
	}
	rows, err := h.deps.ServicePrices.ResolveTargetRows(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("price preview failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]bulkPricePreviewRow, 0, len(rows))
	avgPct := 0.0
	totalOld, totalNew := 0.0, 0.0
	changed := 0
	for _, p := range rows {
		out = append(out, toPreviewRow(p))
		if p.OldPrice > 0 && p.NewPrice != p.OldPrice {
			avgPct += (p.NewPrice - p.OldPrice) / p.OldPrice * 100
			changed++
		}
		totalOld += p.OldPrice
		totalNew += p.NewPrice
	}
	if changed > 0 {
		avgPct /= float64(changed)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"affected":      len(rows),
		"changed":       changed,
		"avg_pct":       round2(avgPct),
		"total_old":     round2(totalOld),
		"total_new":     round2(totalNew),
		"rows":          out,
	})
}

func (h *Handler) applyBulkPriceUpdate(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	var req bulkPriceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	in, err := parseBulkPriceInput(orgID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_input", err.Error())
		return
	}
	res, err := h.deps.ServicePrices.BulkUpdate(r.Context(), in)
	if err != nil {
		h.deps.Log.Error("bulk price update failed", "err", err)
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	// Bulk price change is high-impact — finance team + auditors care.
	h.auditAccess(r.Context(), r, "fiyat.bulk_update", "service_price", "", map[string]any{
		"affected":         res.Affected,
		"inserted":         res.Inserted,
		"skipped":          res.Skipped,
		"kind":             req.Kind,
		"amount":           req.Amount,
		"valid_from":       req.ValidFrom,
		"category":         req.Category,
		"service_count":    len(req.ServiceIDs),
		"institution_count": len(req.InstitutionIDs),
		"include_oop":      req.IncludeOOP,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"affected": res.Affected,
		"inserted": res.Inserted,
		"skipped":  res.Skipped,
	})
}

func round2(n float64) float64 {
	return float64(int64(n*100+0.5)) / 100
}
