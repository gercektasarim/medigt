package repo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------- Operating room ----------

type OperatingRoom struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	BranchID       uuid.UUID
	Code           string
	Name           string
	Floor          *string
	IsActive       bool
	Notes          *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type OperatingRoomRepo struct {
	pool *pgxpool.Pool
}

func NewOperatingRoomRepo(pool *pgxpool.Pool) *OperatingRoomRepo {
	return &OperatingRoomRepo{pool: pool}
}

const orCols = `id, organization_id, branch_id, code, name, floor, is_active, notes, created_at, updated_at`

func scanOR(row pgx.Row) (*OperatingRoom, error) {
	o := &OperatingRoom{}
	err := row.Scan(&o.ID, &o.OrganizationID, &o.BranchID, &o.Code, &o.Name,
		&o.Floor, &o.IsActive, &o.Notes, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
}

type CreateORInput struct {
	OrganizationID uuid.UUID
	BranchID       uuid.UUID
	Code           string
	Name           string
	Floor          *string
	Notes          *string
}

func (r *OperatingRoomRepo) Create(ctx context.Context, in CreateORInput) (*OperatingRoom, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO operating_room (organization_id, branch_id, code, name, floor, notes)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+orCols,
		in.OrganizationID, in.BranchID, in.Code, in.Name, in.Floor, in.Notes)
	return scanOR(row)
}

