package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Visit struct {
	ID                       uuid.UUID
	OrganizationID           uuid.UUID
	BranchID                 uuid.UUID
	PatientID                uuid.UUID
	DoctorID                 *uuid.UUID
	AppointmentID            *uuid.UUID
	EncounterType            string
	Status                   string
	ChiefComplaint           *string
	HistoryOfPresentIllness  *string
	ExaminationFindings      *string
	TreatmentPlan            *string
	Notes                    *string
	OpenedByUserID           *uuid.UUID
	StartedAt                time.Time
	EndedAt                  *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// VisitWithJoins is the list-friendly shape with patient + doctor display fields.
type VisitWithJoins struct {
	Visit            Visit
	PatientMRN       string
	PatientFirstName string
	PatientLastName  string
	PatientPhone     *string
	DoctorFirstName  *string
	DoctorLastName   *string
	DoctorTitle      *string
}

type VisitRepo struct {
	pool *pgxpool.Pool
}

func NewVisitRepo(pool *pgxpool.Pool) *VisitRepo { return &VisitRepo{pool: pool} }

const visitCols = `id, organization_id, branch_id, patient_id, doctor_id,
	appointment_id, encounter_type, status, chief_complaint,
	history_of_present_illness, examination_findings, treatment_plan, notes,
	opened_by_user_id, started_at, ended_at, created_at, updated_at`

func scanVisit(row pgx.Row) (*Visit, error) {
	v := &Visit{}
	err := row.Scan(&v.ID, &v.OrganizationID, &v.BranchID, &v.PatientID, &v.DoctorID,
		&v.AppointmentID, &v.EncounterType, &v.Status, &v.ChiefComplaint,
		&v.HistoryOfPresentIllness, &v.ExaminationFindings, &v.TreatmentPlan, &v.Notes,
		&v.OpenedByUserID, &v.StartedAt, &v.EndedAt, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return v, err
}

type CreateVisitInput struct {
	OrganizationID  uuid.UUID
	BranchID        uuid.UUID
	PatientID       uuid.UUID
	DoctorID        *uuid.UUID
	AppointmentID   *uuid.UUID
	EncounterType   string
	ChiefComplaint  *string
	OpenedByUserID  *uuid.UUID
}

func (r *VisitRepo) Create(ctx context.Context, in CreateVisitInput) (*Visit, error) {
	if in.EncounterType == "" {
		in.EncounterType = "outpatient"
	}
	row := r.pool.QueryRow(ctx,
		`INSERT INTO visit (organization_id, branch_id, patient_id, doctor_id,
		   appointment_id, encounter_type, chief_complaint, opened_by_user_id)
		 VALUES ($1,$2,$3,$4,$5,$6::encounter_type,$7,$8)
		 RETURNING `+visitCols,
		in.OrganizationID, in.BranchID, in.PatientID, in.DoctorID,
		in.AppointmentID, in.EncounterType, in.ChiefComplaint, in.OpenedByUserID)
	return scanVisit(row)
}

// CreateInTx is the same as Create but runs inside an existing pgx.Tx so the
// caller can pair visit creation with an appointment status update atomically.
func (r *VisitRepo) CreateInTx(ctx context.Context, tx pgx.Tx, in CreateVisitInput) (*Visit, error) {
	if in.EncounterType == "" {
		in.EncounterType = "outpatient"
	}
	row := tx.QueryRow(ctx,
		`INSERT INTO visit (organization_id, branch_id, patient_id, doctor_id,
		   appointment_id, encounter_type, chief_complaint, opened_by_user_id)
		 VALUES ($1,$2,$3,$4,$5,$6::encounter_type,$7,$8)
		 RETURNING `+visitCols,
		in.OrganizationID, in.BranchID, in.PatientID, in.DoctorID,
		in.AppointmentID, in.EncounterType, in.ChiefComplaint, in.OpenedByUserID)
	return scanVisit(row)
}

func (r *VisitRepo) GetByID(ctx context.Context, branchID, id uuid.UUID) (*Visit, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+visitCols+` FROM visit WHERE branch_id = $1 AND id = $2`,
		branchID, id)
	return scanVisit(row)
}

func (r *VisitRepo) GetByAppointment(ctx context.Context, branchID, appointmentID uuid.UUID) (*Visit, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+visitCols+` FROM visit WHERE branch_id = $1 AND appointment_id = $2`,
		branchID, appointmentID)
	return scanVisit(row)
}

