package handler

import (
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/integration/hl7"
)

// hl7InboundORU accepts a raw HL7 v2.x ORU^R01 message from the lab
// autoanalizör middleware and routes its observations into our
// lab_order_item rows.
//
// Body: text/plain, raw HL7 (CR-delimited segments). Max 256 KB (single
// HL7 message rarely exceeds 50 KB; we cap generously).
//
// The branch is derived from the standard X-Branch-ID header — the lab
// middleware is configured per-branch.
func (h *Handler) hl7InboundORU(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256*1024)
	defer r.Body.Close()
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_failed", "gövde okunamadı: "+err.Error())
		return
	}
	if len(bodyBytes) == 0 {
		writeError(w, http.StatusBadRequest, "empty", "HL7 mesajı boş")
		return
	}

	msg, err := hl7.ParseORU(string(bodyBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "parse_failed", err.Error())
		return
	}

	res, err := h.deps.LabHL7.Ingest(r.Context(), branchID, msg)
	if err != nil {
		// Order match failure → 404 so the lab middleware can resend later
		// (or notify ops). Internal errors → 500.
		writeError(w, http.StatusNotFound, "ingest_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}
