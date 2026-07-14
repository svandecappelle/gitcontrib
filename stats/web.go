package stats

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
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
	Include  []string `json:"include"`
	Exclude  []string `json:"exclude"`
}

// statsResponse is the /api/stats payload: the aggregated statistics (flattened
// at the top level) enriched with cache metadata and the applied parameters.
type statsResponse struct {
	AggregatedStats
	Params     appliedParams `json:"params"`
	UpdatedAt  time.Time     `json:"updatedAt"`
	Stale      bool          `json:"stale"`
	Refreshing bool          `json:"refreshing"`
	TTLSeconds float64       `json:"ttlSeconds"`
}

// Serve starts an HTTP server exposing the statistics as a JSON API on
// /api/stats and a single-page UI on /. Statistics are cached per parameter set
// to a JSON file: the default set is scanned at startup, each parameter set is
// scanned on first use and then served from cache, and a set is refreshed in
// the background once older than ttl or when /api/refresh is called.
func Serve(opts LaunchOptions, addr string, ttl time.Duration, cacheFile string) error {
	// Keep scans silent: the JSON API is the only response the client sees.
	opts.Dashboard = true

	assets, err := fs.Sub(webUI, "webui")
	if err != nil {
		return err
	}

	cache := newStatsCache(opts, ttl, cacheFile)
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

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(assets)))

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
			Params:          paramsOf(reqOpts),
			UpdatedAt:       entry.UpdatedAt,
			Stale:           stale,
			Refreshing:      refreshing,
			TTLSeconds:      ttl.Seconds(),
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
		w.WriteHeader(http.StatusAccepted)
		writeJSON(w, map[string]bool{"started": started})
	})

	fmt.Printf("gitcontrib web interface listening on %s\n", browsableURL(addr))
	return http.ListenAndServe(addr, mux)
}

// resolve builds the launch options for a request. With no query parameters it
// falls back to the server defaults; otherwise it applies the client overrides.
// It validates the delta so a bad value is reported as a 400 rather than an
// empty result.
func (c *statsCache) resolve(r *http.Request) (LaunchOptions, error) {
	if len(r.URL.Query()) == 0 {
		return c.baseOpts, nil
	}
	params := parseParams(r)
	if _, err := parseDelta(params.Delta, time.Now()); err != nil {
		return LaunchOptions{}, err
	}
	return c.optsFor(params), nil
}

func parseParams(r *http.Request) Params {
	q := r.URL.Query()
	p := Params{
		Delta:    q.Get("delta"),
		User:     q.Get("user"),
		CountAll: isTrue(q.Get("countAll")),
		Merge:    isTrue(q.Get("merge")),
		Include:  splitCSV(q.Get("include")),
		Exclude:  splitCSV(q.Get("exclude")),
	}
	if weeks, err := strconv.Atoi(q.Get("weeks")); err == nil {
		p.Weeks = weeks
	}
	return p
}

// paramsOf reports the parameters a set of options corresponds to, for echoing
// back to the UI.
func paramsOf(o LaunchOptions) appliedParams {
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
