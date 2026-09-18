package stats

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// editTokenHeader carries the token guarding every change made from the web
// UI. It travels as a header rather than as a query parameter, which would end
// up in logs and browsing history.
const editTokenHeader = "X-Gitcontrib-Token"

var (
	// errEditingDisabled is the answer of a server nobody configured for
	// editing: with no token, its UI can only read.
	errEditingDisabled = errors.New(`editing from the web UI is disabled: set "web": {"editToken": "…"} in the config file and restart`)
	// errBadToken is the answer to a wrong (or missing) token.
	errBadToken = errors.New("invalid edit token")
)

// configRequest is the /api/config payload a client sends.
type configRequest struct {
	Config EditableConfig `json:"config"`
}

// configResponse describes the analysis config to the UI: what it may change,
// what it may only read, and whether this very request is allowed to change
// anything.
type configResponse struct {
	File       string         `json:"file"`
	Config     EditableConfig `json:"config"`
	Folders    []string       `json:"folders"` // the repositories scanned right now
	Web        webInfo        `json:"web"`
	Editable   bool           `json:"editable"`   // a token is configured
	Authorized bool           `json:"authorized"` // this request carried it
}

// webInfo is the server's own settings: they are shown for reference, but
// changing them needs a restart, so the UI does not offer to.
type webInfo struct {
	Addr       string `json:"addr"`
	CacheFile  string `json:"cacheFile"`
	GroupsFile string `json:"groupsFile"`
}

// checkToken reports whether a request may change anything on this server.
func (srv ServeOptions) checkToken(r *http.Request) error {
	if srv.EditToken == "" {
		return errEditingDisabled
	}
	given := r.Header.Get(editTokenHeader)
	if subtle.ConstantTimeCompare([]byte(given), []byte(srv.EditToken)) != 1 {
		return errBadToken
	}
	return nil
}

// allowEdit answers a request that is not allowed to change anything and
// reports whether the handler may carry on.
func allowEdit(w http.ResponseWriter, r *http.Request, srv ServeOptions) bool {
	switch err := srv.checkToken(r); {
	case err == nil:
		return true
	case errors.Is(err, errEditingDisabled):
		http.Error(w, err.Error(), http.StatusForbidden)
	default:
		http.Error(w, err.Error(), http.StatusUnauthorized)
	}
	return false
}

// configState reports the config as it currently stands on disk, together with
// what the caller may do with it.
func configState(srv ServeOptions, cache *statsCache, r *http.Request) configResponse {
	state := configResponse{
		File:       srv.ConfigFile,
		Folders:    cache.folders(),
		Web:        webInfo{Addr: srv.Addr, CacheFile: srv.CacheFile, GroupsFile: srv.GroupsFile},
		Editable:   srv.EditToken != "",
		Authorized: srv.checkToken(r) == nil,
	}
	// The token itself is never sent back: Editable says that there is one.
	if cfg, err := LoadConfig(srv.ConfigFile); err == nil {
		state.Config = cfg.Editable()
	}
	return state
}

// applySavedConfig validates a set of edits, writes them to the config file
// and makes them take effect at once: later requests analyze the repositories
// the config now names, with its default window and filters, and the identity
// cards of the newly added repositories are built in the background.
func applySavedConfig(file string, edits EditableConfig, cache *statsCache, identities *identityCache) error {
	folders, err := edits.Validate()
	if err != nil {
		return invalidConfig{err}
	}
	if len(folders) == 0 {
		return invalidConfig{errors.New("at least one repository is needed")}
	}

	saved, err := SaveConfig(file, edits)
	if err != nil {
		return err
	}

	known := map[string]bool{}
	for _, folder := range cache.folders() {
		known[folder] = true
	}

	// Edits are layered on the options the server was started with, so a value
	// cleared in the UI falls back to the command line rather than to whatever
	// was running.
	opts := ApplyConfig(cache.startupOpts, saved)
	opts.Folders = folders
	ttl := cache.startupTTL
	if saved.Web.TTL != nil {
		if parsed, err := parseTTL(*saved.Web.TTL); err == nil {
			ttl = parsed
		}
	}
	cache.setOptions(opts, ttl)
	identities.setTTL(ttl)

	var added []string
	for _, folder := range folders {
		if !known[folder] {
			added = append(added, folder)
		}
	}
	identities.warm(added)
	return nil
}

// invalidConfig marks edits the client got wrong, as opposed to a file that
// could not be written.
type invalidConfig struct{ err error }

func (e invalidConfig) Error() string { return e.err.Error() }
func (e invalidConfig) Unwrap() error { return e.err }

// statusForConfigError tells a rejected payload from a failed write.
func statusForConfigError(err error) int {
	var invalid invalidConfig
	if errors.As(err, &invalid) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

// parseTTL reads a cache lifetime, rejecting a negative one.
func parseTTL(value string) (time.Duration, error) {
	ttl, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if ttl < 0 {
		return 0, fmt.Errorf("a negative ttl makes no sense: %s", value)
	}
	return ttl, nil
}
