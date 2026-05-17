package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------- Invoice ----------

type Invoice struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	BranchID        uuid.UUID
	InvoiceNo       string
	PatientID       uuid.UUID
	InstitutionID   *uuid.UUID
	VisitID         *uuid.UUID
	AdmissionID     *uuid.UUID
	Status          string
	Subtotal        float64
	DiscountTotal   float64
	TaxTotal        float64
	Total           float64
	PaidTotal       float64
	BalanceDue      float64
	IssuedAt        *time.Time
	CancelledAt     *time.Time
	Notes           *string
	CreatedByUserID *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type InvoiceItem struct {
	ID                uuid.UUID
	InvoiceID         uuid.UUID
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
	LineSubtotal      float64
	LineTax           float64
	LineTotal         float64
	SortOrder         int
	Notes             *string
}

type InvoiceWithJoins struct {
	Invoice          Invoice
	PatientMRN       string
	PatientFirstName string
	PatientLastName  string
	InstitutionName  *string
}

type InvoiceRepo struct{ pool *pgxpool.Pool }

func NewInvoiceRepo(pool *pgxpool.Pool) *InvoiceRepo { return &InvoiceRepo{pool: pool} }

const invoiceCols = `id, organization_id, branch_id, invoice_no, patient_id,
	institution_id, visit_id, admission_id, status::text,
	subtotal, discount_total, tax_total, total, paid_total, balance_due,
	issued_at, cancelled_at, notes, created_by_user_id, created_at, updated_at`

func scanInvoice(scanner func(...any) error) (*Invoice, error) {
	i := &Invoice{}
	err := scanner(
		&i.ID, &i.OrganizationID, &i.BranchID, &i.InvoiceNo, &i.PatientID,
		&i.InstitutionID, &i.VisitID, &i.AdmissionID, &i.Status,
		&i.Subtotal, &i.DiscountTotal, &i.TaxTotal, &i.Total, &i.PaidTotal, &i.BalanceDue,
		&i.IssuedAt, &i.CancelledAt, &i.Notes, &i.CreatedByUserID, &i.CreatedAt, &i.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return i, err
}

func (r *InvoiceRepo) NextNo(ctx context.Context) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, `SELECT nextval('invoice_no_seq')`).Scan(&n)
	return n, err
}

func (r *InvoiceRepo) GetByID(ctx context.Context, branchID, id uuid.UUID) (*Invoice, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+invoiceCols+` FROM invoice WHERE branch_id = $1 AND id = $2`,
		branchID, id)
	return scanInvoice(row.Scan)
}

