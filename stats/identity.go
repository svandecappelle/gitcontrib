package stats

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// maxBlobScan is the biggest blob whose lines are counted. Bigger files are
// reported as binary/oversized so a single huge asset cannot stall a scan.
const maxBlobScan = 8 << 20

// topContributorsInCard is how many contributors an identity card carries.
const topContributorsInCard = 5

// LanguageSize is the amount of code of one language in the current version of
// a repository (its HEAD tree), by opposition to [Language] which counts the
// changes made to a language over the analyzed window.
type LanguageSize struct {
	Name  string `json:"name"`
	Files int    `json:"files"`
	Lines int    `json:"lines"`
	Bytes int64  `json:"bytes"`
}

// DependencySet is the dependencies declared by one manifest of the current
// version of a repository (a go.mod, a package.json, …). In the "all
// repositories" overview the sets are merged per ecosystem and Manifest is
// empty.
type DependencySet struct {
	Ecosystem string `json:"ecosystem"`          // "Go modules", "npm", …
	Manifest  string `json:"manifest,omitempty"` // path inside the repository
	Direct    int    `json:"direct"`
	Indirect  int    `json:"indirect"`
	Dev       int    `json:"dev"`
	Total     int    `json:"total"`
}

// CodeSnapshot describes the code as it currently is: the files tracked by the
// HEAD commit, excluding vendored directories (see [ShouldBeIgnored]).
type CodeSnapshot struct {
	Files        int             `json:"files"`        // tracked files
	CodeFiles    int             `json:"codeFiles"`    // files in a programming language
	Lines        int             `json:"lines"`        // lines of all text files
	CodeLines    int             `json:"codeLines"`    // lines of the code files
	BinaryFiles  int             `json:"binaryFiles"`  // files whose lines cannot be counted
	Bytes        int64           `json:"bytes"`        // size of every tracked file
	Vendored     int             `json:"vendored"`     // tracked files skipped as vendored
	Directories  int             `json:"directories"`  // directories holding a tracked file
	Packages     int             `json:"packages"`     // directories holding a code file
	Languages    []LanguageSize  `json:"languages"`    // sorted by lines, descending
	Dependencies []DependencySet `json:"dependencies"` // sorted by total, descending
	DirectDeps   int             `json:"directDeps"`   // declared direct dependencies
	TotalDeps    int             `json:"totalDeps"`    // direct + indirect + dev
}

// AuthorCommits is a contributor's all-time commit count in a repository.
type AuthorCommits struct {
	Author  string `json:"author"`
	Email   string `json:"email,omitempty"`
	Commits int    `json:"commits"`
}

// HistorySnapshot is the all-time contribution history of a repository — the
// whole history reachable from HEAD, independent of the analyzed window.
type HistorySnapshot struct {
	Commits        int             `json:"commits"`
	MergeCommits   int             `json:"mergeCommits"`
	Contributors   int             `json:"contributors"`
	FirstCommit    time.Time       `json:"firstCommit"`
	LastCommit     time.Time       `json:"lastCommit"`
	AgeInDays      int             `json:"ageInDays"`
	ActiveDays     int             `json:"activeDays"`
	CommitsPerWeek float64         `json:"commitsPerWeek"`
	TopAuthors     []AuthorCommits `json:"topAuthors"`

	// authors and days keep the raw grouping so several snapshots can be
	// merged without double-counting. Not serialized.
	authors map[string]*authorTally
	days    map[string]bool
}

// authorTally accumulates one person's commits, keyed by email like the
// contributors ranking (see mergeAuthorAliases).
type authorTally struct {
	email   string
	names   map[string]int // name spelling -> commits
	commits int
}

// RepositoryIdentity is the identity card of a single repository: what the
// repository is, what its history says about contributions, and how much code
// its current version holds.
type RepositoryIdentity struct {
	Name        string          `json:"name"`
	Folder      string          `json:"folder"` // as configured, the repo filter key
	Path        string          `json:"path"`   // absolute, for display
	Remote      string          `json:"remote,omitempty"`
	Branch      string          `json:"branch,omitempty"`
	HeadSHA     string          `json:"headSha,omitempty"`
	HeadSubject string          `json:"headSubject,omitempty"`
	HeadDate    time.Time       `json:"headDate"`
	Tags        int             `json:"tags"`
	Branches    int             `json:"branches"`
	History     HistorySnapshot `json:"history"`
	Code        CodeSnapshot    `json:"code"`
	Error       string          `json:"error,omitempty"`
}

