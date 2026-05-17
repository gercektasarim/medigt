package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------- Medication order ----------

type MedicationOrder struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	BranchID          uuid.UUID
	OrderNo           string
	AdmissionID       uuid.UUID
	PatientID         uuid.UUID
	MedicationID      uuid.UUID
	OrderingDoctorID  *uuid.UUID
	DoseAmount        float64
	DoseUnit          string
	Route             string
	Frequency         string
	ScheduledTimes    []string // "HH:MM" strings
	IsPRN             bool
	PRNReason         *string
	StartsAt          time.Time
	EndsAt            *time.Time
	Instructions      *string
	Status            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	// Joined fields populated by ListForAdmission etc.:
	MedicationName    string
	MedicationCode    *string
	DoctorFirstName   *string
	DoctorLastName    *string
}

type MARRepo struct{ pool *pgxpool.Pool }

func NewMARRepo(pool *pgxpool.Pool) *MARRepo { return &MARRepo{pool: pool} }

const medOrderCols = `mo.id, mo.organization_id, mo.branch_id, mo.order_no,
	mo.admission_id, mo.patient_id, mo.medication_id, mo.ordering_doctor_id,
	mo.dose_amount, mo.dose_unit, mo.route::text, mo.frequency,
	mo.scheduled_times, mo.is_prn, mo.prn_reason,
	mo.starts_at, mo.ends_at, mo.instructions, mo.status::text,
	mo.created_at, mo.updated_at`

