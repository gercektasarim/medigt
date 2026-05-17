package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/medigt/medigt/server/pkg/db/repo"
)

var (
	ErrAppointmentMissing = errors.New("randevu bulunamadı")
	ErrAlreadyHasVisit    = errors.New("bu randevuya bağlı muayene zaten var")
)

type VisitService struct {
	pool   *pgxpool.Pool
	visits *repo.VisitRepo
}

func NewVisitService(pool *pgxpool.Pool, visits *repo.VisitRepo) *VisitService {
	return &VisitService{pool: pool, visits: visits}
}

// StartFromAppointment atomically:
//  1. Re-checks the appointment exists in this branch
//  2. Flips its status to 'in_progress' (stamps started_at)
//  3. Returns the existing visit if one is already linked, otherwise
//     creates a new visit linked to the appointment.
//
// Idempotent: calling it twice on the same appointment returns the same visit.
func (s *VisitService) StartFromAppointment(ctx context.Context, branchID, apptID uuid.UUID, openedBy *uuid.UUID) (*repo.Visit, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock the appointment row + read the fields we need.
	var orgID, patientID uuid.UUID
	var doctorID *uuid.UUID
	row := tx.QueryRow(ctx,
		`SELECT organization_id, patient_id, doctor_id FROM appointment
		 WHERE branch_id = $1 AND id = $2 FOR UPDATE`,
		branchID, apptID)
	if err := row.Scan(&orgID, &patientID, &doctorID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAppointmentMissing
		}
		return nil, err
	}

	// Update appointment status (idempotent — only stamp started_at on first transition).
	if _, err := tx.Exec(ctx,
		`UPDATE appointment
		 SET status = 'in_progress',
		     started_at = COALESCE(started_at, NOW()),
		     arrived_at = COALESCE(arrived_at, NOW())
		 WHERE id = $1`, apptID); err != nil {
		return nil, err
	}

	// Already a visit? Return it.
	existing := &repo.Visit{}
	err = tx.QueryRow(ctx,
		`SELECT id, organization_id, branch_id, patient_id, doctor_id,
		   appointment_id, encounter_type, status, chief_complaint,
		   history_of_present_illness, examination_findings, treatment_plan, notes,
		   opened_by_user_id, started_at, ended_at, created_at, updated_at
		 FROM visit WHERE appointment_id = $1`, apptID).Scan(
		&existing.ID, &existing.OrganizationID, &existing.BranchID, &existing.PatientID,
		&existing.DoctorID, &existing.AppointmentID, &existing.EncounterType, &existing.Status,
		&existing.ChiefComplaint, &existing.HistoryOfPresentIllness, &existing.ExaminationFindings,
		&existing.TreatmentPlan, &existing.Notes, &existing.OpenedByUserID,
		&existing.StartedAt, &existing.EndedAt, &existing.CreatedAt, &existing.UpdatedAt)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// Otherwise create a new visit linked to the appointment.
	v, err := s.visits.CreateInTx(ctx, tx, repo.CreateVisitInput{
		OrganizationID: orgID,
		BranchID:       branchID,
		PatientID:      patientID,
		DoctorID:       doctorID,
		AppointmentID:  &apptID,
		EncounterType:  "outpatient",
		OpenedByUserID: openedBy,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return v, nil
}
