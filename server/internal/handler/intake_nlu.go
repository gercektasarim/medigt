package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/integration/nlu"
)

// Intake NLU endpoint — turns a free-form Turkish transcript into a
// structured slot value for the assistant's current dialog step.
//
// Flow:
//   1) Browser SpeechRecognition (tr-TR) produces a transcript.
//   2) Front-end POSTs { step, transcript } here.
//   3) For the "specialization" step we hydrate the catalog from
//      the org's specializations so fuzzy matching has real names
//      to score against.
//   4) Return the parsed value + confidence + echo string. The UI
//      fills the form field; high-confidence answers may auto-advance,
//      low-confidence ones surface the echo and wait for "evet".
//
// This is a stateless transformation — no DB writes happen here. We
// audit `nlu.parse` though so KVKK reviewers can see what staff /
// patients dictated (transcript itself is NOT stored in audit details
// because it can contain personal information; we stash only the
// step + confidence + whether a slot was extracted).

type parseIntakeReq struct {
	Step       string `json:"step"`
	Transcript string `json:"transcript"`
}

type parseIntakeResp struct {
	Result nlu.ParseResult `json:"result"`
}

func (h *Handler) parseIntake(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	var req parseIntakeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}

	in := nlu.ParseInput{
		Step:       nlu.Step(req.Step),
		Transcript: req.Transcript,
	}
	// Hydrate catalog for specialization fuzzy matching.
	if in.Step == nlu.StepSpecialization {
		specs, err := h.deps.Specializations.List(r.Context(), orgID)
		if err == nil {
			hints := make([]nlu.SpecHint, 0, len(specs))
			for i := range specs {
				hints = append(hints, nlu.SpecHint{
					ID:   specs[i].ID.String(),
					Name: specs[i].Name,
				})
			}
			in.Specializations = hints
		}
	}

	res := nlu.Parse(in)

	// Audit — never log the raw transcript. Track step + whether we
	// extracted anything + confidence bucket.
	h.auditAccess(r.Context(), r, "nlu.parse", "intake", "", map[string]any{
		"step":       req.Step,
		"extracted":  res.Confidence > 0,
		"confidence": res.Confidence,
	})

	writeJSON(w, http.StatusOK, parseIntakeResp{Result: res})
}
