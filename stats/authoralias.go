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
// Identities are grouped by their (case-insensitive) email, like `git
// shortlog`; identities without an email are grouped by name instead. Grouping
// by email — rather than name — collapses the common case of one person
// committing under a stable email with several name spellings, and, unlike
// name grouping, cannot let a bot that authors commits under a human's email
// bridge unrelated people (a bot's commits under different emails stay in their
// own email groups). Use a repository .mailmap to unify a person's several
// emails. The displayed name is the spelling with the most changes, and
// Identities lists the tokens (emails, or names for email-less identities) that
// reproduce this group as a user filter.
func mergeAuthorAliases(editions map[string][2]int) []Contributor {
	type group struct {
		additions, deletions int
		emails               map[string]bool // distinct emails
		nameTotals           map[string]int  // name spelling -> total changes
	}
	groups := map[string]*group{}

	for key, e := range editions {
		name, email := splitAuthorKey(key)
		name = strings.TrimSpace(name)
		email = strings.TrimSpace(email)

		groupKey := "email:" + strings.ToLower(email)
		if email == "" {
			groupKey = "name:" + strings.ToLower(name)
		}
		g := groups[groupKey]
		if g == nil {
			g = &group{emails: map[string]bool{}, nameTotals: map[string]int{}}
			groups[groupKey] = g
		}
		g.additions += e[0]
		g.deletions += e[1]
		if email != "" {
			g.emails[email] = true
		}
		if name != "" {
			g.nameTotals[name] += e[0] + e[1]
		}
	}

	contributors := make([]Contributor, 0, len(groups))
	for _, g := range groups {
		identities := make([]string, 0, len(g.emails))
		for em := range g.emails {
			identities = append(identities, em)
		}
		if len(identities) == 0 { // email-less group: filter by its names
			for n := range g.nameTotals {
				identities = append(identities, n)
			}
		}
		sort.Strings(identities)
		contributors = append(contributors, Contributor{
			Author:     displayName(g.nameTotals, identities),
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

// displayName picks the name spelling with the most changes; ties prefer a
// "proper"-looking name (one with a space) then the alphabetically first, so
// the choice is deterministic. Falls back to the first identity (an email)
// when there is no name.
func displayName(nameTotals map[string]int, identities []string) string {
	best, bestTotal := "", -1
	for name, total := range nameTotals {
		if total > bestTotal || (total == bestTotal && nicerName(name, best)) {
			best, bestTotal = name, total
		}
	}
	if best == "" && len(identities) > 0 {
		return identities[0]
	}
	return best
}

// nicerName reports whether a is a nicer display name than b: a name containing
// a space (looks like "First Last") wins, otherwise the alphabetically first.
func nicerName(a, b string) bool {
	if b == "" {
		return true
	}
	as, bs := strings.Contains(a, " "), strings.Contains(b, " ")
	if as != bs {
		return as
	}
	return a < b
}
