package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PatientAccountEntry is one ledger row on the patient's cari hesap.
// The running balance is SUM(direction * amount) — never denormalised.
type PatientAccountEntry struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	PatientID         uuid.UUID
	Kind              string
	Amount            float64
	Direction         int
	PaymentID         *uuid.UUID
	InvoiceID         *uuid.UUID
	CashMovementID    *uuid.UUID
	RefundID          *uuid.UUID
	Notes             *string
	PerformedByUserID *uuid.UUID
	PerformedAt       time.Time
	CreatedAt         time.Time
}

type CariRepo struct{ pool *pgxpool.Pool }

func NewCariRepo(pool *pgxpool.Pool) *CariRepo { return &CariRepo{pool: pool} }

// BalanceFor returns the running balance + raw signed total in one query.
func (r *CariRepo) BalanceFor(ctx context.Context, patientID uuid.UUID) (float64, error) {
	var balance float64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(direction * amount), 0)::NUMERIC FROM patient_account_entry
		 WHERE patient_id = $1`, patientID).Scan(&balance)
	return balance, err
}

// EntriesFor returns the ledger entries for a patient (most recent first).
func (r *CariRepo) EntriesFor(ctx context.Context, patientID uuid.UUID, limit int) ([]PatientAccountEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, branch_id, patient_id, kind::text, amount, direction,
		        payment_id, invoice_id, cash_movement_id, refund_id, notes,
		        performed_by_user_id, performed_at, created_at
		 FROM patient_account_entry
		 WHERE patient_id = $1
		 ORDER BY performed_at DESC
		 LIMIT $2`, patientID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PatientAccountEntry{}
	for rows.Next() {
		e := PatientAccountEntry{}
		if err := rows.Scan(&e.ID, &e.OrganizationID, &e.BranchID, &e.PatientID, &e.Kind, &e.Amount, &e.Direction,
			&e.PaymentID, &e.InvoiceID, &e.CashMovementID, &e.RefundID, &e.Notes,
			&e.PerformedByUserID, &e.PerformedAt, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---------- Refund ----------

type Refund struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	RefundNo          string
	PatientID         uuid.UUID
	PaymentID         *uuid.UUID
	InvoiceID         *uuid.UUID
	Amount            float64
	Method            string
	CashRegisterID    *uuid.UUID
	CashMovementID    *uuid.UUID
	ToAdvance         bool
	Reason            *string
	PerformedByUserID *uuid.UUID
	PerformedAt       time.Time
	CreatedAt         time.Time
}

type RefundWithJoins struct {
	Refund           Refund
	PatientMRN       string
	PatientFirstName string
	PatientLastName  string
	InvoiceNo        *string
}

type RefundRepo struct{ pool *pgxpool.Pool }

func NewRefundRepo(pool *pgxpool.Pool) *RefundRepo { return &RefundRepo{pool: pool} }

const refundCols = `id, organization_id, branch_id, refund_no, patient_id,
	payment_id, invoice_id, amount, method::text,
	cash_register_id, cash_movement_id, to_advance,
	reason, performed_by_user_id, performed_at, created_at`

func (r *RefundRepo) ListForBranch(ctx context.Context, branchID uuid.UUID, limit int) ([]RefundWithJoins, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx,
		`SELECT r.id, r.organization_id, r.branch_id, r.refund_no, r.patient_id,
		        r.payment_id, r.invoice_id, r.amount, r.method::text,
		        r.cash_register_id, r.cash_movement_id, r.to_advance,
		        r.reason, r.performed_by_user_id, r.performed_at, r.created_at,
		        p.mrn, p.first_name, p.last_name, i.invoice_no
		 FROM refund r
		 JOIN patient p ON p.id = r.patient_id
		 LEFT JOIN invoice i ON i.id = r.invoice_id
		 WHERE r.branch_id = $1
		 ORDER BY r.performed_at DESC
		 LIMIT $2`, branchID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RefundWithJoins{}
	for rows.Next() {
		w := RefundWithJoins{}
		f := &w.Refund
		if err := rows.Scan(&f.ID, &f.OrganizationID, &f.BranchID, &f.RefundNo, &f.PatientID,
			&f.PaymentID, &f.InvoiceID, &f.Amount, &f.Method,
			&f.CashRegisterID, &f.CashMovementID, &f.ToAdvance,
			&f.Reason, &f.PerformedByUserID, &f.PerformedAt, &f.CreatedAt,
			&w.PatientMRN, &w.PatientFirstName, &w.PatientLastName, &w.InvoiceNo); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *RefundRepo) ListForInvoice(ctx context.Context, invoiceID uuid.UUID) ([]Refund, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+refundCols+` FROM refund WHERE invoice_id = $1 ORDER BY performed_at DESC`,
		invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Refund{}
	for rows.Next() {
		f := Refund{}
		if err := rows.Scan(&f.ID, &f.OrganizationID, &f.BranchID, &f.RefundNo, &f.PatientID,
			&f.PaymentID, &f.InvoiceID, &f.Amount, &f.Method,
			&f.CashRegisterID, &f.CashMovementID, &f.ToAdvance,
			&f.Reason, &f.PerformedByUserID, &f.PerformedAt, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ---------- Installment plan ----------

type InstallmentPlan struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	BranchID        uuid.UUID
	InvoiceID       uuid.UUID
	TotalAmount     float64
	NumInstallments int
	Status          string
	Notes           *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Installment struct {
	ID         uuid.UUID
	PlanID     uuid.UUID
	Seq        int
	DueDate    time.Time
	Amount     float64
	PaidAmount float64
	Status     string
	PaidAt     *time.Time
	PaymentID  *uuid.UUID
	Notes      *string
}

type InstallmentPlanRepo struct{ pool *pgxpool.Pool }

func NewInstallmentPlanRepo(pool *pgxpool.Pool) *InstallmentPlanRepo {
	return &InstallmentPlanRepo{pool: pool}
}

func (r *InstallmentPlanRepo) GetForInvoice(ctx context.Context, invoiceID uuid.UUID) (*InstallmentPlan, []Installment, error) {
	planRow := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, branch_id, invoice_id, total_amount,
		        num_installments, status::text, notes, created_at, updated_at
		 FROM installment_plan WHERE invoice_id = $1`, invoiceID)
	plan := &InstallmentPlan{}
	if err := planRow.Scan(&plan.ID, &plan.OrganizationID, &plan.BranchID, &plan.InvoiceID,
		&plan.TotalAmount, &plan.NumInstallments, &plan.Status, &plan.Notes,
		&plan.CreatedAt, &plan.UpdatedAt); err != nil {
		return nil, nil, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, plan_id, seq, due_date, amount, paid_amount, status::text,
		        paid_at, payment_id, notes
		 FROM installment WHERE plan_id = $1 ORDER BY seq`, plan.ID)
	if err != nil {
		return plan, nil, err
	}
	defer rows.Close()
	out := []Installment{}
	for rows.Next() {
		i := Installment{}
		if err := rows.Scan(&i.ID, &i.PlanID, &i.Seq, &i.DueDate, &i.Amount,
			&i.PaidAmount, &i.Status, &i.PaidAt, &i.PaymentID, &i.Notes); err != nil {
			return plan, nil, err
		}
		out = append(out, i)
	}
	return plan, out, rows.Err()
}

// UpcomingForBranch returns pending/partial installments due soon. Used by
// the vezne dashboard to surface today's collections.
type UpcomingInstallment struct {
	Installment      Installment
	InvoiceID        uuid.UUID
	InvoiceNo        string
	PatientID        uuid.UUID
	PatientMRN       string
	PatientFirstName string
	PatientLastName  string
}

func (r *InstallmentPlanRepo) UpcomingForBranch(ctx context.Context, branchID uuid.UUID, throughDate time.Time, limit int) ([]UpcomingInstallment, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx,
		`SELECT i.id, i.plan_id, i.seq, i.due_date, i.amount, i.paid_amount, i.status::text,
		        i.paid_at, i.payment_id, i.notes,
		        inv.id, inv.invoice_no, pat.id, pat.mrn, pat.first_name, pat.last_name
		 FROM installment i
		 JOIN installment_plan pl ON pl.id = i.plan_id
		 JOIN invoice inv ON inv.id = pl.invoice_id
		 JOIN patient pat ON pat.id = inv.patient_id
		 WHERE pl.branch_id = $1
		   AND i.status IN ('pending', 'partial', 'overdue')
		   AND i.due_date <= $2
		 ORDER BY i.due_date, i.seq
		 LIMIT $3`, branchID, throughDate, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UpcomingInstallment{}
	for rows.Next() {
		u := UpcomingInstallment{}
		i := &u.Installment
		if err := rows.Scan(&i.ID, &i.PlanID, &i.Seq, &i.DueDate, &i.Amount, &i.PaidAmount, &i.Status,
			&i.PaidAt, &i.PaymentID, &i.Notes,
			&u.InvoiceID, &u.InvoiceNo, &u.PatientID, &u.PatientMRN, &u.PatientFirstName, &u.PatientLastName); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
