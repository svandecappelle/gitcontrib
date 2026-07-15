package stats

import "testing"

func TestLanguageForFile(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"main.go", "Go"},
		{"src/app.js", "JavaScript"},
		{"index.TS", "TypeScript"}, // extension is case-insensitive
		{"styles/site.scss", "Sass"},
		{"Dockerfile", "Dockerfile"},
		{"build/Makefile", "Makefile"},
		{"go.mod", "Go modules"},
		{"README", "Other"},         // no extension, not a special name
		{"archive.tar.gz", "gz"},    // unknown extension falls back to itself
		{".gitignore", "gitignore"}, // dotfile: Ext == whole name
	}
	for _, c := range cases {
		if got := languageForFile(c.path); got != c.want {
			t.Errorf("languageForFile(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}
