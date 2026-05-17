package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StaffMember struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	UserID          *uuid.UUID
	EmployeeNo      *string
	FirstName       string
	LastName        string
	Title           *string
	EmploymentType  string
	HireDate        *time.Time
	TerminationDate *time.Time
	Phone           *string
	Email           *string
	Notes           *string
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type StaffRepo struct {
	pool *pgxpool.Pool
}

func NewStaffRepo(pool *pgxpool.Pool) *StaffRepo {
	return &StaffRepo{pool: pool}
}

const staffCols = `id, organization_id, user_id, employee_no, first_name, last_name,
	title, employment_type, hire_date, termination_date, phone, email, notes,
	is_active, created_at, updated_at`

func scanStaff(row pgx.Row) (*StaffMember, error) {
	s := &StaffMember{}
	err := row.Scan(&s.ID, &s.OrganizationID, &s.UserID, &s.EmployeeNo, &s.FirstName,
		&s.LastName, &s.Title, &s.EmploymentType, &s.HireDate, &s.TerminationDate,
		&s.Phone, &s.Email, &s.Notes, &s.IsActive, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

type CreateStaffInput struct {
	OrganizationID uuid.UUID
	UserID         *uuid.UUID
	EmployeeNo     *string
	FirstName      string
	LastName       string
	Title          *string
	EmploymentType string
	HireDate       *time.Time
	Phone          *string
	Email          *string
	Notes          *string
}

func (r *StaffRepo) Create(ctx context.Context, in CreateStaffInput) (*StaffMember, error) {
	if in.EmploymentType == "" {
		in.EmploymentType = "full_time"
	}
	row := r.pool.QueryRow(ctx,
		`INSERT INTO staff_member (organization_id, user_id, employee_no, first_name,
		   last_name, title, employment_type, hire_date, phone, email, notes)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		 RETURNING `+staffCols,
		in.OrganizationID, in.UserID, in.EmployeeNo, in.FirstName, in.LastName,
		in.Title, in.EmploymentType, in.HireDate, in.Phone, in.Email, in.Notes)
	return scanStaff(row)
}

func (r *StaffRepo) List(ctx context.Context, orgID uuid.UUID, activeOnly bool) ([]StaffMember, error) {
	q := `SELECT ` + staffCols + ` FROM staff_member WHERE organization_id = $1`
	if activeOnly {
		q += ` AND is_active = TRUE`
	}
	q += ` ORDER BY last_name, first_name`
	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StaffMember{}
	for rows.Next() {
		s := StaffMember{}
		if err := rows.Scan(&s.ID, &s.OrganizationID, &s.UserID, &s.EmployeeNo, &s.FirstName,
			&s.LastName, &s.Title, &s.EmploymentType, &s.HireDate, &s.TerminationDate,
			&s.Phone, &s.Email, &s.Notes, &s.IsActive, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *StaffRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (*StaffMember, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+staffCols+` FROM staff_member WHERE organization_id = $1 AND id = $2`,
		orgID, id)
	return scanStaff(row)
}
