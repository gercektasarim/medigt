package handler

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------- Generic report runner ----------
//
// Each report is a Go function returning a ReportResult. The HTTP handler
// looks the function up by id, parses params from the query string, and
// runs it. The frontend registry mirrors the shape so 1 motor renders 200
// reports (per plan: "200 ayrı sayfa YAZMAYACAĞIZ").

type ReportColumn struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Type  string `json:"type"` // text | number | currency | date | datetime | pct
	Align string `json:"align,omitempty"`
}

type ReportResult struct {
	Columns []ReportColumn   `json:"columns"`
	Rows    []map[string]any `json:"rows"`
	Summary map[string]any   `json:"summary,omitempty"`
}

type reportRunner func(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error)

var registeredReports = map[string]reportRunner{
	// Finans (4)
	"daily-cash":           runDailyCash,
	"doctor-revenue":       runDoctorRevenue,
	"institution-revenue":  runInstitutionRevenue,
	"unpaid-invoices":      runUnpaidInvoices,
	"payment-mix":          runPaymentMix,
	"hourly-collection":    runHourlyCollection,
	"cashier-collection":   runCashierCollection,
	"open-advances":        runOpenAdvances,
	// Klinik (5)
	"outpatient-by-doctor": runOutpatientByDoctor,
	"diagnosis-distribution": runDiagnosisDistribution,
	"polyclinic-by-hour":   runPolyclinicByHour,
	"lab-volume":           runLabVolume,
	"lab-test-volume":      runLabTestVolume,
	// Yatış (2)
	"bed-occupancy":          runBedOccupancy,
	"ward-admission-stats":   runWardAdmissionStats,
	// Stok / İlaç (3)
	"low-expiry-stock":   runLowExpiryStock,
	"stock-valuation":    runStockValuation,
	"top-medications":    runTopMedications,
	// Ameliyat (1)
	"surgeon-performance":   runSurgeonPerformance,
	// Medula (1)
	"medula-success-rate":   runMedulaSuccessRate,
}

func (h *Handler) runReport(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id := chi.URLParam(r, "id")
	fn, ok := registeredReports[id]
	if !ok {
		writeError(w, http.StatusNotFound, "report_not_found", "rapor bulunamadı: "+id)
		return
	}
	res, err := fn(r.Context(), h.deps.Pool, branchID, r.URL.Query())
	if err != nil {
		h.deps.Log.Error("report run failed", "report", id, "err", err)
		writeError(w, http.StatusInternalServerError, "report_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ---------- Helpers ----------

// rangeFromQuery returns [from, to) covering whole days; defaults to today.
func rangeFromQuery(q url.Values) (time.Time, time.Time, error) {
	from := q.Get("from")
	to := q.Get("to")
	if from == "" && to == "" {
		now := time.Now()
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
		return start, start.Add(24 * time.Hour), nil
	}
	f, err := time.Parse("2006-01-02", from)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("from YYYY-MM-DD olmalı")
	}
	t, err := time.Parse("2006-01-02", to)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("to YYYY-MM-DD olmalı")
	}
	fStart := time.Date(f.Year(), f.Month(), f.Day(), 0, 0, 0, 0, time.Local)
	tEnd := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local).Add(24 * time.Hour)
	return fStart, tEnd, nil
}

// ---------- Report: daily-cash ----------
//
// Günlük kasa özeti: tarih aralığında kayıtlı her cash_register oturumu için
// kasiyer adı + açılış/sayım/fark + nakit gelir/gider toplamları.

