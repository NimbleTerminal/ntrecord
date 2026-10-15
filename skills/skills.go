// Package skills carries the skill that teaches agents to use ntrecord, so that
// the binary can print and install it: ntrecord skill.
package skills

import _ "embed"

// NTRecord is skills/ntrecord/SKILL.md, whole, as an agent would install it.
//
//go:embed ntrecord/SKILL.md
var NTRecord string
