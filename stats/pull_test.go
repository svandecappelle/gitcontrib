package stats

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBranchLine(t *testing.T) {
	cases := []struct {
		line     string
		branch   string
		upstream string
		ahead    int
		behind   int
	}{
		{"master...origin/master", "master", "origin/master", 0, 0},
		{"master...origin/master [behind 2]", "master", "origin/master", 0, 2},
		{"develop...origin/develop [ahead 1, behind 3]", "develop", "origin/develop", 1, 3},
		{"feature/x", "feature/x", "", 0, 0},
		{"HEAD (no branch)", "", "", 0, 0},
	}
	for _, c := range cases {
		var status RepositoryStatus
		parseBranchLine(c.line, &status)
		if status.Branch != c.branch || status.Upstream != c.upstream ||
			status.Ahead != c.ahead || status.Behind != c.behind {
			t.Errorf("parseBranchLine(%q) = %+v, want %s/%s ahead %d behind %d",
				c.line, status, c.branch, c.upstream, c.ahead, c.behind)
		}
	}
}

func TestPullFailureExplainsTheUsualCauses(t *testing.T) {
	cases := map[string]string{
		"hint: Diverging branches can't be fast-forwarded":  "diverged",
		"Not possible to fast-forward, aborting.":           "diverged",
		"error: Your local changes would be overwritten by": "overwritten",
		"git@host: Permission denied (publickey).":          "refused the connection",
		"fatal: could not read Username for 'https://x'":    "refused the connection",
		"ssh: Could not resolve hostname host":              "could not be reached",
	}
	for output, want := range cases {
		if got := pullFailure(output, errEditingDisabled); !strings.Contains(got, want) {
			t.Errorf("pullFailure(%q) = %q, want it to mention %q", output, got, want)
		}
	}

	// Anything else keeps git's own words, without the noise of the fetch.
	out := "From ssh://host/repo\n   1234567..89abcde  master     -> origin/master\nfatal: something odd happened"
	if got := pullFailure(out, errEditingDisabled); got != "fatal: something odd happened" {
		t.Errorf("pullFailure = %q, want the failing line", got)
	}
}