func runDailyCash(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	from, to, err := rangeFromQuery(q)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx,
		`SELECT cr.id, cr.register_no, cr.cashier_name, cr.status::text,
		        cr.opening_balance, cr.declared_balance,
		        cr.opened_at, cr.closed_at,
		        COALESCE(SUM(CASE WHEN cm.kind = 'income' AND cm.method = 'cash' THEN cm.amount ELSE 0 END), 0) AS cash_income,
		        COALESCE(SUM(CASE WHEN cm.kind = 'expense' AND cm.method = 'cash' THEN cm.amount ELSE 0 END), 0) AS cash_expense,
		        COALESCE(SUM(CASE WHEN cm.kind = 'refund' AND cm.method = 'cash' THEN cm.amount ELSE 0 END), 0) AS cash_refund
		 FROM cash_register cr
		 LEFT JOIN cash_movement cm ON cm.cash_register_id = cr.id
		 WHERE cr.branch_id = $1
		   AND cr.opened_at >= $2 AND cr.opened_at < $3
		 GROUP BY cr.id
		 ORDER BY cr.opened_at DESC`,
		branchID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "register_no", Label: "Kasa No", Type: "text"},
			{Key: "cashier_name", Label: "Kasiyer", Type: "text"},
			{Key: "status", Label: "Durum", Type: "text"},
			{Key: "opening_balance", Label: "Açılış", Type: "currency", Align: "right"},
			{Key: "cash_income", Label: "Tahsilat (₺)", Type: "currency", Align: "right"},
			{Key: "cash_expense", Label: "Gider (₺)", Type: "currency", Align: "right"},
			{Key: "cash_refund", Label: "İade (₺)", Type: "currency", Align: "right"},
			{Key: "expected", Label: "Beklenen Kasa", Type: "currency", Align: "right"},
			{Key: "declared_balance", Label: "Sayım", Type: "currency", Align: "right"},
			{Key: "variance", Label: "Fark", Type: "currency", Align: "right"},
			{Key: "opened_at", Label: "Açılış", Type: "datetime"},
			{Key: "closed_at", Label: "Kapanış", Type: "datetime"},
		},
		Rows:    []map[string]any{},
		Summary: map[string]any{},
	}
	var totIncome, totExpense, totRefund float64
	for rows.Next() {
		var id uuid.UUID
		var regNo, cashier, status string
		var opening float64
		var declared *float64
		var openedAt time.Time
		var closedAt *time.Time
		var income, expense, refund float64
		if err := rows.Scan(&id, &regNo, &cashier, &status, &opening, &declared,
			&openedAt, &closedAt, &income, &expense, &refund); err != nil {
			return nil, err
		}
		expected := opening + income - expense - refund
		var variance *float64
		if declared != nil {
			v := *declared - expected
			variance = &v
		}
		out.Rows = append(out.Rows, map[string]any{
			"register_no":      regNo,
			"cashier_name":     cashier,
			"status":           status,
			"opening_balance":  opening,
			"cash_income":      income,
			"cash_expense":     expense,
			"cash_refund":      refund,
			"expected":         expected,
			"declared_balance": declared,
			"variance":         variance,
			"opened_at":        openedAt,
			"closed_at":        closedAt,
		})
		totIncome += income
		totExpense += expense
		totRefund += refund
	}
	out.Summary["total_income"] = totIncome
	out.Summary["total_expense"] = totExpense
	out.Summary["total_refund"] = totRefund
	out.Summary["session_count"] = len(out.Rows)
	return out, rows.Err()
}

// ---------- Report: doctor-revenue ----------
//
// Doktor bazlı brüt hasılat + ödenmiş tutar + hizmet sayısı (verilen aralıkta
// oluşturulmuş invoice_item'lardan; status filtrelenmez, tüm faturalar dahil).

func runDoctorRevenue(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	from, to, err := rangeFromQuery(q)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx,
		`SELECT d.id AS doctor_id, sm.title, sm.first_name, sm.last_name,
		        COUNT(*)::int AS item_count,
		        COALESCE(SUM(it.line_total), 0) AS gross,
		        COALESCE(SUM(CASE WHEN i.status = 'paid' THEN it.line_total ELSE 0 END), 0) AS paid,
		        COALESCE(SUM(CASE WHEN i.status = 'cancelled' THEN it.line_total ELSE 0 END), 0) AS cancelled
		 FROM invoice_item it
		 JOIN invoice i ON i.id = it.invoice_id
		 JOIN doctor d ON d.id = it.doctor_id
		 JOIN staff_member sm ON sm.id = d.staff_member_id
		 WHERE i.branch_id = $1
		   AND it.doctor_id IS NOT NULL
		   AND i.created_at >= $2 AND i.created_at < $3
		 GROUP BY d.id, sm.title, sm.first_name, sm.last_name
		 ORDER BY gross DESC`,
		branchID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "doctor", Label: "Doktor", Type: "text"},
			{Key: "item_count", Label: "Hizmet", Type: "number", Align: "right"},
			{Key: "gross", Label: "Brüt", Type: "currency", Align: "right"},
			{Key: "paid", Label: "Tahsil Edilen", Type: "currency", Align: "right"},
			{Key: "cancelled", Label: "İptal", Type: "currency", Align: "right"},
			{Key: "outstanding", Label: "Bekleyen", Type: "currency", Align: "right"},
		},
		Rows:    []map[string]any{},
		Summary: map[string]any{},
	}
	var totGross, totPaid float64
	for rows.Next() {
		var id uuid.UUID
		var title *string
		var fn, ln string
		var count int
		var gross, paid, cancelled float64
		if err := rows.Scan(&id, &title, &fn, &ln, &count, &gross, &paid, &cancelled); err != nil {
			return nil, err
		}
		display := strings.TrimSpace(strings.TrimSpace(strFromPtr(title)) + " " + fn + " " + ln)
		out.Rows = append(out.Rows, map[string]any{
			"doctor":      display,
			"item_count":  count,
			"gross":       gross,
			"paid":        paid,
			"cancelled":   cancelled,
			"outstanding": gross - paid - cancelled,
		})
		totGross += gross
		totPaid += paid
	}
	out.Summary["total_gross"] = totGross
	out.Summary["total_paid"] = totPaid
	out.Summary["row_count"] = len(out.Rows)
	return out, rows.Err()
}

func strFromPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ---------- Report: institution-revenue ----------

