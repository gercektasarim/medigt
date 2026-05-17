package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------- Ward ----------

type Ward struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	BranchID       uuid.UUID
	Code           string
	Name           string
	Kind           string
	Floor          *string
	Capacity       *int
	IsActive       bool
	Notes          *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type WardRepo struct {
	pool *pgxpool.Pool
}

func NewWardRepo(pool *pgxpool.Pool) *WardRepo { return &WardRepo{pool: pool} }

const wardCols = `id, organization_id, branch_id, code, name, kind, floor,
	capacity, is_active, notes, created_at, updated_at`

func scanWard(row pgx.Row) (*Ward, error) {
	w := &Ward{}
	err := row.Scan(&w.ID, &w.OrganizationID, &w.BranchID, &w.Code, &w.Name, &w.Kind,
		&w.Floor, &w.Capacity, &w.IsActive, &w.Notes, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return w, err
}

type CreateWardInput struct {
	OrganizationID uuid.UUID
	BranchID       uuid.UUID
	Code           string
	Name           string
	Kind           string
	Floor          *string
	Capacity       *int
	Notes          *string
}

func (r *WardRepo) Create(ctx context.Context, in CreateWardInput) (*Ward, error) {
	if in.Kind == "" {
		in.Kind = "general"
	}
	row := r.pool.QueryRow(ctx,
		`INSERT INTO ward (organization_id, branch_id, code, name, kind, floor, capacity, notes)
		 VALUES ($1,$2,$3,$4,$5::ward_kind,$6,$7,$8)
		 RETURNING `+wardCols,
		in.OrganizationID, in.BranchID, in.Code, in.Name, in.Kind,
		in.Floor, in.Capacity, in.Notes)
	return scanWard(row)
}

func (r *WardRepo) List(ctx context.Context, branchID uuid.UUID, activeOnly bool) ([]Ward, error) {
	q := `SELECT ` + wardCols + ` FROM ward WHERE branch_id = $1`
	if activeOnly {
		q += ` AND is_active = TRUE`
	}
	q += ` ORDER BY name`
	rows, err := r.pool.Query(ctx, q, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Ward{}
	for rows.Next() {
		w := Ward{}
		if err := rows.Scan(&w.ID, &w.OrganizationID, &w.BranchID, &w.Code, &w.Name, &w.Kind,
			&w.Floor, &w.Capacity, &w.IsActive, &w.Notes, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *WardRepo) GetByID(ctx context.Context, branchID, id uuid.UUID) (*Ward, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+wardCols+` FROM ward WHERE branch_id = $1 AND id = $2`,
		branchID, id)
	return scanWard(row)
}

// ---------- Bed ----------

type Bed struct {
	ID        uuid.UUID
	WardID    uuid.UUID
	Code      string
	Kind      string
	Status    string
	IsActive  bool
	Notes     *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// BedWithJoins enriches Bed with ward + (optional) current admission/patient
// data so the bed-map endpoint is a single round trip.
type BedWithJoins struct {
	Bed              Bed
	WardName         string
	WardKind         string
	AdmissionID      *uuid.UUID
	AdmissionNo      *string
	PatientID        *uuid.UUID
	PatientFirstName *string
	PatientLastName  *string
	PatientMRN       *string
	AdmittedAt       *time.Time
}

type BedRepo struct {
	pool *pgxpool.Pool
}

func NewBedRepo(pool *pgxpool.Pool) *BedRepo { return &BedRepo{pool: pool} }

const bedCols = `id, ward_id, code, kind, status, is_active, notes, created_at, updated_at`

func scanBedRow(row pgx.Row) (*Bed, error) {
	b := &Bed{}
	err := row.Scan(&b.ID, &b.WardID, &b.Code, &b.Kind, &b.Status,
		&b.IsActive, &b.Notes, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

type CreateBedInput struct {
	WardID uuid.UUID
	Code   string
	Kind   string
	Notes  *string
}

func (r *BedRepo) Create(ctx context.Context, in CreateBedInput) (*Bed, error) {
	if in.Kind == "" {
		in.Kind = "standard"
	}
	row := r.pool.QueryRow(ctx,
		`INSERT INTO bed (ward_id, code, kind, notes)
		 VALUES ($1, $2, $3::bed_kind, $4)
		 RETURNING `+bedCols,
		in.WardID, in.Code, in.Kind, in.Notes)
	return scanBedRow(row)
}

// SetStatus updates the bed status (housekeeping use-cases — flipping from
// 'cleaning' to 'free'). Admission flows use the service layer instead.
func (r *BedRepo) SetStatus(ctx context.Context, id uuid.UUID, status string) error {
	res, err := r.pool.Exec(ctx,
		`UPDATE bed SET status = $2::bed_status WHERE id = $1`, id, status)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// BedMap returns every bed in the branch joined with its ward + current
// active admission/patient (if any). The frontend renders this as a colour-
// coded grid grouped by ward.
func (r *BedRepo) BedMap(ctx context.Context, branchID uuid.UUID) ([]BedWithJoins, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT b.id, b.ward_id, b.code, b.kind, b.status, b.is_active,
		        b.notes, b.created_at, b.updated_at,
		        w.name, w.kind,
		        a.id, a.admission_no, a.patient_id, p.first_name, p.last_name,
		        p.mrn, a.admitted_at
		 FROM bed b
		 JOIN ward w ON w.id = b.ward_id
		 LEFT JOIN admission a ON a.bed_id = b.id AND a.status = 'active'
		 LEFT JOIN patient p ON p.id = a.patient_id
		 WHERE w.branch_id = $1 AND b.is_active = TRUE AND w.is_active = TRUE
		 ORDER BY w.name, b.code`,
		branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BedWithJoins{}
	for rows.Next() {
		j := BedWithJoins{}
		b := &j.Bed
		if err := rows.Scan(&b.ID, &b.WardID, &b.Code, &b.Kind, &b.Status, &b.IsActive,
			&b.Notes, &b.CreatedAt, &b.UpdatedAt,
			&j.WardName, &j.WardKind,
			&j.AdmissionID, &j.AdmissionNo, &j.PatientID, &j.PatientFirstName,
			&j.PatientLastName, &j.PatientMRN, &j.AdmittedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// ---------- Admission ----------

type Admission struct {
	ID                   uuid.UUID
	OrganizationID       uuid.UUID
	BranchID             uuid.UUID
	AdmissionNo          string
	PatientID            uuid.UUID
	AdmittingDoctorID    *uuid.UUID
	WardID               uuid.UUID
	BedID                *uuid.UUID
	Kind                 string
	Status               string
	ChiefComplaint       *string
	AdmissionDiagnosis   *string
	Notes                *string
	AdmittedAt           time.Time
	AdmittedByUserID     *uuid.UUID
	DischargedAt         *time.Time
	DischargeKind        *string
	DischargeSummary     *string
	DischargedByUserID   *uuid.UUID
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type AdmissionWithJoins struct {
	Admission        Admission
	PatientMRN       string
	PatientFirstName string
	PatientLastName  string
	PatientPhone     *string
	WardCode         string
	WardName         string
	BedCode          *string
	DoctorFirstName  *string
	DoctorLastName   *string
	DoctorTitle      *string
}

type AdmissionRepo struct {
	pool *pgxpool.Pool
}

func NewAdmissionRepo(pool *pgxpool.Pool) *AdmissionRepo { return &AdmissionRepo{pool: pool} }

const admissionCols = `id, organization_id, branch_id, admission_no, patient_id,
	admitting_doctor_id, ward_id, bed_id, kind, status, chief_complaint,
	admission_diagnosis, notes, admitted_at, admitted_by_user_id,
	discharged_at, discharge_kind, discharge_summary, discharged_by_user_id,
	created_at, updated_at`

func (r *AdmissionRepo) NextNo(ctx context.Context) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, `SELECT nextval('admission_no_seq')`).Scan(&n)
	return n, err
}

type ListAdmissionFilter struct {
	Status string
	WardID *uuid.UUID
	Limit  int
}

func (r *AdmissionRepo) List(ctx context.Context, branchID uuid.UUID, f ListAdmissionFilter) ([]AdmissionWithJoins, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT a.id, a.organization_id, a.branch_id, a.admission_no, a.patient_id,
	             a.admitting_doctor_id, a.ward_id, a.bed_id, a.kind, a.status,
	             a.chief_complaint, a.admission_diagnosis, a.notes,
	             a.admitted_at, a.admitted_by_user_id,
	             a.discharged_at, a.discharge_kind, a.discharge_summary,
	             a.discharged_by_user_id, a.created_at, a.updated_at,
	             p.mrn, p.first_name, p.last_name, p.phone,
	             w.code, w.name,
	             b.code,
	             ds.first_name, ds.last_name, ds.title
	      FROM admission a
	      JOIN patient p ON p.id = a.patient_id
	      JOIN ward w ON w.id = a.ward_id
	      LEFT JOIN bed b ON b.id = a.bed_id
	      LEFT JOIN doctor d ON d.id = a.admitting_doctor_id
	      LEFT JOIN staff_member ds ON ds.id = d.staff_member_id
	      WHERE a.branch_id = $1`
	args := []any{branchID}
	if f.Status != "" {
		args = append(args, f.Status)
		q += ` AND a.status = $` + itoa(len(args)) + `::admission_status`
	}
	if f.WardID != nil {
		args = append(args, *f.WardID)
		q += ` AND a.ward_id = $` + itoa(len(args))
	}
	q += ` ORDER BY a.admitted_at DESC LIMIT ` + itoa(limit)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdmissionWithJoins{}
	for rows.Next() {
		j := AdmissionWithJoins{}
		a := &j.Admission
		if err := rows.Scan(
			&a.ID, &a.OrganizationID, &a.BranchID, &a.AdmissionNo, &a.PatientID,
			&a.AdmittingDoctorID, &a.WardID, &a.BedID, &a.Kind, &a.Status,
			&a.ChiefComplaint, &a.AdmissionDiagnosis, &a.Notes,
			&a.AdmittedAt, &a.AdmittedByUserID,
			&a.DischargedAt, &a.DischargeKind, &a.DischargeSummary,
			&a.DischargedByUserID, &a.CreatedAt, &a.UpdatedAt,
			&j.PatientMRN, &j.PatientFirstName, &j.PatientLastName, &j.PatientPhone,
			&j.WardCode, &j.WardName, &j.BedCode,
			&j.DoctorFirstName, &j.DoctorLastName, &j.DoctorTitle,
		); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (r *AdmissionRepo) GetByID(ctx context.Context, branchID, id uuid.UUID) (*AdmissionWithJoins, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT a.id, a.organization_id, a.branch_id, a.admission_no, a.patient_id,
		        a.admitting_doctor_id, a.ward_id, a.bed_id, a.kind, a.status,
		        a.chief_complaint, a.admission_diagnosis, a.notes,
		        a.admitted_at, a.admitted_by_user_id,
		        a.discharged_at, a.discharge_kind, a.discharge_summary,
		        a.discharged_by_user_id, a.created_at, a.updated_at,
		        p.mrn, p.first_name, p.last_name, p.phone,
		        w.code, w.name,
		        b.code,
		        ds.first_name, ds.last_name, ds.title
		 FROM admission a
		 JOIN patient p ON p.id = a.patient_id
		 JOIN ward w ON w.id = a.ward_id
		 LEFT JOIN bed b ON b.id = a.bed_id
		 LEFT JOIN doctor d ON d.id = a.admitting_doctor_id
		 LEFT JOIN staff_member ds ON ds.id = d.staff_member_id
		 WHERE a.branch_id = $1 AND a.id = $2`,
		branchID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, ErrNotFound
	}
	j := AdmissionWithJoins{}
	a := &j.Admission
	if err := rows.Scan(
		&a.ID, &a.OrganizationID, &a.BranchID, &a.AdmissionNo, &a.PatientID,
		&a.AdmittingDoctorID, &a.WardID, &a.BedID, &a.Kind, &a.Status,
		&a.ChiefComplaint, &a.AdmissionDiagnosis, &a.Notes,
		&a.AdmittedAt, &a.AdmittedByUserID,
		&a.DischargedAt, &a.DischargeKind, &a.DischargeSummary,
		&a.DischargedByUserID, &a.CreatedAt, &a.UpdatedAt,
		&j.PatientMRN, &j.PatientFirstName, &j.PatientLastName, &j.PatientPhone,
		&j.WardCode, &j.WardName, &j.BedCode,
		&j.DoctorFirstName, &j.DoctorLastName, &j.DoctorTitle,
	); err != nil {
		return nil, err
	}
	return &j, nil
}

// ---------- Bed transfer audit ----------

type BedTransfer struct {
	ID                  uuid.UUID
	AdmissionID         uuid.UUID
	FromBedID           *uuid.UUID
	ToBedID             uuid.UUID
	TransferredAt       time.Time
	TransferredByUserID *uuid.UUID
	Reason              *string
	CreatedAt           time.Time

	// Joined fields for the audit list.
	FromBedCode  *string
	ToBedCode    string
	FromWardName *string
	ToWardName   string
}

// InpatientBoardRow is the row shape returned by ListInpatientBoard — admission
// + ward/bed display + the patient's most recent vital signs (if any). This is
// what the nursing dashboard pulls in one round trip via a LATERAL join.
type InpatientBoardRow struct {
	Admission        Admission
	PatientFirstName string
	PatientLastName  string
	PatientMRN       string
	WardName         string
	WardKind         string
	BedCode          *string

	// Latest vital sample (nullable — no vitals taken yet).
	VitalsMeasuredAt *time.Time
	SystolicBP       *int
	DiastolicBP      *int
	Pulse            *int
	TemperatureC     *float64
	Spo2Percent      *int
	Respiration      *int
	PainScore        *int
}

func (r *AdmissionRepo) ListInpatientBoard(ctx context.Context, branchID uuid.UUID, wardID *uuid.UUID) ([]InpatientBoardRow, error) {
	args := []any{branchID}
	q := `SELECT a.id, a.organization_id, a.branch_id, a.admission_no, a.patient_id,
	             a.admitting_doctor_id, a.ward_id, a.bed_id, a.kind, a.status,
	             a.chief_complaint, a.admission_diagnosis, a.notes,
	             a.admitted_at, a.admitted_by_user_id,
	             a.discharged_at, a.discharge_kind, a.discharge_summary,
	             a.discharged_by_user_id, a.created_at, a.updated_at,
	             p.first_name, p.last_name, p.mrn,
	             w.name, w.kind::text, b.code,
	             v.measured_at, v.systolic_bp, v.diastolic_bp, v.pulse,
	             v.temperature_c, v.spo2_percent, v.respiration, v.pain_score
	      FROM admission a
	      JOIN patient p ON p.id = a.patient_id
	      JOIN ward w ON w.id = a.ward_id
	      LEFT JOIN bed b ON b.id = a.bed_id
	      LEFT JOIN LATERAL (
	          SELECT measured_at, systolic_bp, diastolic_bp, pulse,
	                 temperature_c, spo2_percent, respiration, pain_score
	          FROM vital_signs
	          WHERE patient_id = a.patient_id
	          ORDER BY measured_at DESC
	          LIMIT 1
	      ) v ON TRUE
	      WHERE a.branch_id = $1 AND a.status = 'active'`
	if wardID != nil {
		args = append(args, *wardID)
		q += ` AND a.ward_id = $2`
	}
	q += ` ORDER BY w.name, b.code NULLS LAST`

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InpatientBoardRow{}
	for rows.Next() {
		j := InpatientBoardRow{}
		a := &j.Admission
		if err := rows.Scan(
			&a.ID, &a.OrganizationID, &a.BranchID, &a.AdmissionNo, &a.PatientID,
			&a.AdmittingDoctorID, &a.WardID, &a.BedID, &a.Kind, &a.Status,
			&a.ChiefComplaint, &a.AdmissionDiagnosis, &a.Notes,
			&a.AdmittedAt, &a.AdmittedByUserID,
			&a.DischargedAt, &a.DischargeKind, &a.DischargeSummary,
			&a.DischargedByUserID, &a.CreatedAt, &a.UpdatedAt,
			&j.PatientFirstName, &j.PatientLastName, &j.PatientMRN,
			&j.WardName, &j.WardKind, &j.BedCode,
			&j.VitalsMeasuredAt, &j.SystolicBP, &j.DiastolicBP, &j.Pulse,
			&j.TemperatureC, &j.Spo2Percent, &j.Respiration, &j.PainScore,
		); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (r *AdmissionRepo) ListTransfers(ctx context.Context, admissionID uuid.UUID) ([]BedTransfer, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT bt.id, bt.admission_id, bt.from_bed_id, bt.to_bed_id,
		        bt.transferred_at, bt.transferred_by_user_id, bt.reason, bt.created_at,
		        fb.code, tb.code, fw.name, tw.name
		 FROM bed_transfer bt
		 LEFT JOIN bed fb ON fb.id = bt.from_bed_id
		 JOIN bed tb ON tb.id = bt.to_bed_id
		 LEFT JOIN ward fw ON fw.id = fb.ward_id
		 JOIN ward tw ON tw.id = tb.ward_id
		 WHERE bt.admission_id = $1
		 ORDER BY bt.transferred_at DESC`, admissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BedTransfer{}
	for rows.Next() {
		t := BedTransfer{}
		if err := rows.Scan(&t.ID, &t.AdmissionID, &t.FromBedID, &t.ToBedID,
			&t.TransferredAt, &t.TransferredByUserID, &t.Reason, &t.CreatedAt,
			&t.FromBedCode, &t.ToBedCode, &t.FromWardName, &t.ToWardName); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
