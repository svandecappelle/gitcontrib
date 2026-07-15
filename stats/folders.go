package stats

import (
	"os"
	"path/filepath"
	"sort"
)

// ExpandFolders replaces any folder that is not itself a git repository with
// its immediate (one level deep) subdirectories that are git repositories. This
// makes it possible to point the analysis at a parent directory holding several
// repositories. A folder that already is a repository — or that has no
// repository subdirectory — is kept unchanged. The result is de-duplicated.
func ExpandFolders(folders []string) []string {
	out := make([]string, 0, len(folders))
	seen := map[string]bool{}
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}

	for _, folder := range folders {
		if isRepo(folder) {
			add(folder)
			continue
		}
		subs := repoSubdirs(folder)
		if len(subs) == 0 {
			add(folder) // nothing to expand; leave it as-is
			continue
		}
		for _, sub := range subs {
			add(sub)
		}
	}
	return out
}

// repoSubdirs returns the immediate subdirectories of folder that are git
// repositories, sorted for a stable order.
func repoSubdirs(folder string) []string {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil
	}
	var repos []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(folder, entry.Name())
		if isRepo(path) {
			repos = append(repos, path)
		}
	}
	sort.Strings(repos)
	return repos
}