func runInstitutionRevenue(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	from, to, err := rangeFromQuery(q)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx,
		`SELECT COALESCE(ins.name, '— Cepten ödeme —') AS name,
		        COUNT(*)::int AS invoice_count,
		        COALESCE(SUM(i.total), 0) AS total,
		        COALESCE(SUM(i.paid_total), 0) AS paid,
		        COALESCE(SUM(i.balance_due) FILTER (WHERE i.status = 'finalized'), 0) AS outstanding
		 FROM invoice i
		 LEFT JOIN external_institution ins ON ins.id = i.institution_id
		 WHERE i.branch_id = $1
		   AND i.created_at >= $2 AND i.created_at < $3
		   AND i.status != 'cancelled'
		 GROUP BY ins.name
		 ORDER BY total DESC`,
		branchID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "name", Label: "Kurum", Type: "text"},
			{Key: "invoice_count", Label: "Fatura", Type: "number", Align: "right"},
			{Key: "total", Label: "Toplam", Type: "currency", Align: "right"},
			{Key: "paid", Label: "Tahsil", Type: "currency", Align: "right"},
			{Key: "outstanding", Label: "Bekleyen", Type: "currency", Align: "right"},
		},
		Rows: []map[string]any{},
	}
	for rows.Next() {
		var name string
		var count int
		var total, paid, outstanding float64
		if err := rows.Scan(&name, &count, &total, &paid, &outstanding); err != nil {
			return nil, err
		}
		out.Rows = append(out.Rows, map[string]any{
			"name": name, "invoice_count": count,
			"total": total, "paid": paid, "outstanding": outstanding,
		})
	}
	return out, rows.Err()
}

// ---------- Report: unpaid-invoices ----------

func runUnpaidInvoices(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	rows, err := pool.Query(ctx,
		`SELECT i.invoice_no, p.mrn, p.first_name, p.last_name,
		        COALESCE(ins.name, '— Cepten —') AS institution,
		        i.total, i.paid_total, i.balance_due,
		        i.created_at, i.issued_at,
		        (CURRENT_DATE - i.issued_at::date)::int AS aging_days
		 FROM invoice i
		 JOIN patient p ON p.id = i.patient_id
		 LEFT JOIN external_institution ins ON ins.id = i.institution_id
		 WHERE i.branch_id = $1
		   AND i.status = 'finalized'
		   AND i.balance_due > 0
		 ORDER BY i.issued_at`,
		branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "invoice_no", Label: "Fatura", Type: "text"},
			{Key: "patient", Label: "Hasta", Type: "text"},
			{Key: "institution", Label: "Kurum", Type: "text"},
			{Key: "total", Label: "Tutar", Type: "currency", Align: "right"},
			{Key: "paid_total", Label: "Ödenen", Type: "currency", Align: "right"},
			{Key: "balance_due", Label: "Kalan", Type: "currency", Align: "right"},
			{Key: "issued_at", Label: "Tarih", Type: "date"},
			{Key: "aging_days", Label: "Yaş (gün)", Type: "number", Align: "right"},
		},
		Rows:    []map[string]any{},
		Summary: map[string]any{},
	}
	var totBalance float64
	for rows.Next() {
		var invoiceNo, mrn, fn, ln, institution string
		var total, paid, balance float64
		var createdAt time.Time
		var issuedAt *time.Time
		var aging *int
		if err := rows.Scan(&invoiceNo, &mrn, &fn, &ln, &institution,
			&total, &paid, &balance, &createdAt, &issuedAt, &aging); err != nil {
			return nil, err
		}
		out.Rows = append(out.Rows, map[string]any{
			"invoice_no":  invoiceNo,
			"patient":     fn + " " + ln + " (MRN " + mrn + ")",
			"institution": institution,
			"total":       total,
			"paid_total":  paid,
			"balance_due": balance,
			"issued_at":   issuedAt,
			"aging_days":  aging,
		})
		totBalance += balance
	}
	out.Summary["total_outstanding"] = totBalance
	out.Summary["invoice_count"] = len(out.Rows)
	return out, rows.Err()
}

// ---------- Report: outpatient-by-doctor ----------
//
// Tarih aralığında doktor başına muayene sayısı + benzersiz hasta sayısı.

func runOutpatientByDoctor(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	from, to, err := rangeFromQuery(q)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx,
		`SELECT sm.title, sm.first_name, sm.last_name,
		        COUNT(*)::int AS visit_count,
		        COUNT(DISTINCT v.patient_id)::int AS unique_patients,
		        COUNT(*) FILTER (WHERE v.status = 'completed')::int AS completed
		 FROM visit v
		 JOIN doctor d ON d.id = v.doctor_id
		 JOIN staff_member sm ON sm.id = d.staff_member_id
		 WHERE v.branch_id = $1
		   AND v.created_at >= $2 AND v.created_at < $3
		 GROUP BY sm.title, sm.first_name, sm.last_name
		 ORDER BY visit_count DESC`,
		branchID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "doctor", Label: "Doktor", Type: "text"},
			{Key: "visit_count", Label: "Muayene", Type: "number", Align: "right"},
			{Key: "unique_patients", Label: "Benzersiz Hasta", Type: "number", Align: "right"},
			{Key: "completed", Label: "Tamamlanan", Type: "number", Align: "right"},
		},
		Rows: []map[string]any{},
	}
	for rows.Next() {
		var title *string
		var fn, ln string
		var visitCount, unique, completed int
		if err := rows.Scan(&title, &fn, &ln, &visitCount, &unique, &completed); err != nil {
			return nil, err
		}
		out.Rows = append(out.Rows, map[string]any{
			"doctor":          strings.TrimSpace(strFromPtr(title)+" "+fn+" "+ln),
			"visit_count":     visitCount,
			"unique_patients": unique,
			"completed":       completed,
		})
	}
	return out, rows.Err()
}

