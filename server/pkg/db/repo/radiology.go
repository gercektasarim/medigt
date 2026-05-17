package repo

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------- Procedure catalog ----------

type RadiologyProcedure struct {
	ID               uuid.UUID
	OrganizationID   *uuid.UUID
	Code             string
	Name             string
	Modality         string
	BodyRegion       *string
	SutCode          *string
	EstimatedMinutes *int
	PreparationNotes *string
	IsSystem         bool
	IsActive         bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type RadiologyRepo struct {
	pool *pgxpool.Pool
}

func NewRadiologyRepo(pool *pgxpool.Pool) *RadiologyRepo {
	return &RadiologyRepo{pool: pool}
}

const radProcCols = `id, organization_id, code, name, modality, body_region,
	sut_code, estimated_minutes, preparation_notes, is_system, is_active,
	created_at, updated_at`

func scanRadProc(row pgx.Row) (*RadiologyProcedure, error) {
	p := &RadiologyProcedure{}
	err := row.Scan(&p.ID, &p.OrganizationID, &p.Code, &p.Name, &p.Modality,
		&p.BodyRegion, &p.SutCode, &p.EstimatedMinutes, &p.PreparationNotes,
		&p.IsSystem, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func (r *RadiologyRepo) SearchProcedures(ctx context.Context, orgID uuid.UUID, q, modality string, limit int) ([]RadiologyProcedure, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	args := []any{orgID, limit}
	sqlStr := `SELECT ` + radProcCols + ` FROM radiology_procedure
	           WHERE (organization_id IS NULL OR organization_id = $1) AND is_active = TRUE`
	if modality != "" {
		args = append(args, modality)
		sqlStr += ` AND modality = $` + itoa(len(args)) + `::radiology_modality`
	}
	if qq := strings.TrimSpace(q); qq != "" {
		args = append(args, qq+"%", "%"+qq+"%")
		sqlStr += ` AND (code ILIKE $` + itoa(len(args)-1) + ` OR name ILIKE $` + itoa(len(args)) + `)`
	}
	sqlStr += ` ORDER BY modality, name LIMIT $2`

	rows, err := r.pool.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RadiologyProcedure{}
	for rows.Next() {
		p := RadiologyProcedure{}
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.Code, &p.Name, &p.Modality,
			&p.BodyRegion, &p.SutCode, &p.EstimatedMinutes, &p.PreparationNotes,
			&p.IsSystem, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *RadiologyRepo) GetProcedureByID(ctx context.Context, id uuid.UUID) (*RadiologyProcedure, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+radProcCols+` FROM radiology_procedure WHERE id = $1`, id)
	return scanRadProc(row)
}

// ---------- Orders ----------

type RadiologyOrder struct {
	ID                    uuid.UUID
	OrganizationID        uuid.UUID
	BranchID              uuid.UUID
	VisitID               *uuid.UUID
	PatientID             uuid.UUID
	OrderingDoctorID      *uuid.UUID
	OrderNo               string
	Status                string
	Priority              string
	ProcedureID           uuid.UUID
	ProcedureCode         string
	ProcedureName         string
	Modality              string
	BodyRegion            *string
	ClinicalIndication    *string
	ClinicalQuestion      *string
	Notes                 *string
	ScheduledAt           *time.Time
	AcquiredAt            *time.Time
	AcquiredByUserID      *uuid.UUID
	ReportingDoctorID     *uuid.UUID
	Findings              *string
	Impression            *string
	Recommendations       *string
	ReportedAt            *time.Time
	VerifiedAt            *time.Time
	VerifiedByUserID      *uuid.UUID
	PacsStudyUID          *string
	PacsAccessionNumber   *string
	ThumbnailURL          *string
	OrderedByUserID       *uuid.UUID
	OrderedAt             time.Time
	CancelledAt           *time.Time
	CancellationReason    *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type RadiologyOrderWithJoins struct {
	Order            RadiologyOrder
	PatientMRN       string
	PatientFirstName string
	PatientLastName  string
	DoctorFirstName  *string
	DoctorLastName   *string
	DoctorTitle      *string
}

const radOrderCols = `id, organization_id, branch_id, visit_id, patient_id,
	ordering_doctor_id, order_no, status, priority, procedure_id, procedure_code,
	procedure_name, modality, body_region, clinical_indication, clinical_question,
	notes, scheduled_at, acquired_at, acquired_by_user_id, reporting_doctor_id,
	findings, impression, recommendations, reported_at, verified_at,
	verified_by_user_id, pacs_study_uid, pacs_accession_number, thumbnail_url,
	ordered_by_user_id, ordered_at, cancelled_at, cancellation_reason,
	created_at, updated_at`

func scanRadOrder(scanner func(...any) error) (*RadiologyOrder, error) {
	o := &RadiologyOrder{}
	err := scanner(
		&o.ID, &o.OrganizationID, &o.BranchID, &o.VisitID, &o.PatientID,
		&o.OrderingDoctorID, &o.OrderNo, &o.Status, &o.Priority, &o.ProcedureID,
		&o.ProcedureCode, &o.ProcedureName, &o.Modality, &o.BodyRegion,
		&o.ClinicalIndication, &o.ClinicalQuestion, &o.Notes,
		&o.ScheduledAt, &o.AcquiredAt, &o.AcquiredByUserID, &o.ReportingDoctorID,
		&o.Findings, &o.Impression, &o.Recommendations, &o.ReportedAt,
		&o.VerifiedAt, &o.VerifiedByUserID, &o.PacsStudyUID, &o.PacsAccessionNumber,
		&o.ThumbnailURL, &o.OrderedByUserID, &o.OrderedAt, &o.CancelledAt,
		&o.CancellationReason, &o.CreatedAt, &o.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
}

func (r *RadiologyRepo) NextOrderNo(ctx context.Context) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, `SELECT nextval('radiology_order_no_seq')`).Scan(&n)
	return n, err
}

type CreateRadOrderInput struct {
	OrganizationID     uuid.UUID
	BranchID           uuid.UUID
	VisitID            *uuid.UUID
	PatientID          uuid.UUID
	OrderingDoctorID   *uuid.UUID
	OrderedByUserID    *uuid.UUID
	OrderNo            string
	Priority           string
	ProcedureID        uuid.UUID
	ClinicalIndication *string
	ClinicalQuestion   *string
	Notes              *string
}

// Create snapshots the procedure name/code/modality/body_region onto the order
// row in a single tx so historical orders stay readable even if the catalog
// row is edited later.
func (r *RadiologyRepo) Create(ctx context.Context, in CreateRadOrderInput) (*RadiologyOrder, error) {
	if in.Priority == "" {
		in.Priority = "routine"
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var code, name, modality string
	var bodyRegion *string
	err = tx.QueryRow(ctx,
		`SELECT code, name, modality::text, body_region
		 FROM radiology_procedure
		 WHERE id = $1 AND is_active = TRUE`, in.ProcedureID,
	).Scan(&code, &name, &modality, &bodyRegion)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	row := tx.QueryRow(ctx,
		`INSERT INTO radiology_order (organization_id, branch_id, visit_id,
		   patient_id, ordering_doctor_id, order_no, priority, procedure_id,
		   procedure_code, procedure_name, modality, body_region,
		   clinical_indication, clinical_question, notes, ordered_by_user_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7::radiology_order_priority, $8,
		         $9, $10, $11::radiology_modality, $12, $13, $14, $15, $16)
		 RETURNING `+radOrderCols,
		in.OrganizationID, in.BranchID, in.VisitID, in.PatientID, in.OrderingDoctorID,
		in.OrderNo, in.Priority, in.ProcedureID, code, name, modality, bodyRegion,
		in.ClinicalIndication, in.ClinicalQuestion, in.Notes, in.OrderedByUserID)

	o, err := scanRadOrder(row.Scan)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return o, nil
}

type ListRadOrderFilter struct {
	Status   string
	Modality string
	VisitID  *uuid.UUID
	From     *time.Time
	To       *time.Time
	Limit    int
}

func (r *RadiologyRepo) ListOrders(ctx context.Context, branchID uuid.UUID, f ListRadOrderFilter) ([]RadiologyOrderWithJoins, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT ` + radOrderCols + `,
	             p.mrn, p.first_name, p.last_name,
	             ds.first_name, ds.last_name, ds.title
	      FROM radiology_order
	      JOIN patient p ON p.id = radiology_order.patient_id
	      LEFT JOIN doctor d ON d.id = radiology_order.ordering_doctor_id
	      LEFT JOIN staff_member ds ON ds.id = d.staff_member_id
	      WHERE radiology_order.branch_id = $1`
	args := []any{branchID}
	if f.Status != "" {
		args = append(args, f.Status)
		q += ` AND radiology_order.status = $` + itoa(len(args)) + `::radiology_order_status`
	}
	if f.Modality != "" {
		args = append(args, f.Modality)
		q += ` AND radiology_order.modality = $` + itoa(len(args)) + `::radiology_modality`
	}
	if f.VisitID != nil {
		args = append(args, *f.VisitID)
		q += ` AND radiology_order.visit_id = $` + itoa(len(args))
	}
	if f.From != nil {
		args = append(args, *f.From)
		q += ` AND radiology_order.ordered_at >= $` + itoa(len(args))
	}
	if f.To != nil {
		args = append(args, *f.To)
		q += ` AND radiology_order.ordered_at < $` + itoa(len(args))
	}
	q += ` ORDER BY radiology_order.ordered_at DESC LIMIT ` + itoa(limit)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []RadiologyOrderWithJoins{}
	for rows.Next() {
		w := RadiologyOrderWithJoins{}
		o := &w.Order
		if err := rows.Scan(
			&o.ID, &o.OrganizationID, &o.BranchID, &o.VisitID, &o.PatientID,
			&o.OrderingDoctorID, &o.OrderNo, &o.Status, &o.Priority, &o.ProcedureID,
			&o.ProcedureCode, &o.ProcedureName, &o.Modality, &o.BodyRegion,
			&o.ClinicalIndication, &o.ClinicalQuestion, &o.Notes,
			&o.ScheduledAt, &o.AcquiredAt, &o.AcquiredByUserID, &o.ReportingDoctorID,
			&o.Findings, &o.Impression, &o.Recommendations, &o.ReportedAt,
			&o.VerifiedAt, &o.VerifiedByUserID, &o.PacsStudyUID, &o.PacsAccessionNumber,
			&o.ThumbnailURL, &o.OrderedByUserID, &o.OrderedAt, &o.CancelledAt,
			&o.CancellationReason, &o.CreatedAt, &o.UpdatedAt,
			&w.PatientMRN, &w.PatientFirstName, &w.PatientLastName,
			&w.DoctorFirstName, &w.DoctorLastName, &w.DoctorTitle,
		); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *RadiologyRepo) GetOrderByID(ctx context.Context, branchID, id uuid.UUID) (*RadiologyOrderWithJoins, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+radOrderCols+`,
		        p.mrn, p.first_name, p.last_name,
		        ds.first_name, ds.last_name, ds.title
		 FROM radiology_order
		 JOIN patient p ON p.id = radiology_order.patient_id
		 LEFT JOIN doctor d ON d.id = radiology_order.ordering_doctor_id
		 LEFT JOIN staff_member ds ON ds.id = d.staff_member_id
		 WHERE radiology_order.branch_id = $1 AND radiology_order.id = $2`,
		branchID, id)

	w := RadiologyOrderWithJoins{}
	o := &w.Order
	err := row.Scan(
		&o.ID, &o.OrganizationID, &o.BranchID, &o.VisitID, &o.PatientID,
		&o.OrderingDoctorID, &o.OrderNo, &o.Status, &o.Priority, &o.ProcedureID,
		&o.ProcedureCode, &o.ProcedureName, &o.Modality, &o.BodyRegion,
		&o.ClinicalIndication, &o.ClinicalQuestion, &o.Notes,
		&o.ScheduledAt, &o.AcquiredAt, &o.AcquiredByUserID, &o.ReportingDoctorID,
		&o.Findings, &o.Impression, &o.Recommendations, &o.ReportedAt,
		&o.VerifiedAt, &o.VerifiedByUserID, &o.PacsStudyUID, &o.PacsAccessionNumber,
		&o.ThumbnailURL, &o.OrderedByUserID, &o.OrderedAt, &o.CancelledAt,
		&o.CancellationReason, &o.CreatedAt, &o.UpdatedAt,
		&w.PatientMRN, &w.PatientFirstName, &w.PatientLastName,
		&w.DoctorFirstName, &w.DoctorLastName, &w.DoctorTitle,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &w, err
}

func (r *RadiologyRepo) UpdateOrderStatus(ctx context.Context, branchID, orderID uuid.UUID, status string, byUserID *uuid.UUID) error {
	stamp := ""
	switch status {
	case "scheduled":
		stamp = ", scheduled_at = COALESCE(scheduled_at, NOW())"
	case "acquired":
		stamp = ", acquired_at = NOW(), acquired_by_user_id = $4"
	case "cancelled":
		stamp = ", cancelled_at = NOW()"
	}
	q := `UPDATE radiology_order SET status = $3::radiology_order_status` + stamp +
		` WHERE branch_id = $1 AND id = $2`
	args := []any{branchID, orderID, status}
	if status == "acquired" {
		args = append(args, byUserID)
	}
	res, err := r.pool.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type SaveReportInput struct {
	Findings            *string
	Impression          *string
	Recommendations     *string
	ReportingDoctorID   *uuid.UUID
	PacsStudyUID        *string
	PacsAccessionNumber *string
	ThumbnailURL        *string
}

// SaveReport stores the radiologist's narrative + (optional) PACS metadata
// and bumps the order status to 'reported' if it isn't already verified.
func (r *RadiologyRepo) SaveReport(ctx context.Context, branchID, orderID uuid.UUID, in SaveReportInput) (*RadiologyOrder, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`UPDATE radiology_order SET
		   findings              = COALESCE($3, findings),
		   impression            = COALESCE($4, impression),
		   recommendations       = COALESCE($5, recommendations),
		   reporting_doctor_id   = COALESCE($6, reporting_doctor_id),
		   pacs_study_uid        = COALESCE($7, pacs_study_uid),
		   pacs_accession_number = COALESCE($8, pacs_accession_number),
		   thumbnail_url         = COALESCE($9, thumbnail_url),
		   reported_at           = COALESCE(reported_at, NOW()),
		   status                = CASE WHEN status IN ('ordered','scheduled','in_progress','acquired')
		                                THEN 'reported'::radiology_order_status
		                                ELSE status END
		 WHERE branch_id = $1 AND id = $2`,
		branchID, orderID, in.Findings, in.Impression, in.Recommendations,
		in.ReportingDoctorID, in.PacsStudyUID, in.PacsAccessionNumber, in.ThumbnailURL,
	); err != nil {
		return nil, err
	}

	// Re-read the full row.
	row := tx.QueryRow(ctx,
		`SELECT `+radOrderCols+` FROM radiology_order WHERE branch_id = $1 AND id = $2`,
		branchID, orderID)
	o, err := scanRadOrder(row.Scan)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return o, nil
}

func (r *RadiologyRepo) VerifyReport(ctx context.Context, branchID, orderID uuid.UUID, byUserID *uuid.UUID) error {
	res, err := r.pool.Exec(ctx,
		`UPDATE radiology_order
		 SET status = 'verified', verified_at = NOW(), verified_by_user_id = $3
		 WHERE branch_id = $1 AND id = $2 AND status = 'reported'`,
		branchID, orderID, byUserID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
