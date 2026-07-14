package stats

import (
	"sort"
	"strings"
)

// authorIDSep separates the name and email inside an AuthorsEditions map key.
const authorIDSep = "\x1f"

// splitAuthorKey parses an AuthorsEditions key back into its name and email.
func splitAuthorKey(key string) (name, email string) {
	if i := strings.Index(key, authorIDSep); i >= 0 {
		return key[:i], key[i+len(authorIDSep):]
	}
	return key, ""
}

// mergeAuthorAliases groups author identities that belong to the same person
// and returns the merged contributors, sorted by total changes (descending).
//
// Identities are grouped by their (case-insensitive) author name; identities
// without a name are grouped by email instead. Only the name is used as the
// grouping key on purpose: merging by email as well would let a bot that
// authors commits under a human's email (e.g. semantic-release, renovate)
// bridge otherwise-unrelated people into one giant contributor. The displayed
// name is taken from the identity with the most changes, and Identities lists
// the exact tokens (name spellings, or emails for name-less identities) that
// reproduce this group when used as a user filter.
func mergeAuthorAliases(editions map[string][2]int) []Contributor {
	// Group by a stable key: the lowercased name, or "email:<addr>" when the
	// name is empty.
	type group struct {
		additions, deletions int
		bestName             string
		bestTotal            int
		names                map[string]bool // distinct name spellings
		emails               map[string]bool // emails (used only for name-less groups)
	}
	groups := map[string]*group{}

	for key, e := range editions {
		name, email := splitAuthorKey(key)
		name = strings.TrimSpace(name)
		email = strings.TrimSpace(email)

		groupKey := strings.ToLower(name)
		if groupKey == "" {
			groupKey = "email:" + strings.ToLower(email)
		}
		g := groups[groupKey]
		if g == nil {
			g = &group{names: map[string]bool{}, emails: map[string]bool{}}
			groups[groupKey] = g
		}
		g.additions += e[0]
		g.deletions += e[1]
		if name != "" {
			g.names[name] = true
		}
		if email != "" {
			g.emails[email] = true
		}
		if total := e[0] + e[1]; total >= g.bestTotal {
			g.bestTotal = total
			if name != "" {
				g.bestName = name
			} else if g.bestName == "" {
				g.bestName = email
			}
		}
	}

	contributors := make([]Contributor, 0, len(groups))
	for _, g := range groups {
		identities := make([]string, 0, len(g.names))
		for n := range g.names {
			identities = append(identities, n)
		}
		if len(identities) == 0 { // name-less group: filter by its emails
			for em := range g.emails {
				identities = append(identities, em)
			}
		}
		sort.Strings(identities)
		contributors = append(contributors, Contributor{
			Author:     g.bestName,
			Additions:  g.additions,
			Deletions:  g.deletions,
			Total:      g.additions + g.deletions,
			Identities: identities,
		})
	}

	sort.Slice(contributors, func(i, j int) bool {
		return contributors[i].Total > contributors[j].Total
	})
	return contributors
}
