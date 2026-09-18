package stats

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanLines(t *testing.T) {
	cases := []struct {
		name    string
		content string
		lines   int
		binary  bool
	}{
		{"empty", "", 0, false},
		{"one terminated line", "a\n", 1, false},
		{"unterminated last line", "a\nb", 2, false},
		{"blank lines count", "\n\n\n", 3, false},
		{"NUL byte is binary", "PK\x03\x04\x00\x00binary", 0, true},
	}
	for _, c := range cases {
		lines, binary := scanLines(strings.NewReader(c.content))
		if lines != c.lines || binary != c.binary {
			t.Errorf("scanLines(%s) = (%d, %t), want (%d, %t)", c.name, lines, binary, c.lines, c.binary)
		}
	}
}

func TestRepoName(t *testing.T) {
	if got := repoName("/home/user/wd/project"); got != "project" {
		t.Errorf(`repoName("/home/user/wd/project") = %q, want "project"`, got)
	}
	if got := repoName("/home/user/wd/project/"); got != "project" {
		t.Errorf("a trailing slash should not change the name, got %q", got)
	}
	// A relative folder is named after the directory it resolves to.
	wd, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	if got := repoName("."); got != filepath.Base(wd) {
		t.Errorf(`repoName(".") = %q, want %q`, got, filepath.Base(wd))
	}
}

func TestIsVendored(t *testing.T) {
	cases := map[string]bool{
		"stats/web.go":                   false,
		"vendor/github.com/x/y.go":       true,
		"ui/node_modules/react/index.js": true,
		"venv/lib/python3/site.py":       true,
		"src/vendorish/file.go":          false,
	}
	for path, want := range cases {
		if got := isVendored(path); got != want {
			t.Errorf("isVendored(%q) = %t, want %t", path, got, want)
		}
	}
}

func TestIsCodeLanguage(t *testing.T) {
	for _, lang := range []string{"Go", "TypeScript", "Shell"} {
		if !isCodeLanguage(lang) {
			t.Errorf("%s should count as code", lang)
		}
	}
	for _, lang := range []string{"Markdown", "JSON", "YAML", "Other", "lff"} {
		if isCodeLanguage(lang) {
			t.Errorf("%s should not count as code", lang)
		}
	}
}

func TestAuthorGroupKey(t *testing.T) {
	// Identities are grouped by email, whatever the name spelling or case.
	if authorGroupKey("Alice", "a@e") != authorGroupKey("alice", "A@E") {
		t.Error("the same email should group whatever the spelling")
	}
	if authorGroupKey("Alice", "") == authorGroupKey("Bob", "") {
		t.Error("email-less identities should group by name")
	}
}

func TestTopAuthorsRanksAndTruncates(t *testing.T) {
	authors := map[string]*authorTally{
		"email:a@e": {email: "a@e", names: map[string]int{"Alice": 3, "alice": 1}, commits: 4},
		"email:b@e": {email: "b@e", names: map[string]int{"Bob": 9}, commits: 9},
		"email:c@e": {email: "c@e", names: map[string]int{"Carol": 1}, commits: 1},
	}
	top := topAuthors(authors, 2)
	if len(top) != 2 {
		t.Fatalf("topAuthors kept %d authors, want 2", len(top))
	}
	if top[0].Author != "Bob" || top[0].Commits != 9 {
		t.Errorf("the most active author should come first, got %+v", top[0])
	}
	// The displayed name is the most used spelling.
	if top[1].Author != "Alice" {
		t.Errorf("top[1].Author = %q, want %q", top[1].Author, "Alice")
	}
}

