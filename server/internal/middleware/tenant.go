package middleware

import (
	"context"
	"net/http"
)

const (
	ctxOrgID    ctxKey = 10
	ctxBranchID ctxKey = 11
)

// ResolveTenant reads X-Organization-ID and X-Branch-ID headers (set by the
// frontend ApiClient) and stuffs them onto the request context. Membership
// verification happens in RequireBranchAccess.
//
// Future enhancement: parse /h/:orgSlug/:branchSlug/... path segments and
// resolve slugs to IDs server-side, so URL-driven navigation works without
// the frontend pre-populating headers.
func ResolveTenant() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if orgID := r.Header.Get("X-Organization-ID"); orgID != "" {
				ctx = context.WithValue(ctx, ctxOrgID, orgID)
			}
			if branchID := r.Header.Get("X-Branch-ID"); branchID != "" {
				ctx = context.WithValue(ctx, ctxBranchID, branchID)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func OrgIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxOrgID).(string); ok {
		return v
	}
	return ""
}

func BranchIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxBranchID).(string); ok {
		return v
	}
	return ""
}
