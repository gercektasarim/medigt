package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/medigt/medigt/server/internal/util"
)

// FinanceExtensionsService bundles the invariant-critical operations that
// touch patient_account_entry + refund + installment_plan/installment in
// transactional coordination with payment + cash_movement + invoice.
//
// Hard rules enforced here:
//   - patient_account balance never goes negative (avans kullanırken yetersizse 409)
//   - invoice.paid_total - refund.amount >= 0 (over-refund yasak)
//   - installment.paid_amount <= amount per row; plan.completed when all paid
//   - cash refund without open kasa is rejected
//   - cash advance receive without open kasa is rejected
type FinanceExtensionsService struct {
	pool *pgxpool.Pool
}

func NewFinanceExtensionsService(pool *pgxpool.Pool) *FinanceExtensionsService {
	return &FinanceExtensionsService{pool: pool}
}

var (
	ErrInsufficientAdvance   = errors.New("yetersiz avans bakiyesi")
	ErrCashRegisterMissing   = errors.New("nakit işlem için açık kasa gerekli")
	ErrOverRefund            = errors.New("iade tutarı kalan ödenmiş tutarı aşıyor")
	ErrNothingToRefund       = errors.New("iade için referans alınacak ödenmiş fatura/ödeme yok")
	ErrInvoiceNotPayable     = errors.New("fatura ödemeye açık değil")
	ErrInstallmentInvalid    = errors.New("taksit planı geçersiz")
	ErrInstallmentDuplicate  = errors.New("bu fatura için zaten taksit planı var")
)

// ============================================================================
//  Advance (avans) operations
// ============================================================================

// ReceiveAdvanceInput records cash + adds patient credit.
type ReceiveAdvanceInput struct {
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	PatientID         uuid.UUID
	Amount            float64
	Method            string // cash | card | transfer | mobile | other
	CashRegisterID    *uuid.UUID
	Notes             *string
	PerformedByUserID *uuid.UUID
}

