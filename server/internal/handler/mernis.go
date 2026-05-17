package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/integration/mernis"
	"github.com/medigt/medigt/server/internal/middleware"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

// ---------- TC kimlik number checksum (5+7+9) ----------
//
// 11 digits; 1st digit ≠ 0; 10th and 11th are checksum digits per the
// MERNIS algorithm. Same logic the patient create form uses, kept local
// so this file is self-contained.

func tcChecksum(tc string) bool {
	if len(tc) != 11 {
		return false
	}
	digits := [11]int{}
	for i, c := range tc {
		if c < '0' || c > '9' {
			return false
		}
		digits[i] = int(c - '0')
	}
	if digits[0] == 0 {
		return false
	}
	odd := digits[0] + digits[2] + digits[4] + digits[6] + digits[8]
	even := digits[1] + digits[3] + digits[5] + digits[7]
	d10 := (odd*7 - even) % 10
	if d10 < 0 {
		d10 += 10
	}
	if d10 != digits[9] {
		return false
	}
	sum10 := digits[0] + digits[1] + digits[2] + digits[3] + digits[4] + digits[5] + digits[6] + digits[7] + digits[8] + digits[9]
	if sum10%10 != digits[10] {
		return false
	}
	return true
}

type verifyTCReq struct {
	TCKimlikNo string `json:"tc_kimlik_no"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	BirthYear  int    `json:"birth_year"`
}

type verifyTCRes struct {
	Verified     bool   `json:"verified"`
	ResponseCode string `json:"response_code,omitempty"`
	Error        string `json:"error,omitempty"`
	LatencyMs    int64  `json:"latency_ms"`
	LogID        string `json:"log_id"`
}

func (h *Handler) verifyMernis(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := branchIDFromHeader(r) // optional
	var req verifyTCReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	tc := strings.TrimSpace(req.TCKimlikNo)
	if !tcChecksum(tc) {
		writeError(w, http.StatusBadRequest, "bad_tc", "TC kimlik no formatı / kontrol hanesi geçersiz")
		return
	}
	first := strings.TrimSpace(req.FirstName)
	last := strings.TrimSpace(req.LastName)
	if first == "" || last == "" || req.BirthYear == 0 {
		writeError(w, http.StatusBadRequest, "missing_fields", "ad, soyad ve doğum yılı zorunlu")
		return
	}

	start := time.Now()
	res, callErr := h.deps.MernisSvc.Verify(r.Context(), mernis.VerifyInput{
		TCKimlikNo: tc, FirstName: first, LastName: last, BirthYear: req.BirthYear,
	})
	latency := time.Since(start)

	in := repo.LogMernisInput{
		OrganizationID: orgID,
		TCLast4:        tc[len(tc)-4:],
		FirstName:      first,
		LastName:       last,
		BirthYear:      req.BirthYear,
	}
	if branchID != uuid.Nil {
		in.BranchID = &branchID
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.RequestedByUserID = &uid
		}
	}
	if callErr != nil {
		errMsg := callErr.Error()
		in.Verified = false
		in.ErrorMessage = &errMsg
		log, _ := h.deps.Mernis.Log(r.Context(), in)
		out := verifyTCRes{Verified: false, Error: callErr.Error(), LatencyMs: latency.Milliseconds()}
		if log != nil {
			out.LogID = log.ID.String()
		}
		// KVKK — TC sorgusu kişisel veri işleme; ayrıntılı veri
		// mernis_verification_log'da tutuluyor, biz sadece olay izi atıyoruz.
		h.auditAccess(r.Context(), r, "mernis.verify", "mernis_verification_log",
			func() string { if log != nil { return log.ID.String() }; return "" }(),
			map[string]any{"verified": false, "error": errMsg})
		writeJSON(w, http.StatusOK, out)
		return
	}
	in.Verified = res.Verified
	in.ResponseCode = &res.ResponseCode
	log, _ := h.deps.Mernis.Log(r.Context(), in)
	out := verifyTCRes{Verified: res.Verified, ResponseCode: res.ResponseCode, LatencyMs: latency.Milliseconds()}
	h.auditAccess(r.Context(), r, "mernis.verify", "mernis_verification_log",
		func() string { if log != nil { return log.ID.String() }; return "" }(),
		map[string]any{"verified": res.Verified, "response_code": res.ResponseCode})
	if log != nil {
		out.LogID = log.ID.String()
	}
	writeJSON(w, http.StatusOK, out)
}

// branchIDFromHeader returns uuid.Nil if the header is missing/invalid (no
// error response — MERNIS is org-scoped, branch is optional).
func branchIDFromHeader(r *http.Request) uuid.UUID {
	v := r.Header.Get("X-Branch-ID")
	if v == "" {
		return uuid.Nil
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil
	}
	return id
}

type mernisLogPayload struct {
	ID           string    `json:"id"`
	TCLast4      string    `json:"tc_last4"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	BirthYear    int       `json:"birth_year"`
	Verified     bool      `json:"verified"`
	ResponseCode *string   `json:"response_code,omitempty"`
	ErrorMessage *string   `json:"error_message,omitempty"`
	RequestedAt  time.Time `json:"requested_at"`
}

func (h *Handler) listMernisLog(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	items, err := h.deps.Mernis.Recent(r.Context(), orgID, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]mernisLogPayload, 0, len(items))
	for _, m := range items {
		out = append(out, mernisLogPayload{
			ID: m.ID.String(), TCLast4: m.TCLast4,
			FirstName: m.FirstName, LastName: m.LastName, BirthYear: m.BirthYear,
			Verified:  m.Verified, ResponseCode: m.ResponseCode, ErrorMessage: m.ErrorMessage,
			RequestedAt: m.RequestedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
