package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CashRegister struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	BranchID        uuid.UUID
	RegisterNo      string
	CashierUserID   uuid.UUID
	CashierName     string
	Status          string
	OpeningBalance  float64
	DeclaredBalance *float64
	Notes           *string
	OpenedAt        time.Time
	ClosedAt        *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CashRegisterRepo struct{ pool *pgxpool.Pool }

func NewCashRegisterRepo(pool *pgxpool.Pool) *CashRegisterRepo { return &CashRegisterRepo{pool: pool} }

const cashRegCols = `id, organization_id, branch_id, register_no, cashier_user_id, cashier_name,
	status::text, opening_balance, declared_balance, notes,
	opened_at, closed_at, created_at, updated_at`

func scanCashRegister(row pgx.Row) (*CashRegister, error) {
	c := &CashRegister{}
	err := row.Scan(&c.ID, &c.OrganizationID, &c.BranchID, &c.RegisterNo, &c.CashierUserID, &c.CashierName,
		&c.Status, &c.OpeningBalance, &c.DeclaredBalance, &c.Notes,
		&c.OpenedAt, &c.ClosedAt, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

// FindOpenForUser returns the active register for the given cashier, or
// ErrNotFound if none is open. Use to decide whether to show "open kasa"
// or jump straight to the active session.
func (r *CashRegisterRepo) FindOpenForUser(ctx context.Context, userID uuid.UUID) (*CashRegister, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+cashRegCols+` FROM cash_register
		 WHERE cashier_user_id = $1 AND status = 'open'`, userID)
	return scanCashRegister(row)
}

func (r *CashRegisterRepo) GetByID(ctx context.Context, branchID, id uuid.UUID) (*CashRegister, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+cashRegCols+` FROM cash_register
		 WHERE branch_id = $1 AND id = $2`, branchID, id)
	return scanCashRegister(row)
}

type ListRegisterFilter struct {
	Status string
	From   *time.Time
	To     *time.Time
	Limit  int
}

func (r *CashRegisterRepo) List(ctx context.Context, branchID uuid.UUID, f ListRegisterFilter) ([]CashRegister, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT ` + cashRegCols + ` FROM cash_register WHERE branch_id = $1`
	args := []any{branchID}
	if f.Status != "" {
		args = append(args, f.Status)
		q += ` AND status = $` + itoa(len(args)) + `::cash_register_status`
	}
	if f.From != nil {
		args = append(args, *f.From)
		q += ` AND opened_at >= $` + itoa(len(args))
	}
	if f.To != nil {
		args = append(args, *f.To)
		q += ` AND opened_at < $` + itoa(len(args))
	}
	q += ` ORDER BY opened_at DESC LIMIT ` + itoa(limit)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CashRegister{}
	for rows.Next() {
		c := CashRegister{}
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.BranchID, &c.RegisterNo, &c.CashierUserID, &c.CashierName,
			&c.Status, &c.OpeningBalance, &c.DeclaredBalance, &c.Notes,
			&c.OpenedAt, &c.ClosedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ---------- Cash movement ----------

type CashMovement struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	BranchID        uuid.UUID
	CashRegisterID  uuid.UUID
	MovementNo      string
	Kind            string
	Method          string
	Amount          float64
	ReferenceType   *string
	ReferenceID     *uuid.UUID
	Counterparty    *string
	Description     *string
	PerformedByUserID *uuid.UUID
	PerformedAt     time.Time
	CreatedAt       time.Time
}

type CashMovementRepo struct{ pool *pgxpool.Pool }

func NewCashMovementRepo(pool *pgxpool.Pool) *CashMovementRepo { return &CashMovementRepo{pool: pool} }

const cashMvtCols = `id, organization_id, branch_id, cash_register_id, movement_no,
	kind::text, method::text, amount, reference_type, reference_id,
	counterparty, description, performed_by_user_id, performed_at, created_at`

func (r *CashMovementRepo) ListForRegister(ctx context.Context, registerID uuid.UUID) ([]CashMovement, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+cashMvtCols+` FROM cash_movement
		 WHERE cash_register_id = $1 ORDER BY performed_at`, registerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CashMovement{}
	for rows.Next() {
		m := CashMovement{}
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.BranchID, &m.CashRegisterID, &m.MovementNo,
			&m.Kind, &m.Method, &m.Amount, &m.ReferenceType, &m.ReferenceID,
			&m.Counterparty, &m.Description, &m.PerformedByUserID, &m.PerformedAt, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ZReport summarises a register's session: opening balance, movement
// totals grouped by kind + method, expected closing balance, declared
// balance, and variance.
type ZReportSummaryRow struct {
	Kind   string
	Method string
	Total  float64
	Count  int
}

type ZReport struct {
	Register        CashRegister
	Movements       []CashMovement
	ByKindMethod    []ZReportSummaryRow
	TotalIncome     float64
	TotalExpense    float64
	TotalRefund     float64
	ExpectedClose   float64
	Variance        *float64 // declared - expected, when register is closed
}

func (r *CashMovementRepo) ZReport(ctx context.Context, register *CashRegister) (*ZReport, error) {
	movements, err := r.ListForRegister(ctx, register.ID)
	if err != nil {
		return nil, err
	}

	summary := []ZReportSummaryRow{}
	rows, err := r.pool.Query(ctx,
		`SELECT kind::text, method::text, SUM(amount), COUNT(*)
		 FROM cash_movement
		 WHERE cash_register_id = $1
		 GROUP BY kind, method
		 ORDER BY kind, method`, register.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		s := ZReportSummaryRow{}
		if err := rows.Scan(&s.Kind, &s.Method, &s.Total, &s.Count); err != nil {
			return nil, err
		}
		summary = append(summary, s)
	}

	z := &ZReport{Register: *register, Movements: movements, ByKindMethod: summary}
	for _, s := range summary {
		if s.Method != "cash" {
			// Z report compares cash only (kasada görünen para). Other
			// methods are informational.
			continue
		}
		switch s.Kind {
		case "income":
			z.TotalIncome += s.Total
		case "expense":
			z.TotalExpense += s.Total
		case "refund":
			z.TotalRefund += s.Total
		}
	}
	z.ExpectedClose = register.OpeningBalance + z.TotalIncome - z.TotalExpense - z.TotalRefund
	if register.DeclaredBalance != nil {
		v := *register.DeclaredBalance - z.ExpectedClose
		z.Variance = &v
	}
	return z, nil
}