func (s *FinanceExtensionsService) ReceiveAdvance(ctx context.Context, in ReceiveAdvanceInput) (uuid.UUID, error) {
	if in.Amount <= 0 {
		return uuid.Nil, fmt.Errorf("tutar pozitif olmalı")
	}
	if in.Method == "cash" && in.CashRegisterID == nil {
		return uuid.Nil, ErrCashRegisterMissing
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var cashMovementID *uuid.UUID
	if in.Method == "cash" {
		// Verify register open + create cash_movement.
		var status string
		if err = tx.QueryRow(ctx,
			`SELECT status::text FROM cash_register WHERE id = $1 AND branch_id = $2 FOR UPDATE`,
			*in.CashRegisterID, in.BranchID).Scan(&status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return uuid.Nil, ErrCashRegisterMissing
			}
			return uuid.Nil, err
		}
		if status != "open" {
			return uuid.Nil, ErrCashRegisterMissing
		}
		var mvtNo int64
		if err = tx.QueryRow(ctx, `SELECT nextval('cash_movement_no_seq')`).Scan(&mvtNo); err != nil {
			return uuid.Nil, err
		}
		var fn, ln string
		_ = tx.QueryRow(ctx, `SELECT first_name, last_name FROM patient WHERE id = $1`, in.PatientID).Scan(&fn, &ln)
		var mvtID uuid.UUID
		if err = tx.QueryRow(ctx,
			`INSERT INTO cash_movement
			   (organization_id, branch_id, cash_register_id, movement_no,
			    kind, method, amount, reference_type, counterparty, description,
			    performed_by_user_id)
			 VALUES ($1,$2,$3,$4,'income','cash',$5,'advance_in',$6,$7,$8)
			 RETURNING id`,
			in.OrganizationID, in.BranchID, *in.CashRegisterID, util.FormatMRN(mvtNo),
			in.Amount, fn+" "+ln, "Avans alındı", in.PerformedByUserID).Scan(&mvtID); err != nil {
			return uuid.Nil, err
		}
		cashMovementID = &mvtID
	}

	// Append ledger entry (advance_in, +amount).
	var entryID uuid.UUID
	if err = tx.QueryRow(ctx,
		`INSERT INTO patient_account_entry
		   (organization_id, branch_id, patient_id, kind, amount, direction,
		    cash_movement_id, notes, performed_by_user_id)
		 VALUES ($1, $2, $3, 'advance_in', $4, 1, $5, $6, $7)
		 RETURNING id`,
		in.OrganizationID, in.BranchID, in.PatientID, in.Amount,
		cashMovementID, in.Notes, in.PerformedByUserID).Scan(&entryID); err != nil {
		return uuid.Nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return entryID, nil
}

// ApplyAdvanceToInvoiceInput applies advance balance to a finalized invoice
// — creates a payment + allocation + ledger debit.
type ApplyAdvanceInput struct {
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	PatientID         uuid.UUID
	InvoiceID         uuid.UUID
	Amount            float64
	Notes             *string
	PerformedByUserID *uuid.UUID
}

func (s *FinanceExtensionsService) ApplyAdvance(ctx context.Context, in ApplyAdvanceInput) (uuid.UUID, error) {
	if in.Amount <= 0 {
		return uuid.Nil, fmt.Errorf("tutar pozitif olmalı")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock invoice + verify state.
	var invStatus string
	var balanceDue float64
	if err = tx.QueryRow(ctx,
		`SELECT status::text, balance_due FROM invoice
		 WHERE branch_id = $1 AND id = $2 FOR UPDATE`,
		in.BranchID, in.InvoiceID).Scan(&invStatus, &balanceDue); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, fmt.Errorf("fatura bulunamadı")
		}
		return uuid.Nil, err
	}
	if invStatus != "finalized" {
		return uuid.Nil, ErrInvoiceNotPayable
	}
	if in.Amount > balanceDue+0.005 {
		return uuid.Nil, ErrOverAllocate
	}

	// Check advance balance (read with row locks via FOR UPDATE on patient).
	if _, err = tx.Exec(ctx, `SELECT id FROM patient WHERE id = $1 FOR UPDATE`, in.PatientID); err != nil {
		return uuid.Nil, err
	}
	var balance float64
	if err = tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(direction * amount), 0)::NUMERIC
		 FROM patient_account_entry WHERE patient_id = $1`, in.PatientID).Scan(&balance); err != nil {
		return uuid.Nil, err
	}
	if in.Amount > balance+0.005 {
		return uuid.Nil, ErrInsufficientAdvance
	}

	// Create a payment row (method='other' — avans kullanım) + allocation.
	var payNo int64
	if err = tx.QueryRow(ctx, `SELECT nextval('payment_no_seq')`).Scan(&payNo); err != nil {
		return uuid.Nil, err
	}
	paymentNo := util.FormatMRN(payNo)
	var paymentID uuid.UUID
	if err = tx.QueryRow(ctx,
		`INSERT INTO payment (organization_id, branch_id, payment_no, patient_id,
		   method, amount, notes, received_by_user_id)
		 VALUES ($1, $2, $3, $4, 'other'::payment_method, $5, $6, $7)
		 RETURNING id`,
		in.OrganizationID, in.BranchID, paymentNo, in.PatientID,
		in.Amount, ptrOr(in.Notes, "Avans kullanımı"), in.PerformedByUserID).Scan(&paymentID); err != nil {
		return uuid.Nil, err
	}
	if _, err = tx.Exec(ctx,
		`INSERT INTO payment_allocation (organization_id, payment_id, invoice_id, amount)
		 VALUES ($1, $2, $3, $4)`,
		in.OrganizationID, paymentID, in.InvoiceID, in.Amount); err != nil {
		return uuid.Nil, err
	}
	// Bump invoice paid_total + flip to paid when fully covered.
	if _, err = tx.Exec(ctx,
		`UPDATE invoice SET paid_total = paid_total + $2,
		     status = CASE WHEN paid_total + $2 >= total THEN 'paid'::invoice_status ELSE status END
		 WHERE id = $1`,
		in.InvoiceID, in.Amount); err != nil {
		return uuid.Nil, err
	}

	// Debit advance ledger.
	if _, err = tx.Exec(ctx,
		`INSERT INTO patient_account_entry
		   (organization_id, branch_id, patient_id, kind, amount, direction,
		    payment_id, invoice_id, notes, performed_by_user_id)
		 VALUES ($1, $2, $3, 'advance_use', $4, -1, $5, $6, $7, $8)`,
		in.OrganizationID, in.BranchID, in.PatientID, in.Amount,
		paymentID, in.InvoiceID, in.Notes, in.PerformedByUserID); err != nil {
		return uuid.Nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return paymentID, nil
}

// RefundAdvanceInput pays cash back to the patient, debiting advance balance.
type RefundAdvanceInput struct {
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	PatientID         uuid.UUID
	Amount            float64
	CashRegisterID    *uuid.UUID
	Reason            *string
	PerformedByUserID *uuid.UUID
}

func (s *FinanceExtensionsService) RefundAdvance(ctx context.Context, in RefundAdvanceInput) (uuid.UUID, error) {
	if in.Amount <= 0 {
		return uuid.Nil, fmt.Errorf("tutar pozitif olmalı")
	}
	if in.CashRegisterID == nil {
		return uuid.Nil, ErrCashRegisterMissing
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Verify kasa open.
	var status string
	if err = tx.QueryRow(ctx,
		`SELECT status::text FROM cash_register WHERE id = $1 AND branch_id = $2 FOR UPDATE`,
		*in.CashRegisterID, in.BranchID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrCashRegisterMissing
		}
		return uuid.Nil, err
	}
	if status != "open" {
		return uuid.Nil, ErrCashRegisterMissing
	}

	// Check advance balance.
	if _, err = tx.Exec(ctx, `SELECT id FROM patient WHERE id = $1 FOR UPDATE`, in.PatientID); err != nil {
		return uuid.Nil, err
	}
	var balance float64
	if err = tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(direction * amount), 0)::NUMERIC
		 FROM patient_account_entry WHERE patient_id = $1`, in.PatientID).Scan(&balance); err != nil {
		return uuid.Nil, err
	}
	if in.Amount > balance+0.005 {
		return uuid.Nil, ErrInsufficientAdvance
	}

	// Cash movement (refund).
	var mvtNo int64
	if err = tx.QueryRow(ctx, `SELECT nextval('cash_movement_no_seq')`).Scan(&mvtNo); err != nil {
		return uuid.Nil, err
	}
	var fn, ln string
	_ = tx.QueryRow(ctx, `SELECT first_name, last_name FROM patient WHERE id = $1`, in.PatientID).Scan(&fn, &ln)
	var mvtID uuid.UUID
	if err = tx.QueryRow(ctx,
		`INSERT INTO cash_movement
		   (organization_id, branch_id, cash_register_id, movement_no,
		    kind, method, amount, reference_type, counterparty, description,
		    performed_by_user_id)
		 VALUES ($1,$2,$3,$4,'refund','cash',$5,'advance_refund',$6,$7,$8)
		 RETURNING id`,
		in.OrganizationID, in.BranchID, *in.CashRegisterID, util.FormatMRN(mvtNo),
		in.Amount, fn+" "+ln, "Avans iadesi", in.PerformedByUserID).Scan(&mvtID); err != nil {
		return uuid.Nil, err
	}

	// Debit advance ledger.
	var entryID uuid.UUID
	if err = tx.QueryRow(ctx,
		`INSERT INTO patient_account_entry
		   (organization_id, branch_id, patient_id, kind, amount, direction,
		    cash_movement_id, notes, performed_by_user_id)
		 VALUES ($1, $2, $3, 'advance_refund', $4, -1, $5, $6, $7)
		 RETURNING id`,
		in.OrganizationID, in.BranchID, in.PatientID, in.Amount,
		mvtID, in.Reason, in.PerformedByUserID).Scan(&entryID); err != nil {
		return uuid.Nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return entryID, nil
}

