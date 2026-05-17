package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Medication struct {
	ID                  uuid.UUID
	OrganizationID      uuid.UUID
	AtcCode             *string
	Barcode             *string
	Name                string
	GenericName         *string
	Form                string
	Strength            *string
	PackSize            *string
	PrescriptionClass   string
	RequiresColdChain   bool
	IsControlled        bool
	Manufacturer        *string
	ListPrice           *float64
	Notes               *string
	IsActive            bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type MedicationRepo struct {
	pool *pgxpool.Pool
}

func NewMedicationRepo(pool *pgxpool.Pool) *MedicationRepo {
	return &MedicationRepo{pool: pool}
}

const medicationCols = `id, organization_id, atc_code, barcode, name, generic_name,
	form::text, strength, pack_size, prescription_class::text,
	requires_cold_chain, is_controlled, manufacturer, list_price,
	notes, is_active, created_at, updated_at`

func scanMedication(row pgx.Row) (*Medication, error) {
	m := &Medication{}
	err := row.Scan(
		&m.ID, &m.OrganizationID, &m.AtcCode, &m.Barcode, &m.Name, &m.GenericName,
		&m.Form, &m.Strength, &m.PackSize, &m.PrescriptionClass,
		&m.RequiresColdChain, &m.IsControlled, &m.Manufacturer, &m.ListPrice,
		&m.Notes, &m.IsActive, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

type CreateMedicationInput struct {
	OrganizationID    uuid.UUID
	AtcCode           *string
	Barcode           *string
	Name              string
	GenericName       *string
	Form              string
	Strength          *string
	PackSize          *string
	PrescriptionClass string
	RequiresColdChain bool
	IsControlled      bool
	Manufacturer      *string
	ListPrice         *float64
	Notes             *string
}

func (r *MedicationRepo) Create(ctx context.Context, in CreateMedicationInput) (*Medication, error) {
	if in.Form == "" {
		in.Form = "tablet"
	}
	if in.PrescriptionClass == "" {
		in.PrescriptionClass = "normal"
	}
	row := r.pool.QueryRow(ctx,
		`INSERT INTO medication (organization_id, atc_code, barcode, name, generic_name,
		   form, strength, pack_size, prescription_class,
		   requires_cold_chain, is_controlled, manufacturer, list_price, notes)
		 VALUES ($1, $2, $3, $4, $5,
		         $6::medication_form, $7, $8, $9::prescription_class,
		         $10, $11, $12, $13, $14)
		 RETURNING `+medicationCols,
		in.OrganizationID, in.AtcCode, in.Barcode, in.Name, in.GenericName,
		in.Form, in.Strength, in.PackSize, in.PrescriptionClass,
		in.RequiresColdChain, in.IsControlled, in.Manufacturer, in.ListPrice, in.Notes)
	return scanMedication(row)
}

type ListMedicationFilter struct {
	Search             string
	Form               string
	PrescriptionClass  string
	ActiveOnly         bool
	Limit              int
}

func (r *MedicationRepo) List(ctx context.Context, orgID uuid.UUID, f ListMedicationFilter) ([]Medication, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT ` + medicationCols + ` FROM medication WHERE organization_id = $1`
	args := []any{orgID}
	if f.ActiveOnly {
		q += ` AND is_active = TRUE`
	}
	if f.Form != "" {
		args = append(args, f.Form)
		q += ` AND form = $` + itoa(len(args)) + `::medication_form`
	}
	if f.PrescriptionClass != "" {
		args = append(args, f.PrescriptionClass)
		q += ` AND prescription_class = $` + itoa(len(args)) + `::prescription_class`
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		idx := itoa(len(args))
		q += ` AND (name ILIKE $` + idx +
			` OR generic_name ILIKE $` + idx +
			` OR atc_code ILIKE $` + idx +
			` OR barcode ILIKE $` + idx + `)`
	}
	q += ` ORDER BY name LIMIT ` + itoa(limit)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Medication{}
	for rows.Next() {
		m := Medication{}
		if err := rows.Scan(
			&m.ID, &m.OrganizationID, &m.AtcCode, &m.Barcode, &m.Name, &m.GenericName,
			&m.Form, &m.Strength, &m.PackSize, &m.PrescriptionClass,
			&m.RequiresColdChain, &m.IsControlled, &m.Manufacturer, &m.ListPrice,
			&m.Notes, &m.IsActive, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *MedicationRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (*Medication, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+medicationCols+` FROM medication WHERE organization_id = $1 AND id = $2`,
		orgID, id)
	return scanMedication(row)
}
