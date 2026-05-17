package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/medigt/medigt/server/internal/auth"
)

type ctxKey int

const (
	ctxUserID ctxKey = iota
	ctxSessionID
)

func RequireAuth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := r.Header.Get("Authorization")
			if !strings.HasPrefix(authz, "Bearer ") {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			token := strings.TrimPrefix(authz, "Bearer ")
			claims, err := auth.ParseJWT(token, jwtSecret)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxSessionID, claims.SessionID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxUserID).(string); ok {
		return v
	}
	return ""
}

func SessionIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxSessionID).(string); ok {
		return v
	}
	return ""
}
