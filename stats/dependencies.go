package stats

import (
	"encoding/json"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/object"
)

// maxManifestDepth is how deep a dependency manifest is looked for: the
// repository root plus two directory levels, enough for a monorepo without
// dragging in every fixture of a test suite.
const maxManifestDepth = 2

// maxManifestSize caps the manifest content read from the tree.
const maxManifestSize = 1 << 20

// manifestParsers maps a manifest file name to the ecosystem it declares and
// the parser counting its dependencies.
var manifestParsers = map[string]struct {
	ecosystem string
	parse     func(string) DependencySet
}{
	"go.mod":           {"Go modules", parseGoMod},
	"package.json":     {"npm", parsePackageJSON},
	"requirements.txt": {"pip", parseRequirements},
	"pyproject.toml":   {"Python", parsePyProject},
	"Cargo.toml":       {"Cargo", parseCargoToml},
	"composer.json":    {"Composer", parseComposerJSON},
	"Gemfile":          {"Bundler", parseGemfile},
	"pom.xml":          {"Maven", parsePom},
	"build.gradle":     {"Gradle", parseGradle},
	"build.gradle.kts": {"Gradle", parseGradle},
}

// readDependencies parses a tracked file when it is a dependency manifest,
// reporting whether it was one.
func readDependencies(f *object.File) (DependencySet, bool) {
	parser, ok := manifestParsers[path.Base(f.Name)]
	if !ok || manifestDepth(f.Name) > maxManifestDepth || f.Size > maxManifestSize {
		return DependencySet{}, false
	}
	content, err := readFile(f)
	if err != nil {
		return DependencySet{}, false
	}
	set := parser.parse(content)
	if set.Total == 0 {
		// A manifest declaring nothing (a bare go.mod, an npm package with no
		// dependency) is not worth a line on the card.
		return DependencySet{}, false
	}
	set.Ecosystem = parser.ecosystem
	set.Manifest = f.Name
	return set, true
}

// manifestDepth is how many directories deep a path lies (0 at the root).
func manifestDepth(name string) int {
	dir := path.Dir(name)
	if dir == "." || dir == "/" {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

func readFile(f *object.File) (string, error) {
	reader, err := f.Reader()
	if err != nil {
		return "", err
	}
	defer func() { _ = reader.Close() }()
	raw, err := io.ReadAll(io.LimitReader(reader, maxManifestSize))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// total sums the three buckets of a set; every parser ends with it.
func (d DependencySet) total() DependencySet {
	d.Total = d.Direct + d.Indirect + d.Dev
	return d
}

// sortDependencies orders the sets by total, descending, then by ecosystem and
// manifest so the order is stable.
func sortDependencies(sets []DependencySet) []DependencySet {
	sort.Slice(sets, func(i, j int) bool {
		if sets[i].Total != sets[j].Total {
			return sets[i].Total > sets[j].Total
		}
		if sets[i].Ecosystem != sets[j].Ecosystem {
			return sets[i].Ecosystem < sets[j].Ecosystem
		}
		return sets[i].Manifest < sets[j].Manifest
	})
	return sets
}

// countDependencies sums the direct and the overall dependency counts.
func countDependencies(sets []DependencySet) (direct, total int) {
	for _, set := range sets {
		direct += set.Direct
		total += set.Total
	}
	return direct, total
}

var goRequireLine = regexp.MustCompile(`^\s*(?:require\s+)?[^\s()]+\s+\S+`)

// parseGoMod counts the modules a go.mod requires, separating the ones marked
// "// indirect".
func parseGoMod(content string) DependencySet {
	set := DependencySet{}
	inBlock := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case inBlock && trimmed == ")":
			inBlock = false
			continue
		case strings.HasPrefix(trimmed, "require ("):
			inBlock = true
			continue
		case !inBlock && !strings.HasPrefix(trimmed, "require "):
			continue
		case trimmed == "" || strings.HasPrefix(trimmed, "//"):
			continue
		}
		if !goRequireLine.MatchString(trimmed) {
			continue
		}
		if strings.Contains(trimmed, "// indirect") {
			set.Indirect++
		} else {
			set.Direct++
		}
	}
	return set.total()
}

// npmManifest is the dependency-carrying subset of a package.json.
type npmManifest struct {
	Dependencies         map[string]json.RawMessage `json:"dependencies"`
	DevDependencies      map[string]json.RawMessage `json:"devDependencies"`
	PeerDependencies     map[string]json.RawMessage `json:"peerDependencies"`
	OptionalDependencies map[string]json.RawMessage `json:"optionalDependencies"`
}

// parsePackageJSON counts the npm dependencies, runtime ones (plus peer and
// optional) as direct and devDependencies as dev.
func parsePackageJSON(content string) DependencySet {
	var m npmManifest
	if err := json.Unmarshal([]byte(content), &m); err != nil {
		return DependencySet{}
	}
	set := DependencySet{
		Direct: len(m.Dependencies) + len(m.PeerDependencies) + len(m.OptionalDependencies),
		Dev:    len(m.DevDependencies),
	}
	return set.total()
}

// parseRequirements counts the requirements of a pip requirements.txt, ignoring
// comments and option lines ("-r other.txt", "--index-url …").
func parseRequirements(content string) DependencySet {
	set := DependencySet{}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "-") {
			continue
		}
		set.Direct++
	}
	return set.total()
}

var tomlEntry = regexp.MustCompile(`^\s*["']?[A-Za-z0-9_.\-]+["']?\s*=`)

