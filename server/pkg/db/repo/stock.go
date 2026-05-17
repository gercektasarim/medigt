package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------- Warehouse ----------

type Warehouse struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	BranchID       uuid.UUID
	Code           string
	Name           string
	Kind           string
	Location       *string
	IsActive       bool
	Notes          *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type WarehouseRepo struct {
	pool *pgxpool.Pool
}

func NewWarehouseRepo(pool *pgxpool.Pool) *WarehouseRepo { return &WarehouseRepo{pool: pool} }

const warehouseCols = `id, organization_id, branch_id, code, name, kind::text,
	location, is_active, notes, created_at, updated_at`

func scanWarehouse(row pgx.Row) (*Warehouse, error) {
	w := &Warehouse{}
	err := row.Scan(&w.ID, &w.OrganizationID, &w.BranchID, &w.Code, &w.Name, &w.Kind,
		&w.Location, &w.IsActive, &w.Notes, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return w, err
}

type CreateWarehouseInput struct {
	OrganizationID uuid.UUID
	BranchID       uuid.UUID
	Code           string
	Name           string
	Kind           string
	Location       *string
	Notes          *string
}

func (r *WarehouseRepo) Create(ctx context.Context, in CreateWarehouseInput) (*Warehouse, error) {
	if in.Kind == "" {
		in.Kind = "pharmacy"
	}
	row := r.pool.QueryRow(ctx,
		`INSERT INTO warehouse (organization_id, branch_id, code, name, kind, location, notes)
		 VALUES ($1, $2, $3, $4, $5::warehouse_kind, $6, $7)
		 RETURNING `+warehouseCols,
		in.OrganizationID, in.BranchID, in.Code, in.Name, in.Kind, in.Location, in.Notes)
	return scanWarehouse(row)
}

func (r *WarehouseRepo) List(ctx context.Context, branchID uuid.UUID) ([]Warehouse, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+warehouseCols+` FROM warehouse
		 WHERE branch_id = $1 AND is_active = TRUE
		 ORDER BY name`, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Warehouse{}
	for rows.Next() {
		w := Warehouse{}
		if err := rows.Scan(&w.ID, &w.OrganizationID, &w.BranchID, &w.Code, &w.Name,
			&w.Kind, &w.Location, &w.IsActive, &w.Notes, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// ---------- Stock view (live inventory per lot) ----------

type StockRow struct {
	StockID         uuid.UUID
	WarehouseID     uuid.UUID
	WarehouseCode   string
	WarehouseName   string
	MedicationID    uuid.UUID
	MedicationName  string
	GenericName     *string
	Form            string
	Strength        *string
	LotNo           string
	ExpiryDate      *time.Time
	Quantity        float64
	LastMovementAt  time.Time
}

type StockRepo struct {
	pool *pgxpool.Pool
}

func NewStockRepo(pool *pgxpool.Pool) *StockRepo { return &StockRepo{pool: pool} }

type ListStockFilter struct {
	WarehouseID   *uuid.UUID
	MedicationID  *uuid.UUID
	Search        string
	WithZero      bool
	ExpiringDays  int // > 0 → only rows with expiry within N days
	Limit         int
}

// List returns live inventory rows joining stock + warehouse + medication.
func (r *StockRepo) List(ctx context.Context, branchID uuid.UUID, f ListStockFilter) ([]StockRow, error) {
	limit := f.Limit
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	q := `SELECT s.id, s.warehouse_id, w.code, w.name,
	             s.medication_id, m.name, m.generic_name, m.form::text, m.strength,
	             s.lot_no, s.expiry_date, s.quantity, s.last_movement_at
	      FROM medication_stock s
	      JOIN warehouse w ON w.id = s.warehouse_id
	      JOIN medication m ON m.id = s.medication_id
	      WHERE s.branch_id = $1`
	args := []any{branchID}
	if !f.WithZero {
		q += ` AND s.quantity > 0`
	}
	if f.WarehouseID != nil {
		args = append(args, *f.WarehouseID)
		q += ` AND s.warehouse_id = $` + itoa(len(args))
	}
	if f.MedicationID != nil {
		args = append(args, *f.MedicationID)
		q += ` AND s.medication_id = $` + itoa(len(args))
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		idx := itoa(len(args))
		q += ` AND (m.name ILIKE $` + idx + ` OR m.generic_name ILIKE $` + idx + ` OR m.barcode ILIKE $` + idx + ` OR m.atc_code ILIKE $` + idx + `)`
	}
	if f.ExpiringDays > 0 {
		args = append(args, f.ExpiringDays)
		q += ` AND s.expiry_date IS NOT NULL AND s.expiry_date <= (CURRENT_DATE + ($` + itoa(len(args)) + ` || ' days')::INTERVAL)`
	}
	q += ` ORDER BY m.name, s.expiry_date NULLS LAST LIMIT ` + itoa(limit)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StockRow{}
	for rows.Next() {
		s := StockRow{}
		if err := rows.Scan(
			&s.StockID, &s.WarehouseID, &s.WarehouseCode, &s.WarehouseName,
			&s.MedicationID, &s.MedicationName, &s.GenericName, &s.Form, &s.Strength,
			&s.LotNo, &s.ExpiryDate, &s.Quantity, &s.LastMovementAt,
		); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ---------- Movement (audit log) ----------

type StockMovement struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	BranchID        uuid.UUID
	MovementNo      string
	WarehouseID     uuid.UUID
	MedicationID    uuid.UUID
	LotNo           string
	ExpiryDate      *time.Time
	Kind            string
	Quantity        float64
	UnitPrice       *float64
	ReferenceType   *string
	ReferenceID     *uuid.UUID
	Counterparty    *string
	Notes           *string
	PerformedByUserID *uuid.UUID
	PerformedAt     time.Time
	CreatedAt       time.Time
}

type StockMovementWithJoins struct {
	Movement       StockMovement
	WarehouseCode  string
	WarehouseName  string
	MedicationName string
}

type MovementRepo struct {
	pool *pgxpool.Pool
}

func NewMovementRepo(pool *pgxpool.Pool) *MovementRepo { return &MovementRepo{pool: pool} }

const movementCols = `id, organization_id, branch_id, movement_no, warehouse_id,
	medication_id, lot_no, expiry_date, kind::text, quantity, unit_price,
	reference_type, reference_id, counterparty, notes,
	performed_by_user_id, performed_at, created_at`

type ListMovementFilter struct {
	WarehouseID   *uuid.UUID
	MedicationID  *uuid.UUID
	Kind          string
	From          *time.Time
	To            *time.Time
	Limit         int
}

func (r *MovementRepo) List(ctx context.Context, branchID uuid.UUID, f ListMovementFilter) ([]StockMovementWithJoins, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT m.id, m.organization_id, m.branch_id, m.movement_no, m.warehouse_id,
	             m.medication_id, m.lot_no, m.expiry_date, m.kind::text, m.quantity, m.unit_price,
	             m.reference_type, m.reference_id, m.counterparty, m.notes,
	             m.performed_by_user_id, m.performed_at, m.created_at,
	             w.code, w.name, med.name
	      FROM stock_movement m
	      JOIN warehouse w ON w.id = m.warehouse_id
	      JOIN medication med ON med.id = m.medication_id
	      WHERE m.branch_id = $1`
	args := []any{branchID}
	if f.WarehouseID != nil {
		args = append(args, *f.WarehouseID)
		q += ` AND m.warehouse_id = $` + itoa(len(args))
	}
	if f.MedicationID != nil {
		args = append(args, *f.MedicationID)
		q += ` AND m.medication_id = $` + itoa(len(args))
	}
	if f.Kind != "" {
		args = append(args, f.Kind)
		q += ` AND m.kind = $` + itoa(len(args)) + `::stock_movement_kind`
	}
	if f.From != nil {
		args = append(args, *f.From)
		q += ` AND m.performed_at >= $` + itoa(len(args))
	}
	if f.To != nil {
		args = append(args, *f.To)
		q += ` AND m.performed_at < $` + itoa(len(args))
	}
	q += ` ORDER BY m.performed_at DESC LIMIT ` + itoa(limit)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StockMovementWithJoins{}
	for rows.Next() {
		w := StockMovementWithJoins{}
		mv := &w.Movement
		if err := rows.Scan(
			&mv.ID, &mv.OrganizationID, &mv.BranchID, &mv.MovementNo, &mv.WarehouseID,
			&mv.MedicationID, &mv.LotNo, &mv.ExpiryDate, &mv.Kind, &mv.Quantity, &mv.UnitPrice,
			&mv.ReferenceType, &mv.ReferenceID, &mv.Counterparty, &mv.Notes,
			&mv.PerformedByUserID, &mv.PerformedAt, &mv.CreatedAt,
			&w.WarehouseCode, &w.WarehouseName, &w.MedicationName,
		); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *MovementRepo) NextNo(ctx context.Context) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, `SELECT nextval('stock_movement_no_seq')`).Scan(&n)
	return n, err
}
