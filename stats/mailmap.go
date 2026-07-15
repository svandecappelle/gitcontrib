package stats

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// mailmapLine matches one .mailmap entry: an optional proper name and email,
// optionally followed by an optional commit name and commit email.
var mailmapLine = regexp.MustCompile(`^\s*([^<]*?)\s*<([^>]+)>\s*(?:([^<]*?)\s*<([^>]+)>\s*)?$`)

type mmEntry struct {
	name  string // canonical name ("" keeps the commit's name)
	email string // canonical email
}

// Mailmap resolves commit author identities to their canonical form, following
// the git .mailmap rules.
type Mailmap struct {
	byEmail        map[string]mmEntry // keyed by lowercased commit email
	byNameAndEmail map[string]mmEntry // keyed by lowercased "email\x00name"
}

func mmKey(email, name string) string {
	return strings.ToLower(email) + "\x00" + strings.ToLower(name)
}

// loadMailmap reads and parses <repoPath>/.mailmap, returning nil when there is
// no readable mailmap (in which case Resolve is a no-op).
func loadMailmap(repoPath string) *Mailmap {
	data, err := os.ReadFile(filepath.Join(repoPath, ".mailmap"))
	if err != nil {
		return nil
	}
	return parseMailmap(string(data))
}

func parseMailmap(content string) *Mailmap {
	mm := &Mailmap{byEmail: map[string]mmEntry{}, byNameAndEmail: map[string]mmEntry{}}
	for _, raw := range strings.Split(content, "\n") {
		line := raw
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := mailmapLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		properName, properEmail := strings.TrimSpace(m[1]), strings.TrimSpace(m[2])
		commitName, commitEmail := strings.TrimSpace(m[3]), strings.TrimSpace(m[4])

		canonical := mmEntry{name: properName, email: properEmail}
		switch {
		case commitEmail != "" && commitName != "":
			// Proper Name <proper> Commit Name <commit>
			mm.byNameAndEmail[mmKey(commitEmail, commitName)] = canonical
		case commitEmail != "":
			// [Proper Name] <proper> <commit>
			mm.byEmail[strings.ToLower(commitEmail)] = canonical
		default:
			// Proper Name <proper> — set the name for that email
			mm.byEmail[strings.ToLower(properEmail)] = canonical
		}
	}
	return mm
}

// Resolve returns the canonical (name, email) for a commit author.
func (m *Mailmap) Resolve(name, email string) (string, string) {
	if m == nil {
		return name, email
	}
	lowerEmail := strings.ToLower(strings.TrimSpace(email))
	if e, ok := m.byNameAndEmail[mmKey(lowerEmail, name)]; ok {
		return coalesce(e.name, name), coalesce(e.email, email)
	}
	if e, ok := m.byEmail[lowerEmail]; ok {
		return coalesce(e.name, name), coalesce(e.email, email)
	}
	return name, email
}

func coalesce(mapped, original string) string {
	if mapped != "" {
		return mapped
	}
	return original
}
