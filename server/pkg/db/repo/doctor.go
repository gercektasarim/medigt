package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Doctor struct {
	ID                    uuid.UUID
	StaffMemberID         uuid.UUID
	DiplomaNo             *string
	MedulaDoctorCode      *string
	SignatureImagePath    *string
	LicenseExpiresAt      *time.Time
	IsAcceptingPatients   bool
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// DoctorWithProfile is the API/list-friendly join of doctor + staff_member +
// the doctor's specializations.
type DoctorWithProfile struct {
	Doctor          Doctor
	Staff           StaffMember
	Specializations []Specialization
}

type DoctorRepo struct {
	pool *pgxpool.Pool
}

func NewDoctorRepo(pool *pgxpool.Pool) *DoctorRepo {
	return &DoctorRepo{pool: pool}
}

// Plain column list for RETURNING.
const doctorCols = `id, staff_member_id, diploma_no, medula_doctor_code,
	signature_image_path, license_expires_at, is_accepting_patients,
	created_at, updated_at`

// d.-prefixed list for joins.
const doctorColsD = `d.id, d.staff_member_id, d.diploma_no, d.medula_doctor_code,
	d.signature_image_path, d.license_expires_at, d.is_accepting_patients,
	d.created_at, d.updated_at`

type CreateDoctorInput struct {
	StaffMemberID           uuid.UUID
	DiplomaNo               *string
	MedulaDoctorCode        *string
	LicenseExpiresAt        *time.Time
	IsAcceptingPatients     bool
	SpecializationIDs       []uuid.UUID
	PrimarySpecializationID *uuid.UUID
}

func (r *DoctorRepo) Create(ctx context.Context, in CreateDoctorInput) (*Doctor, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx,
		`INSERT INTO doctor (staff_member_id, diploma_no, medula_doctor_code,
		   license_expires_at, is_accepting_patients)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING `+doctorCols,
		in.StaffMemberID, in.DiplomaNo, in.MedulaDoctorCode,
		in.LicenseExpiresAt, in.IsAcceptingPatients)
	d := &Doctor{}
	if err := row.Scan(&d.ID, &d.StaffMemberID, &d.DiplomaNo, &d.MedulaDoctorCode,
		&d.SignatureImagePath, &d.LicenseExpiresAt, &d.IsAcceptingPatients,
		&d.CreatedAt, &d.UpdatedAt); err != nil {
		return nil, err
	}

	for _, sid := range in.SpecializationIDs {
		isPrimary := in.PrimarySpecializationID != nil && *in.PrimarySpecializationID == sid
		if _, err := tx.Exec(ctx,
			`INSERT INTO doctor_specialization (doctor_id, specialization_id, is_primary)
			 VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			d.ID, sid, isPrimary); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return d, nil
}

// ListWithProfiles returns all doctors in the org joined with their staff
// profile, then attaches specializations in one extra query.
func (r *DoctorRepo) ListWithProfiles(ctx context.Context, orgID uuid.UUID) ([]DoctorWithProfile, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+doctorColsD+`,
		        s.id, s.organization_id, s.user_id, s.employee_no, s.first_name, s.last_name,
		        s.title, s.employment_type, s.hire_date, s.termination_date,
		        s.phone, s.email, s.notes, s.is_active, s.created_at, s.updated_at
		 FROM doctor d
		 JOIN staff_member s ON s.id = d.staff_member_id
		 WHERE s.organization_id = $1
		 ORDER BY s.last_name, s.first_name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	doctorIDs := []uuid.UUID{}
	result := []DoctorWithProfile{}
	for rows.Next() {
		dp := DoctorWithProfile{}
		if err := rows.Scan(
			&dp.Doctor.ID, &dp.Doctor.StaffMemberID, &dp.Doctor.DiplomaNo, &dp.Doctor.MedulaDoctorCode,
			&dp.Doctor.SignatureImagePath, &dp.Doctor.LicenseExpiresAt, &dp.Doctor.IsAcceptingPatients,
			&dp.Doctor.CreatedAt, &dp.Doctor.UpdatedAt,
			&dp.Staff.ID, &dp.Staff.OrganizationID, &dp.Staff.UserID, &dp.Staff.EmployeeNo,
			&dp.Staff.FirstName, &dp.Staff.LastName, &dp.Staff.Title, &dp.Staff.EmploymentType,
			&dp.Staff.HireDate, &dp.Staff.TerminationDate, &dp.Staff.Phone, &dp.Staff.Email,
			&dp.Staff.Notes, &dp.Staff.IsActive, &dp.Staff.CreatedAt, &dp.Staff.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, dp)
		doctorIDs = append(doctorIDs, dp.Doctor.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return result, nil
	}

	specRows, err := r.pool.Query(ctx,
		`SELECT ds.doctor_id, sp.id, sp.organization_id, sp.code, sp.name,
		        sp.parent_id, sp.is_system, sp.created_at, sp.updated_at
		 FROM doctor_specialization ds
		 JOIN specialization sp ON sp.id = ds.specialization_id
		 WHERE ds.doctor_id = ANY($1)`, doctorIDs)
	if err != nil {
		return result, nil
	}
	defer specRows.Close()

	specByDoctor := map[uuid.UUID][]Specialization{}
	for specRows.Next() {
		var doctorID uuid.UUID
		s := Specialization{}
		if err := specRows.Scan(&doctorID, &s.ID, &s.OrganizationID, &s.Code, &s.Name,
			&s.ParentID, &s.IsSystem, &s.CreatedAt, &s.UpdatedAt); err != nil {
			continue
		}
		specByDoctor[doctorID] = append(specByDoctor[doctorID], s)
	}
	for i := range result {
		result[i].Specializations = specByDoctor[result[i].Doctor.ID]
	}
	return result, nil
}
