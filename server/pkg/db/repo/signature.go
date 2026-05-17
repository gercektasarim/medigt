package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DigitalSignature struct {
	ID                  uuid.UUID
	OrganizationID      uuid.UUID
	BranchID            *uuid.UUID
	SignerUserID        uuid.UUID
	SignerTC            string
	SignerFullName      string
	TargetTable         string
	TargetID            uuid.UUID
	DocumentKind        string
	DocumentHash        string
	Provider            string
	SessionID           *string
	ChallengeCode       *string
	SignedEnvelope      []byte
	CertificateSerial   *string
	CertificateSubject  *string
	Status              string
	ErrorMessage        *string
	InitiatedAt         time.Time
	SignedAt            *time.Time
	ExpiresAt           time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type SignatureRepo struct{ pool *pgxpool.Pool }

func NewSignatureRepo(pool *pgxpool.Pool) *SignatureRepo { return &SignatureRepo{pool: pool} }

const sigCols = `id, organization_id, branch_id, signer_user_id, signer_tc, signer_full_name,
	target_table, target_id, document_kind, document_hash,
	provider::text, session_id, challenge_code,
	signed_envelope, certificate_serial, certificate_subject,
	status::text, error_message,
	initiated_at, signed_at, expires_at, created_at, updated_at`

func scanSignature(scanner func(...any) error) (*DigitalSignature, error) {
	s := &DigitalSignature{}
	err := scanner(
		&s.ID, &s.OrganizationID, &s.BranchID, &s.SignerUserID, &s.SignerTC, &s.SignerFullName,
		&s.TargetTable, &s.TargetID, &s.DocumentKind, &s.DocumentHash,
		&s.Provider, &s.SessionID, &s.ChallengeCode,
		&s.SignedEnvelope, &s.CertificateSerial, &s.CertificateSubject,
		&s.Status, &s.ErrorMessage,
		&s.InitiatedAt, &s.SignedAt, &s.ExpiresAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

type CreateSignatureInput struct {
	OrganizationID uuid.UUID
	BranchID       *uuid.UUID
	SignerUserID   uuid.UUID
	SignerTC       string
	SignerFullName string
	TargetTable    string
	TargetID       uuid.UUID
	DocumentKind   string
	DocumentHash   string
	Provider       string
	SessionID      *string
	ChallengeCode  *string
}

func (r *SignatureRepo) Create(ctx context.Context, in CreateSignatureInput) (*DigitalSignature, error) {
	if in.Provider == "" {
		in.Provider = "turkkep"
	}
	row := r.pool.QueryRow(ctx,
		`INSERT INTO digital_signature
		   (organization_id, branch_id, signer_user_id, signer_tc, signer_full_name,
		    target_table, target_id, document_kind, document_hash,
		    provider, session_id, challenge_code)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::signature_provider, $11, $12)
		 RETURNING `+sigCols,
		in.OrganizationID, in.BranchID, in.SignerUserID, in.SignerTC, in.SignerFullName,
		in.TargetTable, in.TargetID, in.DocumentKind, in.DocumentHash,
		in.Provider, in.SessionID, in.ChallengeCode)
	return scanSignature(row.Scan)
}

func (r *SignatureRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (*DigitalSignature, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+sigCols+` FROM digital_signature
		 WHERE organization_id = $1 AND id = $2`,
		orgID, id)
	return scanSignature(row.Scan)
}

// ListForSigner returns the active (pending or in_progress) sessions for a
// user; UI uses this to surface "you have N open imza sessions".
func (r *SignatureRepo) ListActive(ctx context.Context, userID uuid.UUID, limit int) ([]DigitalSignature, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+sigCols+` FROM digital_signature
		 WHERE signer_user_id = $1 AND status IN ('pending', 'in_progress') AND expires_at > NOW()
		 ORDER BY initiated_at DESC
		 LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DigitalSignature{}
	for rows.Next() {
		s, err := scanSignature(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func (r *SignatureRepo) UpdateStatusInProgress(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE digital_signature SET status = 'in_progress'
		 WHERE id = $1 AND status = 'pending'`, id)
	return err
}

type CompleteSignatureInput struct {
	ID                 uuid.UUID
	SignedEnvelope     []byte
	CertificateSerial  string
	CertificateSubject string
}

func (r *SignatureRepo) CompleteSigned(ctx context.Context, in CompleteSignatureInput) error {
	res, err := r.pool.Exec(ctx,
		`UPDATE digital_signature SET status = 'signed',
		     signed_envelope = $2, certificate_serial = $3, certificate_subject = $4,
		     signed_at = NOW(), error_message = NULL
		 WHERE id = $1 AND status IN ('pending', 'in_progress')`,
		in.ID, in.SignedEnvelope, in.CertificateSerial, in.CertificateSubject)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SignatureRepo) Cancel(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE digital_signature SET status = 'cancelled', error_message = 'kullanıcı iptal etti'
		 WHERE id = $1 AND status IN ('pending', 'in_progress')`, id)
	return err
}

func (r *SignatureRepo) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE digital_signature SET status = 'failed', error_message = $2
		 WHERE id = $1 AND status IN ('pending', 'in_progress')`, id, errMsg)
	return err
}

func (r *SignatureRepo) MarkExpired(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE digital_signature SET status = 'expired', error_message = 'süre doldu'
		 WHERE id = $1 AND status IN ('pending', 'in_progress')`, id)
	return err
}
