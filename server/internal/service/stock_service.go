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
	"github.com/medigt/medigt/server/pkg/db/repo"
)

// StockService runs the transactional movement primitives that touch both
// medication_stock (live inventory) and stock_movement (audit log).
type StockService struct {
	pool *pgxpool.Pool
}

func NewStockService(pool *pgxpool.Pool) *StockService { return &StockService{pool: pool} }

// ReceiveInput records an intake (irsaliye / mal alımı). Creates or upserts
// the stock lot, then appends a 'receive' movement. Returns the movement_no.
type ReceiveInput struct {
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	WarehouseID       uuid.UUID
	MedicationID      uuid.UUID
	LotNo             string
	ExpiryDate        *time.Time
	Quantity          float64
	UnitPrice         *float64
	Counterparty      *string
	Notes             *string
	PerformedByUserID *uuid.UUID
}

// AdjustInput corrects a lot's quantity to a new absolute value. Stores the
// delta in stock_movement (kind='adjust'), positive or negative — the audit
// row records the magnitude with quantity > 0 and the kind 'adjust'; the
// notes field should explain whether it was an increment or decrement.
type AdjustInput struct {
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	WarehouseID       uuid.UUID
	MedicationID      uuid.UUID
	LotNo             string
	ExpiryDate        *time.Time
	NewQuantity       float64
	Notes             *string
	PerformedByUserID *uuid.UUID
}

// ErrLotNotFound is returned by Adjust when no live lot row exists yet —
// callers should call Receive instead.
var ErrLotNotFound = errors.New("lot bulunamadı; önce mal girişi yapın")

// ErrInvalidQuantity is returned when quantity is <= 0 where the kind
// requires a positive movement.
var ErrInvalidQuantity = errors.New("miktar pozitif olmalı")

// ErrInsufficientStock is returned when the requested dispense exceeds the
// quantity available in the lot.
var ErrInsufficientStock = errors.New("yetersiz stok")

func (s *StockService) Receive(ctx context.Context, in ReceiveInput) (string, error) {
	if in.Quantity <= 0 {
		return "", ErrInvalidQuantity
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Upsert the live stock row. The UNIQUE constraint is
	// (warehouse_id, medication_id, lot_no, expiry_date); ON CONFLICT
	// uses these columns.
	if _, err = tx.Exec(ctx,
		`INSERT INTO medication_stock
		   (organization_id, branch_id, warehouse_id, medication_id, lot_no, expiry_date, quantity, last_movement_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		 ON CONFLICT (warehouse_id, medication_id, lot_no, expiry_date)
		 DO UPDATE SET quantity = medication_stock.quantity + EXCLUDED.quantity,
		               last_movement_at = NOW()`,
		in.OrganizationID, in.BranchID, in.WarehouseID, in.MedicationID,
		in.LotNo, in.ExpiryDate, in.Quantity); err != nil {
		return "", err
	}

	// Append audit row.
	var nextNo int64
	if err = tx.QueryRow(ctx, `SELECT nextval('stock_movement_no_seq')`).Scan(&nextNo); err != nil {
		return "", err
	}
	movementNo := util.FormatMRN(nextNo)
	if _, err = tx.Exec(ctx,
		`INSERT INTO stock_movement
		   (organization_id, branch_id, movement_no, warehouse_id, medication_id,
		    lot_no, expiry_date, kind, quantity, unit_price, counterparty, notes,
		    performed_by_user_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 'receive', $8, $9, $10, $11, $12)`,
		in.OrganizationID, in.BranchID, movementNo, in.WarehouseID, in.MedicationID,
		in.LotNo, in.ExpiryDate, in.Quantity, in.UnitPrice, in.Counterparty, in.Notes,
		in.PerformedByUserID); err != nil {
		return "", err
	}

	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return movementNo, nil
}

func (s *StockService) Adjust(ctx context.Context, in AdjustInput) (string, error) {
	if in.NewQuantity < 0 {
		return "", fmt.Errorf("yeni miktar negatif olamaz")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock the live stock row to compute the delta safely.
	var stockID uuid.UUID
	var oldQty float64
	row := tx.QueryRow(ctx,
		`SELECT id, quantity FROM medication_stock
		 WHERE branch_id = $1 AND warehouse_id = $2 AND medication_id = $3
		   AND lot_no = $4 AND (expiry_date IS NOT DISTINCT FROM $5)
		 FOR UPDATE`,
		in.BranchID, in.WarehouseID, in.MedicationID, in.LotNo, in.ExpiryDate)
	if err = row.Scan(&stockID, &oldQty); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrLotNotFound
		}
		return "", err
	}

	delta := in.NewQuantity - oldQty
	if delta == 0 {
		return "", fmt.Errorf("miktar değişikliği yok")
	}

	if _, err = tx.Exec(ctx,
		`UPDATE medication_stock SET quantity = $2, last_movement_at = NOW()
		 WHERE id = $1`, stockID, in.NewQuantity); err != nil {
		return "", err
	}

	var nextNo int64
	if err = tx.QueryRow(ctx, `SELECT nextval('stock_movement_no_seq')`).Scan(&nextNo); err != nil {
		return "", err
	}
	movementNo := util.FormatMRN(nextNo)

	// We always record the magnitude (positive); the kind+notes carry the
	// direction. The audit row's notes are prefixed with the sign.
	mag := delta
	sign := "+"
	if delta < 0 {
		mag = -delta
		sign = "-"
	}
	notesText := sign + fmt.Sprintf("%.3f", mag)
	if in.Notes != nil && *in.Notes != "" {
		notesText = notesText + " — " + *in.Notes
	}

	if _, err = tx.Exec(ctx,
		`INSERT INTO stock_movement
		   (organization_id, branch_id, movement_no, warehouse_id, medication_id,
		    lot_no, expiry_date, kind, quantity, notes, performed_by_user_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 'adjust', $8, $9, $10)`,
		in.OrganizationID, in.BranchID, movementNo, in.WarehouseID, in.MedicationID,
		in.LotNo, in.ExpiryDate, mag, notesText, in.PerformedByUserID); err != nil {
		return "", err
	}

	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return movementNo, nil
}

