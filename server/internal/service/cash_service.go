package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/medigt/medigt/server/internal/util"
)

// CashService handles open / close / movement operations that must be
// transactional and uniqueness-safe across concurrent cashiers.
type CashService struct {
	pool *pgxpool.Pool
}

func NewCashService(pool *pgxpool.Pool) *CashService { return &CashService{pool: pool} }

var (
	ErrRegisterAlreadyOpen = errors.New("bu kasiyerin zaten açık bir kasası var")
	ErrRegisterNotOpen     = errors.New("kasa kapalı; önce açın")
	ErrRegisterClosed      = errors.New("kasa kapanmış; üzerine işlem yapılamaz")
)

type OpenRegisterInput struct {
	OrganizationID  uuid.UUID
	BranchID        uuid.UUID
	CashierUserID   uuid.UUID
	CashierName     string
	OpeningBalance  float64
	Notes           *string
}

// Open creates a new register session and, if opening_balance > 0, appends
// an 'opening' movement for the audit trail.
func (s *CashService) Open(ctx context.Context, in OpenRegisterInput) (uuid.UUID, string, error) {
	if in.OpeningBalance < 0 {
		return uuid.Nil, "", fmt.Errorf("açılış bakiyesi negatif olamaz")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var nextNo int64
	if err = tx.QueryRow(ctx, `SELECT nextval('cash_register_no_seq')`).Scan(&nextNo); err != nil {
		return uuid.Nil, "", err
	}
	registerNo := "K-" + util.FormatMRN(nextNo)

	var regID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO cash_register
		   (organization_id, branch_id, register_no, cashier_user_id, cashier_name,
		    opening_balance, notes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		in.OrganizationID, in.BranchID, registerNo, in.CashierUserID, in.CashierName,
		in.OpeningBalance, in.Notes).Scan(&regID)
	if err != nil {
		if isUniqueViolationLocal(err) {
			return uuid.Nil, "", ErrRegisterAlreadyOpen
		}
		return uuid.Nil, "", err
	}

	if in.OpeningBalance > 0 {
		if err = s.insertMovementTx(ctx, tx, regID, in.OrganizationID, in.BranchID,
			"opening", "cash", in.OpeningBalance, nil, nil, nil, strPtr("Açılış bakiyesi"),
			&in.CashierUserID); err != nil {
			return uuid.Nil, "", err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, "", err
	}
	return regID, registerNo, nil
}

type CloseRegisterInput struct {
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	RegisterID        uuid.UUID
	DeclaredBalance   float64
	Notes             *string
	PerformedByUserID *uuid.UUID
}

// Close finalises the session: stamps closed_at + declared_balance + status,
// appends a 'closing' movement (audit). Expected balance + variance are
// derived by the Z-report query — we don't denormalise here.
func (s *CashService) Close(ctx context.Context, in CloseRegisterInput) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	res, err := tx.Exec(ctx,
		`UPDATE cash_register SET status = 'closed',
		     declared_balance = $3, closed_at = NOW(),
		     notes = COALESCE($4, notes)
		 WHERE id = $1 AND branch_id = $2 AND status = 'open'`,
		in.RegisterID, in.BranchID, in.DeclaredBalance, in.Notes)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrRegisterNotOpen
	}

	if err = s.insertMovementTx(ctx, tx, in.RegisterID, in.OrganizationID, in.BranchID,
		"closing", "cash", in.DeclaredBalance, nil, nil, nil, strPtr("Kapanış sayımı"),
		in.PerformedByUserID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// MovementInput records an ad-hoc cash movement (income/expense/refund).
// invoice/payment integration arrives in slice 2; today this lets the
// cashier log walk-in tahsilat or gider entries.
type MovementInput struct {
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	RegisterID        uuid.UUID
	Kind              string // income | expense | refund | transfer_in | transfer_out
	Method            string // cash | card | transfer | mobile | other
	Amount            float64
	ReferenceType     *string
	ReferenceID       *uuid.UUID
	Counterparty      *string
	Description       *string
	PerformedByUserID *uuid.UUID
}

func (s *CashService) RecordMovement(ctx context.Context, in MovementInput) (string, error) {
	if in.Amount <= 0 {
		return "", fmt.Errorf("tutar pozitif olmalı")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock the register row to guarantee status is checked under the same tx.
	var status string
	if err = tx.QueryRow(ctx,
		`SELECT status::text FROM cash_register WHERE id = $1 AND branch_id = $2 FOR UPDATE`,
		in.RegisterID, in.BranchID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrRegisterNotOpen
		}
		return "", err
	}
	if status != "open" {
		return "", ErrRegisterClosed
	}

	movementNo, err := s.insertMovementTxNo(ctx, tx, in.RegisterID, in.OrganizationID, in.BranchID,
		in.Kind, in.Method, in.Amount, in.ReferenceType, in.ReferenceID, in.Counterparty,
		in.Description, in.PerformedByUserID)
	if err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return movementNo, nil
}

// insertMovementTx inserts a cash_movement row inside an existing tx,
// minting its movement_no from the sequence.
func (s *CashService) insertMovementTx(
	ctx context.Context, tx pgx.Tx,
	registerID, orgID, branchID uuid.UUID,
	kind, method string, amount float64,
	referenceType *string, referenceID *uuid.UUID, counterparty *string,
	description *string, performedByUserID *uuid.UUID,
) error {
	_, err := s.insertMovementTxNo(ctx, tx, registerID, orgID, branchID, kind, method,
		amount, referenceType, referenceID, counterparty, description, performedByUserID)
	return err
}

func (s *CashService) insertMovementTxNo(
	ctx context.Context, tx pgx.Tx,
	registerID, orgID, branchID uuid.UUID,
	kind, method string, amount float64,
	referenceType *string, referenceID *uuid.UUID, counterparty *string,
	description *string, performedByUserID *uuid.UUID,
) (string, error) {
	var nextNo int64
	if err := tx.QueryRow(ctx, `SELECT nextval('cash_movement_no_seq')`).Scan(&nextNo); err != nil {
		return "", err
	}
	movementNo := util.FormatMRN(nextNo)
	if _, err := tx.Exec(ctx,
		`INSERT INTO cash_movement
		   (organization_id, branch_id, cash_register_id, movement_no,
		    kind, method, amount, reference_type, reference_id,
		    counterparty, description, performed_by_user_id)
		 VALUES ($1, $2, $3, $4, $5::cash_movement_kind, $6::payment_method,
		         $7, $8, $9, $10, $11, $12)`,
		orgID, branchID, registerID, movementNo, kind, method, amount,
		referenceType, referenceID, counterparty, description, performedByUserID); err != nil {
		return "", err
	}
	return movementNo, nil
}

// Local helpers to avoid pulling another import.

func strPtr(s string) *string { return &s }

// isUniqueViolationLocal mirrors handler.isUniqueViolation but lives here
// to keep the service package self-contained.
func isUniqueViolationLocal(err error) bool {
	// pgconn.PgError.Code "23505" → unique_violation. We do a string match to
	// avoid introducing the pgconn import here just for one constant.
	if err == nil {
		return false
	}
	type pgErr interface {
		SQLState() string
	}
	if e, ok := err.(pgErr); ok {
		return e.SQLState() == "23505"
	}
	return false
}

// Unused-time guard: ensures time import isn't dropped if future signatures stop using it.
var _ = time.Now
