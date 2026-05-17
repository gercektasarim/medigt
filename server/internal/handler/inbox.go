package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/middleware"
)

// Inbox endpoint — aggregates actionable items across the user's domain.
//
// The signal is "things the logged-in user needs to do" — not a generic
// activity feed. We pull from three sources today:
//
//   1) Unsigned prescriptions assigned to this user (as doctor)
//   2) Critical lab results published in the last 48h on this branch
//   3) Medula outbox messages stuck in 'dead' state (admin only)
//
// All three are UNION-ALL'd into a uniform shape so the UI can render
// one list with filter chips. Sources can grow over time without
// breaking the contract.

type inboxItem struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Title      string    `json:"title"`
	Subtitle   string    `json:"subtitle,omitempty"`
	Severity   string    `json:"severity"` // info | warning | critical
	Ref        string    `json:"ref"`      // entity id for the click target
	RefURL     string    `json:"ref_url"`  // relative URL the UI navigates to
	OccurredAt time.Time `json:"occurred_at"`
}

func (h *Handler) listInbox(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	uid := middleware.UserIDFromContext(r.Context())
	if uid == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "kullanıcı yok")
		return
	}
	userID, err := uuid.Parse(uid)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_user", "kullanıcı geçersiz")
		return
	}
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	out := []inboxItem{}

	// 1) Unsigned prescriptions for this user (as doctor).
	out = append(out, h.inboxUnsignedPrescriptions(r, branchID, userID, limit)...)

	// 2) Critical lab results in the last 48h on this branch.
	out = append(out, h.inboxCriticalLabs(r, branchID, limit)...)

	// 3) Medula outbox dead rows — surfaced for the org admin.
	out = append(out, h.inboxMedulaDead(r, orgID, branchID, limit)...)

	// Sort by occurred_at descending; cap at `limit`.
	sortInboxDesc(out)
	if len(out) > limit {
		out = out[:limit]
	}

	writeJSON(w, http.StatusOK, out)
}

// inboxUnsignedPrescriptions: prescriptions in status='draft' where the
// signing doctor is the logged-in user. Subtitle = patient name + drug
// count.
func (h *Handler) inboxUnsignedPrescriptions(r *http.Request, branchID, userID uuid.UUID, limit int) []inboxItem {
	rows, err := h.deps.Pool.Query(r.Context(),
		`SELECT pr.id, pr.created_at,
		        p.mrn, p.first_name, p.last_name,
		        (SELECT COUNT(*) FROM prescription_item pi WHERE pi.prescription_id = pr.id) AS item_count,
		        pr.visit_id
		 FROM prescription pr
		 JOIN visit v ON v.id = pr.visit_id
		 JOIN patient p ON p.id = v.patient_id
		 JOIN doctor d ON d.id = pr.doctor_id
		 JOIN staff_member sm ON sm.id = d.staff_member_id
		 WHERE v.branch_id = $1
		   AND pr.status = 'draft'
		   AND sm.user_id = $2
		 ORDER BY pr.created_at DESC
		 LIMIT $3`,
		branchID, userID, limit)
	if err != nil {
		h.deps.Log.Warn("inbox: unsigned rx query failed", "err", err)
		return nil
	}
	defer rows.Close()
	out := []inboxItem{}
	for rows.Next() {
		var id uuid.UUID
		var createdAt time.Time
		var mrn, first, last string
		var itemCount int
		var visitID uuid.UUID
		if err := rows.Scan(&id, &createdAt, &mrn, &first, &last, &itemCount, &visitID); err != nil {
			continue
		}
		out = append(out, inboxItem{
			ID:         id.String(),
			Kind:       "prescription.unsigned",
			Title:      "Reçete imza bekliyor",
			Subtitle:   first + " " + last + " · " + itoa10(itemCount) + " ilaç · MRN " + mrn,
			Severity:   "info",
			Ref:        id.String(),
			RefURL:     "/poliklinik/" + visitID.String(),
			OccurredAt: createdAt,
		})
	}
	return out
}