// ---------- Report: low-expiry-stock ----------

func runLowExpiryStock(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	days := 90
	if v := q.Get("days"); v != "" {
		var d int
		_, _ = fmtSscanInt(v, &d)
		if d > 0 {
			days = d
		}
	}
	rows, err := pool.Query(ctx,
		`SELECT m.name, m.strength, w.name AS warehouse, s.lot_no, s.expiry_date, s.quantity,
		        (s.expiry_date - CURRENT_DATE)::int AS days_left
		 FROM medication_stock s
		 JOIN medication m ON m.id = s.medication_id
		 JOIN warehouse w ON w.id = s.warehouse_id
		 WHERE s.branch_id = $1
		   AND s.quantity > 0
		   AND s.expiry_date IS NOT NULL
		   AND s.expiry_date <= (CURRENT_DATE + ($2 || ' days')::INTERVAL)
		 ORDER BY s.expiry_date`,
		branchID, fmt.Sprint(days))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "medication", Label: "İlaç", Type: "text"},
			{Key: "warehouse", Label: "Depo", Type: "text"},
			{Key: "lot_no", Label: "Lot", Type: "text"},
			{Key: "expiry_date", Label: "SKT", Type: "date"},
			{Key: "days_left", Label: "Kalan Gün", Type: "number", Align: "right"},
			{Key: "quantity", Label: "Miktar", Type: "number", Align: "right"},
		},
		Rows: []map[string]any{},
	}
	for rows.Next() {
		var name string
		var strength *string
		var warehouse, lot string
		var expiry time.Time
		var qty float64
		var daysLeft int
		if err := rows.Scan(&name, &strength, &warehouse, &lot, &expiry, &qty, &daysLeft); err != nil {
			return nil, err
		}
		medDisplay := name
		if strength != nil && *strength != "" {
			medDisplay += " · " + *strength
		}
		out.Rows = append(out.Rows, map[string]any{
			"medication":  medDisplay,
			"warehouse":   warehouse,
			"lot_no":      lot,
			"expiry_date": expiry.Format("2006-01-02"),
			"days_left":   daysLeft,
			"quantity":    qty,
		})
	}
	return out, rows.Err()
}

// ---------- Report: stock-valuation ----------
//
// Depo bazında ilaç başına aktif lotların toplam miktarı + son alış birim
// fiyatına göre yaklaşık değerleme (FIFO/Move-avg yerine basit "son alış").

func runStockValuation(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	rows, err := pool.Query(ctx,
		`WITH last_unit AS (
		   SELECT DISTINCT ON (medication_id) medication_id, unit_price
		   FROM stock_movement
		   WHERE branch_id = $1 AND kind = 'receive' AND unit_price IS NOT NULL
		   ORDER BY medication_id, performed_at DESC
		 )
		 SELECT w.name AS warehouse, m.name AS medication, m.strength,
		        SUM(s.quantity) AS total_qty,
		        COALESCE(lu.unit_price, 0) AS last_unit_price,
		        SUM(s.quantity) * COALESCE(lu.unit_price, 0) AS approx_value
		 FROM medication_stock s
		 JOIN warehouse w ON w.id = s.warehouse_id
		 JOIN medication m ON m.id = s.medication_id
		 LEFT JOIN last_unit lu ON lu.medication_id = s.medication_id
		 WHERE s.branch_id = $1 AND s.quantity > 0
		 GROUP BY w.name, m.name, m.strength, lu.unit_price
		 ORDER BY approx_value DESC`,
		branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "warehouse", Label: "Depo", Type: "text"},
			{Key: "medication", Label: "İlaç", Type: "text"},
			{Key: "total_qty", Label: "Toplam Miktar", Type: "number", Align: "right"},
			{Key: "last_unit_price", Label: "Son Alış (₺)", Type: "currency", Align: "right"},
			{Key: "approx_value", Label: "Yaklaşık Değer", Type: "currency", Align: "right"},
		},
		Rows:    []map[string]any{},
		Summary: map[string]any{},
	}
	var total float64
	for rows.Next() {
		var warehouse, medication string
		var strength *string
		var totalQty, unitPrice, value float64
		if err := rows.Scan(&warehouse, &medication, &strength, &totalQty, &unitPrice, &value); err != nil {
			return nil, err
		}
		medDisplay := medication
		if strength != nil && *strength != "" {
			medDisplay += " · " + *strength
		}
		out.Rows = append(out.Rows, map[string]any{
			"warehouse":       warehouse,
			"medication":      medDisplay,
			"total_qty":       totalQty,
			"last_unit_price": unitPrice,
			"approx_value":    value,
		})
		total += value
	}
	out.Summary["total_value"] = total
	out.Summary["line_count"] = len(out.Rows)
	return out, rows.Err()
}

// ---------- Report: bed-occupancy ----------