// DispenseInput requests a single-lot dispense for a prescription item.
// The medication_id on the item is updated (if previously NULL) so future
// dispenses against the same item can default to the same drug.
type DispenseInput struct {
	OrganizationID     uuid.UUID
	BranchID           uuid.UUID
	PrescriptionItemID uuid.UUID
	MedicationID       uuid.UUID // catalog row chosen by the eczane operator
	WarehouseID        uuid.UUID
	LotNo              string
	ExpiryDate         *time.Time
	Quantity           float64
	Counterparty       *string // hasta adı (snapshot)
	DispensedByUserID  *uuid.UUID
}

// DispenseResult returns the audit IDs created by the dispense.
type DispenseResult struct {
	DispenseID uuid.UUID
	MovementNo string
}

// Dispense decrements the picked lot, appends an 'issue' stock_movement,
// and inserts a prescription_dispense row — all in one transaction.
func (s *StockService) Dispense(ctx context.Context, in DispenseInput) (*DispenseResult, error) {
	if in.Quantity <= 0 {
		return nil, ErrInvalidQuantity
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Step 1: load item context — verifies the prescription_item exists
	// and the caller's org matches.
	er := repo.NewEczaneRepo(s.pool)
	ic, err := er.ItemContext(ctx, tx, in.PrescriptionItemID)
	if err != nil {
		return nil, err
	}
	if ic.OrgID != in.OrganizationID {
		return nil, fmt.Errorf("cross-tenant dispense reddedildi")
	}

	// Step 2: lock the lot row and verify stock.
	var stockID uuid.UUID
	var oldQty float64
	row := tx.QueryRow(ctx,
		`SELECT id, quantity FROM medication_stock
		 WHERE branch_id = $1 AND warehouse_id = $2 AND medication_id = $3
		   AND lot_no = $4 AND (expiry_date IS NOT DISTINCT FROM $5)
		 FOR UPDATE`,
		in.BranchID, in.WarehouseID, in.MedicationID, in.LotNo, in.ExpiryDate)
	if err = row.Scan(&stockID, &oldQty); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLotNotFound
		}
		return nil, err
	}
	if oldQty < in.Quantity {
		return nil, ErrInsufficientStock
	}

	if _, err = tx.Exec(ctx,
		`UPDATE medication_stock SET quantity = quantity - $2, last_movement_at = NOW()
		 WHERE id = $1`, stockID, in.Quantity); err != nil {
		return nil, err
	}

	// Step 3: append stock_movement (issue).
	var nextNo int64
	if err = tx.QueryRow(ctx, `SELECT nextval('stock_movement_no_seq')`).Scan(&nextNo); err != nil {
		return nil, err
	}
	movementNo := util.FormatMRN(nextNo)

	var movementID uuid.UUID
	if err = tx.QueryRow(ctx,
		`INSERT INTO stock_movement
		   (organization_id, branch_id, movement_no, warehouse_id, medication_id,
		    lot_no, expiry_date, kind, quantity,
		    reference_type, reference_id, counterparty,
		    performed_by_user_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 'issue', $8, 'prescription_item', $9, $10, $11)
		 RETURNING id`,
		in.OrganizationID, in.BranchID, movementNo, in.WarehouseID, in.MedicationID,
		in.LotNo, in.ExpiryDate, in.Quantity,
		in.PrescriptionItemID, in.Counterparty, in.DispensedByUserID).Scan(&movementID); err != nil {
		return nil, err
	}

	// Step 4: insert prescription_dispense linked to the movement.
	var dispenseID uuid.UUID
	if err = tx.QueryRow(ctx,
		`INSERT INTO prescription_dispense
		   (organization_id, branch_id, prescription_item_id, warehouse_id, medication_id,
		    lot_no, expiry_date, quantity, stock_movement_id, dispensed_by_user_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id`,
		in.OrganizationID, in.BranchID, in.PrescriptionItemID, in.WarehouseID, in.MedicationID,
		in.LotNo, in.ExpiryDate, in.Quantity, movementID, in.DispensedByUserID).Scan(&dispenseID); err != nil {
		return nil, err
	}

	// Step 5: if the prescription_item didn't have medication_id yet, set it
	// so future dispensations default to the same catalog row.
	if ic.MedicationID == nil {
		if _, err = tx.Exec(ctx,
			`UPDATE prescription_item SET medication_id = $2 WHERE id = $1`,
			in.PrescriptionItemID, in.MedicationID); err != nil {
			return nil, err
		}
	}

	// Step 6: queue İTS notification (Sağlık Bakanlığı İlaç Takip Sistemi).
	// İlaç gerçek bir karekoda bağlandığında bakanlık dispense'i o karekodla
	// eşleştirir. Karekod henüz dispense sırasında girilmiyor — mock'ta
	// lot_no'dan türetilmiş bir tag kullanırız; üretimde stock_movement
	// veya medication tablosundaki karekoda göre güncellenecek.
	if _, err = tx.Exec(ctx,
		`INSERT INTO medula_outgoing_message
		   (organization_id, branch_id, message_type, target_table, target_id, payload)
		 VALUES ($1, $2, 'its_notify', 'prescription_dispense', $3, '{}'::JSONB)`,
		in.OrganizationID, in.BranchID, dispenseID); err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &DispenseResult{DispenseID: dispenseID, MovementNo: movementNo}, nil
}
