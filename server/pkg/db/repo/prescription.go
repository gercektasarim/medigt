package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Prescription struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	VisitID          uuid.UUID
	PatientID        uuid.UUID
	DoctorID         *uuid.UUID
	PrescriptionNo   string
	EPrescriptionNo  *string
	Status           string
	Notes            *string
	SignedAt         *time.Time
	SentToSGKAt      *time.Time
	DispensedAt      *time.Time
	CancelledAt      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type PrescriptionItem struct {
	ID              uuid.UUID
	PrescriptionID  uuid.UUID
	MedicationName  string
	Dosage          *string
	Frequency       *string
	DurationDays    *int
	Quantity        *string
	Instructions    *string
	SortOrder       int
	CreatedAt       time.Time
}

type PrescriptionWithItems struct {
	Prescription Prescription
	Items        []PrescriptionItem
}

type PrescriptionRepo struct {
	pool *pgxpool.Pool
}

func NewPrescriptionRepo(pool *pgxpool.Pool) *PrescriptionRepo {
	return &PrescriptionRepo{pool: pool}
}

const rxCols = `id, organization_id, visit_id, patient_id, doctor_id,
	prescription_no, e_prescription_no, status, notes,
	signed_at, sent_to_sgk_at, dispensed_at, cancelled_at,
	created_at, updated_at`

func scanRx(row pgx.Row) (*Prescription, error) {
	p := &Prescription{}
	err := row.Scan(&p.ID, &p.OrganizationID, &p.VisitID, &p.PatientID, &p.DoctorID,
		&p.PrescriptionNo, &p.EPrescriptionNo, &p.Status, &p.Notes,
		&p.SignedAt, &p.SentToSGKAt, &p.DispensedAt, &p.CancelledAt,
		&p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// NextNo reads the global prescription number sequence; caller zero-pads.
func (r *PrescriptionRepo) NextNo(ctx context.Context) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, `SELECT nextval('prescription_no_seq')`).Scan(&n)
	return n, err
}

type CreatePrescriptionInput struct {
	OrganizationID  uuid.UUID
	VisitID         uuid.UUID
	PatientID       uuid.UUID
	DoctorID        *uuid.UUID
	PrescriptionNo  string
	Notes           *string
	Items           []CreatePrescriptionItemInput
}

type CreatePrescriptionItemInput struct {
	MedicationName string
	Dosage         *string
	Frequency      *string
	DurationDays   *int
	Quantity       *string
	Instructions   *string
}

// Create writes the header + all items in a single tx. Status stays 'draft'.
func (r *PrescriptionRepo) Create(ctx context.Context, in CreatePrescriptionInput) (*PrescriptionWithItems, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx,
		`INSERT INTO prescription (organization_id, visit_id, patient_id, doctor_id,
		   prescription_no, notes)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+rxCols,
		in.OrganizationID, in.VisitID, in.PatientID, in.DoctorID,
		in.PrescriptionNo, in.Notes)
	p, err := scanRx(row)
	if err != nil {
		return nil, err
	}

	items := []PrescriptionItem{}
	for i, it := range in.Items {
		itemRow := tx.QueryRow(ctx,
			`INSERT INTO prescription_item (prescription_id, medication_name,
			   dosage, frequency, duration_days, quantity, instructions, sort_order)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 RETURNING id, prescription_id, medication_name, dosage, frequency,
			   duration_days, quantity, instructions, sort_order, created_at`,
			p.ID, it.MedicationName, it.Dosage, it.Frequency, it.DurationDays,
			it.Quantity, it.Instructions, i)
		x := PrescriptionItem{}
		if err := itemRow.Scan(&x.ID, &x.PrescriptionID, &x.MedicationName, &x.Dosage,
			&x.Frequency, &x.DurationDays, &x.Quantity, &x.Instructions,
			&x.SortOrder, &x.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, x)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &PrescriptionWithItems{Prescription: *p, Items: items}, nil
}

// ListForVisit returns all prescriptions on the visit + each one's items.
func (r *PrescriptionRepo) ListForVisit(ctx context.Context, visitID uuid.UUID) ([]PrescriptionWithItems, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+rxCols+` FROM prescription WHERE visit_id = $1 ORDER BY created_at ASC`,
		visitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	heads := []Prescription{}
	ids := []uuid.UUID{}
	for rows.Next() {
		p := Prescription{}
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.VisitID, &p.PatientID, &p.DoctorID,
			&p.PrescriptionNo, &p.EPrescriptionNo, &p.Status, &p.Notes,
			&p.SignedAt, &p.SentToSGKAt, &p.DispensedAt, &p.CancelledAt,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		heads = append(heads, p)
		ids = append(ids, p.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(heads) == 0 {
		return []PrescriptionWithItems{}, nil
	}

	itemsByRx := map[uuid.UUID][]PrescriptionItem{}
	itemRows, err := r.pool.Query(ctx,
		`SELECT id, prescription_id, medication_name, dosage, frequency,
		   duration_days, quantity, instructions, sort_order, created_at
		 FROM prescription_item
		 WHERE prescription_id = ANY($1)
		 ORDER BY prescription_id, sort_order`, ids)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()
	for itemRows.Next() {
		x := PrescriptionItem{}
		if err := itemRows.Scan(&x.ID, &x.PrescriptionID, &x.MedicationName, &x.Dosage,
			&x.Frequency, &x.DurationDays, &x.Quantity, &x.Instructions,
			&x.SortOrder, &x.CreatedAt); err != nil {
			return nil, err
		}
		itemsByRx[x.PrescriptionID] = append(itemsByRx[x.PrescriptionID], x)
	}

	out := make([]PrescriptionWithItems, 0, len(heads))
	for _, h := range heads {
		out = append(out, PrescriptionWithItems{Prescription: h, Items: itemsByRx[h.ID]})
	}
	return out, nil
}

// Sign without an attached e-imza row. Kept as a thin wrapper so existing
// callers don't change.
func (r *PrescriptionRepo) Sign(ctx context.Context, id uuid.UUID) error {
	return r.SignWithSignature(ctx, id, nil)
}

// SignWithSignature flips a draft prescription to 'signed' AND, optionally,
// links a verified digital_signature row. Aynı tx'te outbox'a
// 'erecete_submit' mesajı düşürülür — reçete imzalandıktan sonra Sağlık
// Bakanlığı e-Reçete sistemine otomatik gönderim için. signatureID nil ise
// digital_signature_id NULL kalır (e-imza zorunlu olmayan iş akışı).
func (r *PrescriptionRepo) SignWithSignature(ctx context.Context, id uuid.UUID, signatureID *uuid.UUID) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var orgID, visitID uuid.UUID
	if err = tx.QueryRow(ctx,
		`UPDATE prescription
		   SET status = 'signed', signed_at = NOW(),
		       e_prescription_status = 'queued',
		       digital_signature_id = COALESCE($2, digital_signature_id)
		 WHERE id = $1 AND status = 'draft'
		 RETURNING organization_id, visit_id`,
		id, signatureID).Scan(&orgID, &visitID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	var branchID *uuid.UUID
	_ = tx.QueryRow(ctx, `SELECT branch_id FROM visit WHERE id = $1`, visitID).Scan(&branchID)

	if _, err = tx.Exec(ctx,
		`INSERT INTO medula_outgoing_message
		   (organization_id, branch_id, message_type, target_table, target_id, payload)
		 VALUES ($1, $2, 'erecete_submit', 'prescription', $3, '{}'::JSONB)`,
		orgID, branchID, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
