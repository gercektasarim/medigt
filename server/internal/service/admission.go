package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/medigt/medigt/server/internal/util"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

var (
	ErrBedUnavailable      = errors.New("seçilen yatak boş değil")
	ErrBedWrongWard        = errors.New("seçilen yatak bu serviste değil")
	ErrPatientAlreadyAdmitted = errors.New("bu hasta zaten yatıyor")
	ErrAdmissionMissing    = errors.New("yatış kaydı bulunamadı")
	ErrAdmissionClosed     = errors.New("yatış zaten taburcu/iptal edilmiş")
)

// AdmissionService owns the multi-table mutations that touch bed status,
// admission rows, and bed_transfer audit. Everything runs inside an explicit
// transaction with SELECT … FOR UPDATE on the target bed so two clerks
// can't double-book the same physical bed.
type AdmissionService struct {
	pool *pgxpool.Pool
}

func NewAdmissionService(pool *pgxpool.Pool) *AdmissionService {
	return &AdmissionService{pool: pool}
}

type AdmitInput struct {
	OrganizationID     uuid.UUID
	BranchID           uuid.UUID
	PatientID          uuid.UUID
	WardID             uuid.UUID
	BedID              *uuid.UUID // may be nil — "ward only", bed assignment later
	AdmittingDoctorID  *uuid.UUID
	Kind               string     // planned | emergency | transfer_in | newborn
	ChiefComplaint     *string
	AdmissionDiagnosis *string
	Notes              *string
	AdmittedByUserID   *uuid.UUID
}

