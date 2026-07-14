package stats

import (
	"regexp"
	"strings"
)

// conventionalCommit matches the "type(scope)!: description" prefix of a
// Conventional Commits message, capturing the type.
var conventionalCommit = regexp.MustCompile(`^([a-zA-Z]+)(\([^)]*\))?!?:`)

// knownCommitTypes is the set of Conventional Commits types recognised as-is;
// anything else is reported as "other".
var knownCommitTypes = map[string]bool{
	"feat":     true,
	"fix":      true,
	"docs":     true,
	"style":    true,
	"refactor": true,
	"perf":     true,
	"test":     true,
	"build":    true,
	"ci":       true,
	"chore":    true,
	"revert":   true,
}

// commitType classifies a commit message by its Conventional Commits type,
// falling back to "other" for messages that don't follow the convention.
func commitType(message string) string {
	firstLine := message
	if i := strings.IndexByte(message, '\n'); i >= 0 {
		firstLine = message[:i]
	}

	m := conventionalCommit.FindStringSubmatch(strings.TrimSpace(firstLine))
	if m == nil {
		return "other"
	}
	t := strings.ToLower(m[1])
	if !knownCommitTypes[t] {
		return "other"
	}
	return t
}
