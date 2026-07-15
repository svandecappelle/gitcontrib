package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// WebConfig holds the web-server default values.
type WebConfig struct {
	Addr      *string `json:"addr,omitempty"`
	TTL       *string `json:"ttl,omitempty"`
	CacheFile *string `json:"cacheFile,omitempty"`
}

// Config holds the default analysis values loaded from a JSON config file.
// Every field is optional: a nil pointer or empty slice means "not set", so a
// command-line flag always takes precedence over the config, which in turn
// takes precedence over the built-in defaults.
type Config struct {
	Weeks           *int      `json:"weeks,omitempty"`
	Delta           *string   `json:"delta,omitempty"`
	User            *string   `json:"user,omitempty"`
	CountAll        *bool     `json:"countAll,omitempty"`
	Merge           *bool     `json:"merge,omitempty"`
	Folders         []string  `json:"folders,omitempty"`
	IncludePatterns []string  `json:"includePatterns,omitempty"`
	ExcludePatterns []string  `json:"excludePatterns,omitempty"`
	Web             WebConfig `json:"web,omitempty"`
}

// DefaultConfigPath returns the default config file location
// (<home>/.gitcontrib.json), falling back to the current directory when the
// home directory is unknown.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".gitcontrib.json"
	}
	return filepath.Join(home, ".gitcontrib.json")
}

// LoadConfig reads and parses the JSON config at path (DefaultConfigPath when
// empty). A missing file is not an error: it yields an empty config. A present
// but invalid file returns an error.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Expand environment variables and "~" in path-like fields, since JSON does
	// not do it (e.g. "folders": ["$HOME/wd"]).
	for i := range cfg.Folders {
		cfg.Folders[i] = expandPath(cfg.Folders[i])
	}
	if cfg.Web.CacheFile != nil {
		expanded := expandPath(*cfg.Web.CacheFile)
		cfg.Web.CacheFile = &expanded
	}
	return &cfg, nil
}

// expandPath expands environment variables ($VAR, ${VAR}) and a leading "~" in
// a filesystem path.
func expandPath(p string) string {
	if p == "" {
		return p
	}
	p = os.ExpandEnv(p)
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
