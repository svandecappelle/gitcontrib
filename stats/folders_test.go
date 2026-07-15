package stats

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
)

func initRepo(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := git.PlainInit(path, false); err != nil {
		t.Fatalf("PlainInit(%s): %v", path, err)
	}
}

func TestExpandFolders(t *testing.T) {
	base := t.TempDir()
	parent := filepath.Join(base, "parent")
	repoA := filepath.Join(parent, "repoA")
	repoB := filepath.Join(parent, "repoB")
	nested := filepath.Join(parent, "plain", "nested") // two levels deep
	solo := filepath.Join(base, "solo")
	empty := filepath.Join(base, "empty")

	initRepo(t, repoA)
	initRepo(t, repoB)
	initRepo(t, nested)
	initRepo(t, solo)
	if err := os.MkdirAll(empty, 0755); err != nil {
		t.Fatal(err)
	}

	got := ExpandFolders([]string{parent, solo, empty})

	want := map[string]bool{
		repoA: true, // direct repo subfolder of a non-repo parent
		repoB: true,
		solo:  true, // already a repo, kept as-is
		empty: true, // no repo subfolders, kept as-is
	}
	if len(got) != len(want) {
		t.Fatalf("ExpandFolders = %v, want keys %v", got, want)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected folder %q (nested repos must not be found)", g)
		}
	}
	for _, g := range got {
		if g == nested {
			t.Errorf("two-levels-deep repo %q should not be expanded", nested)
		}
	}
}
