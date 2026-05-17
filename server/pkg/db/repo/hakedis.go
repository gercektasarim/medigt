package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------- Commission rule ----------

type CommissionRule struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	DoctorID       uuid.UUID
	Category       *string // NULL = all categories
	CommissionPct  float64
	ValidFrom      time.Time
	ValidTo        *time.Time
	Notes          *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type HakedisRepo struct{ pool *pgxpool.Pool }

func NewHakedisRepo(pool *pgxpool.Pool) *HakedisRepo { return &HakedisRepo{pool: pool} }

const ruleCols = `id, organization_id, doctor_id, category::text, commission_pct,
	valid_from, valid_to, notes, created_at, updated_at`

func scanRule(row pgx.Row) (*CommissionRule, error) {
	r := &CommissionRule{}
	err := row.Scan(&r.ID, &r.OrganizationID, &r.DoctorID, &r.Category, &r.CommissionPct,
		&r.ValidFrom, &r.ValidTo, &r.Notes, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return r, err
}

func (r *HakedisRepo) ListRulesForDoctor(ctx context.Context, doctorID uuid.UUID) ([]CommissionRule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+ruleCols+` FROM doctor_commission_rule
		 WHERE doctor_id = $1 ORDER BY valid_from DESC`, doctorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CommissionRule{}
	for rows.Next() {
		c := CommissionRule{}
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.DoctorID, &c.Category, &c.CommissionPct,
			&c.ValidFrom, &c.ValidTo, &c.Notes, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type CreateCommissionRuleInput struct {
	OrganizationID uuid.UUID
	DoctorID       uuid.UUID
	Category       *string
	CommissionPct  float64
	ValidFrom      time.Time
	Notes          *string
}

// BulkCreateRulesInput applies a single commission rule to many doctors at
// once. Either DoctorIDs explicitly lists targets, or
// SpecializationCodes selects every doctor with at least one matching
// specialization. Both filters may be supplied — the union is targeted.
type BulkCreateRulesInput struct {
	OrganizationID      uuid.UUID
	DoctorIDs           []uuid.UUID
	SpecializationCodes []string
	Category            *string
	CommissionPct       float64
	ValidFrom           time.Time
	Notes               *string
}

type BulkCreateRulesResult struct {
	TargetedDoctors int
	RulesAdded      int
	Skipped         int
	Errors          []string
}

// ResolveTargetDoctors returns the set of doctor IDs that match either
// explicit ids (filtered to the org) or the specialization filter. Used
// both for preview ("kaç doktor etkilenecek") and as the first step of
// BulkCreate.
func (r *HakedisRepo) ResolveTargetDoctors(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID, specCodes []string) ([]uuid.UUID, error) {
	seen := make(map[uuid.UUID]struct{})
	out := []uuid.UUID{}

	if len(ids) > 0 {
		rows, err := r.pool.Query(ctx,
			`SELECT d.id
			 FROM doctor d
			 JOIN staff_member sm ON sm.id = d.staff_member_id
			 WHERE sm.organization_id = $1 AND d.id = ANY($2)`,
			orgID, ids)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				out = append(out, id)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	if len(specCodes) > 0 {
		rows, err := r.pool.Query(ctx,
			`SELECT DISTINCT d.id
			 FROM doctor d
			 JOIN staff_member sm ON sm.id = d.staff_member_id
			 JOIN doctor_specialization ds ON ds.doctor_id = d.id
			 JOIN specialization s ON s.id = ds.specialization_id
			 WHERE sm.organization_id = $1
			   AND s.code = ANY($2)`,
			orgID, specCodes)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				out = append(out, id)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// BulkCreate applies one commission rule per resolved doctor in a single
// transaction. For each doctor it first closes any prior open rule for the
// same (doctor, category) by stamping valid_to = ValidFrom - 1 day, then
// inserts the new rule. Per-doctor failures are collected as Errors but
// do not abort the batch.
func (r *HakedisRepo) BulkCreate(ctx context.Context, in BulkCreateRulesInput) (*BulkCreateRulesResult, error) {
	doctorIDs, err := r.ResolveTargetDoctors(ctx, in.OrganizationID, in.DoctorIDs, in.SpecializationCodes)
	if err != nil {
		return nil, err
	}
	res := &BulkCreateRulesResult{TargetedDoctors: len(doctorIDs)}
	if len(doctorIDs) == 0 {
		return res, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	for _, did := range doctorIDs {
		if _, err := tx.Exec(ctx,
			`UPDATE doctor_commission_rule
			   SET valid_to = $3::DATE - INTERVAL '1 day'
			 WHERE doctor_id = $1
			   AND (category IS NOT DISTINCT FROM $2::service_category)
			   AND valid_to IS NULL
			   AND valid_from < $3::DATE`,
			did, in.Category, in.ValidFrom); err != nil {
			res.Errors = append(res.Errors, did.String()+": close prior: "+err.Error())
			res.Skipped++
			continue
		}
		ct, err := tx.Exec(ctx,
			`INSERT INTO doctor_commission_rule
			   (organization_id, doctor_id, category, commission_pct, valid_from, notes)
			 VALUES ($1, $2, $3::service_category, $4, $5, $6)
			 ON CONFLICT (doctor_id, category, valid_from) DO NOTHING`,
			in.OrganizationID, did, in.Category, in.CommissionPct, in.ValidFrom, in.Notes)
		if err != nil {
			res.Errors = append(res.Errors, did.String()+": insert: "+err.Error())
			res.Skipped++
			continue
		}
		if ct.RowsAffected() == 1 {
			res.RulesAdded++
		} else {
			res.Skipped++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return res, nil
}

func (r *HakedisRepo) CreateRule(ctx context.Context, in CreateCommissionRuleInput) (*CommissionRule, error) {
	// Close any open prior rule for the same (doctor, category) by setting
	// valid_to = new rule's valid_from - 1 day.
	if _, err := r.pool.Exec(ctx,
		`UPDATE doctor_commission_rule
		   SET valid_to = $3::DATE - INTERVAL '1 day'
		 WHERE doctor_id = $1
		   AND (category IS NOT DISTINCT FROM $2::service_category)
		   AND valid_to IS NULL
		   AND valid_from < $3::DATE`,
		in.DoctorID, in.Category, in.ValidFrom); err != nil {
		return nil, err
	}
	row := r.pool.QueryRow(ctx,
		`INSERT INTO doctor_commission_rule (organization_id, doctor_id, category, commission_pct, valid_from, notes)
		 VALUES ($1, $2, $3::service_category, $4, $5, $6)
		 RETURNING `+ruleCols,
		in.OrganizationID, in.DoctorID, in.Category, in.CommissionPct, in.ValidFrom, in.Notes)
	return scanRule(row)
}

// ---------- Earnings rollup ----------

type DoctorEarningSummary struct {
	DoctorID         uuid.UUID
	StaffFirstName   string
	StaffLastName    string
	StaffTitle       *string
	ItemCount        int
	GrossRevenue     float64
	EarningTotal     float64
}

// SummaryByDoctor aggregates paid invoice_items whose doctor_id is set,
// computing earnings per doctor by applying the most specific commission
// rule (category-match wins over NULL-category) valid as of each item's
// invoice created_at.
//
// Bracketing rules: a rule covers a row when row's invoice.created_at is
// in [valid_from, valid_to] (inclusive both ends; NULL valid_to = open).
func (r *HakedisRepo) SummaryByDoctor(ctx context.Context, branchID uuid.UUID, from, to time.Time) ([]DoctorEarningSummary, error) {
	rows, err := r.pool.Query(ctx,
		`WITH rows AS (
		   SELECT
		     it.doctor_id,
		     it.line_total,
		     s.category::text AS cat,
		     i.created_at::date AS at
		   FROM invoice_item it
		   JOIN invoice i ON i.id = it.invoice_id
		   LEFT JOIN service_catalog s ON s.id = it.service_id
		   WHERE i.branch_id = $1
		     AND i.status = 'paid'
		     AND it.doctor_id IS NOT NULL
		     AND i.created_at >= $2 AND i.created_at < $3
		 ),
		 with_rule AS (
		   SELECT r.doctor_id, r.line_total,
		          COALESCE(
		            (SELECT rr.commission_pct FROM doctor_commission_rule rr
		             WHERE rr.doctor_id = r.doctor_id
		               AND rr.category::text = r.cat
		               AND rr.valid_from <= r.at
		               AND (rr.valid_to IS NULL OR rr.valid_to >= r.at)
		             ORDER BY rr.valid_from DESC LIMIT 1),
		            (SELECT rr.commission_pct FROM doctor_commission_rule rr
		             WHERE rr.doctor_id = r.doctor_id
		               AND rr.category IS NULL
		               AND rr.valid_from <= r.at
		               AND (rr.valid_to IS NULL OR rr.valid_to >= r.at)
		             ORDER BY rr.valid_from DESC LIMIT 1),
		            0
		          ) AS pct
		   FROM rows r
		 )
		 SELECT wr.doctor_id, sm.first_name, sm.last_name, sm.title,
		        COUNT(*)::int AS item_count,
		        COALESCE(SUM(wr.line_total), 0) AS gross_revenue,
		        COALESCE(SUM(wr.line_total * wr.pct / 100.0), 0) AS earning_total
		 FROM with_rule wr
		 JOIN doctor d ON d.id = wr.doctor_id
		 JOIN staff_member sm ON sm.id = d.staff_member_id
		 GROUP BY wr.doctor_id, sm.first_name, sm.last_name, sm.title
		 ORDER BY earning_total DESC`,
		branchID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DoctorEarningSummary{}
	for rows.Next() {
		s := DoctorEarningSummary{}
		if err := rows.Scan(&s.DoctorID, &s.StaffFirstName, &s.StaffLastName, &s.StaffTitle,
			&s.ItemCount, &s.GrossRevenue, &s.EarningTotal); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DoctorEarningItem is one paid invoice line attributed to a doctor.
type DoctorEarningItem struct {
	InvoiceItemID    uuid.UUID
	InvoiceID        uuid.UUID
	InvoiceNo        string
	IssuedAt         time.Time
	PatientMRN       string
	PatientFirstName string
	PatientLastName  string
	Code             string
	Name             string
	Category         *string
	LineTotal        float64
	CommissionPct    float64
	Earning          float64
}

func (r *HakedisRepo) ItemsForDoctor(ctx context.Context, branchID, doctorID uuid.UUID, from, to time.Time) ([]DoctorEarningItem, error) {
	rows, err := r.pool.Query(ctx,
		`WITH rows AS (
		   SELECT
		     it.id AS item_id, it.invoice_id, i.invoice_no, i.created_at AS issued,
		     p.mrn, p.first_name, p.last_name,
		     it.code, it.name, s.category::text AS cat,
		     it.line_total, i.created_at::date AS at, it.doctor_id
		   FROM invoice_item it
		   JOIN invoice i ON i.id = it.invoice_id
		   JOIN patient p ON p.id = i.patient_id
		   LEFT JOIN service_catalog s ON s.id = it.service_id
		   WHERE i.branch_id = $1
		     AND i.status = 'paid'
		     AND it.doctor_id = $2
		     AND i.created_at >= $3 AND i.created_at < $4
		 ),
		 with_rule AS (
		   SELECT r.*,
		          COALESCE(
		            (SELECT rr.commission_pct FROM doctor_commission_rule rr
		             WHERE rr.doctor_id = r.doctor_id
		               AND rr.category::text = r.cat
		               AND rr.valid_from <= r.at
		               AND (rr.valid_to IS NULL OR rr.valid_to >= r.at)
		             ORDER BY rr.valid_from DESC LIMIT 1),
		            (SELECT rr.commission_pct FROM doctor_commission_rule rr
		             WHERE rr.doctor_id = r.doctor_id
		               AND rr.category IS NULL
		               AND rr.valid_from <= r.at
		               AND (rr.valid_to IS NULL OR rr.valid_to >= r.at)
		             ORDER BY rr.valid_from DESC LIMIT 1),
		            0
		          ) AS pct
		   FROM rows r
		 )
		 SELECT item_id, invoice_id, invoice_no, issued, mrn, first_name, last_name,
		        code, name, cat, line_total, pct, (line_total * pct / 100.0) AS earning
		 FROM with_rule
		 ORDER BY issued DESC`,
		branchID, doctorID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DoctorEarningItem{}
	for rows.Next() {
		x := DoctorEarningItem{}
		if err := rows.Scan(&x.InvoiceItemID, &x.InvoiceID, &x.InvoiceNo, &x.IssuedAt,
			&x.PatientMRN, &x.PatientFirstName, &x.PatientLastName,
			&x.Code, &x.Name, &x.Category, &x.LineTotal, &x.CommissionPct, &x.Earning); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