// ============================================================================
//  Refund (fatura iadesi)
// ============================================================================

type ProcessRefundInput struct {
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	PatientID         uuid.UUID
	PaymentID         *uuid.UUID
	InvoiceID         *uuid.UUID
	Amount            float64
	Method            string
	CashRegisterID    *uuid.UUID
	ToAdvance         bool
	Reason            *string
	PerformedByUserID *uuid.UUID
}

type RefundResult struct {
	RefundID       uuid.UUID
	RefundNo       string
	CashMovementID *uuid.UUID
}

// ProcessRefund executes the full refund flow:
//   - locks the source invoice
//   - verifies amount <= paid_total
//   - if cash + !to_advance: appends cash_movement (refund) — kasa must be open
//   - if to_advance: appends patient_account_entry (refund_to_advance, +amount)
//   - inserts refund row
//   - decrements invoice.paid_total + flips status back to 'finalized' if was 'paid'
func (s *FinanceExtensionsService) ProcessRefund(ctx context.Context, in ProcessRefundInput) (*RefundResult, error) {
	if in.Amount <= 0 {
		return nil, fmt.Errorf("tutar pozitif olmalı")
	}
	if in.PaymentID == nil && in.InvoiceID == nil {
		return nil, ErrNothingToRefund
	}
	if !in.ToAdvance && in.Method == "cash" && in.CashRegisterID == nil {
		return nil, ErrCashRegisterMissing
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Resolve invoice — either directly, or from payment_allocation.
	invoiceID := in.InvoiceID
	if invoiceID == nil && in.PaymentID != nil {
		var resolved uuid.UUID
		if err = tx.QueryRow(ctx,
			`SELECT invoice_id FROM payment_allocation
			 WHERE payment_id = $1 LIMIT 1`, *in.PaymentID).Scan(&resolved); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrNothingToRefund
			}
			return nil, err
		}
		invoiceID = &resolved
	}

	// Lock invoice + verify amount.
	var paidTotal, total float64
	var invStatus string
	if err = tx.QueryRow(ctx,
		`SELECT paid_total, total, status::text FROM invoice
		 WHERE branch_id = $1 AND id = $2 FOR UPDATE`,
		in.BranchID, *invoiceID).Scan(&paidTotal, &total, &invStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("fatura bulunamadı")
		}
		return nil, err
	}
	// Already-refunded total
	var alreadyRefunded float64
	_ = tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0)::NUMERIC FROM refund WHERE invoice_id = $1`,
		*invoiceID).Scan(&alreadyRefunded)
	remaining := paidTotal - alreadyRefunded
	if in.Amount > remaining+0.005 {
		return nil, ErrOverRefund
	}

	// Cash movement (only when paying out cash).
	var cashMovementID *uuid.UUID
	if in.Method == "cash" && !in.ToAdvance {
		var status string
		if err = tx.QueryRow(ctx,
			`SELECT status::text FROM cash_register WHERE id = $1 AND branch_id = $2 FOR UPDATE`,
			*in.CashRegisterID, in.BranchID).Scan(&status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrCashRegisterMissing
			}
			return nil, err
		}
		if status != "open" {
			return nil, ErrCashRegisterMissing
		}
		var mvtNo int64
		if err = tx.QueryRow(ctx, `SELECT nextval('cash_movement_no_seq')`).Scan(&mvtNo); err != nil {
			return nil, err
		}
		var fn, ln string
		_ = tx.QueryRow(ctx, `SELECT first_name, last_name FROM patient WHERE id = $1`, in.PatientID).Scan(&fn, &ln)
		var mvtID uuid.UUID
		if err = tx.QueryRow(ctx,
			`INSERT INTO cash_movement
			   (organization_id, branch_id, cash_register_id, movement_no,
			    kind, method, amount, reference_type, counterparty, description,
			    performed_by_user_id)
			 VALUES ($1,$2,$3,$4,'refund','cash',$5,'invoice_refund',$6,$7,$8)
			 RETURNING id`,
			in.OrganizationID, in.BranchID, *in.CashRegisterID, util.FormatMRN(mvtNo),
			in.Amount, fn+" "+ln, "Fatura iadesi", in.PerformedByUserID).Scan(&mvtID); err != nil {
			return nil, err
		}
		cashMovementID = &mvtID
	}

	// Insert refund row.
	var refNo int64
	if err = tx.QueryRow(ctx, `SELECT nextval('refund_no_seq')`).Scan(&refNo); err != nil {
		return nil, err
	}
	refundNo := util.FormatMRN(refNo)
	var refundID uuid.UUID
	if err = tx.QueryRow(ctx,
		`INSERT INTO refund (organization_id, branch_id, refund_no, patient_id,
		    payment_id, invoice_id, amount, method, cash_register_id, cash_movement_id,
		    to_advance, reason, performed_by_user_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8::payment_method,$9,$10,$11,$12,$13)
		 RETURNING id`,
		in.OrganizationID, in.BranchID, refundNo, in.PatientID,
		in.PaymentID, invoiceID, in.Amount, in.Method,
		in.CashRegisterID, cashMovementID, in.ToAdvance, in.Reason,
		in.PerformedByUserID).Scan(&refundID); err != nil {
		return nil, err
	}

	// Decrement invoice paid_total; if was 'paid' and goes below total → back to 'finalized'.
	if _, err = tx.Exec(ctx,
		`UPDATE invoice SET paid_total = paid_total - $2,
		     status = CASE
		       WHEN paid_total - $2 >= total THEN 'paid'::invoice_status
		       ELSE 'finalized'::invoice_status
		     END
		 WHERE id = $1`,
		*invoiceID, in.Amount); err != nil {
		return nil, err
	}

	// If to_advance: credit the patient account.
	if in.ToAdvance {
		if _, err = tx.Exec(ctx,
			`INSERT INTO patient_account_entry
			   (organization_id, branch_id, patient_id, kind, amount, direction,
			    invoice_id, refund_id, notes, performed_by_user_id)
			 VALUES ($1, $2, $3, 'refund_to_advance', $4, 1, $5, $6, $7, $8)`,
			in.OrganizationID, in.BranchID, in.PatientID, in.Amount,
			invoiceID, refundID, in.Reason, in.PerformedByUserID); err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &RefundResult{RefundID: refundID, RefundNo: refundNo, CashMovementID: cashMovementID}, nil
}

// ============================================================================
//  Installment plan
// ============================================================================

type CreateInstallmentPlanInput struct {
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	InvoiceID         uuid.UUID
	NumInstallments   int
	FirstDueDate      time.Time
	IntervalDays      int    // typically 30 (aylık)
	Notes             *string
	CreatedByUserID   *uuid.UUID
}

// CreatePlan splits a finalized invoice's outstanding total evenly into N
// installments. Refuses if a plan already exists for this invoice.
func (s *FinanceExtensionsService) CreatePlan(ctx context.Context, in CreateInstallmentPlanInput) (uuid.UUID, error) {
	if in.NumInstallments <= 0 || in.NumInstallments > 60 {
		return uuid.Nil, ErrInstallmentInvalid
	}
	if in.IntervalDays <= 0 {
		in.IntervalDays = 30
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock invoice; pick total = balance_due (only outstanding).
	var balance, total float64
	var invStatus string
	if err = tx.QueryRow(ctx,
		`SELECT balance_due, total, status::text FROM invoice
		 WHERE branch_id = $1 AND id = $2 FOR UPDATE`,
		in.BranchID, in.InvoiceID).Scan(&balance, &total, &invStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, fmt.Errorf("fatura bulunamadı")
		}
		return uuid.Nil, err
	}
	if invStatus != "finalized" {
		return uuid.Nil, ErrInvoiceNotPayable
	}
	if balance <= 0 {
		return uuid.Nil, fmt.Errorf("ödenecek bakiye yok")
	}

	// Plan row.
	var planID uuid.UUID
	if err = tx.QueryRow(ctx,
		`INSERT INTO installment_plan
		   (organization_id, branch_id, invoice_id, total_amount, num_installments, notes,
		    created_by_user_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		in.OrganizationID, in.BranchID, in.InvoiceID, balance, in.NumInstallments, in.Notes,
		in.CreatedByUserID).Scan(&planID); err != nil {
		if isUniqueViolationLocal(err) {
			return uuid.Nil, ErrInstallmentDuplicate
		}
		return uuid.Nil, err
	}

	// Evenly split with last installment absorbing rounding remainder.
	base := math.Round(balance/float64(in.NumInstallments)*100) / 100
	due := in.FirstDueDate
	var sumSoFar float64
	for i := 1; i <= in.NumInstallments; i++ {
		amount := base
		if i == in.NumInstallments {
			amount = math.Round((balance-sumSoFar)*100) / 100
		}
		if _, err = tx.Exec(ctx,
			`INSERT INTO installment (plan_id, seq, due_date, amount)
			 VALUES ($1, $2, $3, $4)`,
			planID, i, due, amount); err != nil {
			return uuid.Nil, err
		}
		sumSoFar += amount
		due = due.AddDate(0, 0, in.IntervalDays)
	}

	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return planID, nil
}

