package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Organization struct {
	ID              uuid.UUID
	Slug            string
	Name            string
	Kind            string
	TaxID           *string
	SGKEmployerNo   *string
	LogoURL         *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type OrganizationRepo struct {
	pool *pgxpool.Pool
}

func NewOrganizationRepo(pool *pgxpool.Pool) *OrganizationRepo {
	return &OrganizationRepo{pool: pool}
}

const orgCols = `id, slug, name, kind, tax_id, sgk_employer_no, logo_url, created_at, updated_at`

func scanOrg(row pgx.Row) (*Organization, error) {
	o := &Organization{}
	err := row.Scan(&o.ID, &o.Slug, &o.Name, &o.Kind, &o.TaxID, &o.SGKEmployerNo,
		&o.LogoURL, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
}

type CreateOrgInput struct {
	Slug          string
	Name          string
	Kind          string
	TaxID         *string
	SGKEmployerNo *string
}

func (r *OrganizationRepo) Create(ctx context.Context, in CreateOrgInput) (*Organization, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO organization (slug, name, kind, tax_id, sgk_employer_no)
		 VALUES ($1, $2, $3, $4, $5) RETURNING `+orgCols,
		in.Slug, in.Name, in.Kind, in.TaxID, in.SGKEmployerNo)
	return scanOrg(row)
}

func (r *OrganizationRepo) GetBySlug(ctx context.Context, slug string) (*Organization, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+orgCols+` FROM organization WHERE slug = $1`, slug)
	return scanOrg(row)
}

func (r *OrganizationRepo) GetByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+orgCols+` FROM organization WHERE id = $1`, id)
	return scanOrg(row)
}

func (r *OrganizationRepo) ListForUser(ctx context.Context, userID uuid.UUID) ([]Organization, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT o.id, o.slug, o.name, o.kind, o.tax_id, o.sgk_employer_no, o.logo_url,
		        o.created_at, o.updated_at
		 FROM organization o
		 JOIN org_membership m ON m.organization_id = o.id
		 WHERE m.user_id = $1 AND m.status = 'active'
		 ORDER BY o.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orgs := []Organization{}
	for rows.Next() {
		o := Organization{}
		if err := rows.Scan(&o.ID, &o.Slug, &o.Name, &o.Kind, &o.TaxID, &o.SGKEmployerNo,
			&o.LogoURL, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		orgs = append(orgs, o)
	}
	return orgs, rows.Err()
}
