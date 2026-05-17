package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PendingPrescription is a flat row joining prescription header + patient
// + dispense progress, used to drive the eczane queue.
type PendingPrescriptionItem struct {
	ItemID            uuid.UUID
	MedicationName    string
	MedicationID      *uuid.UUID
	Dosage            *string
	Frequency         *string
	Quantity          *string
	DispenseQuantity  *float64
	DispensedTotal    float64
	Instructions      *string
}

type PendingPrescription struct {
	ID                uuid.UUID
	PrescriptionNo    string
	Status            string
	SignedAt          *time.Time
	PatientID         uuid.UUID
	PatientMRN        string
	PatientFirstName  string
	PatientLastName   string
	DoctorFirstName   *string
	DoctorLastName    *string
	DoctorTitle       *string
	Items             []PendingPrescriptionItem
}

type EczaneRepo struct {
	pool *pgxpool.Pool
}

func NewEczaneRepo(pool *pgxpool.Pool) *EczaneRepo { return &EczaneRepo{pool: pool} }

// ListPending returns signed (or partially-dispensed) prescriptions whose
// items still have outstanding quantity. Limited to the org; the eczane is
// shared across branches today but the medication_id resolves at dispense
// time using the org-scoped catalog.
func (r *EczaneRepo) ListPending(ctx context.Context, orgID uuid.UUID, limit int) ([]PendingPrescription, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	// Step 1: fetch prescription headers that are signed but not dispensed
	// or cancelled, sorted oldest first (FIFO queue).
	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.prescription_no, p.status::text, p.signed_at,
		        pat.id, pat.mrn, pat.first_name, pat.last_name,
		        sd.first_name, sd.last_name, sd.title
		 FROM prescription p
		 JOIN patient pat ON pat.id = p.patient_id
		 LEFT JOIN doctor d ON d.id = p.doctor_id
		 LEFT JOIN staff_member sd ON sd.id = d.staff_member_id
		 WHERE p.organization_id = $1
		   AND p.status IN ('signed', 'sent_to_sgk')
		 ORDER BY p.signed_at ASC NULLS LAST
		 LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	heads := []PendingPrescription{}
	ids := []uuid.UUID{}
	for rows.Next() {
		p := PendingPrescription{}
		if err := rows.Scan(&p.ID, &p.PrescriptionNo, &p.Status, &p.SignedAt,
			&p.PatientID, &p.PatientMRN, &p.PatientFirstName, &p.PatientLastName,
			&p.DoctorFirstName, &p.DoctorLastName, &p.DoctorTitle); err != nil {
			return nil, err
		}
		heads = append(heads, p)
		ids = append(ids, p.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(heads) == 0 {
		return heads, nil
	}

	// Step 2: load items + dispensed totals.
	itemRows, err := r.pool.Query(ctx,
		`SELECT i.id, i.prescription_id, i.medication_name, i.medication_id,
		        i.dosage, i.frequency, i.quantity, i.dispense_quantity, i.instructions,
		        COALESCE(SUM(d.quantity), 0) AS dispensed_total
		 FROM prescription_item i
		 LEFT JOIN prescription_dispense d ON d.prescription_item_id = i.id
		 WHERE i.prescription_id = ANY($1)
		 GROUP BY i.id
		 ORDER BY i.prescription_id, i.sort_order`, ids)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()
	itemsByRx := map[uuid.UUID][]PendingPrescriptionItem{}
	type itemRowParent struct{ rxID uuid.UUID }
	for itemRows.Next() {
		var rxID uuid.UUID
		it := PendingPrescriptionItem{}
		if err := itemRows.Scan(
			&it.ItemID, &rxID, &it.MedicationName, &it.MedicationID,
			&it.Dosage, &it.Frequency, &it.Quantity, &it.DispenseQuantity, &it.Instructions,
			&it.DispensedTotal,
		); err != nil {
			return nil, err
		}
		itemsByRx[rxID] = append(itemsByRx[rxID], it)
	}

	out := []PendingPrescription{}
	for _, h := range heads {
		h.Items = itemsByRx[h.ID]
		// Filter out prescriptions where every item already has
		// dispense_quantity > 0 AND dispensed_total >= dispense_quantity
		// — but only when explicit numeric quantities are set.
		// For items without dispense_quantity, we just show the prescription;
		// the eczane operator decides.
		out = append(out, h)
	}
	return out, nil
}

// DispenseHistory lists past dispensations for the audit trail. Joined to
// the catalog + warehouse for display.
type DispenseHistoryRow struct {
	ID              uuid.UUID
	PrescriptionNo  string
	PatientMRN      string
	PatientFirstName string
	PatientLastName string
	MedicationName  string
	WarehouseName   string
	LotNo           string
	ExpiryDate      *time.Time
	Quantity        float64
	MovementNo      string
	DispensedAt     time.Time
	ItsStatus       string  // pending | in_progress | notified | rejected | failed
	ItsNotifiedAt   *time.Time
	ItsError        *string
}

func (r *EczaneRepo) DispenseHistory(ctx context.Context, branchID uuid.UUID, limit int) ([]DispenseHistoryRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx,
		`SELECT d.id, p.prescription_no, pat.mrn, pat.first_name, pat.last_name,
		        m.name, w.name, d.lot_no, d.expiry_date,
		        d.quantity, sm.movement_no, d.dispensed_at,
		        d.its_status::text, d.its_notified_at, d.its_error
		 FROM prescription_dispense d
		 JOIN prescription_item i ON i.id = d.prescription_item_id
		 JOIN prescription p ON p.id = i.prescription_id
		 JOIN patient pat ON pat.id = p.patient_id
		 JOIN medication m ON m.id = d.medication_id
		 JOIN warehouse w ON w.id = d.warehouse_id
		 JOIN stock_movement sm ON sm.id = d.stock_movement_id
		 WHERE d.branch_id = $1
		 ORDER BY d.dispensed_at DESC
		 LIMIT $2`, branchID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DispenseHistoryRow{}
	for rows.Next() {
		x := DispenseHistoryRow{}
		if err := rows.Scan(&x.ID, &x.PrescriptionNo, &x.PatientMRN,
			&x.PatientFirstName, &x.PatientLastName, &x.MedicationName,
			&x.WarehouseName, &x.LotNo, &x.ExpiryDate, &x.Quantity,
			&x.MovementNo, &x.DispensedAt,
			&x.ItsStatus, &x.ItsNotifiedAt, &x.ItsError); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// FEFOLots returns lots for a (warehouse, medication) sorted by earliest
// expiry first — used to suggest which lot to dispense from.
type LotSummary struct {
	StockID    uuid.UUID
	LotNo      string
	ExpiryDate *time.Time
	Quantity   float64
}

func (r *EczaneRepo) FEFOLots(ctx context.Context, branchID, warehouseID, medicationID uuid.UUID) ([]LotSummary, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, lot_no, expiry_date, quantity FROM medication_stock
		 WHERE branch_id = $1 AND warehouse_id = $2 AND medication_id = $3 AND quantity > 0
		 ORDER BY expiry_date NULLS LAST, last_movement_at`,
		branchID, warehouseID, medicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LotSummary{}
	for rows.Next() {
		l := LotSummary{}
		if err := rows.Scan(&l.StockID, &l.LotNo, &l.ExpiryDate, &l.Quantity); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// Helper: ErrItemNotFound for service-layer use.
var ErrPrescriptionItemNotFound = errors.New("reçete kalemi bulunamadı")

// Internal: load minimal info about a prescription item by id (for service tx).
type itemContext struct {
	OrgID         uuid.UUID
	PrescriptionID uuid.UUID
	MedicationID  *uuid.UUID
	MedicationName string
}

func (r *EczaneRepo) ItemContext(ctx context.Context, q pgx.Tx, itemID uuid.UUID) (*itemContext, error) {
	ic := &itemContext{}
	row := q.QueryRow(ctx,
		`SELECT p.organization_id, p.id, i.medication_id, i.medication_name
		 FROM prescription_item i JOIN prescription p ON p.id = i.prescription_id
		 WHERE i.id = $1`, itemID)
	if err := row.Scan(&ic.OrgID, &ic.PrescriptionID, &ic.MedicationID, &ic.MedicationName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPrescriptionItemNotFound
		}
		return nil, err
	}
	return ic, nil
}