func scanMedOrder(row pgx.Row) (*MedicationOrder, error) {
	o := &MedicationOrder{}
	var sched []string
	err := row.Scan(&o.ID, &o.OrganizationID, &o.BranchID, &o.OrderNo,
		&o.AdmissionID, &o.PatientID, &o.MedicationID, &o.OrderingDoctorID,
		&o.DoseAmount, &o.DoseUnit, &o.Route, &o.Frequency,
		&sched, &o.IsPRN, &o.PRNReason,
		&o.StartsAt, &o.EndsAt, &o.Instructions, &o.Status,
		&o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	o.ScheduledTimes = sched
	return o, err
}

type CreateMedicationOrderInput struct {
	OrganizationID   uuid.UUID
	BranchID         uuid.UUID
	AdmissionID      uuid.UUID
	PatientID        uuid.UUID
	MedicationID     uuid.UUID
	OrderingDoctorID *uuid.UUID
	DoseAmount       float64
	DoseUnit         string
	Route            string
	Frequency        string
	ScheduledTimes   []string
	IsPRN            bool
	PRNReason        *string
	StartsAt         time.Time
	EndsAt           *time.Time
	Instructions     *string
}

func (r *MARRepo) CreateOrder(ctx context.Context, in CreateMedicationOrderInput) (*MedicationOrder, error) {
	var nextNo int64
	if err := r.pool.QueryRow(ctx, `SELECT nextval('medication_order_no_seq')`).Scan(&nextNo); err != nil {
		return nil, err
	}
	orderNo := "MO-" + intToPaddedString(nextNo)

	sched := in.ScheduledTimes
	if sched == nil {
		sched = []string{}
	}

	row := r.pool.QueryRow(ctx,
		`INSERT INTO medication_order
		   (organization_id, branch_id, order_no, admission_id, patient_id,
		    medication_id, ordering_doctor_id, dose_amount, dose_unit, route,
		    frequency, scheduled_times, is_prn, prn_reason,
		    starts_at, ends_at, instructions)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::medication_route,
		         $11, $12::TIME[], $13, $14, $15, $16, $17)
		 RETURNING `+medOrderColsForReturning(),
		in.OrganizationID, in.BranchID, orderNo, in.AdmissionID, in.PatientID,
		in.MedicationID, in.OrderingDoctorID, in.DoseAmount, in.DoseUnit, in.Route,
		in.Frequency, sched, in.IsPRN, in.PRNReason,
		in.StartsAt, in.EndsAt, in.Instructions)
	return scanMedOrderReturning(row)
}

// medOrderColsForReturning strips the `mo.` alias prefix for RETURNING use.
func medOrderColsForReturning() string {
	return `id, organization_id, branch_id, order_no,
	admission_id, patient_id, medication_id, ordering_doctor_id,
	dose_amount, dose_unit, route::text, frequency,
	scheduled_times, is_prn, prn_reason,
	starts_at, ends_at, instructions, status::text,
	created_at, updated_at`
}

func scanMedOrderReturning(row pgx.Row) (*MedicationOrder, error) {
	o := &MedicationOrder{}
	var sched []string
	err := row.Scan(&o.ID, &o.OrganizationID, &o.BranchID, &o.OrderNo,
		&o.AdmissionID, &o.PatientID, &o.MedicationID, &o.OrderingDoctorID,
		&o.DoseAmount, &o.DoseUnit, &o.Route, &o.Frequency,
		&sched, &o.IsPRN, &o.PRNReason,
		&o.StartsAt, &o.EndsAt, &o.Instructions, &o.Status,
		&o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	o.ScheduledTimes = sched
	return o, err
}

// ListForAdmission returns active + recently inactive orders for a given
// admission, joined with medication name + doctor name (snapshot).
func (r *MARRepo) ListForAdmission(ctx context.Context, admissionID uuid.UUID) ([]MedicationOrder, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+medOrderCols+`,
		        m.name AS med_name, m.code AS med_code,
		        sm.first_name, sm.last_name
		 FROM medication_order mo
		 JOIN medication m ON m.id = mo.medication_id
		 LEFT JOIN doctor d ON d.id = mo.ordering_doctor_id
		 LEFT JOIN staff_member sm ON sm.id = d.staff_member_id
		 WHERE mo.admission_id = $1
		 ORDER BY mo.status = 'active' DESC, mo.starts_at DESC`, admissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MedicationOrder{}
	for rows.Next() {
		o := MedicationOrder{}
		var sched []string
		var medCode *string
		if err := rows.Scan(&o.ID, &o.OrganizationID, &o.BranchID, &o.OrderNo,
			&o.AdmissionID, &o.PatientID, &o.MedicationID, &o.OrderingDoctorID,
			&o.DoseAmount, &o.DoseUnit, &o.Route, &o.Frequency,
			&sched, &o.IsPRN, &o.PRNReason,
			&o.StartsAt, &o.EndsAt, &o.Instructions, &o.Status,
			&o.CreatedAt, &o.UpdatedAt,
			&o.MedicationName, &medCode,
			&o.DoctorFirstName, &o.DoctorLastName); err != nil {
			return nil, err
		}
		o.ScheduledTimes = sched
		o.MedicationCode = medCode
		out = append(out, o)
	}
	return out, rows.Err()
}

// UpdateOrderStatus is used for hold / complete / cancel transitions.
func (r *MARRepo) UpdateOrderStatus(ctx context.Context, id uuid.UUID, status string) error {
	res, err := r.pool.Exec(ctx,
		`UPDATE medication_order SET status = $2::medication_order_status WHERE id = $1`,
		id, status)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- Administration record ----------

type MedicationAdministration struct {
	ID                       uuid.UUID
	OrganizationID           uuid.UUID
	BranchID                 uuid.UUID
	MedicationOrderID        uuid.UUID
	AdmissionID              uuid.UUID
	PatientID                uuid.UUID
	ScheduledAt              *time.Time
	AdministeredAt           time.Time
	Status                   string
	FiveRightsChecked        bool
	PatientBarcodeScanned    *string
	MedicationBarcodeScanned *string
	DoseAmount               *float64
	DoseUnit                 *string
	Route                    *string
	Notes                    *string
	PerformedByUserID        *uuid.UUID
	WitnessedByUserID        *uuid.UUID
	CreatedAt                time.Time
}

type RecordAdministrationInput struct {
	OrganizationID           uuid.UUID
	BranchID                 uuid.UUID
	MedicationOrderID        uuid.UUID
	ScheduledAt              *time.Time
	AdministeredAt           time.Time
	Status                   string
	FiveRightsChecked        bool
	PatientBarcodeScanned    *string
	MedicationBarcodeScanned *string
	DoseAmount               *float64
	DoseUnit                 *string
	Route                    *string
	Notes                    *string
	PerformedByUserID        *uuid.UUID
	WitnessedByUserID        *uuid.UUID
}

const adminCols = `ma.id, ma.organization_id, ma.branch_id, ma.medication_order_id,
	ma.admission_id, ma.patient_id, ma.scheduled_at, ma.administered_at,
	ma.status::text, ma.five_rights_checked,
	ma.patient_barcode_scanned, ma.medication_barcode_scanned,
	ma.dose_amount, ma.dose_unit, ma.route::text, ma.notes,
	ma.performed_by_user_id, ma.witnessed_by_user_id, ma.created_at`

func scanAdmin(row pgx.Row) (*MedicationAdministration, error) {
	a := &MedicationAdministration{}
	err := row.Scan(&a.ID, &a.OrganizationID, &a.BranchID, &a.MedicationOrderID,
		&a.AdmissionID, &a.PatientID, &a.ScheduledAt, &a.AdministeredAt,
		&a.Status, &a.FiveRightsChecked,
		&a.PatientBarcodeScanned, &a.MedicationBarcodeScanned,
		&a.DoseAmount, &a.DoseUnit, &a.Route, &a.Notes,
		&a.PerformedByUserID, &a.WitnessedByUserID, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

// RecordAdministration creates a single administration row. The row is
// the source of truth for what was actually given; the order remains
// the prescription.
//
// Service layer enforces the 5-rights check before status='given'.
func (r *MARRepo) RecordAdministration(ctx context.Context, in RecordAdministrationInput) (*MedicationAdministration, error) {
	// Pull admission_id + patient_id from the order to avoid trusting the
	// caller and to ensure foreign-key integrity.
	var admissionID, patientID uuid.UUID
	if err := r.pool.QueryRow(ctx,
		`SELECT admission_id, patient_id FROM medication_order WHERE id = $1`,
		in.MedicationOrderID).Scan(&admissionID, &patientID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	var route any = nil
	if in.Route != nil && *in.Route != "" {
		route = *in.Route
	}

	row := r.pool.QueryRow(ctx,
		`INSERT INTO medication_administration
		   (organization_id, branch_id, medication_order_id, admission_id, patient_id,
		    scheduled_at, administered_at, status, five_rights_checked,
		    patient_barcode_scanned, medication_barcode_scanned,
		    dose_amount, dose_unit, route, notes,
		    performed_by_user_id, witnessed_by_user_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8::administration_status, $9,
		         $10, $11, $12, $13, $14::medication_route, $15, $16, $17)
		 RETURNING `+adminColsForReturning(),
		in.OrganizationID, in.BranchID, in.MedicationOrderID, admissionID, patientID,
		in.ScheduledAt, in.AdministeredAt, in.Status, in.FiveRightsChecked,
		in.PatientBarcodeScanned, in.MedicationBarcodeScanned,
		in.DoseAmount, in.DoseUnit, route, in.Notes,
		in.PerformedByUserID, in.WitnessedByUserID)
	return scanAdminReturning(row)
}

func adminColsForReturning() string {
	return `id, organization_id, branch_id, medication_order_id,
	admission_id, patient_id, scheduled_at, administered_at,
	status::text, five_rights_checked,
	patient_barcode_scanned, medication_barcode_scanned,
	dose_amount, dose_unit, route::text, notes,
	performed_by_user_id, witnessed_by_user_id, created_at`
}

func scanAdminReturning(row pgx.Row) (*MedicationAdministration, error) {
	a := &MedicationAdministration{}
	err := row.Scan(&a.ID, &a.OrganizationID, &a.BranchID, &a.MedicationOrderID,
		&a.AdmissionID, &a.PatientID, &a.ScheduledAt, &a.AdministeredAt,
		&a.Status, &a.FiveRightsChecked,
		&a.PatientBarcodeScanned, &a.MedicationBarcodeScanned,
		&a.DoseAmount, &a.DoseUnit, &a.Route, &a.Notes,
		&a.PerformedByUserID, &a.WitnessedByUserID, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

// ListAdministrationsForOrder returns the chronological dose history of
// a given order — used to render the MAR row for one drug.
func (r *MARRepo) ListAdministrationsForOrder(ctx context.Context, orderID uuid.UUID) ([]MedicationAdministration, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+adminCols+`
		 FROM medication_administration ma
		 WHERE ma.medication_order_id = $1
		 ORDER BY ma.administered_at DESC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MedicationAdministration{}
	for rows.Next() {
		a := MedicationAdministration{}
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.BranchID, &a.MedicationOrderID,
			&a.AdmissionID, &a.PatientID, &a.ScheduledAt, &a.AdministeredAt,
			&a.Status, &a.FiveRightsChecked,
			&a.PatientBarcodeScanned, &a.MedicationBarcodeScanned,
			&a.DoseAmount, &a.DoseUnit, &a.Route, &a.Notes,
			&a.PerformedByUserID, &a.WitnessedByUserID, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListAdministrationsForAdmission — used by the per-admission MAR sheet to
// build a unified timeline across all drugs in a single query.
func (r *MARRepo) ListAdministrationsForAdmission(ctx context.Context, admissionID uuid.UUID) ([]MedicationAdministration, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+adminCols+`
		 FROM medication_administration ma
		 WHERE ma.admission_id = $1
		 ORDER BY ma.administered_at DESC LIMIT 500`, admissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MedicationAdministration{}
	for rows.Next() {
		a := MedicationAdministration{}
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.BranchID, &a.MedicationOrderID,
			&a.AdmissionID, &a.PatientID, &a.ScheduledAt, &a.AdministeredAt,
			&a.Status, &a.FiveRightsChecked,
			&a.PatientBarcodeScanned, &a.MedicationBarcodeScanned,
			&a.DoseAmount, &a.DoseUnit, &a.Route, &a.Notes,
			&a.PerformedByUserID, &a.WitnessedByUserID, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// intToPaddedString — kasa / fatura / order_no helpers elsewhere use
// util.FormatMRN; we replicate the zero-padded format locally to keep this
// repo file dependency-light.
func intToPaddedString(n int64) string {
	s := ""
	if n == 0 {
		return "00000"
	}
	for n > 0 {
		s = string(rune('0'+(n%10))) + s
		n /= 10
	}
	for len(s) < 5 {
		s = "0" + s
	}
	return s
}
