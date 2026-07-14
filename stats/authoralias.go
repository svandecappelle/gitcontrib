package stats

import (
	"sort"
	"strings"
)

// authorIDSep separates the name and email inside an AuthorsEditions map key.
const authorIDSep = "\x1f"

type authorIdentity struct {
	name      string
	email     string
	additions int
	deletions int
}

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
// Two identities are considered the same person when they share a non-empty
// email or a non-empty (case-insensitive) name. Grouping is transitive, so
// "Jane <perso>", "Jane <work>" and "jane-doe <work>" all collapse into one.
// The displayed name is taken from the identity with the most changes.
func mergeAuthorAliases(editions map[string][2]int) []Contributor {
	ids := make([]authorIdentity, 0, len(editions))
	for key, e := range editions {
		name, email := splitAuthorKey(key)
		ids = append(ids, authorIdentity{name: name, email: email, additions: e[0], deletions: e[1]})
	}

	// Union-find over the identities.
	parent := make([]int, len(ids))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) {
		if ra, rb := find(a), find(b); ra != rb {
			parent[ra] = rb
		}
	}

	byEmail := map[string]int{}
	byName := map[string]int{}
	for i, id := range ids {
		if email := strings.ToLower(strings.TrimSpace(id.email)); email != "" {
			if j, ok := byEmail[email]; ok {
				union(i, j)
			} else {
				byEmail[email] = i
			}
		}
		if name := strings.ToLower(strings.TrimSpace(id.name)); name != "" {
			if j, ok := byName[name]; ok {
				union(i, j)
			} else {
				byName[name] = i
			}
		}
	}

	groups := map[int][]int{}
	for i := range ids {
		root := find(i)
		groups[root] = append(groups[root], i)
	}

	contributors := make([]Contributor, 0, len(groups))
	for _, members := range groups {
		additions, deletions, best := 0, 0, members[0]
		for _, m := range members {
			additions += ids[m].additions
			deletions += ids[m].deletions
			if ids[m].additions+ids[m].deletions > ids[best].additions+ids[best].deletions {
				best = m
			}
		}
		name := ids[best].name
		if name == "" {
			name = ids[best].email
		}
		contributors = append(contributors, Contributor{
			Author:    name,
			Additions: additions,
			Deletions: deletions,
			Total:     additions + deletions,
		})
	}

	sort.Slice(contributors, func(i, j int) bool {
		return contributors[i].Total > contributors[j].Total
	})
	return contributors
}
