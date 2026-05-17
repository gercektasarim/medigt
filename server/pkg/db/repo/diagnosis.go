package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Diagnosis struct {
	ID              uuid.UUID
	VisitID         uuid.UUID
	Icd10Code       string
	Icd10Title      string
	Kind            string
	Notes           *string
	CreatedByUserID *uuid.UUID
	CreatedAt       time.Time
}

type DiagnosisRepo struct {
	pool *pgxpool.Pool
}

func NewDiagnosisRepo(pool *pgxpool.Pool) *DiagnosisRepo { return &DiagnosisRepo{pool: pool} }

type AddDiagnosisInput struct {
	VisitID         uuid.UUID
	Icd10Code       string
	Icd10Title      string
	Kind            string
	Notes           *string
	CreatedByUserID *uuid.UUID
}

func (r *DiagnosisRepo) Add(ctx context.Context, in AddDiagnosisInput) (*Diagnosis, error) {
	if in.Kind == "" {
		in.Kind = "primary"
	}
	row := r.pool.QueryRow(ctx,
		`INSERT INTO diagnosis (visit_id, icd10_code, icd10_title, kind, notes, created_by_user_id)
		 VALUES ($1, $2, $3, $4::diagnosis_kind, $5, $6)
		 RETURNING id, visit_id, icd10_code, icd10_title, kind, notes, created_by_user_id, created_at`,
		in.VisitID, in.Icd10Code, in.Icd10Title, in.Kind, in.Notes, in.CreatedByUserID)
	d := &Diagnosis{}
	err := row.Scan(&d.ID, &d.VisitID, &d.Icd10Code, &d.Icd10Title, &d.Kind,
		&d.Notes, &d.CreatedByUserID, &d.CreatedAt)
	return d, err
}

func (r *DiagnosisRepo) ListForVisit(ctx context.Context, visitID uuid.UUID) ([]Diagnosis, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, visit_id, icd10_code, icd10_title, kind, notes, created_by_user_id, created_at
		 FROM diagnosis WHERE visit_id = $1
		 ORDER BY (kind = 'primary') DESC, created_at ASC`, visitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Diagnosis{}
	for rows.Next() {
		d := Diagnosis{}
		if err := rows.Scan(&d.ID, &d.VisitID, &d.Icd10Code, &d.Icd10Title, &d.Kind,
			&d.Notes, &d.CreatedByUserID, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *DiagnosisRepo) Delete(ctx context.Context, visitID, id uuid.UUID) error {
	res, err := r.pool.Exec(ctx,
		`DELETE FROM diagnosis WHERE visit_id = $1 AND id = $2`, visitID, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
