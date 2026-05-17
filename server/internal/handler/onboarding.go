package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// onboardingStatus is the per-org checklist progress. The frontend
// /baslangic page renders this; admins see each box go green as the
// org gets fleshed out.
type onboardingStatus struct {
	OrgID                 string `json:"organization_id"`
	BranchID              string `json:"branch_id"`
	SpecializationsCount  int    `json:"specializations_count"`  // includes system seed
	InstitutionsCount     int    `json:"institutions_count"`
	ServicesCount         int    `json:"services_count"`
	DoctorsCount          int    `json:"doctors_count"`
	PatientsCount         int    `json:"patients_count"`
	WarehousesCount       int    `json:"warehouses_count"`
	CashRegistersCount    int    `json:"cash_registers_count"`
	Icd10SystemCount      int    `json:"icd10_system_count"`
}

func (h *Handler) getOnboardingStatus(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}

	type countQ struct {
		key   string
		query string
		args  []any
	}
	queries := []countQ{
		{"spec", `SELECT COUNT(*) FROM specialization WHERE organization_id IS NULL OR organization_id = $1`, []any{orgID}},
		{"inst", `SELECT COUNT(*) FROM external_institution WHERE organization_id = $1`, []any{orgID}},
		{"svc", `SELECT COUNT(*) FROM service_catalog WHERE organization_id = $1`, []any{orgID}},
		{"doc", `SELECT COUNT(*) FROM doctor d JOIN staff_member s ON s.id = d.staff_member_id WHERE s.organization_id = $1`, []any{orgID}},
		{"pat", `SELECT COUNT(*) FROM patient WHERE organization_id = $1`, []any{orgID}},
		{"wh", `SELECT COUNT(*) FROM warehouse WHERE branch_id = $1`, []any{branchID}},
		{"kasa", `SELECT COUNT(*) FROM cash_register WHERE branch_id = $1`, []any{branchID}},
		{"icd10", `SELECT COUNT(*) FROM icd10_code WHERE organization_id IS NULL AND is_system = TRUE`, []any{}},
	}
	counts := map[string]int{}
	for _, q := range queries {
		var n int
		if err := h.deps.Pool.QueryRow(r.Context(), q.query, q.args...).Scan(&n); err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		counts[q.key] = n
	}

	writeJSON(w, http.StatusOK, onboardingStatus{
		OrgID: orgID.String(), BranchID: branchID.String(),
		SpecializationsCount: counts["spec"],
		InstitutionsCount:    counts["inst"],
		ServicesCount:        counts["svc"],
		DoctorsCount:         counts["doc"],
		PatientsCount:        counts["pat"],
		WarehousesCount:      counts["wh"],
		CashRegistersCount:   counts["kasa"],
		Icd10SystemCount:     counts["icd10"],
	})
}

// seedDefaultsResp tells the UI exactly what each call inserted, so
// re-clicking the button is safe (idempotent counts go to zero).
type seedDefaultsResp struct {
	InstitutionsAdded int `json:"institutions_added"`
	ServicesAdded     int `json:"services_added"`
	WarehousesAdded   int `json:"warehouses_added"`
}

func (h *Handler) seedOnboardingDefaults(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	ctx := r.Context()
	out := seedDefaultsResp{}

	out.InstitutionsAdded = seedInstitutions(ctx, h, orgID)
	out.ServicesAdded = seedServices(ctx, h, orgID)
	out.WarehousesAdded = seedWarehouses(ctx, h, orgID, branchID)

	writeJSON(w, http.StatusOK, out)
}

// seedInstitutions inserts SGK + Cepten Ödeme if missing.
func seedInstitutions(ctx context.Context, h *Handler, orgID uuid.UUID) int {
	type inst struct{ code, name, kind string }
	defaults := []inst{
		{"SGK", "Sosyal Güvenlik Kurumu", "sgk"},
		{"CEPTEN", "Cepten Ödeme", "oop"},
	}
	added := 0
	for _, d := range defaults {
		tag, err := h.deps.Pool.Exec(ctx,
			`INSERT INTO external_institution (organization_id, code, name, kind)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (organization_id, code) DO NOTHING`,
			orgID, d.code, d.name, d.kind)
		if err == nil && tag.RowsAffected() > 0 {
			added++
		}
	}
	return added
}

// seedServices inserts a tiny set of universally-used services so newly
// created orgs can immediately start billing.
func seedServices(ctx context.Context, h *Handler, orgID uuid.UUID) int {
	type svc struct {
		code, name, category, unit string
		vat                        float64
		basePrice                  float64
		requiresDoctor             bool
	}
	defaults := []svc{
		{"MUAYENE_GENEL", "Genel Muayene", "consultation", "adet", 10, 500, true},
		{"MUAYENE_KONTROL", "Kontrol Muayenesi", "consultation", "adet", 10, 250, true},
		{"PANSUMAN", "Pansuman", "procedure", "adet", 10, 150, false},
		{"ENJEKSIYON_IM", "İntramüsküler Enjeksiyon", "procedure", "adet", 10, 50, false},
		{"ENJEKSIYON_IV", "İntravenöz Enjeksiyon", "procedure", "adet", 10, 60, false},
		{"SERUM_TAKILMA", "Serum Takılma + Takip", "procedure", "adet", 10, 250, false},
		{"YATAK_STANDART_GUN", "Standart Yatak / Gün", "inpatient", "gün", 10, 1500, false},
		{"YATAK_VIP_GUN", "VIP Yatak / Gün", "inpatient", "gün", 10, 4000, false},
		{"REFAKATCI_GUN", "Refakatçi / Gün", "inpatient", "gün", 10, 250, false},
		{"AMBULANS_TRANSFER", "Ambulans Transfer", "procedure", "adet", 10, 1500, false},
	}
	added := 0
	for _, d := range defaults {
		tag, err := h.deps.Pool.Exec(ctx,
			`INSERT INTO service_catalog
			   (organization_id, code, name, category, unit, vat_rate,
			    base_price, requires_doctor, is_active)
			 VALUES ($1, $2, $3, $4::service_category, $5, $6, $7, $8, TRUE)
			 ON CONFLICT (organization_id, code) DO NOTHING`,
			orgID, d.code, d.name, d.category, d.unit, d.vat, d.basePrice, d.requiresDoctor)
		if err == nil && tag.RowsAffected() > 0 {
			added++
		}
	}
	return added
}

// seedWarehouses inserts a single "Eczane Deposu" so the pharmacy module
// is immediately functional. Branch-scoped.
func seedWarehouses(ctx context.Context, h *Handler, orgID, branchID uuid.UUID) int {
	tag, err := h.deps.Pool.Exec(ctx,
		`INSERT INTO warehouse (organization_id, branch_id, code, name, kind)
		 VALUES ($1, $2, 'ECZ-1', 'Eczane Deposu', 'pharmacy')
		 ON CONFLICT (branch_id, code) DO NOTHING`,
		orgID, branchID)
	if err == nil && tag.RowsAffected() > 0 {
		return 1
	}
	return 0
}
