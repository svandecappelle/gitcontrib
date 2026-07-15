package stats

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigMissingFile(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("missing file should not be an error, got %v", err)
	}
	if cfg == nil || cfg.Weeks != nil || len(cfg.Folders) != 0 {
		t.Errorf("missing file should yield an empty config, got %+v", cfg)
	}
}

func TestLoadConfigParses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitcontrib.json")
	content := `{
		"weeks": 12,
		"countAll": true,
		"folders": ["/a", "/b"],
		"excludePatterns": ["vendor/"],
		"web": { "addr": ":9000", "ttl": "10m" }
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Weeks == nil || *cfg.Weeks != 12 {
		t.Errorf("Weeks = %v, want 12", cfg.Weeks)
	}
	if cfg.CountAll == nil || !*cfg.CountAll {
		t.Errorf("CountAll = %v, want true", cfg.CountAll)
	}
	if cfg.Merge != nil {
		t.Errorf("Merge should be nil (absent), got %v", cfg.Merge)
	}
	if len(cfg.Folders) != 2 || cfg.Folders[0] != "/a" {
		t.Errorf("Folders = %v, want [/a /b]", cfg.Folders)
	}
	if cfg.Web.Addr == nil || *cfg.Web.Addr != ":9000" {
		t.Errorf("Web.Addr = %v, want :9000", cfg.Web.Addr)
	}
	if cfg.Web.CacheFile != nil {
		t.Errorf("Web.CacheFile should be nil (absent), got %v", cfg.Web.CacheFile)
	}
}

func TestLoadConfigInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{ not json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Error("invalid JSON should return an error")
	}
}

func TestDefaultConfigPath(t *testing.T) {
	if DefaultConfigPath() == "" {
		t.Error("DefaultConfigPath should never be empty")
	}
}
