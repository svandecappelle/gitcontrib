package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

// invalidGroups marks a payload the client got wrong — as opposed to a file
// that could not be read or written, which is the server's problem.
type invalidGroups struct{ err error }

func (e invalidGroups) Error() string { return e.err.Error() }
func (e invalidGroups) Unwrap() error { return e.err }

// rejectGroups reports a payload as the client's mistake.
func rejectGroups(format string, args ...interface{}) error {
	return invalidGroups{fmt.Errorf(format, args...)}
}

// maxGroups caps how many groups the file holds, so a misbehaving client
// cannot grow it without bound.
const maxGroups = 200

// RepoGroup is a named set of repositories, so a selection used often — the
// front-end services, one team's repositories — can be recalled (and shared as
// a link) in one click.
type RepoGroup struct {
	Name         string   `json:"name"`
	Repositories []string `json:"repositories"`
}

// groupsFile is the JSON document the groups are stored in.
type groupsFile struct {
	Groups []RepoGroup `json:"groups"`
}

// groupStore keeps the repository groups in a JSON file of their own, read at
// startup and rewritten whenever the web UI changes them. It is kept apart
// from the analysis config so that rewriting it cannot disturb the values a
// user hand-wrote there.
type groupStore struct {
	file string

	mu     sync.RWMutex
	groups []RepoGroup
}

func newGroupStore(file string) *groupStore {
	return &groupStore{file: file}
}

// load reads the groups file. A missing file is not an error: the store simply
// starts empty. An unreadable or invalid one is reported, and also leaves the
// store empty rather than failing the server's startup.
func (s *groupStore) load() error {
	if s.file == "" {
		return nil
	}
	raw, err := os.ReadFile(s.file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var content groupsFile
	if err := json.Unmarshal(raw, &content); err != nil {
		return fmt.Errorf("invalid groups file %s: %w", s.file, err)
	}
	groups, err := sanitizeGroups(content.Groups)
	if err != nil {
		return fmt.Errorf("invalid groups file %s: %w", s.file, err)
	}
	s.mu.Lock()
	s.groups = groups
	s.mu.Unlock()
	return nil
}

// all returns a copy of the groups, in their stored order.
func (s *groupStore) all() []RepoGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]RepoGroup, len(s.groups))
	for i, g := range s.groups {
		out[i] = RepoGroup{Name: g.Name, Repositories: append([]string(nil), g.Repositories...)}
	}
	return out
}

// find returns the group with that name, matched case-insensitively so a link
// carrying "?group=frontend" finds the group named "Frontend".
func (s *groupStore) find(name string) (RepoGroup, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, g := range s.groups {
		if strings.EqualFold(g.Name, name) {
			return RepoGroup{Name: g.Name, Repositories: append([]string(nil), g.Repositories...)}, true
		}
	}
	return RepoGroup{}, false
}

// nameOf returns the group a selection of folders reproduces, comparing the
// selection with each group restricted to the folders actually scanned. It
// lets the UI show "Frontend" for a link that spells the folders out.
func (s *groupStore) nameOf(selection, available []string) string {
	if len(selection) == 0 {
		return ""
	}
	want := keySet(selection)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, g := range s.groups {
		if sameKeySet(want, keySet(intersectFolders(g.Repositories, available))) {
			return g.Name
		}
	}
	return ""
}

// replace validates and stores a whole new set of groups, then writes the
// file. The client always sends the complete list, so one write keeps the file
// and the store in step.
func (s *groupStore) replace(groups []RepoGroup) ([]RepoGroup, error) {
	sanitized, err := sanitizeGroups(groups)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.groups = sanitized
	s.mu.Unlock()
	if err := s.save(sanitized); err != nil {
		return nil, err
	}
	return s.all(), nil
}

// save writes the groups to their file, through a temporary file so an
// interrupted write cannot leave a truncated document behind.
func (s *groupStore) save(groups []RepoGroup) error {
	if s.file == "" {
		return fmt.Errorf("no groups file configured")
	}
	raw, err := json.MarshalIndent(groupsFile{Groups: groups}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomically(s.file, append(raw, '\n'))
}

// sanitizeGroups trims and de-duplicates the groups, rejecting the ones that
// could not be used: a group needs a name and at least one repository, and two
// groups cannot share a name. Repository paths are stored as given — a group
// may name a repository that is not scanned right now (the folder set is fixed
// at startup, groups outlive it), and such an entry is simply ignored when the
// group is applied.
func sanitizeGroups(groups []RepoGroup) ([]RepoGroup, error) {
	if len(groups) > maxGroups {
		return nil, rejectGroups("too many groups: %d (at most %d)", len(groups), maxGroups)
	}
	out := make([]RepoGroup, 0, len(groups))
	names := map[string]bool{}

	for _, g := range groups {
		name := strings.TrimSpace(g.Name)
		if name == "" {
			return nil, rejectGroups("a group needs a name")
		}
		if names[strings.ToLower(name)] {
			return nil, rejectGroups("two groups are named %q", name)
		}
		names[strings.ToLower(name)] = true

		repos := make([]string, 0, len(g.Repositories))
		seen := map[string]bool{}
		for _, repo := range g.Repositories {
			repo = strings.TrimSpace(repo)
			if repo == "" || seen[repo] {
				continue
			}
			seen[repo] = true
			repos = append(repos, repo)
		}
		if len(repos) == 0 {
			return nil, rejectGroups("group %q holds no repository", name)
		}
		out = append(out, RepoGroup{Name: name, Repositories: repos})
	}
	return out, nil
}

// intersectFolders keeps the repositories that are among the scanned folders,
// in the order the folders were configured.
func intersectFolders(repos, available []string) []string {
	wanted := map[string]bool{}
	for _, r := range repos {
		wanted[r] = true
	}
	out := make([]string, 0, len(repos))
	for _, folder := range available {
		if wanted[folder] {
			out = append(out, folder)
		}
	}
	return out
}

// keySet turns folders into a comparable set.
func keySet(folders []string) map[string]bool {
	set := make(map[string]bool, len(folders))
	for _, f := range folders {
		set[f] = true
	}
	return set
}

func sameKeySet(a, b map[string]bool) bool {
	if len(a) != len(b) || len(a) == 0 {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}
