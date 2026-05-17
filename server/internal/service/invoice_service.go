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

// InvoiceService owns the transactional invariants of the finance core:
//   - line totals stay in lockstep with header totals
//   - payment_allocation rows roll up to paid_total
//   - cash_movement audit row is created in the same tx as the payment
//
// The service never lets balance_due go negative; over-allocation returns
// ErrOverAllocate.
type InvoiceService struct {
	pool *pgxpool.Pool
}

func NewInvoiceService(pool *pgxpool.Pool) *InvoiceService { return &InvoiceService{pool: pool} }

var (
	ErrInvoiceNotFound  = errors.New("fatura bulunamadı")
	ErrInvoiceNotOpen   = errors.New("fatura ödemeye açık değil (taslak veya kapatılmış)")
	ErrOverAllocate     = errors.New("ödeme tutarı kalan bakiyeyi aşıyor")
	ErrAllocateSumMismatch = errors.New("tahsis toplamı ödeme tutarı ile eşleşmiyor")
	ErrInvoiceHasNoItems = errors.New("fatura kalemsiz oluşturulamaz")
	ErrCashNoOpenRegister = errors.New("nakit ödeme için açık kasa gerekli")
)

// ---------- Create invoice ----------

type CreateInvoiceItemInput struct {
	ServiceID         *uuid.UUID
	Code              string
	Name              string
	VisitID           *uuid.UUID
	LabOrderID        *uuid.UUID
	RadiologyOrderID  *uuid.UUID
	SurgeryID         *uuid.UUID
	DoctorID          *uuid.UUID
	Quantity          float64
	UnitPrice         float64
	DiscountPct       float64
	VatRate           float64
	Notes             *string
}

type CreateInvoiceInput struct {
	OrganizationID   uuid.UUID
	BranchID         uuid.UUID
	PatientID        uuid.UUID
	InstitutionID    *uuid.UUID
	VisitID          *uuid.UUID
	AdmissionID      *uuid.UUID
	Notes            *string
	CreatedByUserID  *uuid.UUID
	Items            []CreateInvoiceItemInput
	Finalize         bool // if true, status='finalized' + issued_at = NOW()
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// Create writes the header + items, computes totals, optionally finalises.
func (s *InvoiceService) Create(ctx context.Context, in CreateInvoiceInput) (uuid.UUID, string, error) {
	if len(in.Items) == 0 {
		return uuid.Nil, "", ErrInvoiceHasNoItems
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var nextNo int64
	if err = tx.QueryRow(ctx, `SELECT nextval('invoice_no_seq')`).Scan(&nextNo); err != nil {
		return uuid.Nil, "", err
	}
	invoiceNo := util.FormatMRN(nextNo)

	status := "draft"
	var issuedAt *time.Time
	if in.Finalize {
		status = "finalized"
		now := time.Now()
		issuedAt = &now
	}

	var invoiceID uuid.UUID
	if err = tx.QueryRow(ctx,
		`INSERT INTO invoice (organization_id, branch_id, invoice_no, patient_id,
		   institution_id, visit_id, admission_id, status, issued_at, notes, created_by_user_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8::invoice_status, $9, $10, $11)
		 RETURNING id`,
		in.OrganizationID, in.BranchID, invoiceNo, in.PatientID,
		in.InstitutionID, in.VisitID, in.AdmissionID, status, issuedAt, in.Notes, in.CreatedByUserID,
	).Scan(&invoiceID); err != nil {
		return uuid.Nil, "", err
	}

	var subtotal, discountTotal, taxTotal, total float64
	for i, it := range in.Items {
		if it.Quantity <= 0 || it.UnitPrice < 0 {
			return uuid.Nil, "", fmt.Errorf("kalem #%d için geçersiz miktar/fiyat", i+1)
		}
		gross := it.Quantity * it.UnitPrice
		discount := gross * it.DiscountPct / 100.0
		lineSubtotal := round2(gross - discount)
		lineTax := round2(lineSubtotal * it.VatRate / 100.0)
		lineTotal := round2(lineSubtotal + lineTax)

		if _, err = tx.Exec(ctx,
			`INSERT INTO invoice_item (invoice_id, service_id, code, name,
			   visit_id, lab_order_id, radiology_order_id, surgery_id, doctor_id,
			   quantity, unit_price, discount_pct, vat_rate,
			   line_subtotal, line_tax, line_total, sort_order, notes)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`,
			invoiceID, it.ServiceID, it.Code, it.Name,
			it.VisitID, it.LabOrderID, it.RadiologyOrderID, it.SurgeryID, it.DoctorID,
			it.Quantity, it.UnitPrice, it.DiscountPct, it.VatRate,
			lineSubtotal, lineTax, lineTotal, i, it.Notes); err != nil {
			return uuid.Nil, "", err
		}
		subtotal += lineSubtotal
		discountTotal += round2(discount)
		taxTotal += lineTax
		total += lineTotal
	}
	subtotal = round2(subtotal)
	discountTotal = round2(discountTotal)
	taxTotal = round2(taxTotal)
	total = round2(total)

	if _, err = tx.Exec(ctx,
		`UPDATE invoice SET subtotal = $2, discount_total = $3, tax_total = $4, total = $5
		 WHERE id = $1`,
		invoiceID, subtotal, discountTotal, taxTotal, total); err != nil {
		return uuid.Nil, "", err
	}

	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, "", err
	}
	return invoiceID, invoiceNo, nil
}

// Finalize moves a draft to 'finalized' (becomes payable).
func (s *InvoiceService) Finalize(ctx context.Context, branchID, id uuid.UUID) error {
	res, err := s.pool.Exec(ctx,
		`UPDATE invoice SET status = 'finalized', issued_at = NOW()
		 WHERE branch_id = $1 AND id = $2 AND status = 'draft'`,
		branchID, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrInvoiceNotOpen
	}
	return nil
}

// Cancel moves an unpaid invoice to 'cancelled'. Paid invoices cannot be
// cancelled — issue a refund + new cancel-invoice cycle instead.
func (s *InvoiceService) Cancel(ctx context.Context, branchID, id uuid.UUID, reason *string) error {
	res, err := s.pool.Exec(ctx,
		`UPDATE invoice SET status = 'cancelled', cancelled_at = NOW(),
		     notes = CASE WHEN $3::TEXT IS NULL THEN notes ELSE COALESCE(notes || E'\n', '') || $3 END
		 WHERE branch_id = $1 AND id = $2 AND status IN ('draft', 'finalized') AND paid_total = 0`,
		branchID, id, reason)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("fatura iptal edilemedi: ya kapalı ya da kısmi ödeme alınmış")
	}
	return nil
}