// IdentityOverview is the identity card of every scanned repository at once.
type IdentityOverview struct {
	Repositories int             `json:"repositories"` // cards that could be read
	Failed       int             `json:"failed"`       // folders that could not be read
	Tags         int             `json:"tags"`
	Branches     int             `json:"branches"`
	History      HistorySnapshot `json:"history"`
	Code         CodeSnapshot    `json:"code"`
}

// BuildIdentity computes the identity card of the repository in folder. A
// folder that cannot be read as a repository yields a card carrying the error
// rather than failing the whole set.
func BuildIdentity(folder string) RepositoryIdentity {
	card := RepositoryIdentity{Name: repoName(folder), Folder: folder, Path: absFolder(folder)}

	repo, err := git.PlainOpen(folder)
	if err != nil {
		card.Error = fmt.Sprintf("not a repository: %s", err)
		return card
	}

	card.Remote = remoteURL(repo)
	card.Tags = countTags(repo)
	card.Branches = countBranches(repo)

	head, err := repo.Head()
	if err != nil {
		// A repository with an unborn HEAD simply has no commit yet; anything
		// else is a repository that cannot be read.
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			card.Error = "no commit yet"
		} else {
			card.Error = fmt.Sprintf("cannot read HEAD: %s", err)
		}
		return card
	}
	if head.Name().IsBranch() {
		card.Branch = head.Name().Short()
	} else {
		card.Branch = "detached"
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		card.Error = fmt.Sprintf("cannot read HEAD commit: %s", err)
		return card
	}
	card.HeadSHA = head.Hash().String()
	card.HeadSubject = subject(commit.Message)
	card.HeadDate = commit.Author.When

	mailmap := loadMailmap(folder)
	card.History = historySnapshot(repo, head.Hash(), mailmap)

	code, err := codeSnapshot(commit)
	if err != nil {
		card.Error = fmt.Sprintf("cannot read the HEAD tree: %s", err)
		return card
	}
	card.Code = code
	return card
}

// BuildIdentities computes the identity cards of several folders, in parallel.
func BuildIdentities(folders []string) []RepositoryIdentity {
	cards := make([]RepositoryIdentity, len(folders))
	done := make(chan int, len(folders))
	for i, folder := range folders {
		go func(i int, folder string) {
			cards[i] = BuildIdentity(folder)
			done <- i
		}(i, folder)
	}
	for range folders {
		<-done
	}
	return cards
}

// repoName is the folder's base name, the friendly name of a repository.
func repoName(folder string) string {
	cleaned := strings.TrimSuffix(strings.ReplaceAll(folder, "\\", "/"), "/")
	switch base := path.Base(cleaned); base {
	case ".", "..", "/", "":
		// A relative folder is named after the directory it resolves to.
	default:
		return base
	}
	if abs, err := filepath.Abs(cleaned); err == nil {
		return filepath.Base(abs)
	}
	return cleaned
}

// absFolder resolves a folder to its absolute path, so a card can show where
// a relative folder such as "." really points. It falls back to the folder
// itself when the path cannot be resolved.
func absFolder(folder string) string {
	if abs, err := filepath.Abs(folder); err == nil {
		return abs
	}
	return folder
}

// subject is the first line of a commit message.
func subject(message string) string {
	if i := strings.IndexByte(message, '\n'); i >= 0 {
		message = message[:i]
	}
	return strings.TrimSpace(message)
}

// credentialsInURL matches the "user:password@" part of a remote URL.
var credentialsInURL = regexp.MustCompile(`//[^/@]*@`)

// remoteURL returns the repository's origin URL (or the first remote when
// there is no origin), with any embedded credentials stripped.
func remoteURL(repo *git.Repository) string {
	url := ""
	if origin, err := repo.Remote("origin"); err == nil && len(origin.Config().URLs) > 0 {
		url = origin.Config().URLs[0]
	} else if remotes, err := repo.Remotes(); err == nil {
		for _, r := range remotes {
			if len(r.Config().URLs) > 0 {
				url = r.Config().URLs[0]
				break
			}
		}
	}
	return credentialsInURL.ReplaceAllString(url, "//")
}

func countTags(repo *git.Repository) int {
	iter, err := repo.Tags()
	if err != nil {
		return 0
	}
	n := 0
	_ = iter.ForEach(func(*plumbing.Reference) error { n++; return nil })
	return n
}

func countBranches(repo *git.Repository) int {
	iter, err := repo.Branches()
	if err != nil {
		return 0
	}
	n := 0
	_ = iter.ForEach(func(*plumbing.Reference) error { n++; return nil })
	return n
}

