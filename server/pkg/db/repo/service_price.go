package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ServicePrice struct {
	ID                    uuid.UUID
	OrganizationID        uuid.UUID
	ServiceCatalogID      uuid.UUID
	ExternalInstitutionID *uuid.UUID
	Price                 float64
	Currency              string
	ValidFrom             time.Time
	ValidTo               *time.Time
	Notes                 *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type ServicePriceRepo struct {
	pool *pgxpool.Pool
}

func NewServicePriceRepo(pool *pgxpool.Pool) *ServicePriceRepo {
	return &ServicePriceRepo{pool: pool}
}

const priceCols = `id, organization_id, service_catalog_id, external_institution_id,
	price, currency, valid_from, valid_to, notes, created_at, updated_at`

func scanPrice(row pgx.Row) (*ServicePrice, error) {
	p := &ServicePrice{}
	err := row.Scan(&p.ID, &p.OrganizationID, &p.ServiceCatalogID, &p.ExternalInstitutionID,
		&p.Price, &p.Currency, &p.ValidFrom, &p.ValidTo, &p.Notes,
		&p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

type CreatePriceInput struct {
	OrganizationID        uuid.UUID
	ServiceCatalogID      uuid.UUID
	ExternalInstitutionID *uuid.UUID
	Price                 float64
	Currency              string
	ValidFrom             *time.Time
	ValidTo               *time.Time
	Notes                 *string
}

func (r *ServicePriceRepo) Create(ctx context.Context, in CreatePriceInput) (*ServicePrice, error) {
	if in.Currency == "" {
		in.Currency = "TRY"
	}
	row := r.pool.QueryRow(ctx,
		`INSERT INTO service_price (organization_id, service_catalog_id, external_institution_id,
		   price, currency, valid_from, valid_to, notes)
		 VALUES ($1, $2, $3, $4, $5, COALESCE($6, CURRENT_DATE), $7, $8)
		 RETURNING `+priceCols,
		in.OrganizationID, in.ServiceCatalogID, in.ExternalInstitutionID,
		in.Price, in.Currency, in.ValidFrom, in.ValidTo, in.Notes)
	return scanPrice(row)
}

// ---------- Bulk price update ----------
//
// The vezne admin's #1 ask: "Hizmet kataloğunun tamamına / belirli bir
// kategoriye / belirli kurumlara %10 zam uygula." The current-price view
// is "the row with MAX(valid_from) ≤ today and (valid_to IS NULL OR
// valid_to ≥ today)" per (service, institution). Bulk update derives
// new prices from that view, opens the previous row (stamps valid_to =
// new.valid_from - 1 day), and inserts the new rows in one transaction.

type AdjustmentKind string

const (
	AdjPercent  AdjustmentKind = "percent"  // amount = 10 → +10%
	AdjFixedAdd AdjustmentKind = "fixed"    // amount = 50 → +50 TL (signed)
	AdjFixedSet AdjustmentKind = "set"      // amount = 250 → set to 250 TL
)

type BulkPriceUpdateFilter struct {
	OrganizationID  uuid.UUID
	ServiceIDs      []uuid.UUID // empty = all services
	Category        string      // empty = all categories
	InstitutionIDs  []uuid.UUID // empty = all institutions matching IncludeOOP
	IncludeOOP      bool        // include null-institution (cepten) rows
}

type BulkPriceUpdateInput struct {
	Filter    BulkPriceUpdateFilter
	Kind      AdjustmentKind
	Amount    float64
	ValidFrom time.Time
	Notes     *string
	// MinPrice + MaxPrice cap the new price so a bad percentage doesn't
	// produce nonsense (e.g. accidentally setting prices to 0).
	MinPrice float64
	MaxPrice float64 // 0 = unbounded
}

type BulkPriceUpdatePreviewRow struct {
	ServiceID       uuid.UUID
	ServiceCode     string
	ServiceName     string
	InstitutionID   *uuid.UUID
	InstitutionName *string
	OldPrice        float64
	NewPrice        float64
}

type BulkPriceUpdateResult struct {
	Affected int
	Inserted int
	Skipped  int
}

// ResolveTargetRows returns the current (service, institution) rows that
// match the filter, plus the computed new price. Used for the live
// preview AND as the first phase of Apply — same SQL keeps preview and
// commit in lockstep.
func (r *ServicePriceRepo) ResolveTargetRows(ctx context.Context, in BulkPriceUpdateInput) ([]BulkPriceUpdatePreviewRow, error) {
	where, args := buildBulkWhere(in.Filter)

	q := `
		WITH current_prices AS (
			SELECT DISTINCT ON (sp.service_catalog_id, COALESCE(sp.external_institution_id, '00000000-0000-0000-0000-000000000000'::uuid))
				sp.service_catalog_id, sp.external_institution_id, sp.price
			FROM service_price sp
			WHERE sp.organization_id = $1
			  AND sp.valid_from <= CURRENT_DATE
			  AND (sp.valid_to IS NULL OR sp.valid_to >= CURRENT_DATE)
			ORDER BY sp.service_catalog_id,
			         COALESCE(sp.external_institution_id, '00000000-0000-0000-0000-000000000000'::uuid),
			         sp.valid_from DESC
		)
		SELECT cp.service_catalog_id, sc.code, sc.name,
		       cp.external_institution_id, ei.name,
		       cp.price
		FROM current_prices cp
		JOIN service_catalog sc ON sc.id = cp.service_catalog_id
		LEFT JOIN external_institution ei ON ei.id = cp.external_institution_id
		` + where + `
		ORDER BY sc.code, ei.name NULLS FIRST`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BulkPriceUpdatePreviewRow{}
	for rows.Next() {
		var row BulkPriceUpdatePreviewRow
		if err := rows.Scan(&row.ServiceID, &row.ServiceCode, &row.ServiceName,
			&row.InstitutionID, &row.InstitutionName, &row.OldPrice); err != nil {
			return nil, err
		}
		row.NewPrice = applyAdjustment(row.OldPrice, in.Kind, in.Amount, in.MinPrice, in.MaxPrice)
		out = append(out, row)
	}
	return out, rows.Err()
}

// BulkUpdate applies the resolved rows transactionally. For each
// (service, institution) pair we close the open row (stamp valid_to =
// validFrom - 1 day) and insert a new row at validFrom with the new
// price. Rows whose new price equals the old (e.g. clamped to MinPrice
// at floor) are skipped — no audit churn.
func (r *ServicePriceRepo) BulkUpdate(ctx context.Context, in BulkPriceUpdateInput) (*BulkPriceUpdateResult, error) {
	targets, err := r.ResolveTargetRows(ctx, in)
	if err != nil {
		return nil, err
	}
	res := &BulkPriceUpdateResult{Affected: len(targets)}
	if len(targets) == 0 {
		return res, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	for _, t := range targets {
		if t.NewPrice == t.OldPrice {
			res.Skipped++
			continue
		}
		// Close the prior open row(s) for this (service, institution).
		if _, err := tx.Exec(ctx,
			`UPDATE service_price
			   SET valid_to = $4::DATE - INTERVAL '1 day'
			 WHERE organization_id = $1
			   AND service_catalog_id = $2
			   AND external_institution_id IS NOT DISTINCT FROM $3
			   AND valid_to IS NULL
			   AND valid_from < $4::DATE`,
			in.Filter.OrganizationID, t.ServiceID, t.InstitutionID, in.ValidFrom); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO service_price
			   (organization_id, service_catalog_id, external_institution_id,
			    price, currency, valid_from, notes)
			 VALUES ($1, $2, $3, $4, 'TRY', $5, $6)`,
			in.Filter.OrganizationID, t.ServiceID, t.InstitutionID,
			t.NewPrice, in.ValidFrom, in.Notes); err != nil {
			return nil, err
		}
		res.Inserted++
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return res, nil
}

// applyAdjustment computes the new price from the old one. Cap at min/max
// (max=0 means unbounded). Rounded to 2 decimals to match NUMERIC(12,2).
func applyAdjustment(old float64, kind AdjustmentKind, amount, minPrice, maxPrice float64) float64 {
	var n float64
	switch kind {
	case AdjPercent:
		n = old * (1 + amount/100)
	case AdjFixedAdd:
		n = old + amount
	case AdjFixedSet:
		n = amount
	default:
		n = old
	}
	if minPrice > 0 && n < minPrice {
		n = minPrice
	}
	if maxPrice > 0 && n > maxPrice {
		n = maxPrice
	}
	// Round to 2 decimal places.
	return float64(int64(n*100+0.5)) / 100
}

// buildBulkWhere appends the filter clauses to the "current_prices" CTE.
// $1 is reserved for organization_id at the CTE level.
func buildBulkWhere(f BulkPriceUpdateFilter) (string, []any) {
	args := []any{f.OrganizationID}
	clauses := []string{}
	idx := 2

	if len(f.ServiceIDs) > 0 {
		clauses = append(clauses, "cp.service_catalog_id = ANY($"+itoa(idx)+")")
		args = append(args, f.ServiceIDs)
		idx++
	}
	if f.Category != "" {
		clauses = append(clauses, "sc.category::text = $"+itoa(idx))
		args = append(args, f.Category)
		idx++
	}
	if len(f.InstitutionIDs) > 0 && f.IncludeOOP {
		clauses = append(clauses,
			"(cp.external_institution_id = ANY($"+itoa(idx)+") OR cp.external_institution_id IS NULL)")
		args = append(args, f.InstitutionIDs)
		idx++
	} else if len(f.InstitutionIDs) > 0 {
		clauses = append(clauses, "cp.external_institution_id = ANY($"+itoa(idx)+")")
		args = append(args, f.InstitutionIDs)
		idx++
	} else if !f.IncludeOOP {
		// No institution filter but caller doesn't want OOP — exclude NULL rows.
		clauses = append(clauses, "cp.external_institution_id IS NOT NULL")
	}

	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + clauses[0]
		for _, c := range clauses[1:] {
			where += " AND " + c
		}
	}
	return where, args
}

// ListForService returns all current + historic prices for one service in the org.
func (r *ServicePriceRepo) ListForService(ctx context.Context, orgID, serviceID uuid.UUID) ([]ServicePrice, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+priceCols+` FROM service_price
		 WHERE organization_id = $1 AND service_catalog_id = $2
		 ORDER BY valid_from DESC`,
		orgID, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ServicePrice{}
	for rows.Next() {
		p := ServicePrice{}
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.ServiceCatalogID, &p.ExternalInstitutionID,
			&p.Price, &p.Currency, &p.ValidFrom, &p.ValidTo, &p.Notes,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
