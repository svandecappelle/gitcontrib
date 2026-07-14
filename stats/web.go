package stats

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
)

//go:embed webui/index.html
var webUI embed.FS

// Serve starts an HTTP server exposing the statistics as a JSON API on
// /api/stats and a single-page UI on /. The statistics are recomputed on every
// API request so the numbers always reflect the current state of the scanned
// repositories.
func Serve(opts LaunchOptions, addr string) error {
	// Keep the scan silent: the JSON API is the only response the client sees.
	opts.Dashboard = true

	assets, err := fs.Sub(webUI, "webui")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(assets)))
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, req *http.Request) {
		agg := Aggregate(Launch(opts))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(agg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	fmt.Printf("gitcontrib web interface listening on %s\n", browsableURL(addr))
	return http.ListenAndServe(addr, mux)
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