func TestBuildIdentityOfThisRepository(t *testing.T) {
	card := BuildIdentity("..")
	if card.Error != "" {
		t.Fatalf("building the identity of this repository failed: %s", card.Error)
	}
	if card.Name != "gitcontrib" {
		t.Errorf("card.Name = %q, want %q", card.Name, "gitcontrib")
	}
	if !filepath.IsAbs(card.Path) {
		t.Errorf("card.Path = %q, want an absolute path", card.Path)
	}
	if card.HeadSHA == "" || card.HeadSubject == "" {
		t.Error("the card should describe the HEAD commit")
	}
	if card.History.Commits == 0 || card.History.Contributors == 0 {
		t.Errorf("the history should hold commits and contributors, got %+v", card.History)
	}
	if card.History.FirstCommit.After(card.History.LastCommit) {
		t.Error("the first commit should not be more recent than the last one")
	}
	if card.History.ActiveDays == 0 || card.History.ActiveDays > card.History.Commits {
		t.Errorf("active days = %d, want between 1 and %d", card.History.ActiveDays, card.History.Commits)
	}
	if len(card.History.TopAuthors) == 0 {
		t.Error("the card should rank its authors")
	}

	code := card.Code
	if code.Files == 0 || code.CodeFiles == 0 || code.CodeLines == 0 || code.Bytes == 0 {
		t.Errorf("the code snapshot should not be empty, got %+v", code)
	}
	if code.CodeLines > code.Lines {
		t.Errorf("code lines (%d) cannot exceed all lines (%d)", code.CodeLines, code.Lines)
	}
	if code.Packages == 0 || code.Packages > code.Directories {
		t.Errorf("packages = %d, want between 1 and %d directories", code.Packages, code.Directories)
	}
	if len(code.Languages) == 0 || code.Languages[0].Name != "Go" {
		t.Errorf("Go should be the main language, got %+v", code.Languages)
	}
	if len(code.Dependencies) == 0 || code.Dependencies[0].Ecosystem != "Go modules" {
		t.Fatalf("the go.mod dependencies should be reported, got %+v", code.Dependencies)
	}
	if code.DirectDeps == 0 || code.TotalDeps < code.DirectDeps {
		t.Errorf("dependencies = %d direct / %d total", code.DirectDeps, code.TotalDeps)
	}
}

func TestBuildIdentityOfAFolderThatIsNotARepository(t *testing.T) {
	card := BuildIdentity(t.TempDir())
	if card.Error == "" {
		t.Error("a folder that is not a repository should yield an error on its card")
	}
	if card.Name == "" {
		t.Error("even a failed card should name its folder")
	}
}

func TestBuildIdentities(t *testing.T) {
	cards := BuildIdentities([]string{"..", t.TempDir()})
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(cards))
	}
	// The order of the folders is kept, whatever the order they finish in.
	if cards[0].Name != "gitcontrib" || cards[1].Error == "" {
		t.Errorf("cards are not in the folders' order: %+v", cards)
	}
}

// identityFixture builds a card holding the given history and code, the way
// BuildIdentity would.
func identityFixture(name string, commits int, first time.Time, authors map[string]*authorTally, days []string, code CodeSnapshot) RepositoryIdentity {
	daySet := map[string]bool{}
	for _, d := range days {
		daySet[d] = true
	}
	history := HistorySnapshot{
		Commits:     commits,
		FirstCommit: first,
		LastCommit:  first.AddDate(0, 0, 1),
		authors:     authors,
		days:        daySet,
	}
	history.finalize()
	return RepositoryIdentity{Name: name, Folder: name, Tags: 1, Branches: 2, History: history, Code: code}
}

