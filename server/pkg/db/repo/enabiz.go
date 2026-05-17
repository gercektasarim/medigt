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

// EnabizMessageRepo manages the e-Nabız outbox.
//
// Producer side: clinical handlers call `Enqueue(...)` whenever a new
// resource needs to land at Bakanlık. Worker side: `ClaimNext()` picks
// up the next due row using SELECT ... FOR UPDATE SKIP LOCKED so we can
// run multiple worker instances safely.
//
// Retry policy mirrors medula_outgoing_message:
//   30s → 2m → 10m → 1h → 6h, give up after 5 attempts.

type EnabizMessage struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	BranchID       uuid.UUID
	PatientID      uuid.UUID
	PatientTC      string
	Kind           string
	ResourceJSON   json.RawMessage
	SourceTable    *string
	SourceID       *uuid.UUID
	Status         string
	RetryCount     int
	NextRetryAt    time.Time
	LastError      *string
	ReceiptID      *string
	LastResponse   json.RawMessage
	QueuedAt       time.Time
	SentAt         *time.Time
}

type EnabizRepo struct{ pool *pgxpool.Pool }

func NewEnabizRepo(pool *pgxpool.Pool) *EnabizRepo { return &EnabizRepo{pool: pool} }

type EnqueueEnabizInput struct {
	OrganizationID uuid.UUID
	BranchID       uuid.UUID
	PatientID      uuid.UUID
	PatientTC      string
	Kind           string // matches enabiz_resource_kind enum
	Resource       any    // anything json.Marshal-able
	SourceTable    string
	SourceID       *uuid.UUID
}

// Enqueue inserts a new pending row. The worker will pick it up at the
// next poll cycle.
func (r *EnabizRepo) Enqueue(ctx context.Context, in EnqueueEnabizInput) (uuid.UUID, error) {
	body, err := json.Marshal(in.Resource)
	if err != nil {
		return uuid.Nil, err
	}
	var sourceTable any = nil
	if in.SourceTable != "" {
		sourceTable = in.SourceTable
	}
	var id uuid.UUID
	err = r.pool.QueryRow(ctx,
		`INSERT INTO enabiz_message
		   (organization_id, branch_id, patient_id, patient_tc,
		    kind, resource_json, source_table, source_id)
		 VALUES ($1, $2, $3, $4, $5::enabiz_resource_kind, $6::JSONB, $7, $8)
		 RETURNING id`,
		in.OrganizationID, in.BranchID, in.PatientID, in.PatientTC,
		in.Kind, body, sourceTable, in.SourceID).Scan(&id)
	return id, err
}

// ClaimNext atomically picks the next due message and stamps it
// in_flight. FOR UPDATE SKIP LOCKED lets multiple workers race safely.
func (r *EnabizRepo) ClaimNext(ctx context.Context) (*EnabizMessage, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var m EnabizMessage
	err = tx.QueryRow(ctx,
		`SELECT id, organization_id, branch_id, patient_id, patient_tc,
		        kind::text, resource_json, source_table, source_id,
		        status::text, retry_count, next_retry_at, last_error,
		        receipt_id, last_response, queued_at, sent_at
		 FROM enabiz_message
		 WHERE status IN ('pending', 'failed') AND next_retry_at <= NOW()
		 ORDER BY next_retry_at
		 FOR UPDATE SKIP LOCKED
		 LIMIT 1`).Scan(
		&m.ID, &m.OrganizationID, &m.BranchID, &m.PatientID, &m.PatientTC,
		&m.Kind, &m.ResourceJSON, &m.SourceTable, &m.SourceID,
		&m.Status, &m.RetryCount, &m.NextRetryAt, &m.LastError,
		&m.ReceiptID, &m.LastResponse, &m.QueuedAt, &m.SentAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE enabiz_message SET status = 'in_flight' WHERE id = $1`, m.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &m, nil
}

// CompleteSuccess stamps the row sent + receipt id.
func (r *EnabizRepo) CompleteSuccess(ctx context.Context, id uuid.UUID, receiptID string, response any) error {
	body, _ := json.Marshal(response)
	_, err := r.pool.Exec(ctx,
		`UPDATE enabiz_message
		   SET status = 'sent', sent_at = NOW(),
		       receipt_id = $2, last_response = $3::JSONB, last_error = NULL
		 WHERE id = $1`,
		id, receiptID, body)
	return err
}

// CompleteFailure increments retry_count, computes the next backoff
// window, and either re-queues (retry < 5) or marks dead.
func (r *EnabizRepo) CompleteFailure(ctx context.Context, id uuid.UUID, errMsg string, currentRetry int) error {
	next := currentRetry + 1
	if next >= 5 {
		_, err := r.pool.Exec(ctx,
			`UPDATE enabiz_message
			   SET status = 'dead', retry_count = $2, last_error = $3
			 WHERE id = $1`, id, next, errMsg)
		return err
	}
	backoff := backoffFor(next)
	_, err := r.pool.Exec(ctx,
		`UPDATE enabiz_message
		   SET status = 'failed', retry_count = $2,
		       next_retry_at = NOW() + $3::INTERVAL,
		       last_error = $4
		 WHERE id = $1`, id, next, backoff, errMsg)
	return err
}

// backoffFor returns the wait-window for the given retry attempt.
// Exponential backoff: 30s → 2m → 10m → 1h → 6h.
func backoffFor(retry int) string {
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

// ListForBranch — paginated tail view for the admin status panel.
type EnabizListFilter struct {
	BranchID uuid.UUID
	Status   string
	Limit    int
}

func (r *EnabizRepo) ListForBranch(ctx context.Context, f EnabizListFilter) ([]EnabizMessage, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	args := []any{f.BranchID}
	where := "WHERE branch_id = $1"
	if f.Status != "" {
		where += " AND status = $2::enabiz_message_status"
		args = append(args, f.Status)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, branch_id, patient_id, patient_tc,
		        kind::text, resource_json, source_table, source_id,
		        status::text, retry_count, next_retry_at, last_error,
		        receipt_id, last_response, queued_at, sent_at
		 FROM enabiz_message `+where+`
		 ORDER BY created_at DESC LIMIT `+intToStr(f.Limit), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EnabizMessage{}
	for rows.Next() {
		var m EnabizMessage
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.BranchID, &m.PatientID, &m.PatientTC,
			&m.Kind, &m.ResourceJSON, &m.SourceTable, &m.SourceID,
			&m.Status, &m.RetryCount, &m.NextRetryAt, &m.LastError,
			&m.ReceiptID, &m.LastResponse, &m.QueuedAt, &m.SentAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+(n%10))) + s
		n /= 10
	}
	return s
}