// gitFixture builds a bare "remote" plus a clone of it holding one commit, and
// returns both paths. It skips the test when git is not installed, since
// updating repositories is git's job.
func gitFixture(t *testing.T) (origin, clone string) {
	t.Helper()
	if err := gitAvailable(); err != nil {
		t.Skip(err)
	}
	base := t.TempDir()
	origin = filepath.Join(base, "origin.git")
	clone = filepath.Join(base, "clone")

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Tester", "GIT_AUTHOR_EMAIL=t@e",
			"GIT_COMMITTER_NAME=Tester", "GIT_COMMITTER_EMAIL=t@e")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v (%s)", strings.Join(args, " "), err, out)
		}
	}

	run(base, "init", "--quiet", "--bare", origin)
	run(base, "clone", "--quiet", origin, clone)
	if err := os.WriteFile(filepath.Join(clone, "a.txt"), []byte("one\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run(clone, "add", "a.txt")
	run(clone, "commit", "--quiet", "-m", "feat: one")
	run(clone, "push", "--quiet", "origin", "HEAD:master")
	run(clone, "branch", "--set-upstream-to=origin/master")
	return origin, clone
}

func TestStatusAndPull(t *testing.T) {
	origin, clone := gitFixture(t)

	status := StatusOf(clone)
	if status.Error != "" {
		t.Fatalf("unexpected error: %s", status.Error)
	}
	if !status.Clean || !status.Pullable || status.Upstream != "origin/master" {
		t.Fatalf("a fresh clone should be clean and pullable, got %+v", status)
	}

	// A commit lands on the remote through another clone.
	second := filepath.Join(t.TempDir(), "second")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Tester", "GIT_AUTHOR_EMAIL=t@e",
			"GIT_COMMITTER_NAME=Tester", "GIT_COMMITTER_EMAIL=t@e")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v (%s)", strings.Join(args, " "), err, out)
		}
	}
	run(filepath.Dir(second), "clone", "--quiet", origin, second)
	if err := os.WriteFile(filepath.Join(second, "a.txt"), []byte("one\ntwo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run(second, "commit", "--quiet", "-am", "feat: two")
	run(second, "push", "--quiet", "origin", "HEAD:master")

	// The clone fast-forwards onto it.
	before, _ := runGit(clone, statusTimeout, "rev-parse", "HEAD")
	result := PullRepository(clone)
	if result.Error != "" || result.Skipped {
		t.Fatalf("the clone should have been updated, got %+v", result)
	}
	after, _ := runGit(clone, statusTimeout, "rev-parse", "HEAD")
	if !result.Updated || before == after {
		t.Errorf("HEAD should have moved: %+v", result)
	}

	// Pulling again changes nothing, and says so.
	if again := PullRepository(clone); again.Updated || again.Error != "" {
		t.Errorf("a second pull should be a no-op, got %+v", again)
	}
}

func TestPullLeavesARepositoryWithLocalChangesAlone(t *testing.T) {
	_, clone := gitFixture(t)

	if err := os.WriteFile(filepath.Join(clone, "a.txt"), []byte("edited\n"), 0644); err != nil {
		t.Fatal(err)
	}
	status := StatusOf(clone)
	if status.Clean || status.Pullable || status.Changes != 1 {
		t.Fatalf("a modified worktree should not be pullable, got %+v", status)
	}
	if !strings.Contains(status.Reason, "local change") {
		t.Errorf("reason = %q, want it to mention the local change", status.Reason)
	}

	before, _ := runGit(clone, statusTimeout, "rev-parse", "HEAD")
	result := PullRepository(clone)
	if !result.Skipped || result.Updated {
		t.Errorf("the repository should have been left alone, got %+v", result)
	}
	after, _ := runGit(clone, statusTimeout, "rev-parse", "HEAD")
	if before != after {
		t.Error("HEAD moved although the worktree held local changes")
	}
	// The local change is still there.
	content, err := os.ReadFile(filepath.Join(clone, "a.txt"))
	if err != nil || string(content) != "edited\n" {
		t.Errorf("the local change should be untouched, file holds %q (%v)", content, err)
	}
}

func TestUntrackedFilesDoNotHoldAnUpdateBack(t *testing.T) {
	_, clone := gitFixture(t)
	if err := os.WriteFile(filepath.Join(clone, "scratch.txt"), []byte("not tracked\n"), 0644); err != nil {
		t.Fatal(err)
	}
	status := StatusOf(clone)
	if !status.Clean || !status.Pullable {
		t.Errorf("an untracked file should not hold an update back, got %+v", status)
	}
	if status.Untracked != 1 {
		t.Errorf("untracked = %d, want 1", status.Untracked)
	}
}

func TestPullRepositoriesReportsEachOneInOrder(t *testing.T) {
	_, first := gitFixture(t)
	_, second := gitFixture(t)
	if err := os.WriteFile(filepath.Join(second, "a.txt"), []byte("edited\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results := PullRepositories([]string{first, second})
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Folder != first || results[1].Folder != second {
		t.Errorf("results are not in the order asked: %+v", results)
	}
	if results[0].Skipped {
		t.Errorf("the clean repository should have been pulled, got %+v", results[0])
	}
	if !results[1].Skipped {
		t.Errorf("the modified repository should have been skipped, got %+v", results[1])
	}
}

func TestStatusOfAFolderThatIsNotARepository(t *testing.T) {
	if err := gitAvailable(); err != nil {
		t.Skip(err)
	}
	status := StatusOf(t.TempDir())
	if status.Error == "" {
		t.Error("a folder that is not a repository should be reported as an error")
	}
	if status.Pullable {
		t.Error("it should certainly not be pullable")
	}
}