// Admit creates a new admission. If a bed is selected, this:
//   - locks the bed row
//   - confirms it's free and in the selected ward
//   - flips bed.status='occupied'
// All inside one tx, idempotent under concurrent calls because the
// unique partial index on admission(patient_id) WHERE status='active'
// rejects a second active admission for the same patient.
func (s *AdmissionService) Admit(ctx context.Context, in AdmitInput) (*repo.Admission, error) {
	if in.Kind == "" {
		in.Kind = "planned"
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Bed check (if specified).
	if in.BedID != nil {
		var status string
		var wardID uuid.UUID
		err := tx.QueryRow(ctx,
			`SELECT b.status, b.ward_id FROM bed b
			 WHERE b.id = $1 AND b.is_active = TRUE FOR UPDATE`,
			*in.BedID,
		).Scan(&status, &wardID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, repo.ErrNotFound
			}
			return nil, err
		}
		if wardID != in.WardID {
			return nil, ErrBedWrongWard
		}
		if status != "free" {
			return nil, ErrBedUnavailable
		}
	}

	// MRN-style admission number.
	var admissionNum int64
	if err := tx.QueryRow(ctx, `SELECT nextval('admission_no_seq')`).Scan(&admissionNum); err != nil {
		return nil, err
	}
	admissionNo := util.FormatMRN(admissionNum)

	row := tx.QueryRow(ctx,
		`INSERT INTO admission (organization_id, branch_id, admission_no, patient_id,
		   admitting_doctor_id, ward_id, bed_id, kind, chief_complaint,
		   admission_diagnosis, notes, admitted_by_user_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8::admission_kind, $9, $10, $11, $12)
		 RETURNING id, organization_id, branch_id, admission_no, patient_id,
		   admitting_doctor_id, ward_id, bed_id, kind, status, chief_complaint,
		   admission_diagnosis, notes, admitted_at, admitted_by_user_id,
		   discharged_at, discharge_kind, discharge_summary, discharged_by_user_id,
		   created_at, updated_at`,
		in.OrganizationID, in.BranchID, admissionNo, in.PatientID, in.AdmittingDoctorID,
		in.WardID, in.BedID, in.Kind, in.ChiefComplaint, in.AdmissionDiagnosis,
		in.Notes, in.AdmittedByUserID)

	a := &repo.Admission{}
	if err := row.Scan(&a.ID, &a.OrganizationID, &a.BranchID, &a.AdmissionNo,
		&a.PatientID, &a.AdmittingDoctorID, &a.WardID, &a.BedID, &a.Kind, &a.Status,
		&a.ChiefComplaint, &a.AdmissionDiagnosis, &a.Notes,
		&a.AdmittedAt, &a.AdmittedByUserID,
		&a.DischargedAt, &a.DischargeKind, &a.DischargeSummary,
		&a.DischargedByUserID, &a.CreatedAt, &a.UpdatedAt); err != nil {
		if isUniqueViolationStr(err.Error()) {
			return nil, ErrPatientAlreadyAdmitted
		}
		return nil, err
	}

	if in.BedID != nil {
		if _, err := tx.Exec(ctx,
			`UPDATE bed SET status = 'occupied' WHERE id = $1`, *in.BedID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

type TransferInput struct {
	BranchID            uuid.UUID
	AdmissionID         uuid.UUID
	ToBedID             uuid.UUID
	TransferredByUserID *uuid.UUID
	Reason              *string
}

// Transfer moves an active admission from its current bed (if any) to a new
// bed. The two bed rows are locked together (lowest id first to avoid the
// classic A→B / B→A deadlock), then statuses flipped, an audit row written,
// and the admission's bed_id updated. All in one tx.
func (s *AdmissionService) Transfer(ctx context.Context, in TransferInput) (*repo.Admission, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock admission first to confirm it's active.
	var status string
	var currentBed *uuid.UUID
	var wardID uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT status, bed_id, ward_id FROM admission
		 WHERE branch_id = $1 AND id = $2 FOR UPDATE`,
		in.BranchID, in.AdmissionID,
	).Scan(&status, &currentBed, &wardID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAdmissionMissing
		}
		return nil, err
	}
	if status != "active" {
		return nil, ErrAdmissionClosed
	}

	// Lock the target bed (lowest UUID first when an old bed is also locked).
	type lockedBed struct {
		id     uuid.UUID
		status string
		ward   uuid.UUID
	}
	lockOrder := []*uuid.UUID{}
	if currentBed != nil {
		lockOrder = append(lockOrder, currentBed)
	}
	lockOrder = append(lockOrder, &in.ToBedID)
	// Sort by string repr to ensure deterministic ordering.
	if len(lockOrder) == 2 && lockOrder[0].String() > lockOrder[1].String() {
		lockOrder[0], lockOrder[1] = lockOrder[1], lockOrder[0]
	}
	beds := map[uuid.UUID]lockedBed{}
	for _, bid := range lockOrder {
		b := lockedBed{id: *bid}
		if err := tx.QueryRow(ctx,
			`SELECT status, ward_id FROM bed WHERE id = $1 FOR UPDATE`, *bid,
		).Scan(&b.status, &b.ward); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, repo.ErrNotFound
			}
			return nil, err
		}
		beds[*bid] = b
	}

	target := beds[in.ToBedID]
	if target.status != "free" {
		return nil, ErrBedUnavailable
	}

	// Update beds.
	if _, err := tx.Exec(ctx,
		`UPDATE bed SET status = 'occupied' WHERE id = $1`, in.ToBedID); err != nil {
		return nil, err
	}
	if currentBed != nil && *currentBed != in.ToBedID {
		if _, err := tx.Exec(ctx,
			`UPDATE bed SET status = 'cleaning' WHERE id = $1`, *currentBed); err != nil {
			return nil, err
		}
	}

	// Update admission.
	row := tx.QueryRow(ctx,
		`UPDATE admission SET bed_id = $1, ward_id = $2 WHERE id = $3
		 RETURNING id, organization_id, branch_id, admission_no, patient_id,
		   admitting_doctor_id, ward_id, bed_id, kind, status, chief_complaint,
		   admission_diagnosis, notes, admitted_at, admitted_by_user_id,
		   discharged_at, discharge_kind, discharge_summary, discharged_by_user_id,
		   created_at, updated_at`,
		in.ToBedID, target.ward, in.AdmissionID)
	a := &repo.Admission{}
	if err := row.Scan(&a.ID, &a.OrganizationID, &a.BranchID, &a.AdmissionNo,
		&a.PatientID, &a.AdmittingDoctorID, &a.WardID, &a.BedID, &a.Kind, &a.Status,
		&a.ChiefComplaint, &a.AdmissionDiagnosis, &a.Notes,
		&a.AdmittedAt, &a.AdmittedByUserID,
		&a.DischargedAt, &a.DischargeKind, &a.DischargeSummary,
		&a.DischargedByUserID, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, err
	}

	// Audit.
	if _, err := tx.Exec(ctx,
		`INSERT INTO bed_transfer (admission_id, from_bed_id, to_bed_id,
		   transferred_by_user_id, reason)
		 VALUES ($1, $2, $3, $4, $5)`,
		in.AdmissionID, currentBed, in.ToBedID, in.TransferredByUserID, in.Reason); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

type DischargeInput struct {
	BranchID              uuid.UUID
	AdmissionID           uuid.UUID
	Kind                  string // home | home_with_help | referred | against_advice | left_without_notice | transferred | expired
	Summary               *string
	DischargedByUserID    *uuid.UUID
}

// Discharge closes the admission and frees up the bed (status='cleaning').
func (s *AdmissionService) Discharge(ctx context.Context, in DischargeInput) (*repo.Admission, error) {
	if in.Kind == "" {
		in.Kind = "home"
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	var bedID *uuid.UUID
	if err := tx.QueryRow(ctx,
		`SELECT status, bed_id FROM admission
		 WHERE branch_id = $1 AND id = $2 FOR UPDATE`,
		in.BranchID, in.AdmissionID,
	).Scan(&status, &bedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAdmissionMissing
		}
		return nil, err
	}
	if status != "active" {
		return nil, ErrAdmissionClosed
	}

	row := tx.QueryRow(ctx,
		`UPDATE admission SET
		   status = 'discharged',
		   discharged_at = NOW(),
		   discharge_kind = $2::discharge_kind,
		   discharge_summary = $3,
		   discharged_by_user_id = $4
		 WHERE id = $1
		 RETURNING id, organization_id, branch_id, admission_no, patient_id,
		   admitting_doctor_id, ward_id, bed_id, kind, status, chief_complaint,
		   admission_diagnosis, notes, admitted_at, admitted_by_user_id,
		   discharged_at, discharge_kind, discharge_summary, discharged_by_user_id,
		   created_at, updated_at`,
		in.AdmissionID, in.Kind, in.Summary, in.DischargedByUserID)
	a := &repo.Admission{}
	if err := row.Scan(&a.ID, &a.OrganizationID, &a.BranchID, &a.AdmissionNo,
		&a.PatientID, &a.AdmittingDoctorID, &a.WardID, &a.BedID, &a.Kind, &a.Status,
		&a.ChiefComplaint, &a.AdmissionDiagnosis, &a.Notes,
		&a.AdmittedAt, &a.AdmittedByUserID,
		&a.DischargedAt, &a.DischargeKind, &a.DischargeSummary,
		&a.DischargedByUserID, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, err
	}

	if bedID != nil {
		if _, err := tx.Exec(ctx,
			`UPDATE bed SET status = 'cleaning' WHERE id = $1`, *bedID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

// isUniqueViolationStr keeps service/admission.go free of a pgx import
// just to typecheck the error type; handler-side has the same helper.
func isUniqueViolationStr(s string) bool {
	for i := 0; i+5 <= len(s); i++ {
		if s[i:i+5] == "23505" {
			return true
		}
	}
	return false
}
