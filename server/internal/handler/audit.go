package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/pkg/db/repo"
)

// Audit log viewer — KVKK compliance requirement. Org admins can browse
// the access trail for their organization, filter by actor / action /
// entity / date range, and inspect the structural details JSON. Personal
// identifiers in details should already be masked at write-time
// (util.MaskTC) so no extra redaction is needed here.

type auditEntryPayload struct {
	ID             int64           `json:"id"`
	OrganizationID *string         `json:"organization_id,omitempty"`
	BranchID       *string         `json:"branch_id,omitempty"`
	ActorUserID    *string         `json:"actor_user_id,omitempty"`
	ActorEmail     *string         `json:"actor_email,omitempty"`
	ActorName      *string         `json:"actor_name,omitempty"`
	Action         string          `json:"action"`
	EntityType     string          `json:"entity_type"`
	EntityID       *string         `json:"entity_id,omitempty"`
	Details        json.RawMessage `json:"details"`
	IPAddress      *string         `json:"ip_address,omitempty"`
	UserAgent      *string         `json:"user_agent,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

func toAuditPayload(e *repo.AuditEntry) auditEntryPayload {
	p := auditEntryPayload{
		ID:         e.ID,
		Action:     e.Action,
		EntityType: e.EntityType,
		EntityID:   e.EntityID,
		Details:    e.Details,
		IPAddress:  e.IPAddress,
		UserAgent:  e.UserAgent,
		CreatedAt:  e.CreatedAt,
		ActorEmail: e.ActorEmail,
		ActorName:  e.ActorName,
	}
	if e.OrganizationID != nil {
		s := e.OrganizationID.String()
		p.OrganizationID = &s
	}
	if e.BranchID != nil {
		s := e.BranchID.String()
		p.BranchID = &s
	}
	if e.ActorUserID != nil {
		s := e.ActorUserID.String()
		p.ActorUserID = &s
	}
	if len(p.Details) == 0 {
		p.Details = json.RawMessage("{}")
	}
	return p
}

func (h *Handler) listAuditLog(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	q := r.URL.Query()

	f := repo.ListFilter{OrganizationID: orgID}

	if s := q.Get("branch_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_branch", "branch_id geçersiz")
			return
		}
		f.BranchID = &id
	}
	if s := q.Get("actor_user_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_actor", "actor_user_id geçersiz")
			return
		}
		f.ActorUserID = &id
	}
	f.Action = q.Get("action")
	f.EntityType = q.Get("entity_type")
	f.EntityID = q.Get("entity_id")

	if s := q.Get("from"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_date", "from YYYY-MM-DD olmalı")
			return
		}
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
		f.From = &t
	}
	if s := q.Get("to"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_date", "to YYYY-MM-DD olmalı")
			return
		}
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local).Add(24 * time.Hour)
		f.To = &t
	}

	if s := q.Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			f.Limit = n
		}
	}
	if s := q.Get("offset"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			f.Offset = n
		}
	}

	items, total, err := h.deps.Audit.List(r.Context(), f)
	if err != nil {
		h.deps.Log.Error("audit list failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]auditEntryPayload, 0, len(items))
	for i := range items {
		out = append(out, toAuditPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total":  total,
		"limit":  f.Limit,
		"offset": f.Offset,
		"items":  out,
	})
}

func (h *Handler) listAuditFacets(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	actions, err := h.deps.Audit.DistinctActions(r.Context(), orgID)
	if err != nil {
		h.deps.Log.Error("audit facets actions failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	entities, err := h.deps.Audit.DistinctEntityTypes(r.Context(), orgID)
	if err != nil {
		h.deps.Log.Error("audit facets entities failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"actions":      actions,
		"entity_types": entities,
	})
}
