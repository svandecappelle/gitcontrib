package stats

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/schollz/progressbar/v3"
)

// emptyRepo is a git repository holding no commit at all.
func emptyRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "fresh")
	initRepo(t, dir)
	return dir
}

// A repository with no commit used to end the process: go-git answers
// "reference not found" for its unborn HEAD, and that was a log.Fatalf — which
// took the web server down with it. It must simply hold no contribution.
func TestScanRepositoryWithoutAnyCommit(t *testing.T) {
	dir := emptyRepo(t)
	if !IsRepo(dir) {
		t.Fatalf("%s should be a repository", dir)
	}

	results := Launch(LaunchOptions{
		DurationInWeeks: 4,
		Folders:         []string{dir},
		Dashboard:       true, // silent: this is not a terminal run
	})
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Error != nil {
		t.Errorf("a repository with no commit is not an error, got %v", results[0].Error)
	}

	agg := Aggregate(results)
	if agg.TotalCommits != 0 || agg.Errors != 0 || agg.AnalyzedRepos != 1 {
		t.Errorf("aggregate = %d commits, %d errors, %d repos; want 0, 0, 1",
			agg.TotalCommits, agg.Errors, agg.AnalyzedRepos)
	}
}

// An empty repository still has an identity card: it just has no history.
func TestIdentityOfRepositoryWithoutAnyCommit(t *testing.T) {
	card := BuildIdentity(emptyRepo(t))
	if card.Error == "" {
		t.Error("the card should say that the repository has no HEAD")
	}
	if card.History.Commits != 0 || card.Code.Files != 0 {
		t.Errorf("an empty repository should hold nothing, got %+v", card)
	}

	// It does not spoil the overview of the repositories that do have commits.
	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	all := AggregateIdentities([]RepositoryIdentity{card, BuildIdentity(repo)})
	if all.Repositories != 1 || all.Failed != 1 {
		t.Errorf("overview = %d repositories, %d failed; want 1 and 1", all.Repositories, all.Failed)
	}
	if all.History.Commits == 0 {
		t.Error("the readable repository should still be counted")
	}
}

// A folder that is not a repository at all is reported as an error, without
// bringing the scan of the other folders down.
func TestScanFolderThatIsNotARepository(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	r := &StatsResult{
		Options:        StatsOptions{Folders: []string{dir}, Silent: true},
		DurationInDays: 28,
	}
	populateDurationInDays(LaunchOptions{DurationInWeeks: 4}, r)
	wg.Add(1)
	Stats(r, &wg, progressbar.NewOptions(-1))
	if r.Error == nil {
		t.Error("a folder that is not a repository should be reported as an error")
	}
}
