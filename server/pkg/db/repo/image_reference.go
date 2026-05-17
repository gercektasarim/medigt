package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ImageReference struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	RadiologyOrderID  *uuid.UUID
	PatientID         uuid.UUID
	StudyInstanceUID  string
	SeriesInstanceUID *string
	Modality          string
	StudyDate         *time.Time
	Description       *string
	InstanceCount     int
	PACSBaseURL       *string
	ThumbnailURL      *string
	DownloadURL       *string
	SubmittedAt       *time.Time
	LastSyncedAt      *time.Time
	SyncError         *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ImageReferenceRepo struct{ pool *pgxpool.Pool }

func NewImageReferenceRepo(pool *pgxpool.Pool) *ImageReferenceRepo {
	return &ImageReferenceRepo{pool: pool}
}

const imgCols = `id, organization_id, branch_id, radiology_order_id, patient_id,
	study_instance_uid, series_instance_uid, modality, study_date, description,
	instance_count, pacs_base_url, thumbnail_url, download_url,
	submitted_at, last_synced_at, sync_error, created_at, updated_at`

func scanImageRef(scanner func(...any) error) (*ImageReference, error) {
	r := &ImageReference{}
	err := scanner(
		&r.ID, &r.OrganizationID, &r.BranchID, &r.RadiologyOrderID, &r.PatientID,
		&r.StudyInstanceUID, &r.SeriesInstanceUID, &r.Modality, &r.StudyDate, &r.Description,
		&r.InstanceCount, &r.PACSBaseURL, &r.ThumbnailURL, &r.DownloadURL,
		&r.SubmittedAt, &r.LastSyncedAt, &r.SyncError, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return r, err
}

type CreateImageReferenceInput struct {
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	RadiologyOrderID  *uuid.UUID
	PatientID         uuid.UUID
	StudyInstanceUID  string
	SeriesInstanceUID *string
	Modality          string
	StudyDate         *time.Time
	Description       *string
	InstanceCount     int
	PACSBaseURL       *string
	ThumbnailURL      *string
}

// CreateForOrder inserts an image_reference row when an order is first
// scheduled in PACS. Idempotent via UNIQUE (org, study_uid, series_uid):
// duplicates return the existing row.
func (r *ImageReferenceRepo) Create(ctx context.Context, in CreateImageReferenceInput) (*ImageReference, error) {
	now := time.Now()
	row := r.pool.QueryRow(ctx,
		`INSERT INTO image_reference
		   (organization_id, branch_id, radiology_order_id, patient_id,
		    study_instance_uid, series_instance_uid, modality, study_date,
		    description, instance_count, pacs_base_url, thumbnail_url, submitted_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		 ON CONFLICT (organization_id, study_instance_uid, series_instance_uid)
		 DO UPDATE SET
		   instance_count = COALESCE(EXCLUDED.instance_count, image_reference.instance_count),
		   description    = COALESCE(EXCLUDED.description, image_reference.description),
		   last_synced_at = NOW()
		 RETURNING `+imgCols,
		in.OrganizationID, in.BranchID, in.RadiologyOrderID, in.PatientID,
		in.StudyInstanceUID, in.SeriesInstanceUID, in.Modality, in.StudyDate,
		in.Description, in.InstanceCount, in.PACSBaseURL, in.ThumbnailURL, now)
	return scanImageRef(row.Scan)
}

func (r *ImageReferenceRepo) ListForOrder(ctx context.Context, orderID uuid.UUID) ([]ImageReference, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+imgCols+` FROM image_reference
		 WHERE radiology_order_id = $1
		 ORDER BY study_date DESC NULLS LAST, created_at DESC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ImageReference{}
	for rows.Next() {
		i, err := scanImageRef(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *i)
	}
	return out, rows.Err()
}

func (r *ImageReferenceRepo) ListForPatient(ctx context.Context, patientID uuid.UUID, limit int) ([]ImageReference, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+imgCols+` FROM image_reference
		 WHERE patient_id = $1
		 ORDER BY study_date DESC NULLS LAST
		 LIMIT $2`, patientID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ImageReference{}
	for rows.Next() {
		i, err := scanImageRef(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *i)
	}
	return out, rows.Err()
}
