package repo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// This file extends MedulaRepo with the rest of the SGK service surface
// (invoice submission, referral, e-report, cancellations, sync queries).
// The provision flow stays in medula.go for git history clarity.
//
// Pattern for every async (outbox-driven) op:
//   1. Insert the domain row (status='pending') AND a
//      medula_outgoing_message row in one tx.
//   2. The worker claims the outbox row, dispatches to the Client method,
//      then calls the matching Complete*Success / Complete*Failure here.
//   3. Both update the linked domain row + the outbox row in one tx.

// ============================================================================
//  Outbox helper — message_type → target_table mapping is implicit by repo
//  method choice. The worker, not this layer, knows which Complete* to call.
// ============================================================================

// markOutboxSent updates only the outbox row to 'sent'. Used by every
// Complete*Success method below.
func (r *MedulaRepo) markOutboxSentTx(ctx context.Context, tx pgx.Tx, msgID uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE medula_outgoing_message SET status = 'sent', completed_at = NOW(), last_error = NULL
		 WHERE id = $1`, msgID)
	return err
}

// markOutboxFailedTx applies the exponential backoff + 'dead' terminal
// state. Returns the new outbox status so the caller (typically the
// worker dispatch) can flip the linked domain row to its 'failed'
// equivalent when terminal.
func (r *MedulaRepo) markOutboxFailedTx(ctx context.Context, tx pgx.Tx, msgID uuid.UUID, errorMsg string, retryCount int) (string, error) {
	const maxRetries = 5
	finalStatus := "failed"
	delays := []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute, 1 * time.Hour, 6 * time.Hour}
	var nextRetry time.Time
	if retryCount+1 >= maxRetries {
		finalStatus = "dead"
	} else {
		delay := delays[retryCount]
		nextRetry = time.Now().Add(delay)
	}
	_, err := tx.Exec(ctx,
		`UPDATE medula_outgoing_message
		   SET status = $2::medula_outbox_status,
		       retry_count = retry_count + 1,
		       next_retry_at = COALESCE($3, next_retry_at),
		       last_error = $4
		 WHERE id = $1`,
		msgID, finalStatus, nullableTime(nextRetry), errorMsg)
	return finalStatus, err
}

// ============================================================================
//  Provision cancel + close (extends provision table; queue outbox messages)
// ============================================================================

// QueueProvisionCancel sets the provision's pending cancellation and
// enqueues an outbox 'provision_cancel' message.
func (r *MedulaRepo) QueueProvisionCancel(ctx context.Context, branchID, provisionID uuid.UUID, reason string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var orgID uuid.UUID
	var takipNo *string
	if err = tx.QueryRow(ctx,
		`SELECT organization_id, takip_no FROM medula_provision
		 WHERE branch_id = $1 AND id = $2 FOR UPDATE`,
		branchID, provisionID).Scan(&orgID, &takipNo); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if takipNo == nil || *takipNo == "" {
		return errors.New("takip numarası yoksa iptal gönderilemez")
	}
	if _, err = tx.Exec(ctx,
		`INSERT INTO medula_outgoing_message
		   (organization_id, branch_id, message_type, target_table, target_id, payload)
		 VALUES ($1, $2, 'provision_cancel', 'medula_provision', $3, $4::JSONB)`,
		orgID, branchID, provisionID, mustMarshal(map[string]any{"takip_no": *takipNo, "reason": reason})); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// QueueTakipClose enqueues a takip closure for a provision (after services rendered).
func (r *MedulaRepo) QueueTakipClose(ctx context.Context, branchID, provisionID uuid.UUID) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var orgID uuid.UUID
	var takipNo *string
	if err = tx.QueryRow(ctx,
		`SELECT organization_id, takip_no FROM medula_provision
		 WHERE branch_id = $1 AND id = $2 FOR UPDATE`,
		branchID, provisionID).Scan(&orgID, &takipNo); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if takipNo == nil || *takipNo == "" {
		return errors.New("kapatılacak takip numarası yok")
	}
	if _, err = tx.Exec(ctx,
		`INSERT INTO medula_outgoing_message
		   (organization_id, branch_id, message_type, target_table, target_id, payload)
		 VALUES ($1, $2, 'takip_close', 'medula_provision', $3, $4::JSONB)`,
		orgID, branchID, provisionID, mustMarshal(map[string]any{"takip_no": *takipNo})); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CompleteProvisionCancellation marks the message sent AND the provision
// row as cancelled (with the SGK response payload captured).
func (r *MedulaRepo) CompleteProvisionCancellation(ctx context.Context, msgID, provisionID uuid.UUID, responseCode string, payload map[string]any) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = r.markOutboxSentTx(ctx, tx, msgID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx,
		`UPDATE medula_provision
		   SET status = 'cancelled', cancelled_at = NOW(),
		       response_code = $2, response_payload = $3::JSONB
		 WHERE id = $1`,
		provisionID, responseCode, mustMarshal(payload)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CompleteTakipClosure marks the message sent AND stamps closed_at on
// the provision.
func (r *MedulaRepo) CompleteTakipClosure(ctx context.Context, msgID, provisionID uuid.UUID, responseCode string, payload map[string]any) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = r.markOutboxSentTx(ctx, tx, msgID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx,
		`UPDATE medula_provision
		   SET closed_at = NOW(), response_code = $2,
		       response_payload = $3::JSONB
		 WHERE id = $1`,
		provisionID, responseCode, mustMarshal(payload)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ============================================================================
//  Invoice submission
// ============================================================================

type InvoiceSubmission struct {
	ID                  uuid.UUID
	OrganizationID      uuid.UUID
	BranchID            uuid.UUID
	InvoiceID           uuid.UUID
	ProvisionID         *uuid.UUID
	BatchNo             *string
	SGKInvoiceNo        *string
	Status              string
	ResponseCode        *string
	ErrorMessage        *string
	CancelledAt         *time.Time
	CancellationReason  *string
	RequestPayload      map[string]any
	ResponsePayload     map[string]any
	RequestedByUserID   *uuid.UUID
	RequestedAt         time.Time
	CompletedAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type InvoiceSubmissionWithJoins struct {
	Submission InvoiceSubmission
	InvoiceNo  string
	Total      float64
	PatientFirstName string
	PatientLastName  string
	PatientMRN       string
}

type CreateInvoiceSubmissionInput struct {
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	InvoiceID         uuid.UUID
	ProvisionID       *uuid.UUID
	RequestedByUserID *uuid.UUID
}

// QueueInvoiceSubmission creates the submission row + enqueues the outbox
// message in one tx. Refuses if a submission for this invoice already
// exists (UNIQUE constraint prevents duplicates).
func (r *MedulaRepo) QueueInvoiceSubmission(ctx context.Context, in CreateInvoiceSubmissionInput) (*InvoiceSubmission, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Load invoice header to snapshot total + line count into the payload.
	var total float64
	var lineCount int
	var takipNo *string
	if err = tx.QueryRow(ctx,
		`SELECT i.total, (SELECT COUNT(*) FROM invoice_item WHERE invoice_id = i.id), mp.takip_no
		 FROM invoice i
		 LEFT JOIN medula_provision mp ON mp.id = $2
		 WHERE i.id = $1`, in.InvoiceID, in.ProvisionID).Scan(&total, &lineCount, &takipNo); err != nil {
		return nil, err
	}

	s := &InvoiceSubmission{}
	if err = tx.QueryRow(ctx,
		`INSERT INTO medula_invoice_submission
		   (organization_id, branch_id, invoice_id, provision_id,
		    request_payload, requested_by_user_id)
		 VALUES ($1, $2, $3, $4, $5::JSONB, $6)
		 RETURNING id, organization_id, branch_id, invoice_id, provision_id,
		           batch_no, sgk_invoice_no, status::text,
		           response_code, error_message, cancelled_at, cancellation_reason,
		           request_payload, response_payload,
		           requested_by_user_id, requested_at, completed_at, created_at, updated_at`,
		in.OrganizationID, in.BranchID, in.InvoiceID, in.ProvisionID,
		mustMarshal(map[string]any{"total": total, "line_count": lineCount, "takip_no": derefString(takipNo)}),
		in.RequestedByUserID,
	).Scan(&s.ID, &s.OrganizationID, &s.BranchID, &s.InvoiceID, &s.ProvisionID,
		&s.BatchNo, &s.SGKInvoiceNo, &s.Status,
		&s.ResponseCode, &s.ErrorMessage, &s.CancelledAt, &s.CancellationReason,
		&s.RequestPayload, &s.ResponsePayload,
		&s.RequestedByUserID, &s.RequestedAt, &s.CompletedAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, err
	}

	if _, err = tx.Exec(ctx,
		`INSERT INTO medula_outgoing_message
		   (organization_id, branch_id, message_type, target_table, target_id, payload)
		 VALUES ($1, $2, 'invoice_submit', 'medula_invoice_submission', $3, $4::JSONB)`,
		in.OrganizationID, in.BranchID, s.ID,
		mustMarshal(map[string]any{"invoice_id": in.InvoiceID.String(), "total": total, "line_count": lineCount, "takip_no": derefString(takipNo)})); err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// QueueInvoiceCancel — caller has the submission row. We refuse if no
// SGK invoice number yet (means SGK hasn't accepted, nothing to cancel).
func (r *MedulaRepo) QueueInvoiceCancel(ctx context.Context, branchID, submissionID uuid.UUID, reason string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var orgID uuid.UUID
	var sgkInvoiceNo *string
	if err = tx.QueryRow(ctx,
		`SELECT organization_id, sgk_invoice_no FROM medula_invoice_submission
		 WHERE branch_id = $1 AND id = $2 FOR UPDATE`,
		branchID, submissionID).Scan(&orgID, &sgkInvoiceNo); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if sgkInvoiceNo == nil || *sgkInvoiceNo == "" {
		return errors.New("SGK fatura numarası yoksa iptal gönderilemez")
	}
	if _, err = tx.Exec(ctx,
		`INSERT INTO medula_outgoing_message
		   (organization_id, branch_id, message_type, target_table, target_id, payload)
		 VALUES ($1, $2, 'invoice_cancel', 'medula_invoice_submission', $3, $4::JSONB)`,
		orgID, branchID, submissionID,
		mustMarshal(map[string]any{"sgk_invoice_no": *sgkInvoiceNo, "reason": reason})); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *MedulaRepo) ListInvoiceSubmissions(ctx context.Context, branchID uuid.UUID, status string, limit int) ([]InvoiceSubmissionWithJoins, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT s.id, s.organization_id, s.branch_id, s.invoice_id, s.provision_id,
	             s.batch_no, s.sgk_invoice_no, s.status::text,
	             s.response_code, s.error_message, s.cancelled_at, s.cancellation_reason,
	             s.request_payload, s.response_payload,
	             s.requested_by_user_id, s.requested_at, s.completed_at, s.created_at, s.updated_at,
	             i.invoice_no, i.total, p.first_name, p.last_name, p.mrn
	      FROM medula_invoice_submission s
	      JOIN invoice i ON i.id = s.invoice_id
	      JOIN patient p ON p.id = i.patient_id
	      WHERE s.branch_id = $1`
	args := []any{branchID}
	if status != "" {
		args = append(args, status)
		q += ` AND s.status = $` + itoa(len(args)) + `::medula_submit_status`
	}
	q += ` ORDER BY s.requested_at DESC LIMIT ` + itoa(limit)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InvoiceSubmissionWithJoins{}
	for rows.Next() {
		w := InvoiceSubmissionWithJoins{}
		s := &w.Submission
		if err := rows.Scan(
			&s.ID, &s.OrganizationID, &s.BranchID, &s.InvoiceID, &s.ProvisionID,
			&s.BatchNo, &s.SGKInvoiceNo, &s.Status,
			&s.ResponseCode, &s.ErrorMessage, &s.CancelledAt, &s.CancellationReason,
			&s.RequestPayload, &s.ResponsePayload,
			&s.RequestedByUserID, &s.RequestedAt, &s.CompletedAt, &s.CreatedAt, &s.UpdatedAt,
			&w.InvoiceNo, &w.Total, &w.PatientFirstName, &w.PatientLastName, &w.PatientMRN,
		); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *MedulaRepo) CompleteInvoiceSubmissionSuccess(ctx context.Context, msgID, submissionID uuid.UUID, batchNo, sgkInvoiceNo, responseCode string, payload map[string]any) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = r.markOutboxSentTx(ctx, tx, msgID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx,
		`UPDATE medula_invoice_submission
		   SET status = 'submitted', batch_no = $2, sgk_invoice_no = $3,
		       response_code = $4, response_payload = $5::JSONB, completed_at = NOW()
		 WHERE id = $1`,
		submissionID, batchNo, sgkInvoiceNo, responseCode, mustMarshal(payload)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *MedulaRepo) CompleteInvoiceSubmissionFailure(ctx context.Context, msgID, submissionID uuid.UUID, responseCode, errorMsg string, retryCount int) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	finalStatus, err := r.markOutboxFailedTx(ctx, tx, msgID, errorMsg, retryCount)
	if err != nil {
		return err
	}
	if finalStatus == "dead" {
		if _, err = tx.Exec(ctx,
			`UPDATE medula_invoice_submission
			   SET status = 'failed', response_code = $2, error_message = $3, completed_at = NOW()
			 WHERE id = $1`, submissionID, responseCode, errorMsg); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *MedulaRepo) CompleteInvoiceSubmissionCancellation(ctx context.Context, msgID, submissionID uuid.UUID, responseCode string, payload map[string]any) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = r.markOutboxSentTx(ctx, tx, msgID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx,
		`UPDATE medula_invoice_submission
		   SET status = 'cancelled', cancelled_at = NOW(),
		       response_code = $2, response_payload = $3::JSONB
		 WHERE id = $1`,
		submissionID, responseCode, mustMarshal(payload)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ============================================================================
//  Referral (sevk)
// ============================================================================

type Referral struct {
	ID                 uuid.UUID
	OrganizationID     uuid.UUID
	BranchID           uuid.UUID
	PatientID          uuid.UUID
	ReferringDoctorID  *uuid.UUID
	TargetProviderCode string
	TargetProviderName *string
	TargetBranchCode   *string
	Reason             string
	DiagnosisICD10     *string
	ReferralType       string
	Status             string
	SevkNo             *string
	RequestPayload     map[string]any
	ResponsePayload    map[string]any
	ResponseCode       *string
	ErrorMessage       *string
	RequestedByUserID  *uuid.UUID
	RequestedAt        time.Time
	CompletedAt        *time.Time
	CancelledAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type ReferralWithJoins struct {
	Referral         Referral
	PatientMRN       string
	PatientFirstName string
	PatientLastName  string
}

type CreateReferralInput struct {
	OrganizationID     uuid.UUID
	BranchID           uuid.UUID
	PatientID          uuid.UUID
	ReferringDoctorID  *uuid.UUID
	TargetProviderCode string
	TargetProviderName *string
	TargetBranchCode   *string
	Reason             string
	DiagnosisICD10     *string
	ReferralType       string
	RequestedByUserID  *uuid.UUID
}

func (r *MedulaRepo) QueueReferral(ctx context.Context, in CreateReferralInput) (*Referral, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if in.ReferralType == "" {
		in.ReferralType = "normal"
	}

	ref := &Referral{}
	if err = tx.QueryRow(ctx,
		`INSERT INTO medula_referral
		   (organization_id, branch_id, patient_id, referring_doctor_id,
		    target_provider_code, target_provider_name, target_branch_code,
		    reason, diagnosis_icd10, referral_type, requested_by_user_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, organization_id, branch_id, patient_id, referring_doctor_id,
		           target_provider_code, target_provider_name, target_branch_code,
		           reason, diagnosis_icd10, referral_type, status::text, sevk_no,
		           request_payload, response_payload, response_code, error_message,
		           requested_by_user_id, requested_at, completed_at, cancelled_at,
		           created_at, updated_at`,
		in.OrganizationID, in.BranchID, in.PatientID, in.ReferringDoctorID,
		in.TargetProviderCode, in.TargetProviderName, in.TargetBranchCode,
		in.Reason, in.DiagnosisICD10, in.ReferralType, in.RequestedByUserID,
	).Scan(&ref.ID, &ref.OrganizationID, &ref.BranchID, &ref.PatientID, &ref.ReferringDoctorID,
		&ref.TargetProviderCode, &ref.TargetProviderName, &ref.TargetBranchCode,
		&ref.Reason, &ref.DiagnosisICD10, &ref.ReferralType, &ref.Status, &ref.SevkNo,
		&ref.RequestPayload, &ref.ResponsePayload, &ref.ResponseCode, &ref.ErrorMessage,
		&ref.RequestedByUserID, &ref.RequestedAt, &ref.CompletedAt, &ref.CancelledAt,
		&ref.CreatedAt, &ref.UpdatedAt); err != nil {
		return nil, err
	}

	if _, err = tx.Exec(ctx,
		`INSERT INTO medula_outgoing_message
		   (organization_id, branch_id, message_type, target_table, target_id, payload)
		 VALUES ($1, $2, 'referral_create', 'medula_referral', $3, $4::JSONB)`,
		in.OrganizationID, in.BranchID, ref.ID,
		mustMarshal(map[string]any{
			"target_provider_code": in.TargetProviderCode,
			"target_branch_code":   derefString(in.TargetBranchCode),
			"type":                 in.ReferralType,
		})); err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ref, nil
}

func (r *MedulaRepo) ListReferrals(ctx context.Context, branchID uuid.UUID, status string, limit int) ([]ReferralWithJoins, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT r.id, r.organization_id, r.branch_id, r.patient_id, r.referring_doctor_id,
	             r.target_provider_code, r.target_provider_name, r.target_branch_code,
	             r.reason, r.diagnosis_icd10, r.referral_type, r.status::text, r.sevk_no,
	             r.request_payload, r.response_payload, r.response_code, r.error_message,
	             r.requested_by_user_id, r.requested_at, r.completed_at, r.cancelled_at,
	             r.created_at, r.updated_at,
	             p.mrn, p.first_name, p.last_name
	      FROM medula_referral r
	      JOIN patient p ON p.id = r.patient_id
	      WHERE r.branch_id = $1`
	args := []any{branchID}
	if status != "" {
		args = append(args, status)
		q += ` AND r.status = $` + itoa(len(args)) + `::medula_referral_status`
	}
	q += ` ORDER BY r.requested_at DESC LIMIT ` + itoa(limit)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReferralWithJoins{}
	for rows.Next() {
		w := ReferralWithJoins{}
		ref := &w.Referral
		if err := rows.Scan(
			&ref.ID, &ref.OrganizationID, &ref.BranchID, &ref.PatientID, &ref.ReferringDoctorID,
			&ref.TargetProviderCode, &ref.TargetProviderName, &ref.TargetBranchCode,
			&ref.Reason, &ref.DiagnosisICD10, &ref.ReferralType, &ref.Status, &ref.SevkNo,
			&ref.RequestPayload, &ref.ResponsePayload, &ref.ResponseCode, &ref.ErrorMessage,
			&ref.RequestedByUserID, &ref.RequestedAt, &ref.CompletedAt, &ref.CancelledAt,
			&ref.CreatedAt, &ref.UpdatedAt,
			&w.PatientMRN, &w.PatientFirstName, &w.PatientLastName,
		); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *MedulaRepo) CompleteReferralSuccess(ctx context.Context, msgID, referralID uuid.UUID, sevkNo, responseCode string, payload map[string]any) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = r.markOutboxSentTx(ctx, tx, msgID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx,
		`UPDATE medula_referral
		   SET status = 'created', sevk_no = $2, response_code = $3,
		       response_payload = $4::JSONB, completed_at = NOW()
		 WHERE id = $1`,
		referralID, sevkNo, responseCode, mustMarshal(payload)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *MedulaRepo) CompleteReferralFailure(ctx context.Context, msgID, referralID uuid.UUID, responseCode, errorMsg string, retryCount int) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	finalStatus, err := r.markOutboxFailedTx(ctx, tx, msgID, errorMsg, retryCount)
	if err != nil {
		return err
	}
	if finalStatus == "dead" {
		if _, err = tx.Exec(ctx,
			`UPDATE medula_referral
			   SET status = 'failed', response_code = $2, error_message = $3, completed_at = NOW()
			 WHERE id = $1`, referralID, responseCode, errorMsg); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ============================================================================
//  e-Rapor
// ============================================================================

type Eraport struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	PatientID         uuid.UUID
	DoctorID          *uuid.UUID
	Kind              string
	DiagnosesICD10    []string
	DrugCodes         []string
	ValidFrom         time.Time
	ValidTo           *time.Time
	ReportText        *string
	Status            string
	EraportNo         *string
	RequestPayload    map[string]any
	ResponsePayload   map[string]any
	ResponseCode      *string
	ErrorMessage      *string
	RequestedByUserID *uuid.UUID
	RequestedAt       time.Time
	CompletedAt       *time.Time
	CancelledAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type EraportWithJoins struct {
	Eraport          Eraport
	PatientMRN       string
	PatientFirstName string
	PatientLastName  string
}

type CreateEraportInput struct {
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	PatientID         uuid.UUID
	DoctorID          *uuid.UUID
	Kind              string
	DiagnosesICD10    []string
	DrugCodes         []string
	ValidFrom         time.Time
	ValidTo           *time.Time
	ReportText        *string
	RequestedByUserID *uuid.UUID
}

func (r *MedulaRepo) QueueEraport(ctx context.Context, in CreateEraportInput) (*Eraport, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if in.Kind == "" {
		in.Kind = "chronic_drug"
	}
	diagnosesJSON, _ := json.Marshal(in.DiagnosesICD10)
	drugsJSON, _ := json.Marshal(in.DrugCodes)

	ep := &Eraport{}
	if err = tx.QueryRow(ctx,
		`INSERT INTO medula_eraport
		   (organization_id, branch_id, patient_id, doctor_id, kind,
		    diagnoses_icd10, drug_codes, valid_from, valid_to, report_text,
		    requested_by_user_id)
		 VALUES ($1, $2, $3, $4, $5::medula_eraport_kind,
		         $6::JSONB, $7::JSONB, $8, $9, $10, $11)
		 RETURNING id, organization_id, branch_id, patient_id, doctor_id, kind::text,
		           diagnoses_icd10, drug_codes, valid_from, valid_to, report_text,
		           status::text, eraport_no,
		           request_payload, response_payload, response_code, error_message,
		           requested_by_user_id, requested_at, completed_at, cancelled_at,
		           created_at, updated_at`,
		in.OrganizationID, in.BranchID, in.PatientID, in.DoctorID, in.Kind,
		diagnosesJSON, drugsJSON, in.ValidFrom, in.ValidTo, in.ReportText,
		in.RequestedByUserID,
	).Scan(&ep.ID, &ep.OrganizationID, &ep.BranchID, &ep.PatientID, &ep.DoctorID, &ep.Kind,
		&ep.DiagnosesICD10, &ep.DrugCodes, &ep.ValidFrom, &ep.ValidTo, &ep.ReportText,
		&ep.Status, &ep.EraportNo,
		&ep.RequestPayload, &ep.ResponsePayload, &ep.ResponseCode, &ep.ErrorMessage,
		&ep.RequestedByUserID, &ep.RequestedAt, &ep.CompletedAt, &ep.CancelledAt,
		&ep.CreatedAt, &ep.UpdatedAt); err != nil {
		return nil, err
	}

	if _, err = tx.Exec(ctx,
		`INSERT INTO medula_outgoing_message
		   (organization_id, branch_id, message_type, target_table, target_id, payload)
		 VALUES ($1, $2, 'eraport_submit', 'medula_eraport', $3, $4::JSONB)`,
		in.OrganizationID, in.BranchID, ep.ID,
		mustMarshal(map[string]any{"kind": in.Kind, "valid_from": in.ValidFrom.Format("2006-01-02")})); err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ep, nil
}

func (r *MedulaRepo) QueueEraportCancel(ctx context.Context, branchID, eraportID uuid.UUID, reason string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var orgID uuid.UUID
	var eraportNo *string
	if err = tx.QueryRow(ctx,
		`SELECT organization_id, eraport_no FROM medula_eraport
		 WHERE branch_id = $1 AND id = $2 FOR UPDATE`,
		branchID, eraportID).Scan(&orgID, &eraportNo); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if eraportNo == nil || *eraportNo == "" {
		return errors.New("rapor numarası yoksa iptal gönderilemez")
	}
	if _, err = tx.Exec(ctx,
		`INSERT INTO medula_outgoing_message
		   (organization_id, branch_id, message_type, target_table, target_id, payload)
		 VALUES ($1, $2, 'eraport_cancel', 'medula_eraport', $3, $4::JSONB)`,
		orgID, branchID, eraportID,
		mustMarshal(map[string]any{"eraport_no": *eraportNo, "reason": reason})); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *MedulaRepo) ListEraports(ctx context.Context, branchID uuid.UUID, status string, limit int) ([]EraportWithJoins, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT e.id, e.organization_id, e.branch_id, e.patient_id, e.doctor_id, e.kind::text,
	             e.diagnoses_icd10, e.drug_codes, e.valid_from, e.valid_to, e.report_text,
	             e.status::text, e.eraport_no,
	             e.request_payload, e.response_payload, e.response_code, e.error_message,
	             e.requested_by_user_id, e.requested_at, e.completed_at, e.cancelled_at,
	             e.created_at, e.updated_at,
	             p.mrn, p.first_name, p.last_name
	      FROM medula_eraport e
	      JOIN patient p ON p.id = e.patient_id
	      WHERE e.branch_id = $1`
	args := []any{branchID}
	if status != "" {
		args = append(args, status)
		q += ` AND e.status = $` + itoa(len(args)) + `::medula_eraport_status`
	}
	q += ` ORDER BY e.requested_at DESC LIMIT ` + itoa(limit)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EraportWithJoins{}
	for rows.Next() {
		w := EraportWithJoins{}
		ep := &w.Eraport
		if err := rows.Scan(
			&ep.ID, &ep.OrganizationID, &ep.BranchID, &ep.PatientID, &ep.DoctorID, &ep.Kind,
			&ep.DiagnosesICD10, &ep.DrugCodes, &ep.ValidFrom, &ep.ValidTo, &ep.ReportText,
			&ep.Status, &ep.EraportNo,
			&ep.RequestPayload, &ep.ResponsePayload, &ep.ResponseCode, &ep.ErrorMessage,
			&ep.RequestedByUserID, &ep.RequestedAt, &ep.CompletedAt, &ep.CancelledAt,
			&ep.CreatedAt, &ep.UpdatedAt,
			&w.PatientMRN, &w.PatientFirstName, &w.PatientLastName,
		); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *MedulaRepo) CompleteEraportSuccess(ctx context.Context, msgID, eraportID uuid.UUID, eraportNo, responseCode string, payload map[string]any) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = r.markOutboxSentTx(ctx, tx, msgID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx,
		`UPDATE medula_eraport
		   SET status = 'approved', eraport_no = $2, response_code = $3,
		       response_payload = $4::JSONB, completed_at = NOW()
		 WHERE id = $1`,
		eraportID, eraportNo, responseCode, mustMarshal(payload)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *MedulaRepo) CompleteEraportFailure(ctx context.Context, msgID, eraportID uuid.UUID, responseCode, errorMsg string, retryCount int) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	finalStatus, err := r.markOutboxFailedTx(ctx, tx, msgID, errorMsg, retryCount)
	if err != nil {
		return err
	}
	if finalStatus == "dead" {
		if _, err = tx.Exec(ctx,
			`UPDATE medula_eraport
			   SET status = 'failed', response_code = $2, error_message = $3, completed_at = NOW()
			 WHERE id = $1`, eraportID, responseCode, errorMsg); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *MedulaRepo) CompleteEraportCancellation(ctx context.Context, msgID, eraportID uuid.UUID, responseCode string, payload map[string]any) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = r.markOutboxSentTx(ctx, tx, msgID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx,
		`UPDATE medula_eraport
		   SET status = 'cancelled', cancelled_at = NOW(),
		       response_code = $2, response_payload = $3::JSONB
		 WHERE id = $1`,
		eraportID, responseCode, mustMarshal(payload)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
