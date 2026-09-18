package stats

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WebConfig holds the web-server default values.
type WebConfig struct {
	Addr       *string `json:"addr,omitempty"`
	TTL        *string `json:"ttl,omitempty"`
	CacheFile  *string `json:"cacheFile,omitempty"`
	GroupsFile *string `json:"groupsFile,omitempty"`
	// EditToken guards every change made from the web UI. While it is unset,
	// the UI is read-only: nothing it can do writes to disk.
	EditToken *string `json:"editToken,omitempty"`
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
	if cfg.Web.GroupsFile != nil {
		expanded := expandPath(*cfg.Web.GroupsFile)
		cfg.Web.GroupsFile = &expanded
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

// EditableConfig is the subset of the config the web UI may change. Every
// field is a pointer or a slice so that "not set" (the value is removed from
// the file, falling back to the built-in default) is told from a value.
type EditableConfig struct {
	Weeks           *int     `json:"weeks"`
	Delta           *string  `json:"delta"`
	User            *string  `json:"user"`
	CountAll        *bool    `json:"countAll"`
	Merge           *bool    `json:"merge"`
	Folders         []string `json:"folders"`
	IncludePatterns []string `json:"includePatterns"`
	ExcludePatterns []string `json:"excludePatterns"`
	TTL             *string  `json:"ttl"` // stored as web.ttl
}

// Editable returns the part of a config the web UI may change.
func (c *Config) Editable() EditableConfig {
	return EditableConfig{
		Weeks:           c.Weeks,
		Delta:           c.Delta,
		User:            c.User,
		CountAll:        c.CountAll,
		Merge:           c.Merge,
		Folders:         c.Folders,
		IncludePatterns: c.IncludePatterns,
		ExcludePatterns: c.ExcludePatterns,
		TTL:             c.Web.TTL,
	}
}

// Validate reports what would keep a set of edits from being used: a bad
// delta, an unusable regex, a folder holding no repository. It returns the
// folders the config resolves to, so a caller does not expand them twice.
func (e EditableConfig) Validate() ([]string, error) {
	if e.Weeks != nil && (*e.Weeks < 0 || *e.Weeks > 520) {
		return nil, fmt.Errorf("weeks must be between 0 and 520, got %d", *e.Weeks)
	}
	if e.Delta != nil && *e.Delta != "" {
		if _, err := parseDelta(*e.Delta, time.Now()); err != nil {
			return nil, err
		}
	}
	if e.TTL != nil && *e.TTL != "" {
		if _, err := time.ParseDuration(*e.TTL); err != nil {
			return nil, fmt.Errorf("invalid ttl: %w", err)
		}
	}
	if _, err := compilePatterns(e.IncludePatterns); err != nil {
		return nil, err
	}
	if _, err := compilePatterns(e.ExcludePatterns); err != nil {
		return nil, err
	}

	folders := make([]string, 0, len(e.Folders))
	for _, folder := range e.Folders {
		expanded, err := ResolveRepositories(folder)
		if err != nil {
			return nil, err
		}
		folders = append(folders, expanded...)
	}
	return dedupeFolders(folders), nil
}

// ResolveRepositories turns a path into the repositories it holds: the folder
// itself when it is a repository, otherwise its direct subdirectories that
// are. A path leading to none of them is an error, so a typo is reported
// instead of being stored.
func ResolveRepositories(path string) ([]string, error) {
	expanded := expandPath(strings.TrimSpace(path))
	if expanded == "" {
		return nil, errors.New("a repository path cannot be empty")
	}
	info, err := os.Stat(expanded)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a folder", path)
	}
	if isRepo(expanded) {
		return []string{expanded}, nil
	}
	if subs := repoSubdirs(expanded); len(subs) > 0 {
		return subs, nil
	}
	return nil, fmt.Errorf("%s is not a git repository, and holds none", path)
}

// dedupeFolders keeps the first occurrence of each folder, in order.
func dedupeFolders(folders []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(folders))
	for _, folder := range folders {
		if !seen[folder] {
			seen[folder] = true
			out = append(out, folder)
		}
	}
	return out
}

// SaveConfig writes a set of edits to the config file and returns the config as
// re-read from disk. The file is rewritten from its own raw content, so
// everything the UI does not touch — the edit token first of all, but also any
// other key — is preserved exactly. A nil field removes its key, which brings
// back the built-in default.
func SaveConfig(path string, edits EditableConfig) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	content := map[string]interface{}{}
	switch raw, err := os.ReadFile(path); {
	case err == nil:
		if err := json.Unmarshal(raw, &content); err != nil {
			return nil, fmt.Errorf("the config file %s is not valid JSON: %w", path, err)
		}
	case !os.IsNotExist(err):
		return nil, err
	}

	setKey(content, "weeks", edits.Weeks)
	setKey(content, "delta", edits.Delta)
	setKey(content, "user", edits.User)
	setKey(content, "countAll", edits.CountAll)
	setKey(content, "merge", edits.Merge)
	setList(content, "folders", edits.Folders)
	setList(content, "includePatterns", edits.IncludePatterns)
	setList(content, "excludePatterns", edits.ExcludePatterns)

	web, _ := content["web"].(map[string]interface{})
	if web == nil {
		web = map[string]interface{}{}
	}
	setKey(web, "ttl", edits.TTL)
	if len(web) > 0 {
		content["web"] = web
	} else {
		delete(content, "web")
	}

	raw, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := writeFileAtomically(path, append(raw, '\n')); err != nil {
		return nil, err
	}
	return LoadConfig(path)
}

// setKey stores a value under key, or removes the key when the value is unset.
func setKey[T any](content map[string]interface{}, key string, value *T) {
	if value == nil {
		delete(content, key)
		return
	}
	content[key] = *value
}

// setList stores a list under key, or removes the key when the list is empty.
func setList(content map[string]interface{}, key string, values []string) {
	if len(values) == 0 {
		delete(content, key)
		return
	}
	content[key] = values
}

// writeFileAtomically writes through a temporary file, so an interrupted write
// cannot leave a truncated document behind.
func writeFileAtomically(path string, raw []byte) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// ApplyConfig re-derives the options the server analyzes with from a config,
// keeping the values the config does not set. It is what makes a change saved
// from the web UI take effect without a restart.
func ApplyConfig(base LaunchOptions, cfg *Config) LaunchOptions {
	opts := base
	if len(cfg.Folders) > 0 {
		opts.Folders = ExpandFolders(cfg.Folders)
	}
	if cfg.Weeks != nil && *cfg.Weeks > 0 {
		opts.DurationInWeeks = *cfg.Weeks
	}
	if cfg.Delta != nil {
		opts.Delta = *cfg.Delta
	}
	if cfg.Merge != nil {
		opts.Merge = *cfg.Merge
	}
	opts.PatternToInclude = cfg.IncludePatterns
	opts.PatternToExclude = cfg.ExcludePatterns

	switch {
	case cfg.CountAll != nil && *cfg.CountAll:
		opts.User = nil
	case cfg.User != nil && *cfg.User != "":
		user := *cfg.User
		opts.User = &user
	}
	return opts
}
