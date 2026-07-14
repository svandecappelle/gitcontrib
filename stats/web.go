package stats

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"time"
)

//go:embed webui/index.html
var webUI embed.FS

// statsResponse is the /api/stats payload: the aggregated statistics (flattened
// at the top level) enriched with cache metadata.
type statsResponse struct {
	AggregatedStats
	UpdatedAt  time.Time `json:"updatedAt"`
	Stale      bool      `json:"stale"`
	Refreshing bool      `json:"refreshing"`
	TTLSeconds float64   `json:"ttlSeconds"`
}

// Serve starts an HTTP server exposing the statistics as a JSON API on
// /api/stats and a single-page UI on /. Statistics are cached to a JSON file:
// the cache is scanned synchronously at startup (unless a fresh cache file is
// found), served immediately on every request, and refreshed in the background
// once it grows older than ttl or when /api/refresh is called.
func Serve(opts LaunchOptions, addr string, ttl time.Duration, cacheFile string) error {
	// Keep scans silent: the JSON API is the only response the client sees.
	opts.Dashboard = true

	assets, err := fs.Sub(webUI, "webui")
	if err != nil {
		return err
	}

	cache := newStatsCache(opts, ttl, cacheFile)
	cache.load()
	switch entry, stale, _ := cache.state(); {
	case entry == nil:
		fmt.Println("No usable cache found, scanning repositories…")
		cache.scan()
	case stale:
		fmt.Println("Cache is stale, refreshing in the background…")
		cache.refreshInBackground()
	default:
		fmt.Printf("Loaded cached statistics from %s\n", cacheFile)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(assets)))

	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		entry, stale, refreshing := cache.state()
		if entry == nil {
			http.Error(w, "statistics not ready", http.StatusServiceUnavailable)
			return
		}
		// Stale-while-revalidate: serve the current data right away and kick
		// off a background refresh when it has expired.
		if stale {
			refreshing = cache.refreshInBackground() || refreshing
		}
		writeJSON(w, statsResponse{
			AggregatedStats: entry.Stats,
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
		started := cache.refreshInBackground()
		w.WriteHeader(http.StatusAccepted)
		writeJSON(w, map[string]bool{"started": started})
	})

	fmt.Printf("gitcontrib web interface listening on %s\n", browsableURL(addr))
	return http.ListenAndServe(addr, mux)
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