// ---------- Record payment ----------

type PaymentAllocationInput struct {
	InvoiceID uuid.UUID
	Amount    float64
}

type RecordPaymentInput struct {
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	PatientID         uuid.UUID
	Method            string  // cash | card | transfer | mobile | other
	Amount            float64 // total payment
	Reference         *string
	Notes             *string
	CashRegisterID    *uuid.UUID // required if method='cash'
	Allocations       []PaymentAllocationInput // sum must equal Amount
	ReceivedByUserID  *uuid.UUID
}

type RecordPaymentResult struct {
	PaymentID   uuid.UUID
	PaymentNo   string
	MovementNo  *string // only for cash
}

// RecordPayment writes the payment header, the allocations, bumps each
// invoice's paid_total (transitioning to 'paid' when fully covered), and
// — when method='cash' — appends a cash_movement audit row.
func (s *InvoiceService) RecordPayment(ctx context.Context, in RecordPaymentInput) (*RecordPaymentResult, error) {
	if in.Amount <= 0 {
		return nil, fmt.Errorf("tutar pozitif olmalı")
	}
	if len(in.Allocations) == 0 {
		return nil, fmt.Errorf("en az bir fatura tahsisi gerekli")
	}
	// Cross-check: sum of allocations must equal payment amount.
	var sum float64
	for _, a := range in.Allocations {
		if a.Amount <= 0 {
			return nil, fmt.Errorf("tahsis tutarı pozitif olmalı")
		}
		sum += a.Amount
	}
	if math.Abs(sum-in.Amount) > 0.005 {
		return nil, ErrAllocateSumMismatch
	}
	if in.Method == "cash" && in.CashRegisterID == nil {
		return nil, ErrCashNoOpenRegister
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock + verify each invoice + apply allocation.
	for _, a := range in.Allocations {
		var status string
		var balanceDue float64
		if err = tx.QueryRow(ctx,
			`SELECT status::text, balance_due FROM invoice
			 WHERE branch_id = $1 AND id = $2 FOR UPDATE`,
			in.BranchID, a.InvoiceID).Scan(&status, &balanceDue); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrInvoiceNotFound
			}
			return nil, err
		}
		if status != "finalized" {
			return nil, ErrInvoiceNotOpen
		}
		if a.Amount > balanceDue+0.005 {
			return nil, ErrOverAllocate
		}
	}

	// Mint payment_no and insert header.
	var nextNo int64
	if err = tx.QueryRow(ctx, `SELECT nextval('payment_no_seq')`).Scan(&nextNo); err != nil {
		return nil, err
	}
	paymentNo := util.FormatMRN(nextNo)

	// If cash, also create the cash_movement audit row first so we can link
	// it from the payment.
	var cashMovementID *uuid.UUID
	var movementNo *string
	if in.Method == "cash" && in.CashRegisterID != nil {
		// Verify register is open and in this branch.
		var status string
		if err = tx.QueryRow(ctx,
			`SELECT status::text FROM cash_register
			 WHERE id = $1 AND branch_id = $2 FOR UPDATE`,
			*in.CashRegisterID, in.BranchID).Scan(&status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrCashNoOpenRegister
			}
			return nil, err
		}
		if status != "open" {
			return nil, ErrCashNoOpenRegister
		}

		var mvtNo int64
		if err = tx.QueryRow(ctx, `SELECT nextval('cash_movement_no_seq')`).Scan(&mvtNo); err != nil {
			return nil, err
		}
		mvtNoStr := util.FormatMRN(mvtNo)
		movementNo = &mvtNoStr

		referenceType := "payment"
		// Snapshot patient name for the cash movement counterparty.
		var fn, ln string
		_ = tx.QueryRow(ctx, `SELECT first_name, last_name FROM patient WHERE id = $1`, in.PatientID).Scan(&fn, &ln)
		cp := fn + " " + ln

		var mvtID uuid.UUID
		if err = tx.QueryRow(ctx,
			`INSERT INTO cash_movement (organization_id, branch_id, cash_register_id, movement_no,
			   kind, method, amount, reference_type, reference_id, counterparty, description,
			   performed_by_user_id)
			 VALUES ($1, $2, $3, $4, 'income', 'cash', $5, $6, NULL, $7, $8, $9)
			 RETURNING id`,
			in.OrganizationID, in.BranchID, *in.CashRegisterID, mvtNoStr,
			in.Amount, referenceType, cp, "Fatura tahsilatı",
			in.ReceivedByUserID).Scan(&mvtID); err != nil {
			return nil, err
		}
		cashMovementID = &mvtID
	}

	var paymentID uuid.UUID
	if err = tx.QueryRow(ctx,
		`INSERT INTO payment (organization_id, branch_id, payment_no, patient_id,
		   method, amount, cash_register_id, cash_movement_id, reference, notes,
		   received_by_user_id)
		 VALUES ($1, $2, $3, $4, $5::payment_method, $6, $7, $8, $9, $10, $11)
		 RETURNING id`,
		in.OrganizationID, in.BranchID, paymentNo, in.PatientID,
		in.Method, in.Amount, in.CashRegisterID, cashMovementID,
		in.Reference, in.Notes, in.ReceivedByUserID).Scan(&paymentID); err != nil {
		return nil, err
	}

	// If cash movement was created, back-link its reference_id to the payment.
	if cashMovementID != nil {
		if _, err = tx.Exec(ctx,
			`UPDATE cash_movement SET reference_id = $2 WHERE id = $1`,
			*cashMovementID, paymentID); err != nil {
			return nil, err
		}
	}

	// Insert each allocation + update invoice paid_total / status.
	for _, a := range in.Allocations {
		if _, err = tx.Exec(ctx,
			`INSERT INTO payment_allocation (organization_id, payment_id, invoice_id, amount)
			 VALUES ($1, $2, $3, $4)`,
			in.OrganizationID, paymentID, a.InvoiceID, a.Amount); err != nil {
			return nil, err
		}
		// Update invoice paid_total; if fully paid, set status to 'paid'.
		if _, err = tx.Exec(ctx,
			`UPDATE invoice SET paid_total = paid_total + $2,
			     status = CASE WHEN paid_total + $2 >= total THEN 'paid'::invoice_status ELSE status END
			 WHERE id = $1`,
			a.InvoiceID, a.Amount); err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	res := &RecordPaymentResult{PaymentID: paymentID, PaymentNo: paymentNo, MovementNo: movementNo}
	return res, nil
}
