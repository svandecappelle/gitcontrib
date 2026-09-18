package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGroupStoreLoadsMissingFileAsEmpty(t *testing.T) {
	store := newGroupStore(filepath.Join(t.TempDir(), "groups.json"))
	if err := store.load(); err != nil {
		t.Fatalf("a missing groups file should not be an error, got %v", err)
	}
	if len(store.all()) != 0 {
		t.Errorf("the store should start empty, got %+v", store.all())
	}
}

func TestGroupStoreLoadsAndFinds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "groups.json")
	content := `{"groups":[{"name":"Frontend","repositories":["/wd/ui","/wd/admin"]}]}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	store := newGroupStore(path)
	if err := store.load(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	group, ok := store.find("frontend") // a link's group name is case-insensitive
	if !ok || len(group.Repositories) != 2 {
		t.Fatalf("find(frontend) = %+v, %t, want the Frontend group", group, ok)
	}
	if _, ok := store.find("Backend"); ok {
		t.Error("an unknown group should not be found")
	}
}

func TestGroupStoreLoadRejectsInvalidFiles(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"broken.json":  "{ not json",
		"unnamed.json": `{"groups":[{"name":"","repositories":["/wd/a"]}]}`,
		"empty.json":   `{"groups":[{"name":"Empty","repositories":[]}]}`,
		"twice.json":   `{"groups":[{"name":"A","repositories":["/x"]},{"name":"a","repositories":["/y"]}]}`,
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		store := newGroupStore(path)
		if err := store.load(); err == nil {
			t.Errorf("%s should be reported as invalid", name)
		}
		if len(store.all()) != 0 {
			t.Errorf("%s should leave the store empty", name)
		}
	}
}

func TestGroupStoreReplaceWritesTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "groups.json")
	store := newGroupStore(path)

	saved, err := store.replace([]RepoGroup{
		{Name: "  Frontend  ", Repositories: []string{"/wd/ui", "/wd/ui", " ", "/wd/admin"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(saved) != 1 || saved[0].Name != "Frontend" {
		t.Fatalf("names should be trimmed, got %+v", saved)
	}
	if len(saved[0].Repositories) != 2 {
		t.Errorf("repositories should be de-duplicated, got %+v", saved[0].Repositories)
	}

	// The file is readable again, and holds what was saved.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the groups file should have been created: %v", err)
	}
	var content groupsFile
	if err := json.Unmarshal(raw, &content); err != nil {
		t.Fatalf("the groups file should be valid JSON: %v", err)
	}
	if len(content.Groups) != 1 || content.Groups[0].Name != "Frontend" {
		t.Errorf("the file holds %+v", content.Groups)
	}

	// Replacing sends the whole list: what is left out is forgotten.
	if _, err := store.replace([]RepoGroup{{Name: "Backend", Repositories: []string{"/wd/api"}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if groups := store.all(); len(groups) != 1 || groups[0].Name != "Backend" {
		t.Errorf("groups = %+v, want only Backend", groups)
	}
}

func TestGroupStoreReplaceRejectsUnusableGroups(t *testing.T) {
	store := newGroupStore(filepath.Join(t.TempDir(), "groups.json"))
	cases := map[string][]RepoGroup{
		"no name":        {{Name: " ", Repositories: []string{"/wd/a"}}},
		"no repository":  {{Name: "Empty", Repositories: []string{" "}}},
		"twice the same": {{Name: "A", Repositories: []string{"/x"}}, {Name: "a", Repositories: []string{"/y"}}},
	}
	for name, groups := range cases {
		if _, err := store.replace(groups); err == nil {
			t.Errorf("a group with %s should be rejected", name)
		}
	}
	if len(store.all()) != 0 {
		t.Errorf("a rejected payload should leave the store untouched, got %+v", store.all())
	}
}

func TestGroupStoreNameOf(t *testing.T) {
	store := newGroupStore(filepath.Join(t.TempDir(), "groups.json"))
	if _, err := store.replace([]RepoGroup{
		// The group names a repository that is not scanned any more.
		{Name: "Frontend", Repositories: []string{"/wd/ui", "/wd/admin", "/wd/gone"}},
		{Name: "Backend", Repositories: []string{"/wd/api"}},
	}); err != nil {
		t.Fatal(err)
	}
	available := []string{"/wd/ui", "/wd/admin", "/wd/api"}

	if name := store.nameOf([]string{"/wd/ui", "/wd/admin"}, available); name != "Frontend" {
		t.Errorf("nameOf = %q, want Frontend: a group is matched on its scanned repositories", name)
	}
	if name := store.nameOf([]string{"/wd/ui"}, available); name != "" {
		t.Errorf("nameOf = %q, want no group for a selection matching none", name)
	}
	if name := store.nameOf(nil, available); name != "" {
		t.Errorf("nameOf = %q, want no group for an empty selection", name)
	}
}

func TestIntersectFolders(t *testing.T) {
	available := []string{"/a", "/b", "/c"}
	// The configured order wins, and unknown folders are dropped.
	got := intersectFolders([]string{"/c", "/unknown", "/a"}, available)
	if len(got) != 2 || got[0] != "/a" || got[1] != "/c" {
		t.Errorf("intersectFolders = %v, want [/a /c]", got)
	}
}

func TestInvalidGroupsIsRecognizedAsTheClientsMistake(t *testing.T) {
	store := newGroupStore(filepath.Join(t.TempDir(), "groups.json"))
	_, err := store.replace([]RepoGroup{{Name: "", Repositories: []string{"/x"}}})
	if err == nil {
		t.Fatal("an unnamed group should be rejected")
	}
	if got := statusForGroupError(err); got != 400 {
		t.Errorf("statusForGroupError(%v) = %d, want 400", err, got)
	}

	// A store with nowhere to write fails on the server's side, not the
	// client's.
	nowhere := newGroupStore("")
	_, err = nowhere.replace([]RepoGroup{{Name: "A", Repositories: []string{"/x"}}})
	if err == nil {
		t.Fatal("a store with no file should fail to save")
	}
	if got := statusForGroupError(err); got != 500 {
		t.Errorf("statusForGroupError(%v) = %d, want 500", err, got)
	}
}
