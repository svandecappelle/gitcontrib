package stats

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

//go:embed webui/index.html
var webUI embed.FS

// appliedParams echoes the parameters actually used for a response, so the UI
// can initialize its form with the server defaults.
type appliedParams struct {
	Weeks    int      `json:"weeks"`
	Delta    string   `json:"delta"`
	User     string   `json:"user"`
	CountAll bool     `json:"countAll"`
	Merge    bool     `json:"merge"`
	Repos    []string `json:"repos"`           // empty means every repository
	Group    string   `json:"group,omitempty"` // the group the selection reproduces
	Include  []string `json:"include"`
	Exclude  []string `json:"exclude"`
}

// statsResponse is the /api/stats payload: the aggregated statistics (flattened
// at the top level) enriched with cache metadata and the applied parameters.
type statsResponse struct {
	AggregatedStats
	Params         appliedParams `json:"params"`
	AvailableRepos []string      `json:"availableRepos"`
	UpdatedAt      time.Time     `json:"updatedAt"`
	Stale          bool          `json:"stale"`
	Refreshing     bool          `json:"refreshing"`
	TTLSeconds     float64       `json:"ttlSeconds"`
}

// groupsResponse is the /api/groups payload: the saved repository groups, the
// folders they can name, and where they are stored.
type groupsResponse struct {
	Groups         []RepoGroup `json:"groups"`
	AvailableRepos []string    `json:"availableRepos"`
	File           string      `json:"file"`
}

// identityResponse is the /api/identity payload: one identity card per
// repository plus the card of all of them merged, with the same cache metadata
// as the statistics endpoint.
type identityResponse struct {
	Repositories []RepositoryIdentity `json:"repositories"`
	All          IdentityOverview     `json:"all"`
	UpdatedAt    time.Time            `json:"updatedAt"`
	Stale        bool                 `json:"stale"`
	Refreshing   bool                 `json:"refreshing"`
	TTLSeconds   float64              `json:"ttlSeconds"`
}

// ServeOptions gathers what the web server needs besides the analysis options:
// where to listen, where its files live, and the token guarding every change
// made from the UI.
type ServeOptions struct {
	Addr       string
	TTL        time.Duration
	CacheFile  string
	GroupsFile string
	ConfigFile string // the analysis config the UI can edit
	EditToken  string // empty keeps the UI read-only
}

