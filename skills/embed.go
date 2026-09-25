// Package skills carries the agent skill inside the tt binary, so tt setup
// installs the one written for this version.
package skills

import _ "embed"

// TangoTube is skills/tangotube/SKILL.md.
//
//go:embed tangotube/SKILL.md
var TangoTube []byte