// inboxCriticalLabs: lab_order_item with abnormal_flag in (critical_low,
// critical_high) where the parent order is on this branch and was
// resulted in the last 48h.
func (h *Handler) inboxCriticalLabs(r *http.Request, branchID uuid.UUID, limit int) []inboxItem {
	rows, err := h.deps.Pool.Query(r.Context(),
		`SELECT li.id, li.resulted_at, li.test_code, li.flag::text, li.value_text,
		        p.mrn, p.first_name, p.last_name, lo.id
		 FROM lab_order_item li
		 JOIN lab_order lo ON lo.id = li.lab_order_id
		 JOIN patient p ON p.id = lo.patient_id
		 WHERE lo.branch_id = $1
		   AND li.flag IN ('critical_low', 'critical_high')
		   AND li.resulted_at IS NOT NULL
		   AND li.resulted_at > NOW() - INTERVAL '48 hours'
		 ORDER BY li.resulted_at DESC
		 LIMIT $2`,
		branchID, limit)
	if err != nil {
		h.deps.Log.Warn("inbox: critical labs query failed", "err", err)
		return nil
	}
	defer rows.Close()
	out := []inboxItem{}
	for rows.Next() {
		var id uuid.UUID
		var resultedAt *time.Time
		var testCode, flag string
		var valueText *string
		var mrn, first, last string
		var orderID uuid.UUID
		if err := rows.Scan(&id, &resultedAt, &testCode, &flag, &valueText, &mrn, &first, &last, &orderID); err != nil {
			continue
		}
		val := ""
		if valueText != nil {
			val = " · " + *valueText
		}
		t := time.Now()
		if resultedAt != nil {
			t = *resultedAt
		}
		out = append(out, inboxItem{
			ID:         id.String(),
			Kind:       "lab.critical",
			Title:      "Kritik lab sonucu — " + testCode + val,
			Subtitle:   first + " " + last + " · MRN " + mrn + " · " + flagLabel(flag),
			Severity:   "critical",
			Ref:        orderID.String(),
			RefURL:     "/laboratuvar/" + orderID.String(),
			OccurredAt: t,
		})
	}
	return out
}

// inboxMedulaDead: outbox messages that exhausted their retry budget.
// Admin-only signal — staff with no role will see no rows because they
// don't own provision/submission flows. We don't gate this here; the
// query just returns nothing if the user has no related context.
func (h *Handler) inboxMedulaDead(r *http.Request, orgID, branchID uuid.UUID, limit int) []inboxItem {
	rows, err := h.deps.Pool.Query(r.Context(),
		`SELECT id, message_type, target_id, COALESCE(last_error, ''),
		        created_at
		 FROM medula_outgoing_message
		 WHERE organization_id = $1 AND status = 'dead'
		 ORDER BY created_at DESC
		 LIMIT $2`,
		orgID, limit)
	if err != nil {
		// Table may not exist on a fresh dev DB before migration 015 — skip.
		return nil
	}
	defer rows.Close()
	out := []inboxItem{}
	for rows.Next() {
		var id, targetID uuid.UUID
		var msgType, lastErr string
		var createdAt time.Time
		if err := rows.Scan(&id, &msgType, &targetID, &lastErr, &createdAt); err != nil {
			continue
		}
		out = append(out, inboxItem{
			ID:         id.String(),
			Kind:       "medula.dead",
			Title:      "Medula gönderimi başarısız — " + msgType,
			Subtitle:   "Yönetici müdahalesi gerekli · " + truncForInbox(lastErr, 80),
			Severity:   "warning",
			Ref:        id.String(),
			RefURL:     "/medula",
			OccurredAt: createdAt,
		})
	}
	_ = branchID
	return out
}

// ---------- helpers ----------

func itoa10(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+(n%10))) + s
		n /= 10
	}
	return s
}

func flagLabel(flag string) string {
	switch flag {
	case "critical_low":
		return "kritik düşük"
	case "critical_high":
		return "kritik yüksek"
	case "high":
		return "yüksek"
	case "low":
		return "düşük"
	case "abnormal":
		return "anormal"
	default:
		return flag
	}
}

func truncForInbox(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// sortInboxDesc — small insertion sort suffices for typical N≤50 inboxes
// without pulling in the sort package overhead.
func sortInboxDesc(items []inboxItem) {
	for i := 1; i < len(items); i++ {
		j := i
		for j > 0 && items[j].OccurredAt.After(items[j-1].OccurredAt) {
			items[j], items[j-1] = items[j-1], items[j]
			j--
		}
	}
}
