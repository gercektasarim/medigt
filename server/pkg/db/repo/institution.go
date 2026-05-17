package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExternalInstitution struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	Code             string
	Name             string
	Kind             string
	TaxID            *string
	Address          *string
	Phone            *string
	Email            *string
	ContractNo       *string
	ContractStartsAt *time.Time
	ContractEndsAt   *time.Time
	IsActive         bool
	Notes            *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type InstitutionRepo struct {
	pool *pgxpool.Pool
}

func NewInstitutionRepo(pool *pgxpool.Pool) *InstitutionRepo {
	return &InstitutionRepo{pool: pool}
}

const instCols = `id, organization_id, code, name, kind, tax_id, address, phone, email,
	contract_no, contract_starts_at, contract_ends_at, is_active, notes,
	created_at, updated_at`

func scanInst(row pgx.Row) (*ExternalInstitution, error) {
	i := &ExternalInstitution{}
	err := row.Scan(&i.ID, &i.OrganizationID, &i.Code, &i.Name, &i.Kind, &i.TaxID,
		&i.Address, &i.Phone, &i.Email, &i.ContractNo, &i.ContractStartsAt,
		&i.ContractEndsAt, &i.IsActive, &i.Notes, &i.CreatedAt, &i.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return i, err
}

type CreateInstitutionInput struct {
	OrganizationID uuid.UUID
	Code           string
	Name           string
	Kind           string
	TaxID          *string
	Phone          *string
	Email          *string
	Address        *string
	ContractNo     *string
	Notes          *string
}

func (r *InstitutionRepo) Create(ctx context.Context, in CreateInstitutionInput) (*ExternalInstitution, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO external_institution
		   (organization_id, code, name, kind, tax_id, phone, email, address, contract_no, notes)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 RETURNING `+instCols,
		in.OrganizationID, in.Code, in.Name, in.Kind, in.TaxID,
		in.Phone, in.Email, in.Address, in.ContractNo, in.Notes)
	return scanInst(row)
}

func (r *InstitutionRepo) List(ctx context.Context, orgID uuid.UUID, activeOnly bool) ([]ExternalInstitution, error) {
	q := `SELECT ` + instCols + ` FROM external_institution WHERE organization_id = $1`
	if activeOnly {
		q += ` AND is_active = TRUE`
	}
	q += ` ORDER BY name`
	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExternalInstitution{}
	for rows.Next() {
		i := ExternalInstitution{}
		if err := rows.Scan(&i.ID, &i.OrganizationID, &i.Code, &i.Name, &i.Kind, &i.TaxID,
			&i.Address, &i.Phone, &i.Email, &i.ContractNo, &i.ContractStartsAt,
			&i.ContractEndsAt, &i.IsActive, &i.Notes, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (r *InstitutionRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (*ExternalInstitution, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+instCols+` FROM external_institution WHERE organization_id = $1 AND id = $2`,
		orgID, id)
	return scanInst(row)
}