func (r *OperatingRoomRepo) List(ctx context.Context, branchID uuid.UUID) ([]OperatingRoom, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+orCols+` FROM operating_room
		 WHERE branch_id = $1 AND is_active = TRUE
		 ORDER BY name`, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OperatingRoom{}
	for rows.Next() {
		o := OperatingRoom{}
		if err := rows.Scan(&o.ID, &o.OrganizationID, &o.BranchID, &o.Code, &o.Name,
			&o.Floor, &o.IsActive, &o.Notes, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ---------- Surgery ----------

type SurgeryTeamMember struct {
	StaffMemberID *uuid.UUID `json:"staff_member_id,omitempty"`
	DoctorID      *uuid.UUID `json:"doctor_id,omitempty"`
	Role          string     `json:"role"` // primary_surgeon | assistant | anesthesiologist | scrub_nurse | circulating_nurse | technician
	Name          string     `json:"name"` // display label (snapshot)
}

type Surgery struct {
	ID                  uuid.UUID
	OrganizationID      uuid.UUID
	BranchID            uuid.UUID
	SurgeryNo           string
	PatientID           uuid.UUID
	OperatingRoomID     uuid.UUID
	PrimarySurgeonID    *uuid.UUID
	AdmissionID         *uuid.UUID
	Status              string
	Priority            string
	ProcedureName       string
	ProcedureCodes      []string
	Indication          *string
	AnesthesiaType      string
	ScheduledAt         time.Time
	EstimatedMinutes    int
	Team                []SurgeryTeamMember
	StartedAt           *time.Time
	EndedAt             *time.Time
	OpNote              *string
	Complications       *string
	BloodLossML         *int
	SpecimenSent        bool
	CancelledAt         *time.Time
	CancelledByUserID   *uuid.UUID
	CancellationReason  *string
	CreatedByUserID     *uuid.UUID
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// SurgeryWithJoins enriches Surgery with patient + operating room + primary
// surgeon display fields for list endpoints.
type SurgeryWithJoins struct {
	Surgery          Surgery
	PatientMRN       string
	PatientFirstName string
	PatientLastName  string
	OperatingRoomCode string
	OperatingRoomName string
	SurgeonFirstName *string
	SurgeonLastName  *string
	SurgeonTitle     *string
}

type SurgeryRepo struct {
	pool *pgxpool.Pool
}

func NewSurgeryRepo(pool *pgxpool.Pool) *SurgeryRepo { return &SurgeryRepo{pool: pool} }

const surgeryCols = `id, organization_id, branch_id, surgery_no, patient_id,
	operating_room_id, primary_surgeon_id, admission_id, status, priority,
	procedure_name, procedure_codes, indication, anesthesia_type::text,
	scheduled_at, estimated_minutes, team, started_at, ended_at, op_note,
	complications, blood_loss_ml, specimen_sent, cancelled_at,
	cancelled_by_user_id, cancellation_reason, created_by_user_id,
	created_at, updated_at`

func scanSurgery(scanner func(...any) error) (*Surgery, error) {
	s := &Surgery{}
	var procCodes, team []byte
	err := scanner(
		&s.ID, &s.OrganizationID, &s.BranchID, &s.SurgeryNo, &s.PatientID,
		&s.OperatingRoomID, &s.PrimarySurgeonID, &s.AdmissionID, &s.Status, &s.Priority,
		&s.ProcedureName, &procCodes, &s.Indication, &s.AnesthesiaType,
		&s.ScheduledAt, &s.EstimatedMinutes, &team, &s.StartedAt, &s.EndedAt, &s.OpNote,
		&s.Complications, &s.BloodLossML, &s.SpecimenSent, &s.CancelledAt,
		&s.CancelledByUserID, &s.CancellationReason, &s.CreatedByUserID,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(procCodes) > 0 {
		_ = json.Unmarshal(procCodes, &s.ProcedureCodes)
	}
	if len(team) > 0 {
		_ = json.Unmarshal(team, &s.Team)
	}
	return s, nil
}

func (r *SurgeryRepo) NextNo(ctx context.Context) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, `SELECT nextval('surgery_no_seq')`).Scan(&n)
	return n, err
}

type CreateSurgeryInput struct {
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	SurgeryNo         string
	PatientID         uuid.UUID
	OperatingRoomID   uuid.UUID
	PrimarySurgeonID  *uuid.UUID
	AdmissionID       *uuid.UUID
	Priority          string
	ProcedureName     string
	ProcedureCodes    []string
	Indication        *string
	AnesthesiaType    string
	ScheduledAt       time.Time
	EstimatedMinutes  int
	Team              []SurgeryTeamMember
	CreatedByUserID   *uuid.UUID
}

func (r *SurgeryRepo) Create(ctx context.Context, in CreateSurgeryInput) (*Surgery, error) {
	if in.Priority == "" {
		in.Priority = "elective"
	}
	if in.AnesthesiaType == "" {
		in.AnesthesiaType = "general"
	}
	if in.EstimatedMinutes <= 0 {
		in.EstimatedMinutes = 60
	}
	procCodes, _ := json.Marshal(in.ProcedureCodes)
	if len(procCodes) == 0 {
		procCodes = []byte("[]")
	}
	team, _ := json.Marshal(in.Team)
	if len(team) == 0 {
		team = []byte("[]")
	}
	row := r.pool.QueryRow(ctx,
		`INSERT INTO surgery (organization_id, branch_id, surgery_no, patient_id,
		   operating_room_id, primary_surgeon_id, admission_id, priority,
		   procedure_name, procedure_codes, indication,
		   anesthesia_type, scheduled_at, estimated_minutes, team,
		   created_by_user_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8::surgery_priority,$9,$10,$11,
		         $12::anesthesia_type,$13,$14,$15,$16)
		 RETURNING `+surgeryCols,
		in.OrganizationID, in.BranchID, in.SurgeryNo, in.PatientID,
		in.OperatingRoomID, in.PrimarySurgeonID, in.AdmissionID, in.Priority,
		in.ProcedureName, procCodes, in.Indication,
		in.AnesthesiaType, in.ScheduledAt, in.EstimatedMinutes, team,
		in.CreatedByUserID)
	return scanSurgery(row.Scan)
}

type ListSurgeryFilter struct {
	Status string
	From   *time.Time
	To     *time.Time
	ORID   *uuid.UUID
	Limit  int
}

func (r *SurgeryRepo) List(ctx context.Context, branchID uuid.UUID, f ListSurgeryFilter) ([]SurgeryWithJoins, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT s.id, s.organization_id, s.branch_id, s.surgery_no, s.patient_id,
	             s.operating_room_id, s.primary_surgeon_id, s.admission_id,
	             s.status, s.priority, s.procedure_name, s.procedure_codes,
	             s.indication, s.anesthesia_type::text, s.scheduled_at,
	             s.estimated_minutes, s.team, s.started_at, s.ended_at, s.op_note,
	             s.complications, s.blood_loss_ml, s.specimen_sent, s.cancelled_at,
	             s.cancelled_by_user_id, s.cancellation_reason, s.created_by_user_id,
	             s.created_at, s.updated_at,
	             p.mrn, p.first_name, p.last_name,
	             ord.code, ord.name,
	             ds.first_name, ds.last_name, ds.title
	      FROM surgery s
	      JOIN patient p ON p.id = s.patient_id
	      JOIN operating_room ord ON ord.id = s.operating_room_id
	      LEFT JOIN doctor d ON d.id = s.primary_surgeon_id
	      LEFT JOIN staff_member ds ON ds.id = d.staff_member_id
	      WHERE s.branch_id = $1`
	args := []any{branchID}
	if f.Status != "" {
		args = append(args, f.Status)
		q += ` AND s.status = $` + itoa(len(args)) + `::surgery_status`
	}
	if f.From != nil {
		args = append(args, *f.From)
		q += ` AND s.scheduled_at >= $` + itoa(len(args))
	}
	if f.To != nil {
		args = append(args, *f.To)
		q += ` AND s.scheduled_at < $` + itoa(len(args))
	}
	if f.ORID != nil {
		args = append(args, *f.ORID)
		q += ` AND s.operating_room_id = $` + itoa(len(args))
	}
	q += ` ORDER BY s.scheduled_at ASC LIMIT ` + itoa(limit)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SurgeryWithJoins{}
	for rows.Next() {
		w := SurgeryWithJoins{}
		s := &w.Surgery
		var procCodes, team []byte
		if err := rows.Scan(
			&s.ID, &s.OrganizationID, &s.BranchID, &s.SurgeryNo, &s.PatientID,
			&s.OperatingRoomID, &s.PrimarySurgeonID, &s.AdmissionID,
			&s.Status, &s.Priority, &s.ProcedureName, &procCodes,
			&s.Indication, &s.AnesthesiaType, &s.ScheduledAt,
			&s.EstimatedMinutes, &team, &s.StartedAt, &s.EndedAt, &s.OpNote,
			&s.Complications, &s.BloodLossML, &s.SpecimenSent, &s.CancelledAt,
			&s.CancelledByUserID, &s.CancellationReason, &s.CreatedByUserID,
			&s.CreatedAt, &s.UpdatedAt,
			&w.PatientMRN, &w.PatientFirstName, &w.PatientLastName,
			&w.OperatingRoomCode, &w.OperatingRoomName,
			&w.SurgeonFirstName, &w.SurgeonLastName, &w.SurgeonTitle,
		); err != nil {
			return nil, err
		}
		if len(procCodes) > 0 {
			_ = json.Unmarshal(procCodes, &s.ProcedureCodes)
		}
		if len(team) > 0 {
			_ = json.Unmarshal(team, &s.Team)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *SurgeryRepo) GetByID(ctx context.Context, branchID, id uuid.UUID) (*Surgery, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+surgeryCols+` FROM surgery WHERE branch_id = $1 AND id = $2`,
		branchID, id)
	return scanSurgery(row.Scan)
}

// UpdateStatus walks the state machine and stamps the matching timestamp.
func (r *SurgeryRepo) UpdateStatus(ctx context.Context, branchID, id uuid.UUID, status string) (*Surgery, error) {
	stamp := ""
	switch status {
	case "in_progress":
		stamp = ", started_at = COALESCE(started_at, NOW())"
	case "completed":
		stamp = ", ended_at = COALESCE(ended_at, NOW())"
	case "cancelled":
		stamp = ", cancelled_at = NOW()"
	}
	q := `UPDATE surgery SET status = $3::surgery_status` + stamp +
		` WHERE branch_id = $1 AND id = $2 RETURNING ` + surgeryCols
	row := r.pool.QueryRow(ctx, q, branchID, id, status)
	return scanSurgery(row.Scan)
}

// SaveOpNote stores post-op narrative + optional complications / blood loss /
// specimen flag. Status doesn't auto-bump here — caller transitions to
// 'completed' explicitly so the team can save drafts during the procedure.
func (r *SurgeryRepo) SaveOpNote(ctx context.Context, branchID, id uuid.UUID,
	opNote, complications *string, bloodLoss *int, specimen *bool) (*Surgery, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE surgery SET
		   op_note       = COALESCE($3, op_note),
		   complications = COALESCE($4, complications),
		   blood_loss_ml = COALESCE($5, blood_loss_ml),
		   specimen_sent = COALESCE($6, specimen_sent)
		 WHERE branch_id = $1 AND id = $2
		 RETURNING `+surgeryCols,
		branchID, id, opNote, complications, bloodLoss, specimen)
	return scanSurgery(row.Scan)
}
