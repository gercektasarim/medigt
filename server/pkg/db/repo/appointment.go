package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Appointment struct {
	ID                  uuid.UUID
	OrganizationID      uuid.UUID
	BranchID            uuid.UUID
	PatientID           uuid.UUID
	DoctorID            *uuid.UUID
	DepartmentID        *uuid.UUID
	ScheduledAt         time.Time
	DurationMinutes     int
	Status              string
	Kind                string
	Reason              *string
	Notes               *string
	CreatedByUserID     *uuid.UUID
	ArrivedAt           *time.Time
	StartedAt           *time.Time
	CompletedAt         *time.Time
	CancelledAt         *time.Time
	CancelledByUserID   *uuid.UUID
	CancellationReason  *string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// AppointmentWithJoins is what the list endpoint returns — appointment +
// minimal patient/doctor display fields so the UI doesn't need extra round trips.
type AppointmentWithJoins struct {
	Appointment      Appointment
	PatientMRN       string
	PatientFirstName string
	PatientLastName  string
	PatientPhone     *string
	DoctorFirstName  *string
	DoctorLastName   *string
	DoctorTitle      *string
}

type AppointmentRepo struct {
	pool *pgxpool.Pool
}

func NewAppointmentRepo(pool *pgxpool.Pool) *AppointmentRepo {
	return &AppointmentRepo{pool: pool}
}

const apptCols = `a.id, a.organization_id, a.branch_id, a.patient_id, a.doctor_id,
	a.department_id, a.scheduled_at, a.duration_minutes, a.status, a.kind,
	a.reason, a.notes, a.created_by_user_id, a.arrived_at, a.started_at,
	a.completed_at, a.cancelled_at, a.cancelled_by_user_id,
	a.cancellation_reason, a.created_at, a.updated_at`

type CreateAppointmentInput struct {
	OrganizationID  uuid.UUID
	BranchID        uuid.UUID
	PatientID       uuid.UUID
	DoctorID        *uuid.UUID
	DepartmentID    *uuid.UUID
	ScheduledAt     time.Time
	DurationMinutes int
	Kind            string
	Reason          *string
	Notes           *string
	CreatedByUserID *uuid.UUID
}

func (r *AppointmentRepo) Create(ctx context.Context, in CreateAppointmentInput) (*Appointment, error) {
	if in.DurationMinutes <= 0 {
		in.DurationMinutes = 20
	}
	if in.Kind == "" {
		in.Kind = "outpatient"
	}
	row := r.pool.QueryRow(ctx,
		`INSERT INTO appointment (organization_id, branch_id, patient_id, doctor_id,
		   department_id, scheduled_at, duration_minutes, kind, reason, notes,
		   created_by_user_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8::visit_kind,$9,$10,$11)
		 RETURNING id, organization_id, branch_id, patient_id, doctor_id,
		   department_id, scheduled_at, duration_minutes, status, kind,
		   reason, notes, created_by_user_id, arrived_at, started_at,
		   completed_at, cancelled_at, cancelled_by_user_id,
		   cancellation_reason, created_at, updated_at`,
		in.OrganizationID, in.BranchID, in.PatientID, in.DoctorID,
		in.DepartmentID, in.ScheduledAt, in.DurationMinutes, in.Kind,
		in.Reason, in.Notes, in.CreatedByUserID)
	a := &Appointment{}
	err := row.Scan(&a.ID, &a.OrganizationID, &a.BranchID, &a.PatientID, &a.DoctorID,
		&a.DepartmentID, &a.ScheduledAt, &a.DurationMinutes, &a.Status, &a.Kind,
		&a.Reason, &a.Notes, &a.CreatedByUserID, &a.ArrivedAt, &a.StartedAt,
		&a.CompletedAt, &a.CancelledAt, &a.CancelledByUserID,
		&a.CancellationReason, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

// ListByDay returns all appointments in [from, to) for the given branch,
// optionally filtered by doctor. Sorted by scheduled_at ASC.
func (r *AppointmentRepo) ListByDay(ctx context.Context, branchID uuid.UUID,
	from, to time.Time, doctorID *uuid.UUID) ([]AppointmentWithJoins, error) {

	args := []any{branchID, from, to}
	q := `SELECT ` + apptCols + `,
	             p.mrn, p.first_name, p.last_name, p.phone,
	             ds.first_name, ds.last_name, ds.title
	      FROM appointment a
	      JOIN patient p ON p.id = a.patient_id
	      LEFT JOIN doctor d ON d.id = a.doctor_id
	      LEFT JOIN staff_member ds ON ds.id = d.staff_member_id
	      WHERE a.branch_id = $1 AND a.scheduled_at >= $2 AND a.scheduled_at < $3`
	if doctorID != nil {
		args = append(args, *doctorID)
		q += ` AND a.doctor_id = $4`
	}
	q += ` ORDER BY a.scheduled_at ASC`

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []AppointmentWithJoins{}
	for rows.Next() {
		w := AppointmentWithJoins{}
		a := &w.Appointment
		if err := rows.Scan(
			&a.ID, &a.OrganizationID, &a.BranchID, &a.PatientID, &a.DoctorID,
			&a.DepartmentID, &a.ScheduledAt, &a.DurationMinutes, &a.Status, &a.Kind,
			&a.Reason, &a.Notes, &a.CreatedByUserID, &a.ArrivedAt, &a.StartedAt,
			&a.CompletedAt, &a.CancelledAt, &a.CancelledByUserID,
			&a.CancellationReason, &a.CreatedAt, &a.UpdatedAt,
			&w.PatientMRN, &w.PatientFirstName, &w.PatientLastName, &w.PatientPhone,
			&w.DoctorFirstName, &w.DoctorLastName, &w.DoctorTitle,
		); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *AppointmentRepo) GetByID(ctx context.Context, branchID, id uuid.UUID) (*Appointment, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, branch_id, patient_id, doctor_id,
		   department_id, scheduled_at, duration_minutes, status, kind,
		   reason, notes, created_by_user_id, arrived_at, started_at,
		   completed_at, cancelled_at, cancelled_by_user_id,
		   cancellation_reason, created_at, updated_at
		 FROM appointment WHERE branch_id = $1 AND id = $2`, branchID, id)
	a := &Appointment{}
	err := row.Scan(&a.ID, &a.OrganizationID, &a.BranchID, &a.PatientID, &a.DoctorID,
		&a.DepartmentID, &a.ScheduledAt, &a.DurationMinutes, &a.Status, &a.Kind,
		&a.Reason, &a.Notes, &a.CreatedByUserID, &a.ArrivedAt, &a.StartedAt,
		&a.CompletedAt, &a.CancelledAt, &a.CancelledByUserID,
		&a.CancellationReason, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

// UpdateStatus is the single mutation path for state transitions.
// arrived / in_progress / completed / no_show all flow through here so we
// can stamp the corresponding timestamp atomically.
func (r *AppointmentRepo) UpdateStatus(ctx context.Context, branchID, id uuid.UUID, status string) error {
	stampCol := ""
	switch status {
	case "arrived":
		stampCol = "arrived_at"
	case "in_progress":
		stampCol = "started_at"
	case "completed":
		stampCol = "completed_at"
	}
	q := `UPDATE appointment SET status = $3::appointment_status`
	if stampCol != "" {
		q += `, ` + stampCol + ` = NOW()`
	}
	q += ` WHERE branch_id = $1 AND id = $2`
	res, err := r.pool.Exec(ctx, q, branchID, id, status)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AppointmentRepo) Cancel(ctx context.Context, branchID, id uuid.UUID, byUserID uuid.UUID, reason string) error {
	res, err := r.pool.Exec(ctx,
		`UPDATE appointment
		 SET status = 'cancelled', cancelled_at = NOW(),
		     cancelled_by_user_id = $3, cancellation_reason = NULLIF($4, '')
		 WHERE branch_id = $1 AND id = $2 AND status NOT IN ('cancelled', 'completed', 'no_show')`,
		branchID, id, byUserID, reason)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
