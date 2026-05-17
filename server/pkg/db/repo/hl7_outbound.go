package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HL7OutboundRepo manages the ADT outbox.
//
// Producer side: AdmissionService calls Enqueue after each Admit /
// Transfer / Discharge commit. Worker side: ClaimNext uses
// FOR UPDATE SKIP LOCKED so multiple worker instances can co-exist.

type HL7OutboundMessage struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	BranchID         uuid.UUID
	MessageControlID string
	EventType        string
	PatientID        uuid.UUID
	AdmissionID      *uuid.UUID
	RawMessage       string
	Status           string
	RetryCount       int
	NextRetryAt      time.Time
	LastError        *string
	AckRaw           *string
	SentAt           *time.Time
	CreatedAt        time.Time
}

type HL7OutboundRepo struct{ pool *pgxpool.Pool }

func NewHL7OutboundRepo(pool *pgxpool.Pool) *HL7OutboundRepo {
	return &HL7OutboundRepo{pool: pool}
}

type EnqueueHL7Input struct {
	OrganizationID   uuid.UUID
	BranchID         uuid.UUID
	MessageControlID string
	EventType        string
	PatientID        uuid.UUID
	AdmissionID      *uuid.UUID
	RawMessage       string
}

func (r *HL7OutboundRepo) Enqueue(ctx context.Context, in EnqueueHL7Input) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx,
		`INSERT INTO hl7_outbound_message
		   (organization_id, branch_id, message_control_id, event_type,
		    patient_id, admission_id, raw_message)
		 VALUES ($1, $2, $3, $4::hl7_adt_event, $5, $6, $7)
		 RETURNING id`,
		in.OrganizationID, in.BranchID, in.MessageControlID, in.EventType,
		in.PatientID, in.AdmissionID, in.RawMessage).Scan(&id)
	return id, err
}

func (r *HL7OutboundRepo) ClaimNext(ctx context.Context) (*HL7OutboundMessage, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var m HL7OutboundMessage
	err = tx.QueryRow(ctx,
		`SELECT id, organization_id, branch_id, message_control_id, event_type::text,
		        patient_id, admission_id, raw_message, status::text,
		        retry_count, next_retry_at, last_error, ack_raw, sent_at, created_at
		 FROM hl7_outbound_message
		 WHERE status IN ('pending', 'failed') AND next_retry_at <= NOW()
		 ORDER BY next_retry_at
		 FOR UPDATE SKIP LOCKED
		 LIMIT 1`).Scan(
		&m.ID, &m.OrganizationID, &m.BranchID, &m.MessageControlID, &m.EventType,
		&m.PatientID, &m.AdmissionID, &m.RawMessage, &m.Status,
		&m.RetryCount, &m.NextRetryAt, &m.LastError, &m.AckRaw, &m.SentAt, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE hl7_outbound_message SET status = 'in_flight' WHERE id = $1`, m.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *HL7OutboundRepo) CompleteSuccess(ctx context.Context, id uuid.UUID, ack string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE hl7_outbound_message
		   SET status = 'sent', sent_at = NOW(), ack_raw = $2, last_error = NULL
		 WHERE id = $1`,
		id, ack)
	return err
}

func (r *HL7OutboundRepo) CompleteFailure(ctx context.Context, id uuid.UUID, errMsg string, currentRetry int) error {
	next := currentRetry + 1
	if next >= 5 {
		_, err := r.pool.Exec(ctx,
			`UPDATE hl7_outbound_message
			   SET status = 'dead', retry_count = $2, last_error = $3
			 WHERE id = $1`, id, next, errMsg)
		return err
	}
	backoff := hl7BackoffFor(next)
	_, err := r.pool.Exec(ctx,
		`UPDATE hl7_outbound_message
		   SET status = 'failed', retry_count = $2,
		       next_retry_at = NOW() + $3::INTERVAL,
		       last_error = $4
		 WHERE id = $1`, id, next, backoff, errMsg)
	return err
}

// hl7BackoffFor — same curve as Medula / e-Nabız: 30s → 6h.
func hl7BackoffFor(retry int) string {
	switch retry {
	case 1:
		return "30 seconds"
	case 2:
		return "2 minutes"
	case 3:
		return "10 minutes"
	case 4:
		return "1 hour"
	default:
		return "6 hours"
	}
}

// ListForAdmission — admission detail sayfası "Gönderilen HL7 mesajları"
// bölümü için.
func (r *HL7OutboundRepo) ListForAdmission(ctx context.Context, admissionID uuid.UUID) ([]HL7OutboundMessage, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, branch_id, message_control_id, event_type::text,
		        patient_id, admission_id, raw_message, status::text,
		        retry_count, next_retry_at, last_error, ack_raw, sent_at, created_at
		 FROM hl7_outbound_message
		 WHERE admission_id = $1
		 ORDER BY created_at DESC`, admissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HL7OutboundMessage{}
	for rows.Next() {
		m := HL7OutboundMessage{}
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.BranchID, &m.MessageControlID, &m.EventType,
			&m.PatientID, &m.AdmissionID, &m.RawMessage, &m.Status,
			&m.RetryCount, &m.NextRetryAt, &m.LastError, &m.AckRaw, &m.SentAt, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
