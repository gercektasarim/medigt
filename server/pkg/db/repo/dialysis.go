package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------- Dialysis machine ----------

type DialysisMachine struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	BranchID       uuid.UUID
	Code           string
	Name           string
	Manufacturer   *string
	Model          *string
	Location       *string
	IsActive       bool
	Notes          *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type DialysisMachineRepo struct {
	pool *pgxpool.Pool
}

func NewDialysisMachineRepo(pool *pgxpool.Pool) *DialysisMachineRepo {
	return &DialysisMachineRepo{pool: pool}
}

const dialysisMachineCols = `id, organization_id, branch_id, code, name, manufacturer,
	model, location, is_active, notes, created_at, updated_at`

func scanDialysisMachine(row pgx.Row) (*DialysisMachine, error) {
	m := &DialysisMachine{}
	err := row.Scan(&m.ID, &m.OrganizationID, &m.BranchID, &m.Code, &m.Name,
		&m.Manufacturer, &m.Model, &m.Location, &m.IsActive, &m.Notes,
		&m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

type CreateDialysisMachineInput struct {
	OrganizationID uuid.UUID
	BranchID       uuid.UUID
	Code           string
	Name           string
	Manufacturer   *string
	Model          *string
	Location       *string
	Notes          *string
}

func (r *DialysisMachineRepo) Create(ctx context.Context, in CreateDialysisMachineInput) (*DialysisMachine, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO dialysis_machine (organization_id, branch_id, code, name,
		   manufacturer, model, location, notes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING `+dialysisMachineCols,
		in.OrganizationID, in.BranchID, in.Code, in.Name,
		in.Manufacturer, in.Model, in.Location, in.Notes)
	return scanDialysisMachine(row)
}

func (r *DialysisMachineRepo) List(ctx context.Context, branchID uuid.UUID) ([]DialysisMachine, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+dialysisMachineCols+` FROM dialysis_machine
		 WHERE branch_id = $1 AND is_active = TRUE
		 ORDER BY code`, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DialysisMachine{}
	for rows.Next() {
		m := DialysisMachine{}
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.BranchID, &m.Code, &m.Name,
			&m.Manufacturer, &m.Model, &m.Location, &m.IsActive, &m.Notes,
			&m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ---------- Dialysis session ----------

type DialysisSession struct {
	ID                       uuid.UUID
	OrganizationID           uuid.UUID
	BranchID                 uuid.UUID
	SessionNo                string
	PatientID                uuid.UUID
	MachineID                *uuid.UUID
	AdmissionID              *uuid.UUID
	PrimaryNurseID           *uuid.UUID
	SupervisorDoctorID       *uuid.UUID
	Status                   string
	Modality                 string
	VascularAccess           string
	ScheduledAt              time.Time
	DurationMinutes          int
	PreWeightKg              *float64
	PreSystolicBP            *int
	PreDiastolicBP           *int
	DryWeightKg              *float64
	DialyzerType             *string
	Anticoagulant            *string
	UltrafiltrationTargetML  *int
	BloodFlowRate            *int
	DialysateFlowRate        *int
	StartedAt                *time.Time
	EndedAt                  *time.Time
	PostWeightKg             *float64
	PostSystolicBP           *int
	PostDiastolicBP          *int
	ActualUltrafiltrationML  *int
	Complications            *string
	SessionNotes             *string
	CancelledAt              *time.Time
	CancelledByUserID        *uuid.UUID
	CancellationReason       *string
	CreatedByUserID          *uuid.UUID
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// DialysisSessionWithJoins enriches session rows with patient + machine display
// fields for list endpoints.
type DialysisSessionWithJoins struct {
	Session          DialysisSession
	PatientMRN       string
	PatientFirstName string
	PatientLastName  string
	MachineCode      *string
	MachineName      *string
}

type DialysisSessionRepo struct {
	pool *pgxpool.Pool
}

func NewDialysisSessionRepo(pool *pgxpool.Pool) *DialysisSessionRepo {
	return &DialysisSessionRepo{pool: pool}
}

const dialysisSessionCols = `id, organization_id, branch_id, session_no, patient_id,
	machine_id, admission_id, primary_nurse_id, supervisor_doctor_id,
	status::text, modality::text, vascular_access::text,
	scheduled_at, duration_minutes,
	pre_weight_kg, pre_systolic_bp, pre_diastolic_bp, dry_weight_kg,
	dialyzer_type, anticoagulant, ultrafiltration_target_ml,
	blood_flow_rate, dialysate_flow_rate,
	started_at, ended_at,
	post_weight_kg, post_systolic_bp, post_diastolic_bp,
	actual_ultrafiltration_ml, complications, session_notes,
	cancelled_at, cancelled_by_user_id, cancellation_reason,
	created_by_user_id, created_at, updated_at`

func scanDialysisSession(scanner func(...any) error) (*DialysisSession, error) {
	s := &DialysisSession{}
	err := scanner(
		&s.ID, &s.OrganizationID, &s.BranchID, &s.SessionNo, &s.PatientID,
		&s.MachineID, &s.AdmissionID, &s.PrimaryNurseID, &s.SupervisorDoctorID,
		&s.Status, &s.Modality, &s.VascularAccess,
		&s.ScheduledAt, &s.DurationMinutes,
		&s.PreWeightKg, &s.PreSystolicBP, &s.PreDiastolicBP, &s.DryWeightKg,
		&s.DialyzerType, &s.Anticoagulant, &s.UltrafiltrationTargetML,
		&s.BloodFlowRate, &s.DialysateFlowRate,
		&s.StartedAt, &s.EndedAt,
		&s.PostWeightKg, &s.PostSystolicBP, &s.PostDiastolicBP,
		&s.ActualUltrafiltrationML, &s.Complications, &s.SessionNotes,
		&s.CancelledAt, &s.CancelledByUserID, &s.CancellationReason,
		&s.CreatedByUserID, &s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

func (r *DialysisSessionRepo) NextNo(ctx context.Context) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, `SELECT nextval('dialysis_session_no_seq')`).Scan(&n)
	return n, err
}

type CreateDialysisSessionInput struct {
	OrganizationID          uuid.UUID
	BranchID                uuid.UUID
	SessionNo               string
	PatientID               uuid.UUID
	MachineID               *uuid.UUID
	AdmissionID             *uuid.UUID
	PrimaryNurseID          *uuid.UUID
	SupervisorDoctorID      *uuid.UUID
	Modality                string
	VascularAccess          string
	ScheduledAt             time.Time
	DurationMinutes         int
	DryWeightKg             *float64
	DialyzerType            *string
	Anticoagulant           *string
	UltrafiltrationTargetML *int
	BloodFlowRate           *int
	DialysateFlowRate       *int
	CreatedByUserID         *uuid.UUID
}

func (r *DialysisSessionRepo) Create(ctx context.Context, in CreateDialysisSessionInput) (*DialysisSession, error) {
	if in.Modality == "" {
		in.Modality = "hemodialysis"
	}
	if in.VascularAccess == "" {
		in.VascularAccess = "av_fistula"
	}
	if in.DurationMinutes <= 0 {
		in.DurationMinutes = 240
	}
	row := r.pool.QueryRow(ctx,
		`INSERT INTO dialysis_session (organization_id, branch_id, session_no,
		   patient_id, machine_id, admission_id, primary_nurse_id, supervisor_doctor_id,
		   modality, vascular_access, scheduled_at, duration_minutes,
		   dry_weight_kg, dialyzer_type, anticoagulant, ultrafiltration_target_ml,
		   blood_flow_rate, dialysate_flow_rate, created_by_user_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,
		         $9::dialysis_modality, $10::vascular_access_type,
		         $11, $12, $13, $14, $15, $16, $17, $18, $19)
		 RETURNING `+dialysisSessionCols,
		in.OrganizationID, in.BranchID, in.SessionNo,
		in.PatientID, in.MachineID, in.AdmissionID, in.PrimaryNurseID, in.SupervisorDoctorID,
		in.Modality, in.VascularAccess, in.ScheduledAt, in.DurationMinutes,
		in.DryWeightKg, in.DialyzerType, in.Anticoagulant, in.UltrafiltrationTargetML,
		in.BloodFlowRate, in.DialysateFlowRate, in.CreatedByUserID)
	return scanDialysisSession(row.Scan)
}

type ListDialysisSessionFilter struct {
	Status    string
	From      *time.Time
	To        *time.Time
	PatientID *uuid.UUID
	MachineID *uuid.UUID
	Limit     int
}

func (r *DialysisSessionRepo) List(ctx context.Context, branchID uuid.UUID, f ListDialysisSessionFilter) ([]DialysisSessionWithJoins, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT s.id, s.organization_id, s.branch_id, s.session_no, s.patient_id,
	             s.machine_id, s.admission_id, s.primary_nurse_id, s.supervisor_doctor_id,
	             s.status::text, s.modality::text, s.vascular_access::text,
	             s.scheduled_at, s.duration_minutes,
	             s.pre_weight_kg, s.pre_systolic_bp, s.pre_diastolic_bp, s.dry_weight_kg,
	             s.dialyzer_type, s.anticoagulant, s.ultrafiltration_target_ml,
	             s.blood_flow_rate, s.dialysate_flow_rate,
	             s.started_at, s.ended_at,
	             s.post_weight_kg, s.post_systolic_bp, s.post_diastolic_bp,
	             s.actual_ultrafiltration_ml, s.complications, s.session_notes,
	             s.cancelled_at, s.cancelled_by_user_id, s.cancellation_reason,
	             s.created_by_user_id, s.created_at, s.updated_at,
	             p.mrn, p.first_name, p.last_name,
	             m.code, m.name
	      FROM dialysis_session s
	      JOIN patient p ON p.id = s.patient_id
	      LEFT JOIN dialysis_machine m ON m.id = s.machine_id
	      WHERE s.branch_id = $1`
	args := []any{branchID}
	if f.Status != "" {
		args = append(args, f.Status)
		q += ` AND s.status = $` + itoa(len(args)) + `::dialysis_status`
	}
	if f.From != nil {
		args = append(args, *f.From)
		q += ` AND s.scheduled_at >= $` + itoa(len(args))
	}
	if f.To != nil {
		args = append(args, *f.To)
		q += ` AND s.scheduled_at < $` + itoa(len(args))
	}
	if f.PatientID != nil {
		args = append(args, *f.PatientID)
		q += ` AND s.patient_id = $` + itoa(len(args))
	}
	if f.MachineID != nil {
		args = append(args, *f.MachineID)
		q += ` AND s.machine_id = $` + itoa(len(args))
	}
	q += ` ORDER BY s.scheduled_at ASC LIMIT ` + itoa(limit)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DialysisSessionWithJoins{}
	for rows.Next() {
		w := DialysisSessionWithJoins{}
		s := &w.Session
		if err := rows.Scan(
			&s.ID, &s.OrganizationID, &s.BranchID, &s.SessionNo, &s.PatientID,
			&s.MachineID, &s.AdmissionID, &s.PrimaryNurseID, &s.SupervisorDoctorID,
			&s.Status, &s.Modality, &s.VascularAccess,
			&s.ScheduledAt, &s.DurationMinutes,
			&s.PreWeightKg, &s.PreSystolicBP, &s.PreDiastolicBP, &s.DryWeightKg,
			&s.DialyzerType, &s.Anticoagulant, &s.UltrafiltrationTargetML,
			&s.BloodFlowRate, &s.DialysateFlowRate,
			&s.StartedAt, &s.EndedAt,
			&s.PostWeightKg, &s.PostSystolicBP, &s.PostDiastolicBP,
			&s.ActualUltrafiltrationML, &s.Complications, &s.SessionNotes,
			&s.CancelledAt, &s.CancelledByUserID, &s.CancellationReason,
			&s.CreatedByUserID, &s.CreatedAt, &s.UpdatedAt,
			&w.PatientMRN, &w.PatientFirstName, &w.PatientLastName,
			&w.MachineCode, &w.MachineName,
		); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *DialysisSessionRepo) GetByID(ctx context.Context, branchID, id uuid.UUID) (*DialysisSession, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+dialysisSessionCols+` FROM dialysis_session
		 WHERE branch_id = $1 AND id = $2`,
		branchID, id)
	return scanDialysisSession(row.Scan)
}

// UpdateStatus walks the dialysis state machine and stamps the matching
// timestamp (started_at / ended_at / cancelled_at).
func (r *DialysisSessionRepo) UpdateStatus(ctx context.Context, branchID, id uuid.UUID, status string) (*DialysisSession, error) {
	stamp := ""
	switch status {
	case "in_progress":
		stamp = ", started_at = COALESCE(started_at, NOW())"
	case "completed":
		stamp = ", ended_at = COALESCE(ended_at, NOW())"
	case "cancelled":
		stamp = ", cancelled_at = NOW()"
	}
	q := `UPDATE dialysis_session SET status = $3::dialysis_status` + stamp +
		` WHERE branch_id = $1 AND id = $2 RETURNING ` + dialysisSessionCols
	row := r.pool.QueryRow(ctx, q, branchID, id, status)
	return scanDialysisSession(row.Scan)
}

// SaveRecord stores pre/post readings + complications + notes. Caller
// transitions status explicitly so nurses can save partial records during
// the session.
type SaveDialysisRecordInput struct {
	PreWeightKg              *float64
	PreSystolicBP            *int
	PreDiastolicBP           *int
	PostWeightKg             *float64
	PostSystolicBP           *int
	PostDiastolicBP          *int
	ActualUltrafiltrationML  *int
	Complications            *string
	SessionNotes             *string
}

func (r *DialysisSessionRepo) SaveRecord(ctx context.Context, branchID, id uuid.UUID, in SaveDialysisRecordInput) (*DialysisSession, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE dialysis_session SET
		   pre_weight_kg             = COALESCE($3,  pre_weight_kg),
		   pre_systolic_bp           = COALESCE($4,  pre_systolic_bp),
		   pre_diastolic_bp          = COALESCE($5,  pre_diastolic_bp),
		   post_weight_kg            = COALESCE($6,  post_weight_kg),
		   post_systolic_bp          = COALESCE($7,  post_systolic_bp),
		   post_diastolic_bp         = COALESCE($8,  post_diastolic_bp),
		   actual_ultrafiltration_ml = COALESCE($9,  actual_ultrafiltration_ml),
		   complications             = COALESCE($10, complications),
		   session_notes             = COALESCE($11, session_notes)
		 WHERE branch_id = $1 AND id = $2
		 RETURNING `+dialysisSessionCols,
		branchID, id,
		in.PreWeightKg, in.PreSystolicBP, in.PreDiastolicBP,
		in.PostWeightKg, in.PostSystolicBP, in.PostDiastolicBP,
		in.ActualUltrafiltrationML, in.Complications, in.SessionNotes)
	return scanDialysisSession(row.Scan)
}
