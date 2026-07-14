package stats

import (
	"path/filepath"
	"strings"
)

// extLanguages maps a lowercased file extension (with the leading dot) to a
// human-readable language / file-type name.
var extLanguages = map[string]string{
	".go":         "Go",
	".mod":        "Go modules",
	".sum":        "Go modules",
	".js":         "JavaScript",
	".jsx":        "JavaScript",
	".mjs":        "JavaScript",
	".cjs":        "JavaScript",
	".ts":         "TypeScript",
	".tsx":        "TypeScript",
	".py":         "Python",
	".rb":         "Ruby",
	".php":        "PHP",
	".java":       "Java",
	".kt":         "Kotlin",
	".kts":        "Kotlin",
	".scala":      "Scala",
	".c":          "C",
	".h":          "C",
	".cc":         "C++",
	".cpp":        "C++",
	".cxx":        "C++",
	".hpp":        "C++",
	".cs":         "C#",
	".rs":         "Rust",
	".swift":      "Swift",
	".m":          "Objective-C",
	".dart":       "Dart",
	".lua":        "Lua",
	".r":          "R",
	".pl":         "Perl",
	".ex":         "Elixir",
	".exs":        "Elixir",
	".clj":        "Clojure",
	".hs":         "Haskell",
	".sh":         "Shell",
	".bash":       "Shell",
	".zsh":        "Shell",
	".ps1":        "PowerShell",
	".html":       "HTML",
	".htm":        "HTML",
	".css":        "CSS",
	".scss":       "Sass",
	".sass":       "Sass",
	".less":       "Less",
	".vue":        "Vue",
	".svelte":     "Svelte",
	".json":       "JSON",
	".yaml":       "YAML",
	".yml":        "YAML",
	".toml":       "TOML",
	".xml":        "XML",
	".ini":        "Config",
	".cfg":        "Config",
	".conf":       "Config",
	".md":         "Markdown",
	".markdown":   "Markdown",
	".rst":        "reStructuredText",
	".txt":        "Text",
	".sql":        "SQL",
	".proto":      "Protobuf",
	".tf":         "Terraform",
	".gradle":     "Gradle",
	".dockerfile": "Dockerfile",
}

// specialNames maps whole file names (no useful extension) to a language.
var specialNames = map[string]string{
	"Dockerfile": "Dockerfile",
	"Makefile":   "Makefile",
	"go.mod":     "Go modules",
	"go.sum":     "Go modules",
}

// languageForFile classifies a file path into a language / file-type label.
// Unknown extensions fall back to the extension itself; files without a useful
// extension fall back to "Other".
func languageForFile(name string) string {
	base := filepath.Base(name)
	if lang, ok := specialNames[base]; ok {
		return lang
	}

	ext := strings.ToLower(filepath.Ext(base))
	if ext == "" {
		return "Other"
	}
	if lang, ok := extLanguages[ext]; ok {
		return lang
	}
	return strings.TrimPrefix(ext, ".")
}