// historySnapshot walks the whole history reachable from head — without
// computing per-commit diffs, which would be far too slow — and summarizes the
// contributions it holds.
func historySnapshot(repo *git.Repository, head plumbing.Hash, mailmap *Mailmap) HistorySnapshot {
	h := HistorySnapshot{
		authors: map[string]*authorTally{},
		days:    map[string]bool{},
	}
	iter, err := repo.Log(&git.LogOptions{From: head})
	if err != nil {
		return h
	}
	defer iter.Close()

	_ = iter.ForEach(func(c *object.Commit) error {
		h.Commits++
		if len(c.ParentHashes) > 1 {
			h.MergeCommits++
		}

		name, email := mailmap.Resolve(c.Author.Name, c.Author.Email)
		tally := h.authors[authorGroupKey(name, email)]
		if tally == nil {
			tally = &authorTally{email: email, names: map[string]int{}}
			h.authors[authorGroupKey(name, email)] = tally
		}
		tally.commits++
		if name != "" {
			tally.names[name]++
		}

		when := c.Author.When
		h.days[when.Format("2006-01-02")] = true
		if h.FirstCommit.IsZero() || when.Before(h.FirstCommit) {
			h.FirstCommit = when
		}
		if when.After(h.LastCommit) {
			h.LastCommit = when
		}
		return nil
	})

	h.finalize()
	return h
}

// authorGroupKey groups identities by email — like the contributors ranking —
// falling back to the name for email-less identities.
func authorGroupKey(name, email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "name:" + strings.ToLower(strings.TrimSpace(name))
	}
	return "email:" + email
}

// finalize derives the counters that depend on the whole walk: contributors,
// age, activity rate and the top authors.
func (h *HistorySnapshot) finalize() {
	h.Contributors = len(h.authors)
	h.ActiveDays = len(h.days)
	if !h.FirstCommit.IsZero() {
		h.AgeInDays = int(time.Since(h.FirstCommit).Hours()/24) + 1
	}
	if h.AgeInDays > 0 {
		h.CommitsPerWeek = float64(h.Commits) / (float64(h.AgeInDays) / 7)
	}
	h.TopAuthors = topAuthors(h.authors, topContributorsInCard)
}

// topAuthors ranks the tallies by commits and keeps the first n, naming each
// person by their most-used name spelling.
func topAuthors(authors map[string]*authorTally, n int) []AuthorCommits {
	ranked := make([]AuthorCommits, 0, len(authors))
	for _, t := range authors {
		ranked = append(ranked, AuthorCommits{
			Author:  displayName(t.names, []string{t.email}),
			Email:   t.email,
			Commits: t.commits,
		})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Commits != ranked[j].Commits {
			return ranked[i].Commits > ranked[j].Commits
		}
		return ranked[i].Author < ranked[j].Author
	})
	if len(ranked) > n {
		ranked = ranked[:n]
	}
	return ranked
}

// codeSnapshot measures the files tracked by a commit: how many there are, how
// big they are, which languages they belong to and which dependencies they
// declare. Vendored directories are skipped: they are dependencies, not the
// repository's own code.
func codeSnapshot(commit *object.Commit) (CodeSnapshot, error) {
	tree, err := commit.Tree()
	if err != nil {
		return CodeSnapshot{}, err
	}

	code := CodeSnapshot{}
	langs := map[string]*LanguageSize{}
	dirs := map[string]bool{}
	packages := map[string]bool{}
	var deps []DependencySet

	err = tree.Files().ForEach(func(f *object.File) error {
		if isVendored(f.Name) {
			code.Vendored++
			return nil
		}
		dir := path.Dir(f.Name)
		dirs[dir] = true
		code.Files++
		code.Bytes += f.Size

		lang := languageForFile(f.Name)
		size := langs[lang]
		if size == nil {
			size = &LanguageSize{Name: lang}
			langs[lang] = size
		}
		size.Files++
		size.Bytes += f.Size

		if isCodeLanguage(lang) {
			code.CodeFiles++
			packages[dir] = true
		}

		lines, binary := countBlobLines(f)
		if binary {
			code.BinaryFiles++
		} else {
			code.Lines += lines
			size.Lines += lines
			if isCodeLanguage(lang) {
				code.CodeLines += lines
			}
		}

		if set, ok := readDependencies(f); ok {
			deps = append(deps, set)
		}
		return nil
	})
	if err != nil {
		return CodeSnapshot{}, err
	}

	code.Directories = len(dirs)
	code.Packages = len(packages)
	code.Languages = sortedLanguages(langs)
	code.Dependencies = sortDependencies(deps)
	code.DirectDeps, code.TotalDeps = countDependencies(code.Dependencies)
	return code, nil
}

