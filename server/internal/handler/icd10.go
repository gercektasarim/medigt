package handler

import (
	"bufio"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/pkg/db/repo"
)

type icd10Payload struct {
	ID         string    `json:"id"`
	Code       string    `json:"code"`
	TitleTR    string    `json:"title_tr"`
	TitleEN    *string   `json:"title_en,omitempty"`
	ParentCode *string   `json:"parent_code,omitempty"`
	Chapter    *string   `json:"chapter,omitempty"`
	IsActive   bool      `json:"is_active"`
	IsSystem   bool      `json:"is_system"`
	CreatedAt  time.Time `json:"created_at"`
}

func toIcdPayload(c *repo.Icd10Code) icd10Payload {
	return icd10Payload{
		ID: c.ID.String(), Code: c.Code, TitleTR: c.TitleTR, TitleEN: c.TitleEN,
		ParentCode: c.ParentCode, Chapter: c.Chapter,
		IsActive: c.IsActive, IsSystem: c.IsSystem, CreatedAt: c.CreatedAt,
	}
}

func (h *Handler) searchIcd10(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	q := r.URL.Query().Get("q")
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			limit = v
		}
	}
	items, err := h.deps.Icd10.Search(r.Context(), orgID, q, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]icd10Payload, 0, len(items))
	for i := range items {
		out = append(out, toIcdPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

// importIcd10TSV accepts a TSV upload and bulk-loads it into the system
// catalog (organization_id NULL, is_system TRUE). Mirrors the CLI command;
// org admins can use this from the UI without shell access.
//
// Body: raw TSV (text/tab-separated-values or text/plain). Max 5MB —
// Sağlık Bakanlığı listesi ~2MB civarındadır, çatlak payı bıraktık.
func (h *Handler) importIcd10TSV(w http.ResponseWriter, r *http.Request) {
	const maxBytes = 5 * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	defer r.Body.Close()

	codes, err := parseIcd10TSVStream(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "parse_error", err.Error())
		return
	}
	if len(codes) == 0 {
		writeError(w, http.StatusBadRequest, "empty", "dosya boş ya da geçerli satır yok")
		return
	}
	if len(codes) > 50000 {
		writeError(w, http.StatusBadRequest, "too_many", "tek istekte 50.000 satırdan fazlası kabul edilmiyor")
		return
	}
	ins, upd, err := h.deps.Icd10.BulkUpsertSystem(r.Context(), codes)
	if err != nil {
		h.deps.Log.Error("icd10 import failed", "err", err)
		writeError(w, http.StatusInternalServerError, "import_failed", "yükleme başarısız")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"processed": len(codes),
		"inserted":  ins,
		"updated":   upd,
	})
}

// parseIcd10TSVStream mirrors the CLI's parser but consumes io.Reader so
// the HTTP body can stream in. Same column rules:
//   code<TAB>title_tr<TAB>chapter[<TAB>parent_code]
func parseIcd10TSVStream(r io.Reader) ([]repo.CreateIcd10Input, error) {
	out := []repo.CreateIcd10Input{}
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := strings.TrimRight(scanner.Text(), "\r\n")
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		if lineNo == 1 && strings.HasPrefix(strings.ToLower(raw), "code\t") {
			continue
		}
		parts := strings.Split(raw, "\t")
		if len(parts) < 2 {
			return nil, errorAtLine(lineNo, "en az code + title_tr gerekli")
		}
		c := repo.CreateIcd10Input{
			Code:    strings.TrimSpace(parts[0]),
			TitleTR: strings.TrimSpace(parts[1]),
		}
		if c.Code == "" || c.TitleTR == "" {
			return nil, errorAtLine(lineNo, "code/title boş olamaz")
		}
		if len(parts) >= 3 && strings.TrimSpace(parts[2]) != "" {
			ch := strings.TrimSpace(parts[2])
			c.Chapter = &ch
		}
		if len(parts) >= 4 && strings.TrimSpace(parts[3]) != "" {
			p := strings.TrimSpace(parts[3])
			c.ParentCode = &p
		}
		out = append(out, c)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type lineErr struct {
	Line int
	Msg  string
}

func (e *lineErr) Error() string {
	return "satır " + strconv.Itoa(e.Line) + ": " + e.Msg
}

func errorAtLine(line int, msg string) error { return &lineErr{Line: line, Msg: msg} }
