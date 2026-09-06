package cli

import (
	"errors"
	"os"

	"github.com/ucpr/atama/internal/service"
	"github.com/ucpr/atama/internal/store"
)

// Exit codes, so agent callers can branch without parsing text
// (requirements §5.6).
const (
	ExitOK         = 0
	ExitError      = 1 // unexpected / I/O error
	ExitValidation = 2 // bad input, cycle detected, etc.
	ExitNotFound   = 3 // task/goal/store not found
)

// ReportError prints err in the format selected by --output: a structured
// JSON object on stdout for "json", or a plain message on stderr for
// "text" (requirements §5.6).
func ReportError(err error) {
	if isJSON() {
		printJSONError(err)
		return
	}
	printErrText(os.Stderr, err)
}

// ExitCodeFor classifies err for use as the process exit code.
func ExitCodeFor(err error) int {
	if err == nil {
		return ExitOK
	}
	var verr *service.ValidationError
	switch {
	case errors.As(err, &verr):
		return ExitValidation
	case errors.Is(err, store.ErrNotFound):
		return ExitNotFound
	default:
		return ExitError
	}
}