// MarkInstallmentPayment is called by the caller AFTER a regular payment is
// recorded against an invoice that has a plan — applies the payment amount
// to installments in seq order, flipping each to paid/partial.
// Idempotent-ish: caller should pass the payment_id so we can backlink.
func (s *FinanceExtensionsService) MarkInstallmentPayment(ctx context.Context, branchID, invoiceID, paymentID uuid.UUID, amount float64) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var planID uuid.UUID
	if err = tx.QueryRow(ctx,
		`SELECT id FROM installment_plan WHERE branch_id = $1 AND invoice_id = $2 FOR UPDATE`,
		branchID, invoiceID).Scan(&planID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No plan for this invoice — nothing to do.
			return nil
		}
		return err
	}

	rows, err := tx.Query(ctx,
		`SELECT id, seq, amount, paid_amount, status::text
		 FROM installment
		 WHERE plan_id = $1 AND status IN ('pending','partial','overdue')
		 ORDER BY seq FOR UPDATE`, planID)
	if err != nil {
		return err
	}
	type instLoad struct {
		ID         uuid.UUID
		Seq        int
		Amount     float64
		PaidAmount float64
		Status     string
	}
	loaded := []instLoad{}
	for rows.Next() {
		var l instLoad
		if err = rows.Scan(&l.ID, &l.Seq, &l.Amount, &l.PaidAmount, &l.Status); err != nil {
			rows.Close()
			return err
		}
		loaded = append(loaded, l)
	}
	rows.Close()

	remaining := amount
	for _, l := range loaded {
		if remaining <= 0 {
			break
		}
		need := l.Amount - l.PaidAmount
		applied := remaining
		if applied > need {
			applied = need
		}
		newPaid := l.PaidAmount + applied
		newStatus := "partial"
		var paidAt *time.Time
		var pID *uuid.UUID
		if newPaid >= l.Amount-0.005 {
			newStatus = "paid"
			t := time.Now()
			paidAt = &t
			pID = &paymentID
		}
		if _, err = tx.Exec(ctx,
			`UPDATE installment SET paid_amount = $2, status = $3::installment_status,
			     paid_at = $4, payment_id = COALESCE($5, payment_id)
			 WHERE id = $1`,
			l.ID, newPaid, newStatus, paidAt, pID); err != nil {
			return err
		}
		remaining -= applied
	}

	// If every installment is paid, mark plan completed.
	var pendingCount int
	if err = tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM installment
		 WHERE plan_id = $1 AND status IN ('pending','partial','overdue')`, planID).Scan(&pendingCount); err != nil {
		return err
	}
	if pendingCount == 0 {
		if _, err = tx.Exec(ctx,
			`UPDATE installment_plan SET status = 'completed' WHERE id = $1`, planID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// ---------- Helpers ----------

func ptrOr(p *string, def string) string {
	if p == nil || *p == "" {
		return def
	}
	return *p
}