type ListVisitFilter struct {
	Status   string   // "" = any
	OnlyMine *uuid.UUID
	From     *time.Time
	To       *time.Time
	Limit    int
}

func (r *VisitRepo) ListWithJoins(ctx context.Context, branchID uuid.UUID, f ListVisitFilter) ([]VisitWithJoins, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT v.id, v.organization_id, v.branch_id, v.patient_id, v.doctor_id,
	             v.appointment_id, v.encounter_type, v.status, v.chief_complaint,
	             v.history_of_present_illness, v.examination_findings,
	             v.treatment_plan, v.notes, v.opened_by_user_id,
	             v.started_at, v.ended_at, v.created_at, v.updated_at,
	             p.mrn, p.first_name, p.last_name, p.phone,
	             ds.first_name, ds.last_name, ds.title
	      FROM visit v
	      JOIN patient p ON p.id = v.patient_id
	      LEFT JOIN doctor d ON d.id = v.doctor_id
	      LEFT JOIN staff_member ds ON ds.id = d.staff_member_id
	      WHERE v.branch_id = $1`
	args := []any{branchID}
	if f.Status != "" {
		args = append(args, f.Status)
		q += ` AND v.status = $` + itoa(len(args)) + `::visit_status`
	}
	if f.OnlyMine != nil {
		args = append(args, *f.OnlyMine)
		q += ` AND v.doctor_id = $` + itoa(len(args))
	}
	if f.From != nil {
		args = append(args, *f.From)
		q += ` AND v.started_at >= $` + itoa(len(args))
	}
	if f.To != nil {
		args = append(args, *f.To)
		q += ` AND v.started_at < $` + itoa(len(args))
	}
	q += ` ORDER BY v.started_at DESC LIMIT ` + itoa(limit)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []VisitWithJoins{}
	for rows.Next() {
		w := VisitWithJoins{}
		v := &w.Visit
		if err := rows.Scan(
			&v.ID, &v.OrganizationID, &v.BranchID, &v.PatientID, &v.DoctorID,
			&v.AppointmentID, &v.EncounterType, &v.Status, &v.ChiefComplaint,
			&v.HistoryOfPresentIllness, &v.ExaminationFindings, &v.TreatmentPlan, &v.Notes,
			&v.OpenedByUserID, &v.StartedAt, &v.EndedAt, &v.CreatedAt, &v.UpdatedAt,
			&w.PatientMRN, &w.PatientFirstName, &w.PatientLastName, &w.PatientPhone,
			&w.DoctorFirstName, &w.DoctorLastName, &w.DoctorTitle,
		); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

type UpdateVisitNotesInput struct {
	ChiefComplaint           *string
	HistoryOfPresentIllness  *string
	ExaminationFindings      *string
	TreatmentPlan            *string
	Notes                    *string
}

// UpdateNotes saves the doctor's narrative fields. NULL means "don't change",
// empty string means "clear" — we mirror that with a tiny COALESCE pattern.
func (r *VisitRepo) UpdateNotes(ctx context.Context, branchID, id uuid.UUID, in UpdateVisitNotesInput) error {
	res, err := r.pool.Exec(ctx,
		`UPDATE visit SET
		   chief_complaint            = COALESCE($3, chief_complaint),
		   history_of_present_illness = COALESCE($4, history_of_present_illness),
		   examination_findings       = COALESCE($5, examination_findings),
		   treatment_plan             = COALESCE($6, treatment_plan),
		   notes                      = COALESCE($7, notes)
		 WHERE branch_id = $1 AND id = $2`,
		branchID, id,
		in.ChiefComplaint, in.HistoryOfPresentIllness, in.ExaminationFindings,
		in.TreatmentPlan, in.Notes)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *VisitRepo) Complete(ctx context.Context, branchID, id uuid.UUID) error {
	res, err := r.pool.Exec(ctx,
		`UPDATE visit SET status = 'completed', ended_at = NOW()
		 WHERE branch_id = $1 AND id = $2 AND status = 'in_progress'`,
		branchID, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