func (r *InvoiceRepo) ListItems(ctx context.Context, invoiceID uuid.UUID) ([]InvoiceItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, invoice_id, service_id, code, name,
		        visit_id, lab_order_id, radiology_order_id, surgery_id, doctor_id,
		        quantity, unit_price, discount_pct, vat_rate,
		        line_subtotal, line_tax, line_total, sort_order, notes
		 FROM invoice_item
		 WHERE invoice_id = $1 ORDER BY sort_order`, invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InvoiceItem{}
	for rows.Next() {
		it := InvoiceItem{}
		if err := rows.Scan(
			&it.ID, &it.InvoiceID, &it.ServiceID, &it.Code, &it.Name,
			&it.VisitID, &it.LabOrderID, &it.RadiologyOrderID, &it.SurgeryID, &it.DoctorID,
			&it.Quantity, &it.UnitPrice, &it.DiscountPct, &it.VatRate,
			&it.LineSubtotal, &it.LineTax, &it.LineTotal, &it.SortOrder, &it.Notes,
		); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

type ListInvoiceFilter struct {
	Status      string
	PatientID   *uuid.UUID
	From        *time.Time
	To          *time.Time
	OnlyUnpaid  bool
	Limit       int
}

func (r *InvoiceRepo) List(ctx context.Context, branchID uuid.UUID, f ListInvoiceFilter) ([]InvoiceWithJoins, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT i.id, i.organization_id, i.branch_id, i.invoice_no, i.patient_id,
	             i.institution_id, i.visit_id, i.admission_id, i.status::text,
	             i.subtotal, i.discount_total, i.tax_total, i.total, i.paid_total, i.balance_due,
	             i.issued_at, i.cancelled_at, i.notes, i.created_by_user_id,
	             i.created_at, i.updated_at,
	             p.mrn, p.first_name, p.last_name, ins.name
	      FROM invoice i
	      JOIN patient p ON p.id = i.patient_id
	      LEFT JOIN external_institution ins ON ins.id = i.institution_id
	      WHERE i.branch_id = $1`
	args := []any{branchID}
	if f.Status != "" {
		args = append(args, f.Status)
		q += ` AND i.status = $` + itoa(len(args)) + `::invoice_status`
	}
	if f.PatientID != nil {
		args = append(args, *f.PatientID)
		q += ` AND i.patient_id = $` + itoa(len(args))
	}
	if f.From != nil {
		args = append(args, *f.From)
		q += ` AND i.created_at >= $` + itoa(len(args))
	}
	if f.To != nil {
		args = append(args, *f.To)
		q += ` AND i.created_at < $` + itoa(len(args))
	}
	if f.OnlyUnpaid {
		q += ` AND i.status IN ('finalized') AND i.balance_due > 0`
	}
	q += ` ORDER BY i.created_at DESC LIMIT ` + itoa(limit)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InvoiceWithJoins{}
	for rows.Next() {
		w := InvoiceWithJoins{}
		inv := &w.Invoice
		if err := rows.Scan(
			&inv.ID, &inv.OrganizationID, &inv.BranchID, &inv.InvoiceNo, &inv.PatientID,
			&inv.InstitutionID, &inv.VisitID, &inv.AdmissionID, &inv.Status,
			&inv.Subtotal, &inv.DiscountTotal, &inv.TaxTotal, &inv.Total, &inv.PaidTotal, &inv.BalanceDue,
			&inv.IssuedAt, &inv.CancelledAt, &inv.Notes, &inv.CreatedByUserID,
			&inv.CreatedAt, &inv.UpdatedAt,
			&w.PatientMRN, &w.PatientFirstName, &w.PatientLastName, &w.InstitutionName,
		); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// ---------- Payment ----------

type Payment struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	BranchID        uuid.UUID
	PaymentNo       string
	PatientID       uuid.UUID
	Method          string
	Amount          float64
	CashRegisterID  *uuid.UUID
	CashMovementID  *uuid.UUID
	Reference       *string
	Notes           *string
	ReceivedByUserID *uuid.UUID
	ReceivedAt      time.Time
	CreatedAt       time.Time
}

type PaymentAllocation struct {
	ID         uuid.UUID
	PaymentID  uuid.UUID
	InvoiceID  uuid.UUID
	Amount     float64
	InvoiceNo  string // joined for display
}

type PaymentRepo struct{ pool *pgxpool.Pool }

func NewPaymentRepo(pool *pgxpool.Pool) *PaymentRepo { return &PaymentRepo{pool: pool} }

const paymentCols = `id, organization_id, branch_id, payment_no, patient_id,
	method::text, amount, cash_register_id, cash_movement_id, reference, notes,
	received_by_user_id, received_at, created_at`

func (r *PaymentRepo) NextNo(ctx context.Context) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, `SELECT nextval('payment_no_seq')`).Scan(&n)
	return n, err
}

func (r *PaymentRepo) ListForInvoice(ctx context.Context, invoiceID uuid.UUID) ([]Payment, []PaymentAllocation, error) {
	allocRows, err := r.pool.Query(ctx,
		`SELECT pa.id, pa.payment_id, pa.invoice_id, pa.amount, i.invoice_no
		 FROM payment_allocation pa JOIN invoice i ON i.id = pa.invoice_id
		 WHERE pa.invoice_id = $1 ORDER BY pa.created_at`, invoiceID)
	if err != nil {
		return nil, nil, err
	}
	defer allocRows.Close()
	allocs := []PaymentAllocation{}
	paymentIDs := []uuid.UUID{}
	for allocRows.Next() {
		a := PaymentAllocation{}
		if err := allocRows.Scan(&a.ID, &a.PaymentID, &a.InvoiceID, &a.Amount, &a.InvoiceNo); err != nil {
			return nil, nil, err
		}
		allocs = append(allocs, a)
		paymentIDs = append(paymentIDs, a.PaymentID)
	}
	if len(paymentIDs) == 0 {
		return []Payment{}, allocs, nil
	}
	payRows, err := r.pool.Query(ctx,
		`SELECT `+paymentCols+` FROM payment WHERE id = ANY($1) ORDER BY received_at DESC`, paymentIDs)
	if err != nil {
		return nil, nil, err
	}
	defer payRows.Close()
	payments := []Payment{}
	for payRows.Next() {
		p := Payment{}
		if err := payRows.Scan(&p.ID, &p.OrganizationID, &p.BranchID, &p.PaymentNo, &p.PatientID,
			&p.Method, &p.Amount, &p.CashRegisterID, &p.CashMovementID, &p.Reference, &p.Notes,
			&p.ReceivedByUserID, &p.ReceivedAt, &p.CreatedAt); err != nil {
			return nil, nil, err
		}
		payments = append(payments, p)
	}
	return payments, allocs, nil
}
