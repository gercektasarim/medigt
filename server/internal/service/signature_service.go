package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/medigt/medigt/server/internal/integration/turkkep"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

// SignatureService coordinates the e-imza lifecycle between our DB and the
// configured cloud provider (TURKKEP today; production swap = different
// Client impl).
//
// Lifecycle:
//
//	InitSign  → repo.Create(status=pending) + turkkep.Init → store sessionId
//	PollSign  → turkkep.Poll → on signed: repo.CompleteSigned;
//	            on failed/expired/cancelled: repo.MarkFailed/Expired/Cancel
//	CancelSign→ turkkep.Cancel + repo.Cancel
type SignatureService struct {
	pool   *pgxpool.Pool
	repo   *repo.SignatureRepo
	client turkkep.Client
}

func NewSignatureService(pool *pgxpool.Pool, sigRepo *repo.SignatureRepo, client turkkep.Client) *SignatureService {
	return &SignatureService{pool: pool, repo: sigRepo, client: client}
}

var (
	ErrSignatureExpired     = errors.New("e-imza oturumu süresi doldu")
	ErrSignatureNotPollable = errors.New("e-imza oturumu beklemiyor")
	ErrSignatureFailed      = errors.New("e-imza başarısız")
)

type InitSignInput struct {
	OrganizationID uuid.UUID
	BranchID       *uuid.UUID
	SignerUserID   uuid.UUID
	SignerTC       string
	SignerName     string
	TargetTable    string
	TargetID       uuid.UUID
	DocumentKind   string
	// Either DocumentBytes (server hashes it) OR DocumentHash (caller
	// already computed it). DocumentBytes wins if both provided.
	DocumentBytes []byte
	DocumentHash  string
}

// InitSign computes the doc hash (if needed), opens a TURKKEP session,
// and stores the row with status=pending.
func (s *SignatureService) InitSign(ctx context.Context, in InitSignInput) (*repo.DigitalSignature, error) {
	if in.TargetTable == "" || in.TargetID == uuid.Nil {
		return nil, errors.New("target_table ve target_id zorunlu")
	}
	if in.SignerTC == "" {
		return nil, errors.New("imzalayıcı TC kimlik no zorunlu")
	}
	hash := in.DocumentHash
	if len(in.DocumentBytes) > 0 {
		sum := sha256.Sum256(in.DocumentBytes)
		hash = hex.EncodeToString(sum[:])
	}
	if hash == "" {
		return nil, errors.New("document_bytes veya document_hash gerekli")
	}

	resp, err := s.client.Init(ctx, turkkep.InitInput{
		SignerTC:     in.SignerTC,
		SignerName:   in.SignerName,
		DocumentHash: hash,
		DocumentKind: in.DocumentKind,
	})
	if err != nil {
		return nil, fmt.Errorf("turkkep init: %w", err)
	}

	sessionID := resp.SessionID
	challenge := resp.ChallengeCode
	return s.repo.Create(ctx, repo.CreateSignatureInput{
		OrganizationID: in.OrganizationID,
		BranchID:       in.BranchID,
		SignerUserID:   in.SignerUserID,
		SignerTC:       in.SignerTC,
		SignerFullName: in.SignerName,
		TargetTable:    in.TargetTable,
		TargetID:       in.TargetID,
		DocumentKind:   in.DocumentKind,
		DocumentHash:   hash,
		Provider:       "turkkep",
		SessionID:      &sessionID,
		ChallengeCode:  &challenge,
	})
}

// PollSign queries the provider and persists the resulting state. Idempotent:
// callable repeatedly from the UI; the row's status reflects truth.
func (s *SignatureService) PollSign(ctx context.Context, orgID, id uuid.UUID) (*repo.DigitalSignature, error) {
	cur, err := s.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	// Already terminal — nothing to poll.
	if cur.Status == "signed" || cur.Status == "cancelled" || cur.Status == "failed" || cur.Status == "expired" {
		return cur, nil
	}
	if cur.SessionID == nil || *cur.SessionID == "" {
		return cur, errors.New("session_id boş — provider çağrısı yapılamaz")
	}

	resp, err := s.client.Poll(ctx, *cur.SessionID)
	if err != nil {
		// Transient — leave row in pending/in_progress.
		return cur, fmt.Errorf("turkkep poll: %w", err)
	}
	switch resp.Status {
	case "signed":
		if err := s.repo.CompleteSigned(ctx, repo.CompleteSignatureInput{
			ID:                 cur.ID,
			SignedEnvelope:     resp.SignedEnvelope,
			CertificateSerial:  resp.CertificateSerial,
			CertificateSubject: resp.CertificateSubject,
		}); err != nil {
			return nil, err
		}
	case "in_progress":
		_ = s.repo.UpdateStatusInProgress(ctx, cur.ID)
	case "cancelled":
		_ = s.repo.Cancel(ctx, cur.ID)
	case "failed":
		_ = s.repo.MarkFailed(ctx, cur.ID, resp.ErrorMessage)
	case "expired":
		_ = s.repo.MarkExpired(ctx, cur.ID)
	}
	return s.repo.GetByID(ctx, orgID, id)
}

// CancelSign aborts an open session both at provider and locally.
func (s *SignatureService) CancelSign(ctx context.Context, orgID, id uuid.UUID) error {
	cur, err := s.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return err
	}
	if cur.SessionID != nil && *cur.SessionID != "" {
		// Best-effort; provider error doesn't block local cancel.
		_ = s.client.Cancel(ctx, *cur.SessionID)
	}
	return s.repo.Cancel(ctx, id)
}

// VerifyLinked confirms that the given signature is signed AND matches the
// expected target. Used by PrescriptionRepo.SignWithSignature to gate the
// sign flow when the org requires e-imza.
func (s *SignatureService) VerifyLinked(
	ctx context.Context, orgID, signatureID, expectedSignerID uuid.UUID,
	expectedTargetTable string, expectedTargetID uuid.UUID,
) error {
	cur, err := s.repo.GetByID(ctx, orgID, signatureID)
	if err != nil {
		return err
	}
	if cur.Status != "signed" {
		return fmt.Errorf("imza %s durumunda — yalnızca 'signed' kabul edilir", cur.Status)
	}
	if cur.SignerUserID != expectedSignerID {
		return errors.New("imza farklı bir kullanıcıya ait")
	}
	if cur.TargetTable != expectedTargetTable || cur.TargetID != expectedTargetID {
		return errors.New("imza farklı bir dokümana ait")
	}
	return nil
}
