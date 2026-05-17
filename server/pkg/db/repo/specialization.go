package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Specialization struct {
	ID             uuid.UUID
	OrganizationID *uuid.UUID
	Code           string
	Name           string
	ParentID       *uuid.UUID
	IsSystem       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type SpecializationRepo struct {
	pool *pgxpool.Pool
}

func NewSpecializationRepo(pool *pgxpool.Pool) *SpecializationRepo {
	return &SpecializationRepo{pool: pool}
}

const specCols = `id, organization_id, code, name, parent_id, is_system, created_at, updated_at`

func scanSpec(row pgx.Row) (*Specialization, error) {
	s := &Specialization{}
	err := row.Scan(&s.ID, &s.OrganizationID, &s.Code, &s.Name, &s.ParentID,
		&s.IsSystem, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

type CreateSpecializationInput struct {
	OrganizationID *uuid.UUID
	Code           string
	Name           string
	ParentID       *uuid.UUID
}

func (r *SpecializationRepo) Create(ctx context.Context, in CreateSpecializationInput) (*Specialization, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO specialization (organization_id, code, name, parent_id, is_system)
		 VALUES ($1, $2, $3, $4, FALSE) RETURNING `+specCols,
		in.OrganizationID, in.Code, in.Name, in.ParentID)
	return scanSpec(row)
}

// List returns system catalog plus org-specific entries.
func (r *SpecializationRepo) List(ctx context.Context, orgID uuid.UUID) ([]Specialization, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+specCols+` FROM specialization
		 WHERE organization_id IS NULL OR organization_id = $1
		 ORDER BY name`,
		orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Specialization{}
	for rows.Next() {
		s := Specialization{}
		if err := rows.Scan(&s.ID, &s.OrganizationID, &s.Code, &s.Name, &s.ParentID,
			&s.IsSystem, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *SpecializationRepo) GetByID(ctx context.Context, id uuid.UUID) (*Specialization, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+specCols+` FROM specialization WHERE id = $1`, id)
	return scanSpec(row)
}

func (r *SpecializationRepo) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	// Refuse to delete system entries (organization_id NULL).
	res, err := r.pool.Exec(ctx,
		`DELETE FROM specialization WHERE id = $1 AND organization_id = $2`, id, orgID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
