package stats

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckTokenRefusesWhileEditingIsDisabled(t *testing.T) {
	srv := ServeOptions{} // no token configured
	r := httptest.NewRequest("PUT", "/api/config", nil)
	r.Header.Set(editTokenHeader, "anything")
	if err := srv.checkToken(r); err != errEditingDisabled {
		t.Errorf("checkToken = %v, want editing to be disabled", err)
	}

	w := httptest.NewRecorder()
	if allowEdit(w, r, srv) {
		t.Error("allowEdit should refuse")
	}
	if w.Code != 403 {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestCheckTokenMatchesTheConfiguredOne(t *testing.T) {
	srv := ServeOptions{EditToken: "s3cret"}
	cases := map[string]bool{"s3cret": true, "S3CRET": false, "": false, "s3cre": false}
	for token, want := range cases {
		r := httptest.NewRequest("PUT", "/api/config", nil)
		if token != "" {
			r.Header.Set(editTokenHeader, token)
		}
		if got := srv.checkToken(r) == nil; got != want {
			t.Errorf("token %q accepted = %t, want %t", token, got, want)
		}
	}

	w := httptest.NewRecorder()
	if allowEdit(w, httptest.NewRequest("PUT", "/api/config", nil), srv) {
		t.Error("allowEdit should refuse a request with no token")
	}
	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestConfigStateNeverExposesTheToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	content := `{"weeks":12,"folders":["/wd/a"],"web":{"editToken":"s3cret","ttl":"5m"}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	srv := ServeOptions{ConfigFile: path, EditToken: "s3cret", Addr: ":8080"}
	cache := testCache(t, []string{"/wd/a"})

	r := httptest.NewRequest("GET", "/api/config", nil)
	state := configState(srv, cache, r)
	if !state.Editable || state.Authorized {
		t.Errorf("state = %+v, want editable but not authorized", state)
	}
	if state.Config.Weeks == nil || *state.Config.Weeks != 12 || state.Config.TTL == nil {
		t.Errorf("the editable config should be reported, got %+v", state.Config)
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "s3cret") {
		t.Fatalf("the edit token leaked into the response: %s", raw)
	}

	r.Header.Set(editTokenHeader, "s3cret")
	if !configState(srv, cache, r).Authorized {
		t.Error("a request carrying the token should be reported as authorized")
	}
}

func TestSaveConfigKeepsWhatTheUIDoesNotEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	content := `{
	  "weeks": 12,
	  "user": "someone@example.com",
	  "web": { "editToken": "s3cret", "addr": ":9000", "ttl": "5m" },
	  "somethingElse": {"kept": true}
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	weeks := 4
	ttl := "30s"
	saved, err := SaveConfig(path, EditableConfig{
		Weeks:   &weeks,
		TTL:     &ttl,
		Folders: []string{"/wd/a"},
		// user, delta, countAll and merge are left unset: their keys go away.
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved.Weeks == nil || *saved.Weeks != 4 {
		t.Errorf("weeks = %v, want 4", saved.Weeks)
	}
	if saved.User != nil {
		t.Errorf("user should have been removed, got %v", *saved.User)
	}
	if saved.Web.EditToken == nil || *saved.Web.EditToken != "s3cret" {
		t.Error("the edit token must survive a save from the UI")
	}
	if saved.Web.Addr == nil || *saved.Web.Addr != ":9000" {
		t.Error("a web value the UI does not edit must survive a save")
	}
	if saved.Web.TTL == nil || *saved.Web.TTL != "30s" {
		t.Errorf("ttl = %v, want 30s", saved.Web.TTL)
	}

	// Keys gitcontrib knows nothing about are kept as they were.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var content2 map[string]interface{}
	if err := json.Unmarshal(raw, &content2); err != nil {
		t.Fatalf("the config file should stay valid JSON: %v", err)
	}
	if _, ok := content2["somethingElse"]; !ok {
		t.Errorf("an unknown key should be preserved, file is now: %s", raw)
	}
}

func TestSaveConfigCreatesAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	weeks := 8
	if _, err := SaveConfig(path, EditableConfig{Weeks: &weeks, Folders: []string{"/wd/a"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg, err := LoadConfig(path)
	if err != nil || cfg.Weeks == nil || *cfg.Weeks != 8 {
		t.Errorf("the saved config should be readable again, got %+v (%v)", cfg, err)
	}
}

func TestEditableConfigValidate(t *testing.T) {
	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	bad := "nope"
	tooMany := 900
	badTTL := "5 lightyears"
	cases := map[string]EditableConfig{
		"an invalid delta":   {Delta: &bad},
		"too many weeks":     {Weeks: &tooMany},
		"an invalid ttl":     {TTL: &badTTL},
		"an invalid regex":   {IncludePatterns: []string{"("}},
		"a missing folder":   {Folders: []string{filepath.Join(t.TempDir(), "gone")}},
		"a folder with none": {Folders: []string{t.TempDir()}},
	}
	for name, edits := range cases {
		if _, err := edits.Validate(); err == nil {
			t.Errorf("%s should be rejected", name)
		}
	}

	folders, err := EditableConfig{Folders: []string{repo, repo}}.Validate()
	if err != nil {
		t.Fatalf("a repository should be accepted: %v", err)
	}
	if len(folders) != 1 || folders[0] != repo {
		t.Errorf("folders = %v, want the repository once", folders)
	}
}

func TestResolveRepositories(t *testing.T) {
	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	// A repository resolves to itself.
	if got, err := ResolveRepositories(repo); err != nil || len(got) != 1 || got[0] != repo {
		t.Errorf("ResolveRepositories(repo) = %v (%v), want the repository itself", got, err)
	}
	// Its parent resolves to the repositories it holds, this one included.
	got, err := ResolveRepositories(filepath.Dir(repo))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsFolder(got, repo) {
		t.Errorf("the parent folder should resolve to %s, got %v", repo, got)
	}
	for _, path := range []string{"", filepath.Join(t.TempDir(), "gone"), t.TempDir()} {
		if _, err := ResolveRepositories(path); err == nil {
			t.Errorf("%q should not resolve to a repository", path)
		}
	}
}

func TestApplySavedConfigTakesEffectAtOnce(t *testing.T) {
	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"web":{"editToken":"s3cret"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	cache := testCache(t, []string{"/wd/gone"})
	identities := newIdentityCache(time.Minute)

	weeks := 6
	ttl := "30s"
	countAll := true
	edits := EditableConfig{Weeks: &weeks, TTL: &ttl, CountAll: &countAll, Folders: []string{repo}}
	if err := applySavedConfig(path, edits, cache, identities); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	opts := cache.options()
	if len(opts.Folders) != 1 || opts.Folders[0] != repo {
		t.Errorf("the server should now scan %s, got %v", repo, opts.Folders)
	}
	if opts.DurationInWeeks != 6 {
		t.Errorf("weeks = %d, want 6", opts.DurationInWeeks)
	}
	if opts.User != nil {
		t.Error("countAll should have cleared the user filter")
	}
	if cache.lifetime() != 30*time.Second {
		t.Errorf("ttl = %s, want 30s", cache.lifetime())
	}
	// The token is still there, so the server stays editable.
	if cfg, err := LoadConfig(path); err != nil || cfg.Web.EditToken == nil {
		t.Errorf("the edit token should have survived: %+v (%v)", cfg, err)
	}

	// Edits that cannot be used are rejected, and change nothing.
	if err := applySavedConfig(path, EditableConfig{Folders: nil}, cache, identities); err == nil {
		t.Error("a config naming no repository should be rejected")
	} else if statusForConfigError(err) != 400 {
		t.Errorf("status = %d, want 400", statusForConfigError(err))
	}
	if len(cache.options().Folders) != 1 {
		t.Error("a rejected config should leave the server untouched")
	}
}
