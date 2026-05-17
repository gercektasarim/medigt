package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MernisLog struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	BranchID          *uuid.UUID
	TCLast4           string
	FirstName         string
	LastName          string
	BirthYear         int
	Verified          bool
	ResponseCode      *string
	ErrorMessage      *string
	RequestedByUserID *uuid.UUID
	RequestedAt       time.Time
	ResponseAt        *time.Time
	CreatedAt         time.Time
}

type MernisRepo struct{ pool *pgxpool.Pool }

func NewMernisRepo(pool *pgxpool.Pool) *MernisRepo { return &MernisRepo{pool: pool} }

type LogMernisInput struct {
	OrganizationID    uuid.UUID
	BranchID          *uuid.UUID
	TCLast4           string
	FirstName         string
	LastName          string
	BirthYear         int
	Verified          bool
	ResponseCode      *string
	ErrorMessage      *string
	RequestedByUserID *uuid.UUID
}

func (r *MernisRepo) Log(ctx context.Context, in LogMernisInput) (*MernisLog, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO mernis_verification_log
		   (organization_id, branch_id, tc_last4, first_name, last_name, birth_year,
		    verified, response_code, error_message, requested_by_user_id, response_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
		 RETURNING id, organization_id, branch_id, tc_last4, first_name, last_name,
		           birth_year, verified, response_code, error_message,
		           requested_by_user_id, requested_at, response_at, created_at`,
		in.OrganizationID, in.BranchID, in.TCLast4, in.FirstName, in.LastName, in.BirthYear,
		in.Verified, in.ResponseCode, in.ErrorMessage, in.RequestedByUserID)
	m := &MernisLog{}
	err := row.Scan(&m.ID, &m.OrganizationID, &m.BranchID, &m.TCLast4, &m.FirstName, &m.LastName,
		&m.BirthYear, &m.Verified, &m.ResponseCode, &m.ErrorMessage,
		&m.RequestedByUserID, &m.RequestedAt, &m.ResponseAt, &m.CreatedAt)
	return m, err
}

func (r *MernisRepo) Recent(ctx context.Context, orgID uuid.UUID, limit int) ([]MernisLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, branch_id, tc_last4, first_name, last_name,
		        birth_year, verified, response_code, error_message,
		        requested_by_user_id, requested_at, response_at, created_at
		 FROM mernis_verification_log
		 WHERE organization_id = $1
		 ORDER BY requested_at DESC
		 LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MernisLog{}
	for rows.Next() {
		m := MernisLog{}
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.BranchID, &m.TCLast4, &m.FirstName, &m.LastName,
			&m.BirthYear, &m.Verified, &m.ResponseCode, &m.ErrorMessage,
			&m.RequestedByUserID, &m.RequestedAt, &m.ResponseAt, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