// parsePyProject counts the dependencies of a PEP 621 [project] table and of a
// Poetry [tool.poetry…] table, treating optional and dev groups as dev.
func parsePyProject(content string) DependencySet {
	set := DependencySet{}
	section, inArray, arrayIsDev := "", false, false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if inArray {
			if strings.HasPrefix(trimmed, "]") {
				inArray = false
				continue
			}
			if strings.HasPrefix(trimmed, `"`) || strings.HasPrefix(trimmed, "'") {
				if arrayIsDev {
					set.Dev++
				} else {
					set.Direct++
				}
			}
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			section = strings.Trim(trimmed, "[]")
			continue
		}
		// A PEP 621 dependency array, on one line or spread over several.
		if key, rest, ok := strings.Cut(trimmed, "="); ok && section == "project" {
			name := strings.TrimSpace(key)
			if name == "dependencies" || name == "optional-dependencies" {
				arrayIsDev = name != "dependencies"
				items := strings.Count(rest, `"`)/2 + strings.Count(rest, `'`)/2
				if strings.Contains(rest, "[") && !strings.Contains(rest, "]") {
					inArray = true
				}
				if arrayIsDev {
					set.Dev += items
				} else {
					set.Direct += items
				}
				continue
			}
		}
		// Poetry declares one dependency per line in its own tables.
		if strings.HasPrefix(section, "tool.poetry") && strings.Contains(section, "dependencies") {
			if !tomlEntry.MatchString(trimmed) || strings.HasPrefix(trimmed, "python") {
				continue
			}
			if strings.Contains(section, "dev") {
				set.Dev++
			} else {
				set.Direct++
			}
		}
	}
	return set.total()
}

// parseCargoToml counts the crates declared in the dependency tables of a
// Cargo.toml, both as "name = …" entries and as [dependencies.name] tables.
func parseCargoToml(content string) DependencySet {
	set := DependencySet{}
	section := ""
	isDeps := func(s string) bool {
		s = strings.TrimPrefix(s, "workspace.")
		return s == "dependencies" || s == "dev-dependencies" || s == "build-dependencies"
	}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			section = strings.Trim(trimmed, "[]")
			// [dependencies.serde] declares one crate on its own.
			if base, name, ok := strings.Cut(section, "."); ok && isDeps(base) && name != "" {
				if strings.Contains(base, "dev") {
					set.Dev++
				} else {
					set.Direct++
				}
				section = ""
			}
			continue
		}
		if !isDeps(section) || !tomlEntry.MatchString(trimmed) {
			continue
		}
		if strings.Contains(section, "dev") {
			set.Dev++
		} else {
			set.Direct++
		}
	}
	return set.total()
}

// composerManifest is the dependency-carrying subset of a composer.json.
type composerManifest struct {
	Require    map[string]json.RawMessage `json:"require"`
	RequireDev map[string]json.RawMessage `json:"require-dev"`
}

// parseComposerJSON counts the packages a composer.json requires, leaving out
// the platform requirements (php, ext-*) which are not packages.
func parseComposerJSON(content string) DependencySet {
	var m composerManifest
	if err := json.Unmarshal([]byte(content), &m); err != nil {
		return DependencySet{}
	}
	count := func(deps map[string]json.RawMessage) int {
		n := 0
		for name := range deps {
			if name == "php" || strings.HasPrefix(name, "ext-") || strings.HasPrefix(name, "lib-") {
				continue
			}
			n++
		}
		return n
	}
	set := DependencySet{Direct: count(m.Require), Dev: count(m.RequireDev)}
	return set.total()
}

// parseGemfile counts the "gem" lines of a Gemfile, those inside a :development
// or :test group counting as dev.
func parseGemfile(content string) DependencySet {
	set := DependencySet{}
	devGroup := 0
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "group "):
			if strings.Contains(trimmed, ":development") || strings.Contains(trimmed, ":test") {
				devGroup++
			}
		case trimmed == "end" && devGroup > 0:
			devGroup--
		case strings.HasPrefix(trimmed, "gem "):
			if devGroup > 0 || strings.Contains(trimmed, ":development") || strings.Contains(trimmed, ":test") {
				set.Dev++
			} else {
				set.Direct++
			}
		}
	}
	return set.total()
}

// parsePom counts the <dependency> elements of a Maven pom.xml, those in
// <test> scope counting as dev.
func parsePom(content string) DependencySet {
	set := DependencySet{}
	rest := content
	for {
		_, after, ok := strings.Cut(rest, "<dependency>")
		if !ok {
			break
		}
		block, remainder, _ := strings.Cut(after, "</dependency>")
		if strings.Contains(block, "<scope>test</scope>") || strings.Contains(block, "<scope>provided</scope>") {
			set.Dev++
		} else {
			set.Direct++
		}
		rest = remainder
	}
	return set.total()
}

var gradleDependency = regexp.MustCompile(`^(implementation|api|compileOnly|runtimeOnly|annotationProcessor|kapt|testImplementation|testCompileOnly|testRuntimeOnly|androidTestImplementation)[\s(]`)

// parseGradle counts the dependency declarations of a build.gradle(.kts), the
// test configurations counting as dev.
func parseGradle(content string) DependencySet {
	set := DependencySet{}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		match := gradleDependency.FindString(trimmed)
		if match == "" {
			continue
		}
		if strings.HasPrefix(match, "test") || strings.HasPrefix(match, "androidTest") {
			set.Dev++
		} else {
			set.Direct++
		}
	}
	return set.total()
}