func runBedOccupancy(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	rows, err := pool.Query(ctx,
		`SELECT w.name AS ward,
		        COUNT(*) FILTER (WHERE b.status = 'free')::int AS available,
		        COUNT(*) FILTER (WHERE b.status = 'occupied')::int AS occupied,
		        COUNT(*) FILTER (WHERE b.status = 'reserved')::int AS reserved,
		        COUNT(*) FILTER (WHERE b.status = 'cleaning')::int AS cleaning,
		        COUNT(*) FILTER (WHERE b.status = 'blocked')::int AS blocked,
		        COUNT(*)::int AS total
		 FROM ward w
		 LEFT JOIN bed b ON b.ward_id = w.id
		 WHERE w.branch_id = $1 AND w.is_active = TRUE
		 GROUP BY w.name
		 ORDER BY w.name`,
		branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "ward", Label: "Servis", Type: "text"},
			{Key: "available", Label: "Boş", Type: "number", Align: "right"},
			{Key: "occupied", Label: "Dolu", Type: "number", Align: "right"},
			{Key: "reserved", Label: "Rezerve", Type: "number", Align: "right"},
			{Key: "cleaning", Label: "Temizlikte", Type: "number", Align: "right"},
			{Key: "blocked", Label: "Bakım", Type: "number", Align: "right"},
			{Key: "total", Label: "Toplam Yatak", Type: "number", Align: "right"},
			{Key: "occupancy_pct", Label: "Doluluk %", Type: "pct", Align: "right"},
		},
		Rows:    []map[string]any{},
		Summary: map[string]any{},
	}
	var totOcc, totBeds int
	for rows.Next() {
		var ward string
		var available, occupied, reserved, cleaning, blocked, total int
		if err := rows.Scan(&ward, &available, &occupied, &reserved, &cleaning, &blocked, &total); err != nil {
			return nil, err
		}
		var pct float64
		if total > 0 {
			pct = float64(occupied) * 100.0 / float64(total)
		}
		out.Rows = append(out.Rows, map[string]any{
			"ward": ward, "available": available, "occupied": occupied,
			"reserved": reserved, "cleaning": cleaning, "blocked": blocked,
			"total": total, "occupancy_pct": pct,
		})
		totOcc += occupied
		totBeds += total
	}
	var totalPct float64
	if totBeds > 0 {
		totalPct = float64(totOcc) * 100.0 / float64(totBeds)
	}
	out.Summary["total_occupied"] = totOcc
	out.Summary["total_beds"] = totBeds
	out.Summary["occupancy_pct"] = totalPct
	return out, rows.Err()
}

// ---------- Report: lab-volume ----------

func runLabVolume(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	from, to, err := rangeFromQuery(q)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx,
		`SELECT lo.status::text AS status, COUNT(*)::int AS count
		 FROM lab_order lo
		 WHERE lo.branch_id = $1 AND lo.created_at >= $2 AND lo.created_at < $3
		 GROUP BY lo.status
		 ORDER BY count DESC`,
		branchID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "status", Label: "Durum", Type: "text"},
			{Key: "count", Label: "Adet", Type: "number", Align: "right"},
		},
		Rows: []map[string]any{},
	}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		out.Rows = append(out.Rows, map[string]any{"status": status, "count": count})
	}
	return out, rows.Err()
}

// ---------- Report: payment-mix ----------

func runPaymentMix(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	from, to, err := rangeFromQuery(q)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx,
		`SELECT method::text AS method, COUNT(*)::int AS count, COALESCE(SUM(amount), 0) AS total
		 FROM payment
		 WHERE branch_id = $1 AND received_at >= $2 AND received_at < $3
		 GROUP BY method
		 ORDER BY total DESC`,
		branchID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "method", Label: "Yöntem", Type: "text"},
			{Key: "count", Label: "Adet", Type: "number", Align: "right"},
			{Key: "total", Label: "Toplam (₺)", Type: "currency", Align: "right"},
		},
		Rows:    []map[string]any{},
		Summary: map[string]any{},
	}
	var grand float64
	for rows.Next() {
		var method string
		var count int
		var total float64
		if err := rows.Scan(&method, &count, &total); err != nil {
			return nil, err
		}
		out.Rows = append(out.Rows, map[string]any{"method": method, "count": count, "total": total})
		grand += total
	}
	out.Summary["grand_total"] = grand
	return out, rows.Err()
}

// ---------- Helpers shared with stock.go ----------

// guard to prevent unused-import drop
var _ = errors.New

// ============================================================================
//  Yeni raporlar (slice 2): finans/klinik/stok/ameliyat/medula
// ============================================================================

// ---------- hourly-collection ----------
//
// Saat saat tahsilat dağılımı (vezne yoğunluğunun pikleri için).

func runHourlyCollection(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	from, to, err := rangeFromQuery(q)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx,
		`SELECT EXTRACT(HOUR FROM received_at AT TIME ZONE 'Europe/Istanbul')::int AS hour,
		        COUNT(*)::int AS count,
		        COALESCE(SUM(amount), 0) AS total
		 FROM payment
		 WHERE branch_id = $1 AND received_at >= $2 AND received_at < $3
		 GROUP BY hour
		 ORDER BY hour`,
		branchID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "hour", Label: "Saat", Type: "number"},
			{Key: "count", Label: "Tahsilat", Type: "number", Align: "right"},
			{Key: "total", Label: "Toplam (₺)", Type: "currency", Align: "right"},
		},
		Rows:    []map[string]any{},
		Summary: map[string]any{},
	}
	var grandTotal float64
	for rows.Next() {
		var h, c int
		var total float64
		if err := rows.Scan(&h, &c, &total); err != nil {
			return nil, err
		}
		out.Rows = append(out.Rows, map[string]any{
			"hour":  fmt.Sprintf("%02d:00", h),
			"count": c,
			"total": total,
		})
		grandTotal += total
	}
	out.Summary["grand_total"] = grandTotal
	return out, rows.Err()
}

