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

type Patient struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	MRN               string
	FirstName         string
	LastName          string
	BirthDate         *time.Time
	Gender            string
	BloodType         string
	IdentifierKind    *string
	IdentifierValue   *string
	MernisVerifiedAt  *time.Time
	Phone             *string
	Email             *string
	Address           *string
	NextOfKinName     *string
	NextOfKinPhone    *string
	Notes             *string
	IsDeceased        bool
	DeceasedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type PatientRepo struct {
	pool *pgxpool.Pool
}

func NewPatientRepo(pool *pgxpool.Pool) *PatientRepo {
	return &PatientRepo{pool: pool}
}

const patientCols = `id, organization_id, mrn, first_name, last_name, birth_date,
	gender, blood_type, identifier_kind, identifier_value, mernis_verified_at,
	phone, email, address, next_of_kin_name, next_of_kin_phone, notes,
	is_deceased, deceased_at, created_at, updated_at`

func scanPatient(row pgx.Row) (*Patient, error) {
	p := &Patient{}
	err := row.Scan(&p.ID, &p.OrganizationID, &p.MRN, &p.FirstName, &p.LastName,
		&p.BirthDate, &p.Gender, &p.BloodType, &p.IdentifierKind, &p.IdentifierValue,
		&p.MernisVerifiedAt, &p.Phone, &p.Email, &p.Address, &p.NextOfKinName,
		&p.NextOfKinPhone, &p.Notes, &p.IsDeceased, &p.DeceasedAt,
		&p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

type CreatePatientInput struct {
	OrganizationID    uuid.UUID
	MRN               string
	FirstName         string
	LastName          string
	BirthDate         *time.Time
	Gender            string
	BloodType         string
	IdentifierKind    *string
	IdentifierValue   *string
	MernisVerifiedAt  *time.Time
	Phone             *string
	Email             *string
	Address           *string
	NextOfKinName     *string
	NextOfKinPhone    *string
	Notes             *string
}

func (r *PatientRepo) Create(ctx context.Context, in CreatePatientInput) (*Patient, error) {
	if in.Gender == "" {
		in.Gender = "unknown"
	}
	if in.BloodType == "" {
		in.BloodType = "unknown"
	}
	row := r.pool.QueryRow(ctx,
		`INSERT INTO patient (organization_id, mrn, first_name, last_name, birth_date,
		   gender, blood_type, identifier_kind, identifier_value, mernis_verified_at,
		   phone, email, address, next_of_kin_name, next_of_kin_phone, notes)
		 VALUES ($1,$2,$3,$4,$5,$6::patient_gender,$7::blood_type,
		         $8::patient_identifier_kind,$9,$10,$11,$12,$13,$14,$15,$16)
		 RETURNING `+patientCols,
		in.OrganizationID, in.MRN, in.FirstName, in.LastName, in.BirthDate,
		in.Gender, in.BloodType, in.IdentifierKind, in.IdentifierValue, in.MernisVerifiedAt,
		in.Phone, in.Email, in.Address, in.NextOfKinName, in.NextOfKinPhone, in.Notes)
	return scanPatient(row)
}

// NextMRN reads the per-org MRN sequence and returns the raw integer; the
// service layer is responsible for zero-padding (util.FormatMRN).
func (r *PatientRepo) NextMRN(ctx context.Context) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, `SELECT nextval('patient_mrn_seq')`).Scan(&n)
	return n, err
}

func (r *PatientRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (*Patient, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+patientCols+` FROM patient WHERE organization_id = $1 AND id = $2`,
		orgID, id)
	return scanPatient(row)
}

func (r *PatientRepo) GetByIdentifier(ctx context.Context, orgID uuid.UUID, kind, value string) (*Patient, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+patientCols+` FROM patient
		 WHERE organization_id = $1 AND identifier_kind = $2::patient_identifier_kind
		   AND identifier_value = $3`,
		orgID, kind, value)
	return scanPatient(row)
}

type ListPatientFilter struct {
	Search string
	Limit  int
}

// List returns patients matching the search across name / TC / phone / MRN.
// Uses the GIN FTS index for non-empty searches, plain ORDER BY otherwise.
func (r *PatientRepo) List(ctx context.Context, orgID uuid.UUID, f ListPatientFilter) ([]Patient, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	q := strings.TrimSpace(f.Search)
	if q == "" {
		rows, err := r.pool.Query(ctx,
			`SELECT `+patientCols+` FROM patient WHERE organization_id = $1
			 ORDER BY updated_at DESC LIMIT $2`, orgID, limit)
		return collectPatients(rows, err)
	}
	// Prefix/ILIKE match across the same fields the FTS index covers.
	rows, err := r.pool.Query(ctx,
		`SELECT `+patientCols+` FROM patient
		 WHERE organization_id = $1
		   AND (
		     first_name ILIKE $2 OR last_name ILIKE $2 OR
		     identifier_value ILIKE $2 OR phone ILIKE $2 OR mrn ILIKE $2 OR
		     (first_name || ' ' || last_name) ILIKE $2
		   )
		 ORDER BY last_name, first_name LIMIT $3`,
		orgID, "%"+q+"%", limit)
	return collectPatients(rows, err)
}

func collectPatients(rows pgx.Rows, err error) ([]Patient, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Patient{}
	for rows.Next() {
		p := Patient{}
		if scanErr := rows.Scan(&p.ID, &p.OrganizationID, &p.MRN, &p.FirstName, &p.LastName,
			&p.BirthDate, &p.Gender, &p.BloodType, &p.IdentifierKind, &p.IdentifierValue,
			&p.MernisVerifiedAt, &p.Phone, &p.Email, &p.Address, &p.NextOfKinName,
			&p.NextOfKinPhone, &p.Notes, &p.IsDeceased, &p.DeceasedAt,
			&p.CreatedAt, &p.UpdatedAt); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ---------- Insurance + Allergy ----------

type PatientInsurance struct {
	ID                    uuid.UUID
	OrganizationID        uuid.UUID
	PatientID             uuid.UUID
	ExternalInstitutionID uuid.UUID
	PolicyNo              *string
	IsPrimary             bool
	Status                string
	ValidFrom             *time.Time
	ValidTo               *time.Time
	Notes                 *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func (r *PatientRepo) ListInsurance(ctx context.Context, orgID, patientID uuid.UUID) ([]PatientInsurance, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, patient_id, external_institution_id, policy_no,
		        is_primary, status, valid_from, valid_to, notes, created_at, updated_at
		 FROM patient_insurance
		 WHERE organization_id = $1 AND patient_id = $2
		 ORDER BY is_primary DESC, created_at DESC`, orgID, patientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PatientInsurance{}
	for rows.Next() {
		v := PatientInsurance{}
		if err := rows.Scan(&v.ID, &v.OrganizationID, &v.PatientID, &v.ExternalInstitutionID,
			&v.PolicyNo, &v.IsPrimary, &v.Status, &v.ValidFrom, &v.ValidTo, &v.Notes,
			&v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
