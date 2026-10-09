package record

import (
	"fmt"
	"strings"

	"nimbleterminal.dev/ntrecord/render"
)

// Keep diagnostics bounded and omit free-text messages from the program.
func statusSummary(records []render.ProgramStatus) string {
	if len(records) == 0 {
		return "none"
	}
	var out []string
	for _, s := range records[:min(len(records), 8)] {
		out = append(out, fmt.Sprintf("{id=%q app=%q state=%s kind=%s}", s.ID, s.App, s.State, s.Kind))
	}
	if len(records) > 8 {
		out = append(out, fmt.Sprintf("(%d more)", len(records)-8))
	}
	return strings.Join(out, " ")
}
