package stats

import (
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"
)

// testCache builds a statistics cache over folders, with the given groups.
func testCache(t *testing.T, folders []string, groups ...RepoGroup) *statsCache {
	t.Helper()
	store := newGroupStore(filepath.Join(t.TempDir(), "groups.json"))
	if len(groups) > 0 {
		if _, err := store.replace(groups); err != nil {
			t.Fatal(err)
		}
	}
	return newStatsCache(LaunchOptions{Folders: folders, DurationInWeeks: 4}, time.Minute, "", store)
}

func TestRepoSelectionAcceptsBothSpellings(t *testing.T) {
	cases := map[string][]string{
		"repo=/a&repo=/b": {"/a", "/b"}, // repeated
		"repo=/a,/b":      {"/a", "/b"}, // comma-separated
		"repos=/a&repo=b": {"b", "/a"},  // both names, "repo" first
		"weeks=4":         nil,
	}
	for query, want := range cases {
		values, err := url.ParseQuery(query)
		if err != nil {
			t.Fatal(err)
		}
		got := repoSelection(values)
		if len(got) != len(want) {
			t.Errorf("repoSelection(%q) = %v, want %v", query, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("repoSelection(%q) = %v, want %v", query, got, want)
				break
			}
		}
	}
}

func TestFolderSelection(t *testing.T) {
	folders := []string{"/wd/ui", "/wd/admin", "/wd/api"}
	cache := testCache(t, folders,
		RepoGroup{Name: "Frontend", Repositories: []string{"/wd/admin", "/wd/ui", "/wd/gone"}},
		RepoGroup{Name: "Gone", Repositories: []string{"/wd/gone"}},
	)

	// No selection: every scanned folder.
	got, err := cache.folderSelection(Params{})
	if err != nil || len(got) != 3 {
		t.Errorf("an empty selection should cover every folder, got %v (%v)", got, err)
	}

	// An explicit selection, put back in the configured order.
	got, err = cache.folderSelection(Params{Repos: []string{"/wd/api", "/wd/ui"}})
	if err != nil || len(got) != 2 || got[0] != "/wd/ui" || got[1] != "/wd/api" {
		t.Errorf("selection = %v (%v), want [/wd/ui /wd/api]", got, err)
	}

	// A group is expanded to the repositories it names that are still scanned.
	got, err = cache.folderSelection(Params{Group: "frontend"})
	if err != nil || len(got) != 2 || got[0] != "/wd/ui" || got[1] != "/wd/admin" {
		t.Errorf("group selection = %v (%v), want [/wd/ui /wd/admin]", got, err)
	}

	// An explicit selection wins over a group.
	got, err = cache.folderSelection(Params{Repos: []string{"/wd/api"}, Group: "Frontend"})
	if err != nil || len(got) != 1 || got[0] != "/wd/api" {
		t.Errorf("selection = %v (%v), want [/wd/api]", got, err)
	}

	if _, err := cache.folderSelection(Params{Repos: []string{"/elsewhere"}}); err == nil {
		t.Error("selecting a folder that is not scanned should be an error")
	}
	if _, err := cache.folderSelection(Params{Group: "Unknown"}); err == nil {
		t.Error("selecting an unknown group should be an error")
	}
	if _, err := cache.folderSelection(Params{Group: "Gone"}); err == nil {
		t.Error("a group holding no scanned repository should be an error")
	}
}

func TestResolveAppliesTheSelection(t *testing.T) {
	folders := []string{"/wd/ui", "/wd/admin", "/wd/api"}
	cache := testCache(t, folders, RepoGroup{Name: "Frontend", Repositories: []string{"/wd/ui", "/wd/admin"}})

	opts, err := cache.resolve(httptest.NewRequest("GET", "/api/stats?group=Frontend&weeks=8", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts.Folders) != 2 || opts.DurationInWeeks != 8 {
		t.Errorf("options = %+v, want the two Frontend folders over 8 weeks", opts)
	}

	// The applied parameters echo the selection, named after the group it is.
	applied := cache.paramsOf(opts)
	if len(applied.Repos) != 2 || applied.Group != "Frontend" {
		t.Errorf("applied params = %+v, want the selection and its group name", applied)
	}

	// The full set is not a selection.
	all, err := cache.resolve(httptest.NewRequest("GET", "/api/stats?weeks=8", nil))
	if err != nil {
		t.Fatal(err)
	}
	if applied := cache.paramsOf(all); len(applied.Repos) != 0 || applied.Group != "" {
		t.Errorf("applied params = %+v, want no selection", applied)
	}

	if _, err := cache.resolve(httptest.NewRequest("GET", "/api/stats?repo=/elsewhere", nil)); err == nil {
		t.Error("an unknown repository should be rejected")
	}
	if _, err := cache.resolve(httptest.NewRequest("GET", "/api/stats?delta=nope", nil)); err == nil {
		t.Error("an invalid delta should be rejected")
	}
}

func TestIdentityFoldersFollowsTheSelection(t *testing.T) {
	folders := []string{"/wd/ui", "/wd/admin", "/wd/api"}
	cache := testCache(t, folders, RepoGroup{Name: "Frontend", Repositories: []string{"/wd/ui", "/wd/admin"}})

	got, err := cache.identityFolders(httptest.NewRequest("GET", "/api/identity?repo=/wd/api&repo=/wd/ui", nil))
	if err != nil || len(got) != 2 {
		t.Errorf("identity folders = %v (%v), want two folders", got, err)
	}
	got, err = cache.identityFolders(httptest.NewRequest("GET", "/api/identity?group=Frontend", nil))
	if err != nil || len(got) != 2 || got[0] != "/wd/ui" {
		t.Errorf("identity folders = %v (%v), want the Frontend folders", got, err)
	}
	got, err = cache.identityFolders(httptest.NewRequest("GET", "/api/identity", nil))
	if err != nil || len(got) != 3 {
		t.Errorf("identity folders = %v (%v), want every folder", got, err)
	}
}

func TestCacheKeyFollowsTheSelection(t *testing.T) {
	folders := []string{"/wd/ui", "/wd/admin"}
	cache := testCache(t, folders)
	all := cache.optsFor(Params{}, folders)
	one := cache.optsFor(Params{Repos: []string{"/wd/ui"}}, []string{"/wd/ui"})
	if cacheKey(all) == cacheKey(one) {
		t.Error("two different selections should be cached apart")
	}
}