// sortedLanguages flattens the language map, biggest (in lines, then bytes)
// first.
func sortedLanguages(langs map[string]*LanguageSize) []LanguageSize {
	out := make([]LanguageSize, 0, len(langs))
	for _, l := range langs {
		out = append(out, *l)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Lines != out[j].Lines {
			return out[i].Lines > out[j].Lines
		}
		if out[i].Bytes != out[j].Bytes {
			return out[i].Bytes > out[j].Bytes
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// isVendored reports whether a tracked path lies inside a vendored directory.
func isVendored(name string) bool {
	for _, segment := range strings.Split(path.Dir(name), "/") {
		if ShouldBeIgnored(segment) {
			return true
		}
	}
	return false
}

// countBlobLines counts the lines of a tracked file, reporting binary (or
// oversized) content instead of a line count.
func countBlobLines(f *object.File) (lines int, binary bool) {
	if f.Size > maxBlobScan {
		return 0, true
	}
	reader, err := f.Reader()
	if err != nil {
		return 0, true
	}
	defer func() { _ = reader.Close() }()
	return scanLines(reader)
}

// scanLines counts the newline-terminated lines of a reader (plus a trailing
// unterminated one), reporting content holding a NUL byte in its first bytes
// as binary — the same heuristic git itself uses.
func scanLines(r io.Reader) (lines int, binary bool) {
	buf := make([]byte, 32*1024)
	first, last := true, byte('\n')
	total := 0
	for {
		n, err := r.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if first {
				first = false
				head := chunk
				if len(head) > 8000 {
					head = head[:8000]
				}
				if bytes.IndexByte(head, 0) >= 0 {
					return 0, true
				}
			}
			lines += bytes.Count(chunk, []byte{'\n'})
			last = chunk[n-1]
			total += n
		}
		if err != nil {
			break
		}
	}
	if total > 0 && last != '\n' {
		lines++
	}
	return lines, false
}

// AggregateIdentities merges the per-repository cards into the identity card
// of the whole scanned set: counts are summed, contributors and active days
// are de-duplicated across repositories, and dependencies are grouped per
// ecosystem.
func AggregateIdentities(cards []RepositoryIdentity) IdentityOverview {
	all := IdentityOverview{}
	authors := map[string]*authorTally{}
	days := map[string]bool{}
	langs := map[string]*LanguageSize{}
	ecosystems := map[string]*DependencySet{}

	for _, card := range cards {
		if card.Error != "" {
			all.Failed++
			continue
		}
		all.Repositories++
		all.Tags += card.Tags
		all.Branches += card.Branches

		h := card.History
		all.History.Commits += h.Commits
		all.History.MergeCommits += h.MergeCommits
		if !h.FirstCommit.IsZero() && (all.History.FirstCommit.IsZero() || h.FirstCommit.Before(all.History.FirstCommit)) {
			all.History.FirstCommit = h.FirstCommit
		}
		if h.LastCommit.After(all.History.LastCommit) {
			all.History.LastCommit = h.LastCommit
		}
		for key, tally := range h.authors {
			merged := authors[key]
			if merged == nil {
				merged = &authorTally{email: tally.email, names: map[string]int{}}
				authors[key] = merged
			}
			merged.commits += tally.commits
			for name, n := range tally.names {
				merged.names[name] += n
			}
		}
		for day := range h.days {
			days[day] = true
		}

		c := card.Code
		all.Code.Files += c.Files
		all.Code.CodeFiles += c.CodeFiles
		all.Code.Lines += c.Lines
		all.Code.CodeLines += c.CodeLines
		all.Code.BinaryFiles += c.BinaryFiles
		all.Code.Bytes += c.Bytes
		all.Code.Vendored += c.Vendored
		all.Code.Directories += c.Directories
		all.Code.Packages += c.Packages
		for _, l := range c.Languages {
			size := langs[l.Name]
			if size == nil {
				size = &LanguageSize{Name: l.Name}
				langs[l.Name] = size
			}
			size.Files += l.Files
			size.Lines += l.Lines
			size.Bytes += l.Bytes
		}
		for _, d := range c.Dependencies {
			set := ecosystems[d.Ecosystem]
			if set == nil {
				set = &DependencySet{Ecosystem: d.Ecosystem}
				ecosystems[d.Ecosystem] = set
			}
			set.Direct += d.Direct
			set.Indirect += d.Indirect
			set.Dev += d.Dev
			set.Total += d.Total
		}
	}

	all.History.authors = authors
	all.History.days = days
	all.History.finalize()

	all.Code.Languages = sortedLanguages(langs)
	merged := make([]DependencySet, 0, len(ecosystems))
	for _, set := range ecosystems {
		merged = append(merged, *set)
	}
	all.Code.Dependencies = sortDependencies(merged)
	all.Code.DirectDeps, all.Code.TotalDeps = countDependencies(all.Code.Dependencies)
	return all
}
