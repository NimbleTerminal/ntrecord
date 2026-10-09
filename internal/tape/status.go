package tape

import (
	"fmt"
	"regexp"
	"strings"
)

// StatusCondition matches one current OSC 7501 record. An omitted ID matches
// any record; an explicitly empty ID matches only the root record.
type StatusCondition struct {
	State, App, ID, Kind string
	HasID                bool
}

var statusApp = regexp.MustCompile(`^[A-Za-z0-9_.+-]{1,32}$`)
var statusID = regexp.MustCompile(`^[A-Za-z0-9_.+-]{1,32}(/[A-Za-z0-9_.+-]{1,32}){0,7}$`)

func parseStatusCondition(state string, filters []string) (StatusCondition, error) {
	c := StatusCondition{State: state}
	switch state {
	case "idle", "working", "done", "blocked", "error":
	default:
		return c, fmt.Errorf("Wait+Status requires idle, working, done, blocked, or error")
	}
	seen := map[string]bool{}
	for _, filter := range filters {
		key, value, ok := strings.Cut(filter, "=")
		if !ok || seen[key] {
			return c, fmt.Errorf("invalid or duplicate Wait+Status filter %q", filter)
		}
		seen[key] = true
		switch key {
		case "app":
			if !statusApp.MatchString(value) {
				return c, fmt.Errorf("invalid Wait+Status app %q", value)
			}
			c.App = value
		case "id":
			if value != "" && (len(value) > 128 || !statusID.MatchString(value)) {
				return c, fmt.Errorf("invalid Wait+Status id %q", value)
			}
			c.ID, c.HasID = value, true
		case "kind":
			if state != "blocked" || (value != "permission" && value != "question" && value != "auth") {
				return c, fmt.Errorf("Wait+Status kind requires blocked and permission, question, or auth")
			}
			c.Kind = value
		default:
			return c, fmt.Errorf("unknown Wait+Status filter %q", key)
		}
	}
	return c, nil
}

func (c StatusCondition) Matches(state, app, id, kind string) bool {
	return state == c.State && (c.App == "" || c.App == app) &&
		(!c.HasID || c.ID == id) && (c.Kind == "" || c.Kind == kind)
}

func (c StatusCondition) String() string {
	s := c.State
	if c.App != "" {
		s += " app=" + c.App
	}
	if c.HasID {
		s += " id=" + c.ID
	}
	if c.Kind != "" {
		s += " kind=" + c.Kind
	}
	return s
}
