package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VitalSigns struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	PatientID        uuid.UUID
	VisitID          *uuid.UUID
	MeasuredAt       time.Time
	MeasuredByUserID *uuid.UUID
	SystolicBP       *int
	DiastolicBP      *int
	Pulse            *int
	TemperatureC     *float64
	Spo2Percent      *int
	Respiration      *int
	WeightKg         *float64
	HeightCm         *float64
	PainScore        *int
	Notes            *string
	CreatedAt        time.Time
}

type VitalSignsRepo struct {
	pool *pgxpool.Pool
}

func NewVitalSignsRepo(pool *pgxpool.Pool) *VitalSignsRepo {
	return &VitalSignsRepo{pool: pool}
}

const vitalCols = `id, organization_id, patient_id, visit_id, measured_at,
	measured_by_user_id, systolic_bp, diastolic_bp, pulse, temperature_c,
	spo2_percent, respiration, weight_kg, height_cm, pain_score, notes, created_at`

type CreateVitalsInput struct {
	OrganizationID   uuid.UUID
	PatientID        uuid.UUID
	VisitID          *uuid.UUID
	MeasuredByUserID *uuid.UUID
	SystolicBP       *int
	DiastolicBP      *int
	Pulse            *int
	TemperatureC     *float64
	Spo2Percent      *int
	Respiration      *int
	WeightKg         *float64
	HeightCm         *float64
	PainScore        *int
	Notes            *string
}

func (r *VitalSignsRepo) Add(ctx context.Context, in CreateVitalsInput) (*VitalSigns, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO vital_signs (organization_id, patient_id, visit_id,
		   measured_by_user_id, systolic_bp, diastolic_bp, pulse,
		   temperature_c, spo2_percent, respiration, weight_kg, height_cm,
		   pain_score, notes)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		 RETURNING `+vitalCols,
		in.OrganizationID, in.PatientID, in.VisitID, in.MeasuredByUserID,
		in.SystolicBP, in.DiastolicBP, in.Pulse, in.TemperatureC,
		in.Spo2Percent, in.Respiration, in.WeightKg, in.HeightCm,
		in.PainScore, in.Notes)
	v := &VitalSigns{}
	err := row.Scan(&v.ID, &v.OrganizationID, &v.PatientID, &v.VisitID, &v.MeasuredAt,
		&v.MeasuredByUserID, &v.SystolicBP, &v.DiastolicBP, &v.Pulse,
		&v.TemperatureC, &v.Spo2Percent, &v.Respiration, &v.WeightKg,
		&v.HeightCm, &v.PainScore, &v.Notes, &v.CreatedAt)
	return v, err
}

func (r *VitalSignsRepo) ListForVisit(ctx context.Context, visitID uuid.UUID) ([]VitalSigns, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+vitalCols+` FROM vital_signs WHERE visit_id = $1
		 ORDER BY measured_at DESC LIMIT 200`, visitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []VitalSigns{}
	for rows.Next() {
		v := VitalSigns{}
		if err := rows.Scan(&v.ID, &v.OrganizationID, &v.PatientID, &v.VisitID, &v.MeasuredAt,
			&v.MeasuredByUserID, &v.SystolicBP, &v.DiastolicBP, &v.Pulse,
			&v.TemperatureC, &v.Spo2Percent, &v.Respiration, &v.WeightKg,
			&v.HeightCm, &v.PainScore, &v.Notes, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *VitalSignsRepo) ListForPatient(ctx context.Context, orgID, patientID uuid.UUID, limit int) ([]VitalSigns, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+vitalCols+` FROM vital_signs
		 WHERE organization_id = $1 AND patient_id = $2
		 ORDER BY measured_at DESC LIMIT $3`, orgID, patientID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []VitalSigns{}
	for rows.Next() {
		v := VitalSigns{}
		if err := rows.Scan(&v.ID, &v.OrganizationID, &v.PatientID, &v.VisitID, &v.MeasuredAt,
			&v.MeasuredByUserID, &v.SystolicBP, &v.DiastolicBP, &v.Pulse,
			&v.TemperatureC, &v.Spo2Percent, &v.Respiration, &v.WeightKg,
			&v.HeightCm, &v.PainScore, &v.Notes, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
