package repo

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------- Catalog ----------

type LabTest struct {
	ID              uuid.UUID
	OrganizationID  *uuid.UUID
	Code            string
	LoincCode       *string
	SutCode         *string
	Name            string
	SampleType      string
	Unit            *string
	ReferenceRange  *string
	IsSystem        bool
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type LabRepo struct {
	pool *pgxpool.Pool
}

func NewLabRepo(pool *pgxpool.Pool) *LabRepo { return &LabRepo{pool: pool} }

const labTestCols = `id, organization_id, code, loinc_code, sut_code, name,
	sample_type, unit, reference_range, is_system, is_active, created_at, updated_at`

func scanLabTest(row pgx.Row) (*LabTest, error) {
	t := &LabTest{}
	err := row.Scan(&t.ID, &t.OrganizationID, &t.Code, &t.LoincCode, &t.SutCode,
		&t.Name, &t.SampleType, &t.Unit, &t.ReferenceRange,
		&t.IsSystem, &t.IsActive, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

func (r *LabRepo) SearchTests(ctx context.Context, orgID uuid.UUID, q string, limit int) ([]LabTest, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	q = strings.TrimSpace(q)
	args := []any{orgID, limit}
	sqlStr := `SELECT ` + labTestCols + ` FROM lab_test_catalog
	           WHERE (organization_id IS NULL OR organization_id = $1) AND is_active = TRUE`
	if q != "" {
		args = append(args, q+"%", "%"+q+"%")
		sqlStr += ` AND (code ILIKE $3 OR name ILIKE $4)`
	}
	sqlStr += ` ORDER BY (organization_id IS NULL) DESC, name LIMIT $2`

	rows, err := r.pool.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LabTest{}
	for rows.Next() {
		t := LabTest{}
		if err := rows.Scan(&t.ID, &t.OrganizationID, &t.Code, &t.LoincCode, &t.SutCode,
			&t.Name, &t.SampleType, &t.Unit, &t.ReferenceRange,
			&t.IsSystem, &t.IsActive, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *LabRepo) GetTestByID(ctx context.Context, id uuid.UUID) (*LabTest, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+labTestCols+` FROM lab_test_catalog WHERE id = $1`, id)
	return scanLabTest(row)
}

// ---------- Orders ----------

type LabOrder struct {
	ID                  uuid.UUID
	OrganizationID      uuid.UUID
	BranchID            uuid.UUID
	VisitID             *uuid.UUID
	PatientID           uuid.UUID
	OrderingDoctorID    *uuid.UUID
	OrderNo             string
	Status              string
	Priority            string
	ClinicalIndication  *string
	Notes               *string
	OrderedByUserID     *uuid.UUID
	OrderedAt           time.Time
	SampledAt           *time.Time
	SampledByUserID     *uuid.UUID
	CompletedAt         *time.Time
	CancelledAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type LabOrderItem struct {
	ID                   uuid.UUID
	LabOrderID           uuid.UUID
	LabTestCatalogID     uuid.UUID
	TestCode             string
	TestName             string
	SampleType           string
	Unit                 *string
	ReferenceRange       *string
	Status               string
	SortOrder            int
	ValueNumeric         *float64
	ValueText            *string
	Flag                 *string
	ResultedAt           *time.Time
	ResultedByUserID     *uuid.UUID
	VerifiedAt           *time.Time
	VerifiedByUserID     *uuid.UUID
	Notes                *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type LabOrderWithJoins struct {
	Order            LabOrder
	Items            []LabOrderItem
	PatientMRN       string
	PatientFirstName string
	PatientLastName  string
	DoctorFirstName  *string
	DoctorLastName   *string
	DoctorTitle      *string
}

const labOrderCols = `id, organization_id, branch_id, visit_id, patient_id,
	ordering_doctor_id, order_no, status, priority, clinical_indication, notes,
	ordered_by_user_id, ordered_at, sampled_at, sampled_by_user_id,
	completed_at, cancelled_at, created_at, updated_at`

const labOrderItemCols = `id, lab_order_id, lab_test_catalog_id, test_code,
	test_name, sample_type, unit, reference_range, status, sort_order,
	value_numeric, value_text, flag, resulted_at, resulted_by_user_id,
	verified_at, verified_by_user_id, notes, created_at, updated_at`

func (r *LabRepo) NextOrderNo(ctx context.Context) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, `SELECT nextval('lab_order_no_seq')`).Scan(&n)
	return n, err
}

type CreateLabOrderInput struct {
	OrganizationID     uuid.UUID
	BranchID           uuid.UUID
	VisitID            *uuid.UUID
	PatientID          uuid.UUID
	OrderingDoctorID   *uuid.UUID
	OrderedByUserID    *uuid.UUID
	OrderNo            string
	Priority           string
	ClinicalIndication *string
	Notes              *string
	TestCatalogIDs     []uuid.UUID
}

// Create writes the order header + one item per requested test in a single tx.
// Each item snapshots the catalog row's test code, name, sample type, unit,
// and reference range so historical results stay readable.
func (r *LabRepo) Create(ctx context.Context, in CreateLabOrderInput) (*LabOrderWithJoins, error) {
	if len(in.TestCatalogIDs) == 0 {
		return nil, errors.New("at least one test required")
	}
	if in.Priority == "" {
		in.Priority = "routine"
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx,
		`INSERT INTO lab_order (organization_id, branch_id, visit_id, patient_id,
		   ordering_doctor_id, order_no, priority, clinical_indication, notes,
		   ordered_by_user_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7::lab_order_priority,$8,$9,$10)
		 RETURNING `+labOrderCols,
		in.OrganizationID, in.BranchID, in.VisitID, in.PatientID, in.OrderingDoctorID,
		in.OrderNo, in.Priority, in.ClinicalIndication, in.Notes, in.OrderedByUserID)
	order := LabOrder{}
	if err := row.Scan(&order.ID, &order.OrganizationID, &order.BranchID, &order.VisitID,
		&order.PatientID, &order.OrderingDoctorID, &order.OrderNo, &order.Status,
		&order.Priority, &order.ClinicalIndication, &order.Notes,
		&order.OrderedByUserID, &order.OrderedAt, &order.SampledAt, &order.SampledByUserID,
		&order.CompletedAt, &order.CancelledAt, &order.CreatedAt, &order.UpdatedAt); err != nil {
		return nil, err
	}

	// Read the catalog rows for snapshot. ANY($1) keeps it a single round trip.
	catRows, err := tx.Query(ctx,
		`SELECT id, code, name, sample_type, unit, reference_range
		 FROM lab_test_catalog
		 WHERE id = ANY($1) AND is_active = TRUE`,
		in.TestCatalogIDs)
	if err != nil {
		return nil, err
	}
	type catSnap struct {
		id    uuid.UUID
		code  string
		name  string
		stype string
		unit  *string
		ref   *string
	}
	snaps := make(map[uuid.UUID]catSnap, len(in.TestCatalogIDs))
	for catRows.Next() {
		s := catSnap{}
		if err := catRows.Scan(&s.id, &s.code, &s.name, &s.stype, &s.unit, &s.ref); err != nil {
			catRows.Close()
			return nil, err
		}
		snaps[s.id] = s
	}
	catRows.Close()
	if err := catRows.Err(); err != nil {
		return nil, err
	}

	items := []LabOrderItem{}
	for i, tid := range in.TestCatalogIDs {
		s, ok := snaps[tid]
		if !ok {
			return nil, errors.New("unknown lab test in order")
		}
		itemRow := tx.QueryRow(ctx,
			`INSERT INTO lab_order_item (lab_order_id, lab_test_catalog_id,
			   test_code, test_name, sample_type, unit, reference_range, sort_order)
			 VALUES ($1, $2, $3, $4, $5::lab_sample_type, $6, $7, $8)
			 RETURNING `+labOrderItemCols,
			order.ID, s.id, s.code, s.name, s.stype, s.unit, s.ref, i)
		it := LabOrderItem{}
		if err := itemRow.Scan(&it.ID, &it.LabOrderID, &it.LabTestCatalogID, &it.TestCode,
			&it.TestName, &it.SampleType, &it.Unit, &it.ReferenceRange,
			&it.Status, &it.SortOrder, &it.ValueNumeric, &it.ValueText, &it.Flag,
			&it.ResultedAt, &it.ResultedByUserID, &it.VerifiedAt, &it.VerifiedByUserID,
			&it.Notes, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &LabOrderWithJoins{Order: order, Items: items}, nil
}

type ListLabOrderFilter struct {
	Status  string
	From    *time.Time
	To      *time.Time
	VisitID *uuid.UUID
	Limit   int
}

func (r *LabRepo) ListOrders(ctx context.Context, branchID uuid.UUID, f ListLabOrderFilter) ([]LabOrderWithJoins, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT ` + labOrderCols + `,
	             p.mrn, p.first_name, p.last_name,
	             ds.first_name, ds.last_name, ds.title
	      FROM lab_order
	      JOIN patient p ON p.id = lab_order.patient_id
	      LEFT JOIN doctor d ON d.id = lab_order.ordering_doctor_id
	      LEFT JOIN staff_member ds ON ds.id = d.staff_member_id
	      WHERE lab_order.branch_id = $1`
	args := []any{branchID}
	if f.Status != "" {
		args = append(args, f.Status)
		q += ` AND lab_order.status = $` + itoa(len(args)) + `::lab_order_status`
	}
	if f.VisitID != nil {
		args = append(args, *f.VisitID)
		q += ` AND lab_order.visit_id = $` + itoa(len(args))
	}
	if f.From != nil {
		args = append(args, *f.From)
		q += ` AND lab_order.ordered_at >= $` + itoa(len(args))
	}
	if f.To != nil {
		args = append(args, *f.To)
		q += ` AND lab_order.ordered_at < $` + itoa(len(args))
	}
	q += ` ORDER BY lab_order.ordered_at DESC LIMIT ` + itoa(limit)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type orderWithMeta struct {
		LabOrderWithJoins
	}

	heads := []LabOrderWithJoins{}
	ids := []uuid.UUID{}
	for rows.Next() {
		w := LabOrderWithJoins{}
		o := &w.Order
		if err := rows.Scan(
			&o.ID, &o.OrganizationID, &o.BranchID, &o.VisitID, &o.PatientID,
			&o.OrderingDoctorID, &o.OrderNo, &o.Status, &o.Priority,
			&o.ClinicalIndication, &o.Notes, &o.OrderedByUserID, &o.OrderedAt,
			&o.SampledAt, &o.SampledByUserID, &o.CompletedAt, &o.CancelledAt,
			&o.CreatedAt, &o.UpdatedAt,
			&w.PatientMRN, &w.PatientFirstName, &w.PatientLastName,
			&w.DoctorFirstName, &w.DoctorLastName, &w.DoctorTitle,
		); err != nil {
			return nil, err
		}
		heads = append(heads, w)
		ids = append(ids, o.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(heads) == 0 {
		return heads, nil
	}

	// Items for all heads in one extra round trip.
	itemRows, err := r.pool.Query(ctx,
		`SELECT `+labOrderItemCols+` FROM lab_order_item
		 WHERE lab_order_id = ANY($1)
		 ORDER BY lab_order_id, sort_order`, ids)
	if err != nil {
		return heads, nil
	}
	defer itemRows.Close()
	byOrder := map[uuid.UUID][]LabOrderItem{}
	for itemRows.Next() {
		it := LabOrderItem{}
		if err := itemRows.Scan(&it.ID, &it.LabOrderID, &it.LabTestCatalogID, &it.TestCode,
			&it.TestName, &it.SampleType, &it.Unit, &it.ReferenceRange,
			&it.Status, &it.SortOrder, &it.ValueNumeric, &it.ValueText, &it.Flag,
			&it.ResultedAt, &it.ResultedByUserID, &it.VerifiedAt, &it.VerifiedByUserID,
			&it.Notes, &it.CreatedAt, &it.UpdatedAt); err != nil {
			continue
		}
		byOrder[it.LabOrderID] = append(byOrder[it.LabOrderID], it)
	}
	for i := range heads {
		heads[i].Items = byOrder[heads[i].Order.ID]
	}
	return heads, nil
}

func (r *LabRepo) GetOrderWithItems(ctx context.Context, branchID, orderID uuid.UUID) (*LabOrderWithJoins, error) {
	heads, err := r.ListOrders(ctx, branchID, ListLabOrderFilter{Limit: 1})
	_ = heads
	_ = err

	// Direct fetch (simpler, narrower).
	row := r.pool.QueryRow(ctx,
		`SELECT `+labOrderCols+`,
		        p.mrn, p.first_name, p.last_name,
		        ds.first_name, ds.last_name, ds.title
		 FROM lab_order
		 JOIN patient p ON p.id = lab_order.patient_id
		 LEFT JOIN doctor d ON d.id = lab_order.ordering_doctor_id
		 LEFT JOIN staff_member ds ON ds.id = d.staff_member_id
		 WHERE lab_order.branch_id = $1 AND lab_order.id = $2`,
		branchID, orderID)

	w := LabOrderWithJoins{}
	o := &w.Order
	if err := row.Scan(
		&o.ID, &o.OrganizationID, &o.BranchID, &o.VisitID, &o.PatientID,
		&o.OrderingDoctorID, &o.OrderNo, &o.Status, &o.Priority,
		&o.ClinicalIndication, &o.Notes, &o.OrderedByUserID, &o.OrderedAt,
		&o.SampledAt, &o.SampledByUserID, &o.CompletedAt, &o.CancelledAt,
		&o.CreatedAt, &o.UpdatedAt,
		&w.PatientMRN, &w.PatientFirstName, &w.PatientLastName,
		&w.DoctorFirstName, &w.DoctorLastName, &w.DoctorTitle,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	itemRows, err := r.pool.Query(ctx,
		`SELECT `+labOrderItemCols+` FROM lab_order_item
		 WHERE lab_order_id = $1 ORDER BY sort_order`, orderID)
	if err != nil {
		return &w, nil
	}
	defer itemRows.Close()
	for itemRows.Next() {
		it := LabOrderItem{}
		if err := itemRows.Scan(&it.ID, &it.LabOrderID, &it.LabTestCatalogID, &it.TestCode,
			&it.TestName, &it.SampleType, &it.Unit, &it.ReferenceRange,
			&it.Status, &it.SortOrder, &it.ValueNumeric, &it.ValueText, &it.Flag,
			&it.ResultedAt, &it.ResultedByUserID, &it.VerifiedAt, &it.VerifiedByUserID,
			&it.Notes, &it.CreatedAt, &it.UpdatedAt); err != nil {
			continue
		}
		w.Items = append(w.Items, it)
	}
	return &w, nil
}

func (r *LabRepo) UpdateOrderStatus(ctx context.Context, branchID, orderID uuid.UUID, status string, byUserID *uuid.UUID) error {
	stampCol := ""
	switch status {
	case "sampled":
		stampCol = "sampled_at = NOW(), sampled_by_user_id = $4"
	case "verified":
		stampCol = "completed_at = NOW()"
	case "cancelled":
		stampCol = "cancelled_at = NOW()"
	}
	q := `UPDATE lab_order SET status = $3::lab_order_status`
	if stampCol != "" {
		q += `, ` + stampCol
	}
	q += ` WHERE branch_id = $1 AND id = $2`
	args := []any{branchID, orderID, status}
	if status == "sampled" {
		args = append(args, byUserID)
	}
	res, err := r.pool.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type UpdateItemResultInput struct {
	ValueNumeric     *float64
	ValueText        *string
	Flag             *string
	Notes            *string
	ResultedByUserID *uuid.UUID
}

func (r *LabRepo) UpdateItemResult(ctx context.Context, itemID uuid.UUID, in UpdateItemResultInput) (*LabOrderItem, error) {
	// Atomically write the result + flip the item status. Also bubble the
	// parent order's status from 'ordered'/'sampled'/'in_progress' to
	// 'resulted' once any item has a value.
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx,
		`UPDATE lab_order_item
		 SET value_numeric = $2, value_text = $3, flag = $4::lab_result_flag,
		     notes = $5, status = 'resulted',
		     resulted_at = NOW(), resulted_by_user_id = $6
		 WHERE id = $1
		 RETURNING `+labOrderItemCols,
		itemID, in.ValueNumeric, in.ValueText, in.Flag, in.Notes, in.ResultedByUserID)
	it := LabOrderItem{}
	if err := row.Scan(&it.ID, &it.LabOrderID, &it.LabTestCatalogID, &it.TestCode,
		&it.TestName, &it.SampleType, &it.Unit, &it.ReferenceRange,
		&it.Status, &it.SortOrder, &it.ValueNumeric, &it.ValueText, &it.Flag,
		&it.ResultedAt, &it.ResultedByUserID, &it.VerifiedAt, &it.VerifiedByUserID,
		&it.Notes, &it.CreatedAt, &it.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Bump parent order status if it isn't already 'resulted'/'verified'.
	if _, err := tx.Exec(ctx,
		`UPDATE lab_order
		 SET status = 'resulted'
		 WHERE id = $1 AND status IN ('ordered', 'sampled', 'in_progress')`,
		it.LabOrderID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &it, nil
}