// ---------- cashier-collection ----------

func runCashierCollection(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	from, to, err := rangeFromQuery(q)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx,
		`SELECT cr.cashier_name,
		        COUNT(DISTINCT cr.id)::int AS sessions,
		        COUNT(cm.id) FILTER (WHERE cm.kind = 'income')::int AS income_count,
		        COALESCE(SUM(cm.amount) FILTER (WHERE cm.kind = 'income'), 0) AS income_total,
		        COALESCE(SUM(cm.amount) FILTER (WHERE cm.kind = 'refund'), 0) AS refund_total
		 FROM cash_register cr
		 LEFT JOIN cash_movement cm ON cm.cash_register_id = cr.id
		 WHERE cr.branch_id = $1
		   AND cr.opened_at >= $2 AND cr.opened_at < $3
		 GROUP BY cr.cashier_name
		 ORDER BY income_total DESC`,
		branchID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "cashier_name", Label: "Kasiyer", Type: "text"},
			{Key: "sessions", Label: "Oturum", Type: "number", Align: "right"},
			{Key: "income_count", Label: "Tahsilat #", Type: "number", Align: "right"},
			{Key: "income_total", Label: "Tahsilat ₺", Type: "currency", Align: "right"},
			{Key: "refund_total", Label: "İade ₺", Type: "currency", Align: "right"},
			{Key: "net", Label: "Net ₺", Type: "currency", Align: "right"},
		},
		Rows: []map[string]any{},
	}
	for rows.Next() {
		var name string
		var sessions, count int
		var income, refund float64
		if err := rows.Scan(&name, &sessions, &count, &income, &refund); err != nil {
			return nil, err
		}
		out.Rows = append(out.Rows, map[string]any{
			"cashier_name": name, "sessions": sessions, "income_count": count,
			"income_total": income, "refund_total": refund, "net": income - refund,
		})
	}
	return out, rows.Err()
}

// ---------- open-advances ----------
//
// Pozitif bakiyesi olan hastalar (avans alacaklı).

func runOpenAdvances(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	rows, err := pool.Query(ctx,
		`SELECT p.mrn, p.first_name, p.last_name,
		        COALESCE(SUM(pae.direction * pae.amount), 0) AS balance,
		        MAX(pae.performed_at) AS last_at
		 FROM patient_account_entry pae
		 JOIN patient p ON p.id = pae.patient_id
		 WHERE pae.branch_id = $1
		 GROUP BY p.mrn, p.first_name, p.last_name
		 HAVING SUM(pae.direction * pae.amount) > 0
		 ORDER BY balance DESC`, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "patient", Label: "Hasta", Type: "text"},
			{Key: "balance", Label: "Avans Bakiye ₺", Type: "currency", Align: "right"},
			{Key: "last_at", Label: "Son Hareket", Type: "datetime"},
		},
		Rows:    []map[string]any{},
		Summary: map[string]any{},
	}
	var total float64
	for rows.Next() {
		var mrn, fn, ln string
		var balance float64
		var lastAt time.Time
		if err := rows.Scan(&mrn, &fn, &ln, &balance, &lastAt); err != nil {
			return nil, err
		}
		out.Rows = append(out.Rows, map[string]any{
			"patient": fn + " " + ln + " (MRN " + mrn + ")",
			"balance": balance,
			"last_at": lastAt,
		})
		total += balance
	}
	out.Summary["total_open_advance"] = total
	out.Summary["patient_count"] = len(out.Rows)
	return out, rows.Err()
}

// ---------- diagnosis-distribution ----------

func runDiagnosisDistribution(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	from, to, err := rangeFromQuery(q)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx,
		`SELECT d.icd10_code, d.icd10_title, COUNT(*)::int AS count
		 FROM diagnosis d
		 JOIN visit v ON v.id = d.visit_id
		 WHERE v.branch_id = $1 AND v.created_at >= $2 AND v.created_at < $3
		 GROUP BY d.icd10_code, d.icd10_title
		 ORDER BY count DESC
		 LIMIT 50`, branchID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "icd10", Label: "ICD-10", Type: "text"},
			{Key: "title", Label: "Tanı", Type: "text"},
			{Key: "count", Label: "Adet", Type: "number", Align: "right"},
		},
		Rows: []map[string]any{},
	}
	for rows.Next() {
		var code, title string
		var count int
		if err := rows.Scan(&code, &title, &count); err != nil {
			return nil, err
		}
		out.Rows = append(out.Rows, map[string]any{"icd10": code, "title": title, "count": count})
	}
	return out, rows.Err()
}

// ---------- polyclinic-by-hour ----------

