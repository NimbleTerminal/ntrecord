package render

import (
	"slices"
	"strings"

	vt "go.mitchellh.com/libghostty"
)

// ProgramStatus is an OSC 7501 record. State is idle, working, done, blocked,
// or error. Kind is permission, question, auth, or empty, and applies only to
// blocked records. Progress is 0–100, or -1 when unspecified. ID is empty for
// the root record. Title and Message are decoded plain text, never markup.
// Values are owned by Go and remain valid after subsequent writes or Close.
type ProgramStatus struct {
	ID, State, Kind, App, Title, Message string
	Progress                             int
}

// ProgramStatuses returns an independent snapshot sorted by ID. App includes
// inheritance from the nearest ancestor with an explicit app name. At most 256
// records are retained; the least recently updated is evicted at capacity.
// Status does not change the rendered pixels. Calls, like Write, must be serialized.
func (r *Renderer) ProgramStatuses() []ProgramStatus {
	result := slices.Clone(r.statuses)
	for i := range result {
		id := result[i].ID
		for result[i].App == "" && id != "" {
			if at := strings.LastIndexByte(id, '/'); at >= 0 {
				id = id[:at]
			} else {
				id = ""
			}
			for _, parent := range r.statuses {
				if parent.ID == id {
					result[i].App = parent.App
					break
				}
			}
		}
	}
	slices.SortFunc(result, func(a, b ProgramStatus) int { return strings.Compare(a.ID, b.ID) })
	return result
}

// ProgramExited removes working and blocked records after the attached process
// exits. Call it after feeding all final output. Done, error, and idle persist.
// Shell prompt notifications perform the same cleanup automatically; RIS clears
// all records, while alternate-screen switches and soft resets preserve them.
func (r *Renderer) ProgramExited() {
	r.statuses = slices.DeleteFunc(r.statuses, func(s ProgramStatus) bool {
		return s.State == "working" || s.State == "blocked"
	})
}

func (r *Renderer) updateProgramStatus(s vt.ProgramStatus) {
	if s.State == vt.ProgramStatusStateClear {
		r.statuses = slices.DeleteFunc(r.statuses, func(old ProgramStatus) bool {
			return s.ID == "" || old.ID == s.ID || strings.HasPrefix(old.ID, s.ID+"/")
		})
		return
	}
	state := ""
	switch s.State {
	case vt.ProgramStatusStateIdle:
		state = "idle"
	case vt.ProgramStatusStateWorking:
		state = "working"
	case vt.ProgramStatusStateDone:
		state = "done"
	case vt.ProgramStatusStateBlocked:
		state = "blocked"
	case vt.ProgramStatusStateError:
		state = "error"
	default:
		return
	}
	kind := ""
	switch s.Kind {
	case vt.ProgramStatusKindPermission:
		kind = "permission"
	case vt.ProgramStatusKindQuestion:
		kind = "question"
	case vt.ProgramStatusKindAuth:
		kind = "auth"
	}
	r.statuses = slices.DeleteFunc(r.statuses, func(old ProgramStatus) bool { return old.ID == s.ID })
	if len(r.statuses) == 256 {
		r.statuses = slices.Delete(r.statuses, 0, 1)
	}
	r.statuses = append(r.statuses, ProgramStatus{
		ID: s.ID, State: state, Kind: kind, Progress: int(s.Progress),
		App: s.App, Title: s.Title, Message: s.Message,
	})
}
