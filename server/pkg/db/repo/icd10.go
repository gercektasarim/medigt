package repo

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Icd10Code struct {
	ID             uuid.UUID
	OrganizationID *uuid.UUID
	Code           string
	TitleTR        string
	TitleEN        *string
	ParentCode     *string
	Chapter        *string
	IsActive       bool
	IsSystem       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Icd10Repo struct {
	pool *pgxpool.Pool
}

func NewIcd10Repo(pool *pgxpool.Pool) *Icd10Repo {
	return &Icd10Repo{pool: pool}
}

const icdCols = `id, organization_id, code, title_tr, title_en, parent_code,
	chapter, is_active, is_system, created_at, updated_at`

func scanIcd(row pgx.Row) (*Icd10Code, error) {
	c := &Icd10Code{}
	err := row.Scan(&c.ID, &c.OrganizationID, &c.Code, &c.TitleTR, &c.TitleEN,
		&c.ParentCode, &c.Chapter, &c.IsActive, &c.IsSystem,
		&c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

// Search returns up to `limit` codes matching the query. When q is non-empty
// we prefer prefix matches on the code (so "I10" surfaces above "AI10..."),
// then fall back to ILIKE on the Turkish title. System catalog
// (organization_id NULL) plus the org's own codes are both included.
func (r *Icd10Repo) Search(ctx context.Context, orgID uuid.UUID, q string, limit int) ([]Icd10Code, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q = strings.TrimSpace(q)
	if q == "" {
		rows, err := r.pool.Query(ctx,
			`SELECT `+icdCols+` FROM icd10_code
			 WHERE (organization_id IS NULL OR organization_id = $1) AND is_active = TRUE
			 ORDER BY code LIMIT $2`, orgID, limit)
		return collectIcd(rows, err)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+icdCols+` FROM icd10_code
		 WHERE (organization_id IS NULL OR organization_id = $1) AND is_active = TRUE
		   AND (code ILIKE $2 OR title_tr ILIKE $3)
		 ORDER BY (code ILIKE $2) DESC, code
		 LIMIT $4`,
		orgID, q+"%", "%"+q+"%", limit)
	return collectIcd(rows, err)
}

func collectIcd(rows pgx.Rows, err error) ([]Icd10Code, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Icd10Code{}
	for rows.Next() {
		c := Icd10Code{}
		if scanErr := rows.Scan(&c.ID, &c.OrganizationID, &c.Code, &c.TitleTR, &c.TitleEN,
			&c.ParentCode, &c.Chapter, &c.IsActive, &c.IsSystem,
			&c.CreatedAt, &c.UpdatedAt); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (r *Icd10Repo) GetByCode(ctx context.Context, orgID uuid.UUID, code string) (*Icd10Code, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+icdCols+` FROM icd10_code
		 WHERE (organization_id IS NULL OR organization_id = $1) AND code = $2
		 ORDER BY organization_id NULLS LAST LIMIT 1`,
		orgID, code)
	return scanIcd(row)
}

type CreateIcd10Input struct {
	OrganizationID *uuid.UUID
	Code           string
	TitleTR        string
	TitleEN        *string
	ParentCode     *string
	Chapter        *string
}

func (r *Icd10Repo) Create(ctx context.Context, in CreateIcd10Input) (*Icd10Code, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO icd10_code (organization_id, code, title_tr, title_en, parent_code, chapter, is_system)
		 VALUES ($1, $2, $3, $4, $5, $6, FALSE)
		 RETURNING `+icdCols,
		in.OrganizationID, in.Code, in.TitleTR, in.TitleEN, in.ParentCode, in.Chapter)
	return scanIcd(row)
}

// BulkUpsertSystem inserts/updates *system* ICD-10 rows in batches.
// organization_id stays NULL (shared across all tenants). Returns
// counts: inserted (new rows), updated (title or chapter changed),
// skipped (no-op).
func (r *Icd10Repo) BulkUpsertSystem(ctx context.Context, in []CreateIcd10Input) (inserted, updated int, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, c := range in {
		// is_system rows live with organization_id = NULL; the schema's
		// UNIQUE NULLS NOT DISTINCT (organization_id, code) collapses
		// NULL+code pairs so ON CONFLICT works without a partial index.
		var wasInserted bool
		err := tx.QueryRow(ctx,
			`INSERT INTO icd10_code (organization_id, code, title_tr, title_en, parent_code, chapter, is_system)
			 VALUES (NULL, $1, $2, $3, $4, $5, TRUE)
			 ON CONFLICT (organization_id, code)
			 DO UPDATE SET title_tr = EXCLUDED.title_tr,
			               title_en = COALESCE(EXCLUDED.title_en, icd10_code.title_en),
			               parent_code = COALESCE(EXCLUDED.parent_code, icd10_code.parent_code),
			               chapter = COALESCE(EXCLUDED.chapter, icd10_code.chapter),
			               updated_at = NOW()
			 RETURNING (xmax = 0) AS inserted`,
			c.Code, c.TitleTR, c.TitleEN, c.ParentCode, c.Chapter).Scan(&wasInserted)
		if err != nil {
			return inserted, updated, err
		}
		if wasInserted {
			inserted++
		} else {
			updated++
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return inserted, updated, err
	}
	return inserted, updated, nil
}