func runPolyclinicByHour(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	from, to, err := rangeFromQuery(q)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx,
		`SELECT EXTRACT(HOUR FROM a.scheduled_at AT TIME ZONE 'Europe/Istanbul')::int AS hour,
		        COUNT(*)::int AS visits,
		        COUNT(*) FILTER (WHERE a.status = 'completed')::int AS completed,
		        COUNT(*) FILTER (WHERE a.status IN ('cancelled', 'no_show'))::int AS dropped
		 FROM appointment a
		 WHERE a.branch_id = $1
		   AND a.scheduled_at >= $2 AND a.scheduled_at < $3
		 GROUP BY hour
		 ORDER BY hour`,
		branchID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "hour", Label: "Saat", Type: "text"},
			{Key: "visits", Label: "Randevu", Type: "number", Align: "right"},
			{Key: "completed", Label: "Tamamlanan", Type: "number", Align: "right"},
			{Key: "dropped", Label: "İptal/Gelmeyen", Type: "number", Align: "right"},
		},
		Rows: []map[string]any{},
	}
	for rows.Next() {
		var h, v, c, d int
		if err := rows.Scan(&h, &v, &c, &d); err != nil {
			return nil, err
		}
		out.Rows = append(out.Rows, map[string]any{
			"hour": fmt.Sprintf("%02d:00", h),
			"visits": v, "completed": c, "dropped": d,
		})
	}
	return out, rows.Err()
}

// ---------- lab-test-volume ----------
//
// Test bazlı (her test kalemi için kaç istek + kaç sonuçlandı).

func runLabTestVolume(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	from, to, err := rangeFromQuery(q)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx,
		`SELECT li.test_code, li.test_name,
		        COUNT(*)::int AS ordered,
		        COUNT(*) FILTER (WHERE li.status = 'resulted')::int AS resulted,
		        COUNT(*) FILTER (WHERE li.flag IN ('critical_low','critical_high'))::int AS critical
		 FROM lab_order_item li
		 JOIN lab_order lo ON lo.id = li.lab_order_id
		 WHERE lo.branch_id = $1 AND lo.created_at >= $2 AND lo.created_at < $3
		 GROUP BY li.test_code, li.test_name
		 ORDER BY ordered DESC LIMIT 50`,
		branchID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "test_code", Label: "Kod", Type: "text"},
			{Key: "test_name", Label: "Test", Type: "text"},
			{Key: "ordered", Label: "İstendi", Type: "number", Align: "right"},
			{Key: "resulted", Label: "Sonuçlandı", Type: "number", Align: "right"},
			{Key: "critical", Label: "Kritik", Type: "number", Align: "right"},
		},
		Rows: []map[string]any{},
	}
	for rows.Next() {
		var code, name string
		var ordered, resulted, critical int
		if err := rows.Scan(&code, &name, &ordered, &resulted, &critical); err != nil {
			return nil, err
		}
		out.Rows = append(out.Rows, map[string]any{
			"test_code": code, "test_name": name,
			"ordered": ordered, "resulted": resulted, "critical": critical,
		})
	}
	return out, rows.Err()
}

// ---------- ward-admission-stats ----------

func runWardAdmissionStats(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	from, to, err := rangeFromQuery(q)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx,
		`SELECT w.name AS ward,
		        COUNT(*) FILTER (WHERE a.admitted_at >= $2 AND a.admitted_at < $3)::int AS admissions,
		        COUNT(*) FILTER (WHERE a.status = 'discharged' AND a.discharged_at >= $2 AND a.discharged_at < $3)::int AS discharges,
		        COUNT(*) FILTER (WHERE a.status = 'active')::int AS active,
		        AVG(EXTRACT(EPOCH FROM (COALESCE(a.discharged_at, NOW()) - a.admitted_at)) / 86400.0) FILTER (
		          WHERE a.admitted_at >= $2 AND a.admitted_at < $3
		        ) AS avg_los_days
		 FROM ward w
		 LEFT JOIN admission a ON a.ward_id = w.id
		 WHERE w.branch_id = $1 AND w.is_active = TRUE
		 GROUP BY w.name
		 ORDER BY admissions DESC`,
		branchID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "ward", Label: "Servis", Type: "text"},
			{Key: "admissions", Label: "Yatış", Type: "number", Align: "right"},
			{Key: "discharges", Label: "Taburcu", Type: "number", Align: "right"},
			{Key: "active", Label: "Aktif", Type: "number", Align: "right"},
			{Key: "avg_los_days", Label: "Ort. yatış (gün)", Type: "number", Align: "right"},
		},
		Rows: []map[string]any{},
	}
	for rows.Next() {
		var ward string
		var adm, disc, active int
		var avgLos *float64
		if err := rows.Scan(&ward, &adm, &disc, &active, &avgLos); err != nil {
			return nil, err
		}
		row := map[string]any{"ward": ward, "admissions": adm, "discharges": disc, "active": active}
		if avgLos != nil {
			row["avg_los_days"] = math.Round(*avgLos*10) / 10
		}
		out.Rows = append(out.Rows, row)
	}
	return out, rows.Err()
}

// ---------- top-medications ----------

