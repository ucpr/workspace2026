package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// printJSON marshals v as indented JSON to stdout.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// printJSONError writes a structured error object to stdout, so a JSON
// consumer can rely on stdout alone (requirements §5.6: errors must be
// distinguishable via exit code and, in JSON mode, a structured message).
func printJSONError(err error) {
	_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
}

// printText writes a human-readable line to stdout.
func printText(format string, args ...any) {
	fmt.Fprintf(os.Stdout, format+"\n", args...)
}

func printErrText(w io.Writer, err error) {
	fmt.Fprintf(w, "Error: %s\n", err.Error())
}