func TestAggregateIdentities(t *testing.T) {
	older := time.Date(2020, 3, 1, 12, 0, 0, 0, time.UTC)
	newer := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	first := identityFixture("a", 10, older,
		map[string]*authorTally{
			"email:alice@e": {email: "alice@e", names: map[string]int{"Alice": 7}, commits: 7},
			"email:bob@e":   {email: "bob@e", names: map[string]int{"Bob": 3}, commits: 3},
		},
		[]string{"2020-03-01", "2020-03-02"},
		CodeSnapshot{
			Files: 10, CodeFiles: 8, Lines: 500, CodeLines: 400, Bytes: 1000, Directories: 3, Packages: 2,
			Languages:    []LanguageSize{{Name: "Go", Files: 8, Lines: 400, Bytes: 800}},
			Dependencies: []DependencySet{{Ecosystem: "Go modules", Manifest: "go.mod", Direct: 4, Indirect: 10, Total: 14}},
			DirectDeps:   4, TotalDeps: 14,
		})

	second := identityFixture("b", 5, newer,
		map[string]*authorTally{
			// Alice contributes to both repositories: she is one person.
			"email:alice@e": {email: "alice@e", names: map[string]int{"Alice": 5}, commits: 5},
		},
		[]string{"2020-03-02", "2024-06-01"},
		CodeSnapshot{
			Files: 4, CodeFiles: 4, Lines: 100, CodeLines: 100, Bytes: 200, Directories: 1, Packages: 1,
			Languages:    []LanguageSize{{Name: "Go", Files: 2, Lines: 60, Bytes: 120}, {Name: "Markdown", Files: 2, Lines: 40, Bytes: 80}},
			Dependencies: []DependencySet{{Ecosystem: "Go modules", Manifest: "go.mod", Direct: 1, Indirect: 2, Total: 3}},
			DirectDeps:   1, TotalDeps: 3,
		})

	broken := RepositoryIdentity{Name: "c", Folder: "c", Error: "not a repository"}

	all := AggregateIdentities([]RepositoryIdentity{first, second, broken})

	if all.Repositories != 2 || all.Failed != 1 {
		t.Errorf("got %d repositories and %d failed, want 2 and 1", all.Repositories, all.Failed)
	}
	if all.Tags != 2 || all.Branches != 4 {
		t.Errorf("tags/branches = %d/%d, want 2/4", all.Tags, all.Branches)
	}
	if all.History.Commits != 15 {
		t.Errorf("commits = %d, want 15", all.History.Commits)
	}
	if all.History.Contributors != 2 {
		t.Errorf("contributors = %d, want 2: Alice contributes to both repositories", all.History.Contributors)
	}
	if all.History.ActiveDays != 3 {
		t.Errorf("active days = %d, want 3: a day active in two repositories counts once", all.History.ActiveDays)
	}
	if !all.History.FirstCommit.Equal(older) || !all.History.LastCommit.Equal(newer.AddDate(0, 0, 1)) {
		t.Errorf("the window should span every repository, got %s → %s", all.History.FirstCommit, all.History.LastCommit)
	}
	if len(all.History.TopAuthors) == 0 || all.History.TopAuthors[0].Author != "Alice" || all.History.TopAuthors[0].Commits != 12 {
		t.Errorf("top authors = %+v, want Alice with 12 commits", all.History.TopAuthors)
	}

	if all.Code.Files != 14 || all.Code.CodeLines != 500 || all.Code.Packages != 3 {
		t.Errorf("code = %+v, want 14 files, 500 code lines and 3 packages", all.Code)
	}
	if len(all.Code.Languages) != 2 || all.Code.Languages[0].Name != "Go" || all.Code.Languages[0].Lines != 460 {
		t.Errorf("languages = %+v, want Go first with 460 lines", all.Code.Languages)
	}
	if len(all.Code.Dependencies) != 1 || all.Code.Dependencies[0].Total != 17 {
		t.Errorf("dependencies should be merged per ecosystem, got %+v", all.Code.Dependencies)
	}
	if all.Code.DirectDeps != 5 || all.Code.TotalDeps != 17 {
		t.Errorf("dependencies = %d direct / %d total, want 5 / 17", all.Code.DirectDeps, all.Code.TotalDeps)
	}
}

func TestAggregateIdentitiesOfNothing(t *testing.T) {
	all := AggregateIdentities(nil)
	if all.Repositories != 0 || all.History.Commits != 0 || len(all.Code.Languages) != 0 {
		t.Errorf("aggregating no card should yield an empty overview, got %+v", all)
	}
}