func runTopMedications(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	from, to, err := rangeFromQuery(q)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx,
		`SELECT m.name, m.strength,
		        COUNT(d.id)::int AS dispense_count,
		        COALESCE(SUM(d.quantity), 0) AS total_qty
		 FROM prescription_dispense d
		 JOIN medication m ON m.id = d.medication_id
		 WHERE d.branch_id = $1 AND d.dispensed_at >= $2 AND d.dispensed_at < $3
		 GROUP BY m.name, m.strength
		 ORDER BY dispense_count DESC LIMIT 30`,
		branchID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "medication", Label: "İlaç", Type: "text"},
			{Key: "dispense_count", Label: "Dispense", Type: "number", Align: "right"},
			{Key: "total_qty", Label: "Toplam Miktar", Type: "number", Align: "right"},
		},
		Rows: []map[string]any{},
	}
	for rows.Next() {
		var name string
		var strength *string
		var count int
		var total float64
		if err := rows.Scan(&name, &strength, &count, &total); err != nil {
			return nil, err
		}
		display := name
		if strength != nil && *strength != "" {
			display += " · " + *strength
		}
		out.Rows = append(out.Rows, map[string]any{
			"medication": display, "dispense_count": count, "total_qty": total,
		})
	}
	return out, rows.Err()
}

// ---------- surgeon-performance ----------

func runSurgeonPerformance(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	from, to, err := rangeFromQuery(q)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx,
		`SELECT sm.title, sm.first_name, sm.last_name,
		        COUNT(*)::int AS total,
		        COUNT(*) FILTER (WHERE s.status = 'completed')::int AS completed,
		        COUNT(*) FILTER (WHERE s.status = 'cancelled')::int AS cancelled,
		        AVG(EXTRACT(EPOCH FROM (s.ended_at - s.started_at)) / 60.0) FILTER (
		          WHERE s.status = 'completed' AND s.started_at IS NOT NULL AND s.ended_at IS NOT NULL
		        ) AS avg_duration_min
		 FROM surgery s
		 JOIN doctor d ON d.id = s.primary_surgeon_id
		 JOIN staff_member sm ON sm.id = d.staff_member_id
		 WHERE s.branch_id = $1 AND s.scheduled_at >= $2 AND s.scheduled_at < $3
		 GROUP BY sm.title, sm.first_name, sm.last_name
		 ORDER BY completed DESC`,
		branchID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "surgeon", Label: "Cerrah", Type: "text"},
			{Key: "total", Label: "Toplam", Type: "number", Align: "right"},
			{Key: "completed", Label: "Tamamlanan", Type: "number", Align: "right"},
			{Key: "cancelled", Label: "İptal", Type: "number", Align: "right"},
			{Key: "avg_duration_min", Label: "Ort. süre (dk)", Type: "number", Align: "right"},
		},
		Rows: []map[string]any{},
	}
	for rows.Next() {
		var title *string
		var fn, ln string
		var total, completed, cancelled int
		var avgDur *float64
		if err := rows.Scan(&title, &fn, &ln, &total, &completed, &cancelled, &avgDur); err != nil {
			return nil, err
		}
		row := map[string]any{
			"surgeon": strings.TrimSpace(strFromPtr(title) + " " + fn + " " + ln),
			"total": total, "completed": completed, "cancelled": cancelled,
		}
		if avgDur != nil {
			row["avg_duration_min"] = math.Round(*avgDur)
		}
		out.Rows = append(out.Rows, row)
	}
	return out, rows.Err()
}

// ---------- medula-success-rate ----------

func runMedulaSuccessRate(ctx context.Context, pool *pgxpool.Pool, branchID uuid.UUID, q url.Values) (*ReportResult, error) {
	from, to, err := rangeFromQuery(q)
	if err != nil {
		return nil, err
	}
	// Provision metrikleri (aynı pattern fatura/sevk/e-rapor için 3 yeni
	// sorgu daha eklenebilir; bu ilk kart sadece provision).
	rows, err := pool.Query(ctx,
		`SELECT status::text, COUNT(*)::int FROM medula_provision
		 WHERE branch_id = $1 AND requested_at >= $2 AND requested_at < $3
		 GROUP BY status`, branchID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := map[string]int{}
	for rows.Next() {
		var st string
		var c int
		if err := rows.Scan(&st, &c); err != nil {
			return nil, err
		}
		stats[st] = c
	}

	completed := stats["completed"]
	failed := stats["failed"]
	pending := stats["pending"] + stats["in_progress"]
	cancelled := stats["cancelled"]
	total := completed + failed + pending + cancelled
	successRate := 0.0
	if completed+failed > 0 {
		successRate = float64(completed) * 100 / float64(completed+failed)
	}

	out := &ReportResult{
		Columns: []ReportColumn{
			{Key: "metric", Label: "Metrik", Type: "text"},
			{Key: "value", Label: "Değer", Type: "number", Align: "right"},
		},
		Rows: []map[string]any{
			{"metric": "Toplam provizyon isteği", "value": total},
			{"metric": "Tamamlandı (başarı)", "value": completed},
			{"metric": "Reddedildi", "value": failed},
			{"metric": "Bekleyen", "value": pending},
			{"metric": "İptal", "value": cancelled},
			{"metric": "Başarı oranı %", "value": math.Round(successRate*10) / 10},
		},
	}
	return out, rows.Err()
}