// Serve starts an HTTP server exposing the statistics as a JSON API on
// /api/stats, the repository identity cards on /api/identity, the saved
// repository groups on /api/groups, the analysis config on /api/config, and a
// single-page UI on /. Statistics are cached per parameter set to a JSON file:
// the default set is scanned at startup, each parameter set is scanned on first
// use and then served from cache, and a set is refreshed in the background once
// older than the TTL or when /api/refresh is called. Identity cards are cached
// per repository, in memory, with the same TTL.
//
// Every endpoint that writes to disk — the config, the repositories it scans,
// the groups — requires the edit token in an X-Gitcontrib-Token header. While
// no token is configured, those endpoints refuse everything: a server nobody
// configured for editing cannot be changed through its UI.
func Serve(opts LaunchOptions, srv ServeOptions) error {
	// Keep scans silent: the JSON API is the only response the client sees.
	opts.Dashboard = true
	addr, ttl, cacheFile, groupsFile := srv.Addr, srv.TTL, srv.CacheFile, srv.GroupsFile

	assets, err := fs.Sub(webUI, "webui")
	if err != nil {
		return err
	}

	groups := newGroupStore(groupsFile)
	if err := groups.load(); err != nil {
		// A broken groups file must not keep the statistics from being served.
		fmt.Printf("Ignoring the repository groups: %s\n", err)
	}

	cache := newStatsCache(opts, ttl, cacheFile, groups)
	cache.load()

	// Warm the default parameter set so the first page load is instant.
	defaultKey := cacheKey(opts)
	switch entry, stale, _ := cache.state(defaultKey); {
	case entry == nil:
		fmt.Println("No usable cache found, scanning repositories…")
		cache.scan(defaultKey, opts)
	case stale:
		fmt.Println("Cache is stale, refreshing in the background…")
		cache.refreshInBackground(defaultKey, opts)
	default:
		fmt.Printf("Loaded cached statistics from %s\n", cacheFile)
	}

	// Identity cards describe the repositories themselves, not a parameter
	// set: they are built per folder and warmed in the background so the first
	// page load does not wait for them.
	identities := newIdentityCache(ttl)
	identities.warm(opts.Folders)

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(assets)))

	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
		case http.MethodPut, http.MethodPost:
			if !allowEdit(w, r, srv) {
				return
			}
			var body configRequest
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxGroupsBody)).Decode(&body); err != nil {
				http.Error(w, "invalid config payload: "+err.Error(), http.StatusBadRequest)
				return
			}
			if err := applySavedConfig(srv.ConfigFile, body.Config, cache, identities); err != nil {
				http.Error(w, err.Error(), statusForConfigError(err))
				return
			}
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, configState(srv, cache, r))
	})

	// Resolving a path into the repositories it holds reads the server's
	// filesystem, so it is guarded like the writes it prepares.
	mux.HandleFunc("/api/repositories/scan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !allowEdit(w, r, srv) {
			return
		}
		var body struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxGroupsBody)).Decode(&body); err != nil {
			http.Error(w, "invalid payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		folders, err := ResolveRepositories(body.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]interface{}{"path": body.Path, "folders": folders})
	})

	mux.HandleFunc("/api/repositories/status", func(w http.ResponseWriter, r *http.Request) {
		folders, err := cache.identityFolders(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]interface{}{"repositories": RepositoryStatuses(folders)})
	})

	// Updating a repository writes to the filesystem, so it is guarded like
	// the configuration is.
	mux.HandleFunc("/api/repositories/pull", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !allowEdit(w, r, srv) {
			return
		}
		var body struct {
			Repos []string `json:"repos"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxGroupsBody)).Decode(&body); err != nil {
			http.Error(w, "invalid payload: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Only the repositories this server scans can be updated, and an empty
		// list means every one of them.
		scanned := cache.folders()
		folders := scanned
		if len(body.Repos) > 0 {
			for _, repo := range body.Repos {
				if !containsFolder(scanned, repo) {
					http.Error(w, "unknown repository: "+repo, http.StatusBadRequest)
					return
				}
			}
			folders = intersectFolders(body.Repos, scanned)
		}

		results := PullRepositories(folders)

		// What moved is no longer what was measured.
		var updated []string
		for _, result := range results {
			if result.Updated {
				updated = append(updated, result.Folder)
			}
		}
		if len(updated) > 0 {
			log.Printf("Updated %d repositor%s: %s", len(updated), map[bool]string{true: "y", false: "ies"}[len(updated) == 1], strings.Join(updated, ", "))
			cache.invalidateFolders(updated)
			identities.forceRefresh(updated)
		}
		writeJSON(w, map[string]interface{}{"results": results})
	})

	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		reqOpts, err := cache.resolve(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		key := cacheKey(reqOpts)

		entry, stale, refreshing := cache.state(key)
		switch {
		case entry == nil:
			// First request for this parameter set: scan synchronously.
			cache.scan(key, reqOpts)
			entry, stale, refreshing = cache.state(key)
		case stale:
			// Stale-while-revalidate: serve now, refresh in the background.
			refreshing = cache.refreshInBackground(key, reqOpts) || refreshing
		}
		if entry == nil {
			http.Error(w, "statistics not ready", http.StatusServiceUnavailable)
			return
		}

		writeJSON(w, statsResponse{
			AggregatedStats: entry.Stats,
			Params:          cache.paramsOf(reqOpts),
			AvailableRepos:  cache.folders(),
			UpdatedAt:       entry.UpdatedAt,
			Stale:           stale,
			Refreshing:      refreshing,
			TTLSeconds:      ttl.Seconds(),
		})
	})

	mux.HandleFunc("/api/groups", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
		case http.MethodPut, http.MethodPost:
			if !allowEdit(w, r, srv) {
				return
			}
			var body groupsResponse
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxGroupsBody)).Decode(&body); err != nil {
				http.Error(w, "invalid groups payload: "+err.Error(), http.StatusBadRequest)
				return
			}
			if _, err := groups.replace(body.Groups); err != nil {
				http.Error(w, err.Error(), statusForGroupError(err))
				return
			}
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		writeJSON(w, groupsResponse{
			Groups:         groups.all(),
			AvailableRepos: cache.folders(),
			File:           groupsFile,
		})
	})

	mux.HandleFunc("/api/identity", func(w http.ResponseWriter, r *http.Request) {
		folders, err := cache.identityFolders(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		cards, updatedAt, stale, refreshing := identities.cards(folders)
		writeJSON(w, identityResponse{
			Repositories: cards,
			All:          AggregateIdentities(cards),
			UpdatedAt:    updatedAt,
			Stale:        stale,
			Refreshing:   refreshing,
			TTLSeconds:   ttl.Seconds(),
		})
	})

	mux.HandleFunc("/api/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		reqOpts, err := cache.resolve(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		started := cache.refreshInBackground(cacheKey(reqOpts), reqOpts)
		identities.forceRefresh(reqOpts.Folders)
		w.WriteHeader(http.StatusAccepted)
		writeJSON(w, map[string]bool{"started": started})
	})

	fmt.Printf("gitcontrib web interface listening on %s\n", browsableURL(addr))
	return http.ListenAndServe(addr, mux)
}

// maxGroupsBody caps the size of a groups payload the server accepts.
const maxGroupsBody = 1 << 20

// statusForGroupError tells a rejected payload (the client's fault) from a
// failed write (the server's).
func statusForGroupError(err error) int {
	var invalid invalidGroups
	if errors.As(err, &invalid) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

// resolve builds the launch options for a request. With no query parameters it
// falls back to the server defaults; otherwise it applies the client overrides.
// It validates the delta and the repository selection so a bad value is
// reported as a 400 rather than as an empty result.
func (c *statsCache) resolve(r *http.Request) (LaunchOptions, error) {
	if len(r.URL.Query()) == 0 {
		return c.options(), nil
	}
	params := parseParams(r)
	if _, err := parseDelta(params.Delta, time.Now()); err != nil {
		return LaunchOptions{}, err
	}
	folders, err := c.folderSelection(params)
	if err != nil {
		return LaunchOptions{}, err
	}
	return c.optsFor(params, folders), nil
}

// identityFolders returns the folders an identity request covers: the selected
// repositories, the ones of the selected group, or every configured folder.
func (c *statsCache) identityFolders(r *http.Request) ([]string, error) {
	return c.folderSelection(parseParams(r))
}

func parseParams(r *http.Request) Params {
	q := r.URL.Query()
	p := Params{
		Delta:    q.Get("delta"),
		User:     q.Get("user"),
		CountAll: isTrue(q.Get("countAll")),
		Merge:    isTrue(q.Get("merge")),
		Repos:    repoSelection(q),
		Group:    strings.TrimSpace(q.Get("group")),
		Include:  splitCSV(q.Get("include")),
		Exclude:  splitCSV(q.Get("exclude")),
	}
	if weeks, err := strconv.Atoi(q.Get("weeks")); err == nil {
		p.Weeks = weeks
	}
	return p
}

// repoSelection reads the selected repositories from the query string. They
// may be spelled as a comma-separated "repo" (or "repos") value, or as the
// parameter repeated once per repository — both forms make a shareable link.
func repoSelection(q url.Values) []string {
	var repos []string
	for _, key := range []string{"repo", "repos"} {
		for _, value := range q[key] {
			repos = append(repos, splitCSV(value)...)
		}
	}
	return repos
}

// paramsOf reports the parameters a set of options corresponds to, for echoing
// back to the UI.
func (c *statsCache) paramsOf(o LaunchOptions) appliedParams {
	ap := appliedParams{
		Weeks:   o.DurationInWeeks,
		Delta:   o.Delta,
		Merge:   o.Merge,
		Include: o.PatternToInclude,
		Exclude: o.PatternToExclude,
	}
	if o.User == nil {
		ap.CountAll = true
	} else {
		ap.User = *o.User
	}
	// Folders that are not the full configured set are a repository selection;
	// when they reproduce a saved group, the UI is told which one.
	if scanned := c.folders(); !sameFolders(o.Folders, scanned) {
		ap.Repos = o.Folders
		ap.Group = c.groups.nameOf(o.Folders, scanned)
	}
	return ap
}

func isTrue(v string) bool {
	return v == "true" || v == "1" || v == "on"
}

func splitCSV(v string) []string {
	if v == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(v, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// browsableURL turns a listen address into a URL a user can click. A bare
// ":8080" address is bound to localhost for display purposes.
func browsableURL(addr string) string {
	host := addr
	if len(addr) > 0 && addr[0] == ':' {
		host = "localhost" + addr
	}
	return "http://" + host
}
