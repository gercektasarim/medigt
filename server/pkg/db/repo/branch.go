package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Branch struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	Slug             string
	Name             string
	Kind             string
	Address          *string
	Phone            *string
	SGKFacilityCode  *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type BranchRepo struct {
	pool *pgxpool.Pool
}

func NewBranchRepo(pool *pgxpool.Pool) *BranchRepo {
	return &BranchRepo{pool: pool}
}

const branchCols = `id, organization_id, slug, name, kind, address, phone,
	sgk_facility_code, created_at, updated_at`

func scanBranch(row pgx.Row) (*Branch, error) {
	b := &Branch{}
	err := row.Scan(&b.ID, &b.OrganizationID, &b.Slug, &b.Name, &b.Kind, &b.Address,
		&b.Phone, &b.SGKFacilityCode, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

type CreateBranchInput struct {
	OrganizationID   uuid.UUID
	Slug             string
	Name             string
	Kind             string
	Address          *string
	Phone            *string
	SGKFacilityCode  *string
}

func (r *BranchRepo) Create(ctx context.Context, in CreateBranchInput) (*Branch, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO branch (organization_id, slug, name, kind, address, phone, sgk_facility_code)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+branchCols,
		in.OrganizationID, in.Slug, in.Name, in.Kind, in.Address, in.Phone, in.SGKFacilityCode)
	return scanBranch(row)
}

func (r *BranchRepo) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Branch, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+branchCols+` FROM branch WHERE organization_id = $1 AND slug = $2`,
		orgID, slug)
	return scanBranch(row)
}

func (r *BranchRepo) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Branch, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+branchCols+` FROM branch WHERE organization_id = $1 ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	branches := []Branch{}
	for rows.Next() {
		b := Branch{}
		if err := rows.Scan(&b.ID, &b.OrganizationID, &b.Slug, &b.Name, &b.Kind,
			&b.Address, &b.Phone, &b.SGKFacilityCode, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		branches = append(branches, b)
	}
	return branches, rows.Err()
}

// ListAccessibleForUser returns branches the user has any assignment to within the org.
func (r *BranchRepo) ListAccessibleForUser(ctx context.Context, userID, orgID uuid.UUID) ([]Branch, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT b.id, b.organization_id, b.slug, b.name, b.kind, b.address, b.phone,
		        b.sgk_facility_code, b.created_at, b.updated_at
		 FROM branch b
		 WHERE b.organization_id = $1
		   AND (
		     EXISTS (SELECT 1 FROM branch_assignment ba
		             WHERE ba.branch_id = b.id AND ba.user_id = $2)
		     OR EXISTS (SELECT 1 FROM org_membership m
		             WHERE m.organization_id = b.organization_id AND m.user_id = $2
		             AND m.system_role IN ('platform_admin', 'org_owner', 'org_admin'))
		   )
		 ORDER BY b.name`,
		orgID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	branches := []Branch{}
	for rows.Next() {
		b := Branch{}
		if err := rows.Scan(&b.ID, &b.OrganizationID, &b.Slug, &b.Name, &b.Kind,
			&b.Address, &b.Phone, &b.SGKFacilityCode, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		branches = append(branches, b)
	}
	return branches, rows.Err()
}
