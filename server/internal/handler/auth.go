package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/middleware"
	"github.com/medigt/medigt/server/internal/service"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

type sendCodeReq struct {
	Email string `json:"email"`
}

type verifyCodeReq struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type loginResp struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	User         userPayload `json:"user"`
	IsNewUser    bool        `json:"is_new_user"`
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

type refreshResp struct {
	AccessToken string      `json:"access_token"`
	User        userPayload `json:"user"`
}

func (h *Handler) authSendCode(w http.ResponseWriter, r *http.Request) {
	var req sendCodeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		writeError(w, http.StatusBadRequest, "email_required", "e-posta zorunlu")
		return
	}

	err := h.deps.Auth.SendLoginCode(r.Context(), email)
	switch {
	case errors.Is(err, service.ErrResendCooldown):
		writeError(w, http.StatusTooManyRequests, "cooldown", "yeni kod için biraz bekleyin")
		return
	case errors.Is(err, service.ErrEmailNotAllowed),
		errors.Is(err, service.ErrSignupDisabled):
		// Don't leak whether an account exists. Pretend success.
		writeJSON(w, http.StatusOK, map[string]any{"sent": true})
		return
	case err != nil:
		h.deps.Log.Error("send code failed", "err", err, "email", email)
		writeError(w, http.StatusInternalServerError, "send_failed", "kod gönderilemedi")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": true})
}

func (h *Handler) authVerifyCode(w http.ResponseWriter, r *http.Request) {
	var req verifyCodeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	email := strings.TrimSpace(req.Email)
	code := strings.TrimSpace(req.Code)
	if email == "" || code == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "e-posta ve kod zorunlu")
		return
	}

	result, err := h.deps.Auth.VerifyLoginCode(r.Context(), email, code,
		r.UserAgent(), r.RemoteAddr)
	// Failed login attempts are KVKK-relevant too — record reason without
	// the email body (PII). The email is the only identifier; we keep its
	// hash-like first 3 chars + domain for forensic correlation.
	recordFailure := func(reason string) {
		if h.deps.Audit == nil {
			return
		}
		_ = h.deps.Audit.Write(r.Context(), repo.WriteInput{
			Action:     "auth.failed_login",
			EntityType: "app_user",
			Details:    map[string]any{"reason": reason, "email_hint": maskEmail(email)},
			IPAddress:  r.RemoteAddr,
			UserAgent:  r.UserAgent(),
		})
	}
	switch {
	case errors.Is(err, service.ErrInvalidCode):
		recordFailure("invalid_code")
		writeError(w, http.StatusUnauthorized, "invalid_code", "kod hatalı")
		return
	case errors.Is(err, service.ErrCodeExpired):
		recordFailure("code_expired")
		writeError(w, http.StatusUnauthorized, "code_expired", "kod süresi geçti")
		return
	case errors.Is(err, service.ErrTooManyAttempts):
		recordFailure("too_many_attempts")
		writeError(w, http.StatusTooManyRequests, "too_many_attempts", "çok fazla yanlış deneme")
		return
	case errors.Is(err, service.ErrSignupDisabled):
		recordFailure("signup_disabled")
		writeError(w, http.StatusForbidden, "signup_disabled", "kayıt kapalı")
		return
	case err != nil:
		h.deps.Log.Error("verify code failed", "err", err, "email", email)
		recordFailure("internal_error")
		writeError(w, http.StatusInternalServerError, "verify_failed", "doğrulama hatası")
		return
	}

	// KVKK audit — record every successful login so the org admin can
	// review access trail. Context has no org/branch/session yet (public
	// route), so we set ActorUserID directly from the auth result.
	if h.deps.Audit != nil {
		uid := result.User.ID
		eid := uid.String()
		_ = h.deps.Audit.Write(r.Context(), repo.WriteInput{
			ActorUserID: &uid,
			Action:      "auth.login",
			EntityType:  "app_user",
			EntityID:    &eid,
			Details:     map[string]any{"is_new_user": result.IsNewUser},
			IPAddress:   r.RemoteAddr,
			UserAgent:   r.UserAgent(),
		})
	}

	writeJSON(w, http.StatusOK, loginResp{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		User:         toUserPayload(result.User),
		IsNewUser:    result.IsNewUser,
	})
}

func (h *Handler) authRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.RefreshToken) == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "refresh_token zorunlu")
		return
	}
	access, user, err := h.deps.Auth.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_refresh", "refresh token geçersiz")
		return
	}
	writeJSON(w, http.StatusOK, refreshResp{AccessToken: access, User: toUserPayload(user)})
}

func (h *Handler) authLogout(w http.ResponseWriter, r *http.Request) {
	sessIDStr := middleware.SessionIDFromContext(r.Context())
	if sessIDStr == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	sessID, err := uuid.Parse(sessIDStr)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := h.deps.Auth.Logout(r.Context(), sessID); err != nil {
		h.deps.Log.Warn("logout failed", "err", err)
	}
	h.auditAccess(r.Context(), r, "auth.logout", "user_session", sessID.String(), nil)
	w.WriteHeader(http.StatusNoContent)
}

// maskEmail keeps only the first 3 chars + domain for failed-login audit
// telemetry. Example: turker.aktas81@gmail.com -> "tur***@gmail.com".
func maskEmail(e string) string {
	at := strings.IndexByte(e, '@')
	if at < 0 {
		if len(e) <= 3 {
			return "***"
		}
		return e[:3] + "***"
	}
	head := e[:at]
	if len(head) <= 3 {
		return "***" + e[at:]
	}
	return head[:3] + "***" + e[at:]
}
