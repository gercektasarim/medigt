package handler

import "github.com/medigt/medigt/server/internal/util"

// validateTCFromHandler is a tiny re-export so the patient handler does not
// have to import the util package directly (keeps imports flat in patient.go).
func validateTCFromHandler(tc string) bool {
	return util.ValidateTC(tc)
}
