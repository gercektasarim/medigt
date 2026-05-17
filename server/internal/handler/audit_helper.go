package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/middleware"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

// auditAccess records one audit_log row. It is the canonical entry-point
// for handlers; it pulls actor/session/org/branch off the request context
// and forwards IP + UA so we don't repeat ourselves at every call site.
//
// Errors are swallowed and logged — an audit failure must never abort the
// user-facing request. Production-grade auditing should be retried via a
// queue, but for V1 a logged miss is acceptable.
func (h *Handler) auditAccess(ctx context.Context, r *http.Request, action, entityType string, entityID string, details any) {
	if h.deps.Audit == nil {
		return
	}
	in := repo.WriteInput{
		Action:     action,
		EntityType: entityType,
		Details:    details,
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	}
	if entityID != "" {
		s := entityID
		in.EntityID = &s
	}
	if s := middleware.OrgIDFromContext(ctx); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			in.OrganizationID = &id
		}
	}
	if s := middleware.BranchIDFromContext(ctx); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			in.BranchID = &id
		}
	}
	if s := middleware.UserIDFromContext(ctx); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			in.ActorUserID = &id
		}
	}
	if s := middleware.SessionIDFromContext(ctx); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			in.ActorSessionID = &id
		}
	}
	if err := h.deps.Audit.Write(ctx, in); err != nil {
		h.deps.Log.Warn("audit write failed", "err", err, "action", action, "entity", entityType)
	}
}
