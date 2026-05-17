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

// ---------- Provision ----------

type MedulaProvision struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	PatientID         uuid.UUID
	InstitutionID     *uuid.UUID
	TakipNo           *string
	ProvisionType     string
	BranchCode        *string
	Status            string
	RequestPayload    map[string]any
	ResponsePayload   map[string]any
	ResponseCode      *string
	ErrorMessage      *string
	RequestedByUserID *uuid.UUID
	RequestedAt       time.Time
	CompletedAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type MedulaProvisionWithJoins struct {
	Provision        MedulaProvision
	PatientMRN       string
	PatientFirstName string
	PatientLastName  string
	InstitutionName  *string
}

type MedulaRepo struct{ pool *pgxpool.Pool }

func NewMedulaRepo(pool *pgxpool.Pool) *MedulaRepo { return &MedulaRepo{pool: pool} }

type CreateProvisionInput struct {
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	PatientID         uuid.UUID
	InstitutionID     *uuid.UUID
	ProvisionType     string
	BranchCode        *string
	RequestedByUserID *uuid.UUID
}

// Create writes a pending medula_provision row AND a corresponding
// medula_outgoing_message in one transaction. The worker picks the outbox
// row up and runs the SOAP call.
func (r *MedulaRepo) Create(ctx context.Context, in CreateProvisionInput) (*MedulaProvision, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	provisionType := in.ProvisionType
	if provisionType == "" {
		provisionType = "normal"
	}
	var p MedulaProvision
	if err = tx.QueryRow(ctx,
		`INSERT INTO medula_provision
		   (organization_id, branch_id, patient_id, institution_id,
		    provision_type, branch_code, requested_by_user_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, organization_id, branch_id, patient_id, institution_id,
		           takip_no, provision_type, branch_code, status::text,
		           request_payload, response_payload, response_code, error_message,
		           requested_by_user_id, requested_at, completed_at, created_at, updated_at`,
		in.OrganizationID, in.BranchID, in.PatientID, in.InstitutionID,
		provisionType, in.BranchCode, in.RequestedByUserID,
	).Scan(&p.ID, &p.OrganizationID, &p.BranchID, &p.PatientID, &p.InstitutionID,
		&p.TakipNo, &p.ProvisionType, &p.BranchCode, &p.Status,
		&p.RequestPayload, &p.ResponsePayload, &p.ResponseCode, &p.ErrorMessage,
		&p.RequestedByUserID, &p.RequestedAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}

	if _, err = tx.Exec(ctx,
		`INSERT INTO medula_outgoing_message
		   (organization_id, branch_id, message_type, target_table, target_id, payload)
		 VALUES ($1, $2, 'provision_request', 'medula_provision', $3, $4::JSONB)`,
		in.OrganizationID, in.BranchID, p.ID,
		mustMarshal(map[string]any{
			"provision_type": provisionType,
			"branch_code":    derefString(in.BranchCode),
		}),
	); err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *MedulaRepo) GetByID(ctx context.Context, branchID, id uuid.UUID) (*MedulaProvision, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, branch_id, patient_id, institution_id,
		        takip_no, provision_type, branch_code, status::text,
		        request_payload, response_payload, response_code, error_message,
		        requested_by_user_id, requested_at, completed_at, created_at, updated_at
		 FROM medula_provision WHERE branch_id = $1 AND id = $2`, branchID, id)
	p := &MedulaProvision{}
	err := row.Scan(&p.ID, &p.OrganizationID, &p.BranchID, &p.PatientID, &p.InstitutionID,
		&p.TakipNo, &p.ProvisionType, &p.BranchCode, &p.Status,
		&p.RequestPayload, &p.ResponsePayload, &p.ResponseCode, &p.ErrorMessage,
		&p.RequestedByUserID, &p.RequestedAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func (r *MedulaRepo) List(ctx context.Context, branchID uuid.UUID, status string, limit int) ([]MedulaProvisionWithJoins, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT mp.id, mp.organization_id, mp.branch_id, mp.patient_id, mp.institution_id,
	             mp.takip_no, mp.provision_type, mp.branch_code, mp.status::text,
	             mp.request_payload, mp.response_payload, mp.response_code, mp.error_message,
	             mp.requested_by_user_id, mp.requested_at, mp.completed_at, mp.created_at, mp.updated_at,
	             p.mrn, p.first_name, p.last_name, ins.name
	      FROM medula_provision mp
	      JOIN patient p ON p.id = mp.patient_id
	      LEFT JOIN external_institution ins ON ins.id = mp.institution_id
	      WHERE mp.branch_id = $1`
	args := []any{branchID}
	if status != "" {
		args = append(args, status)
		q += ` AND mp.status = $` + itoa(len(args)) + `::medula_provision_status`
	}
	q += ` ORDER BY mp.requested_at DESC LIMIT ` + itoa(limit)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MedulaProvisionWithJoins{}
	for rows.Next() {
		w := MedulaProvisionWithJoins{}
		p := &w.Provision
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.BranchID, &p.PatientID, &p.InstitutionID,
			&p.TakipNo, &p.ProvisionType, &p.BranchCode, &p.Status,
			&p.RequestPayload, &p.ResponsePayload, &p.ResponseCode, &p.ErrorMessage,
			&p.RequestedByUserID, &p.RequestedAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt,
			&w.PatientMRN, &w.PatientFirstName, &w.PatientLastName, &w.InstitutionName); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// ---------- Outbox ----------

type OutboxMessage struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	BranchID       *uuid.UUID
	MessageType    string
	TargetTable    string
	TargetID       uuid.UUID
	Payload        map[string]any
	Status         string
	RetryCount     int
	NextRetryAt    time.Time
	LastError      *string
	SentAt         *time.Time
	CompletedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ClaimNext finds the next due outbox row in (pending|failed) status, marks
// it 'in_progress' atomically using SELECT FOR UPDATE SKIP LOCKED, and
// returns it for the worker. Returns nil if no work is available.
func (r *MedulaRepo) ClaimNext(ctx context.Context) (*OutboxMessage, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx,
		`SELECT id, organization_id, branch_id, message_type, target_table, target_id,
		        payload, status::text, retry_count, next_retry_at, last_error,
		        sent_at, completed_at, created_at, updated_at
		 FROM medula_outgoing_message
		 WHERE status IN ('pending', 'failed') AND next_retry_at <= NOW()
		 ORDER BY next_retry_at
		 LIMIT 1 FOR UPDATE SKIP LOCKED`)
	m := &OutboxMessage{}
	if err = row.Scan(&m.ID, &m.OrganizationID, &m.BranchID, &m.MessageType, &m.TargetTable, &m.TargetID,
		&m.Payload, &m.Status, &m.RetryCount, &m.NextRetryAt, &m.LastError,
		&m.SentAt, &m.CompletedAt, &m.CreatedAt, &m.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if _, err = tx.Exec(ctx,
		`UPDATE medula_outgoing_message SET status = 'in_progress', sent_at = NOW()
		 WHERE id = $1`, m.ID); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return m, nil
}

// CompleteSuccess marks the outbox row 'sent' and updates the linked
// medula_provision with the takip_no + response payload.
func (r *MedulaRepo) CompleteSuccess(ctx context.Context, msgID, provisionID uuid.UUID, takipNo, responseCode string, responsePayload map[string]any) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx,
		`UPDATE medula_outgoing_message SET status = 'sent', completed_at = NOW(), last_error = NULL
		 WHERE id = $1`, msgID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx,
		`UPDATE medula_provision
		   SET status = 'completed', takip_no = $2, response_code = $3,
		       response_payload = $4::JSONB, completed_at = NOW()
		 WHERE id = $1`,
		provisionID, takipNo, responseCode, mustMarshal(responsePayload)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CompleteFailure marks the message 'failed' with exponential backoff up to
// max retries (5), at which point it's 'dead' and surfaced for ops review.
// Updates the linked provision's status to 'failed' on permanent failure.
func (r *MedulaRepo) CompleteFailure(ctx context.Context, msgID, provisionID uuid.UUID, responseCode, errorMsg string, retryCount int) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const maxRetries = 5
	finalStatus := "failed"
	// Backoff schedule: 30s → 2m → 10m → 1h → 6h
	delays := []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute, 1 * time.Hour, 6 * time.Hour}
	var nextRetry time.Time
	if retryCount+1 >= maxRetries {
		finalStatus = "dead"
	} else {
		delay := delays[retryCount]
		nextRetry = time.Now().Add(delay)
	}
	if _, err = tx.Exec(ctx,
		`UPDATE medula_outgoing_message
		   SET status = $2::medula_outbox_status,
		       retry_count = retry_count + 1,
		       next_retry_at = COALESCE($3, next_retry_at),
		       last_error = $4
		 WHERE id = $1`,
		msgID, finalStatus, nullableTime(nextRetry), errorMsg); err != nil {
		return err
	}
	if finalStatus == "dead" {
		if _, err = tx.Exec(ctx,
			`UPDATE medula_provision SET status = 'failed', response_code = $2,
			     error_message = $3, completed_at = NOW()
			 WHERE id = $1`, provisionID, responseCode, errorMsg); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ---------- Internal helpers ----------

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
