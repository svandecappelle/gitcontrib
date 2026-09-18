package stats

import "testing"

func TestParseGoMod(t *testing.T) {
	content := `module github.com/example/project

go 1.25.0

require (
	github.com/fatih/color v1.19.0
	github.com/go-git/go-git/v5 v5.19.1
	// a comment inside the block
	github.com/mattn/go-isatty v0.0.22 // indirect
)

require golang.org/x/term v0.45.0

replace github.com/foo/bar => ../bar
`
	set := parseGoMod(content)
	if set.Direct != 3 || set.Indirect != 1 || set.Total != 4 {
		t.Errorf("parseGoMod = %+v, want 3 direct and 1 indirect", set)
	}
}

func TestParsePackageJSON(t *testing.T) {
	content := `{
	  "name": "app",
	  "dependencies": { "react": "^18.0.0", "lodash": "^4.0.0" },
	  "devDependencies": { "vite": "^5.0.0" },
	  "peerDependencies": { "typescript": "^5.0.0" }
	}`
	set := parsePackageJSON(content)
	if set.Direct != 3 || set.Dev != 1 || set.Total != 4 {
		t.Errorf("parsePackageJSON = %+v, want 3 direct and 1 dev", set)
	}
	if got := parsePackageJSON("not json"); got.Total != 0 {
		t.Errorf("an unparsable package.json should declare nothing, got %+v", got)
	}
}

func TestParseRequirements(t *testing.T) {
	content := `# comment
django==5.0
requests>=2.0

-r other-requirements.txt
--index-url https://example.com
`
	if set := parseRequirements(content); set.Direct != 2 || set.Total != 2 {
		t.Errorf("parseRequirements = %+v, want 2 direct", set)
	}
}

func TestParsePyProject(t *testing.T) {
	content := `[project]
name = "app"
dependencies = [
  "django>=5.0",
  "requests",
]

[tool.poetry.dependencies]
python = "^3.12"
httpx = "^0.27"

[tool.poetry.group.dev.dependencies]
pytest = "^8.0"
`
	set := parsePyProject(content)
	if set.Direct != 3 || set.Dev != 1 {
		t.Errorf("parsePyProject = %+v, want 3 direct (2 PEP 621 + 1 poetry) and 1 dev", set)
	}

	inline := `[project]
dependencies = ["django", "requests", "httpx"]
`
	if got := parsePyProject(inline); got.Direct != 3 {
		t.Errorf("parsePyProject(inline array) = %+v, want 3 direct", got)
	}
}

func TestParseCargoToml(t *testing.T) {
	content := `[package]
name = "app"
version = "0.1.0"

[dependencies]
serde = "1.0"
tokio = { version = "1", features = ["full"] }

[dependencies.clap]
version = "4"

[dev-dependencies]
criterion = "0.5"
`
	set := parseCargoToml(content)
	if set.Direct != 3 || set.Dev != 1 {
		t.Errorf("parseCargoToml = %+v, want 3 direct and 1 dev", set)
	}
}

func TestParseComposerJSON(t *testing.T) {
	content := `{
	  "require": { "php": ">=8.2", "ext-json": "*", "symfony/console": "^7.0" },
	  "require-dev": { "phpunit/phpunit": "^11.0" }
	}`
	set := parseComposerJSON(content)
	if set.Direct != 1 || set.Dev != 1 {
		t.Errorf("parseComposerJSON = %+v, want 1 direct (platform requirements excluded) and 1 dev", set)
	}
}

func TestParseGemfile(t *testing.T) {
	content := `source "https://rubygems.org"

gem "rails", "~> 7.1"
gem "pg"

group :development, :test do
  gem "rspec"
end

gem "puma", group: :test
`
	set := parseGemfile(content)
	if set.Direct != 2 || set.Dev != 2 {
		t.Errorf("parseGemfile = %+v, want 2 direct and 2 dev", set)
	}
}

func TestParsePom(t *testing.T) {
	content := `<project>
	<dependencies>
		<dependency><groupId>com.example</groupId><artifactId>lib</artifactId></dependency>
		<dependency><groupId>junit</groupId><artifactId>junit</artifactId><scope>test</scope></dependency>
	</dependencies>
</project>`
	set := parsePom(content)
	if set.Direct != 1 || set.Dev != 1 {
		t.Errorf("parsePom = %+v, want 1 direct and 1 test dependency", set)
	}
}

func TestParseGradle(t *testing.T) {
	content := `dependencies {
    implementation("com.example:lib:1.0")
    api 'com.example:api:1.0'
    testImplementation("org.junit:junit:5.0")
    // implementation("commented:out:1.0")
}`
	set := parseGradle(content)
	if set.Direct != 2 || set.Dev != 1 {
		t.Errorf("parseGradle = %+v, want 2 direct and 1 dev", set)
	}
}

func TestManifestDepth(t *testing.T) {
	cases := map[string]int{
		"go.mod":                    0,
		"backend/go.mod":            1,
		"services/api/package.json": 2,
		"a/b/c/package.json":        3,
	}
	for path, want := range cases {
		if got := manifestDepth(path); got != want {
			t.Errorf("manifestDepth(%q) = %d, want %d", path, got, want)
		}
	}
}

func TestSortAndCountDependencies(t *testing.T) {
	sets := sortDependencies([]DependencySet{
		{Ecosystem: "npm", Direct: 1, Total: 1},
		{Ecosystem: "Go modules", Direct: 8, Indirect: 30, Total: 38},
	})
	if sets[0].Ecosystem != "Go modules" {
		t.Errorf("the biggest set should come first, got %q", sets[0].Ecosystem)
	}
	direct, total := countDependencies(sets)
	if direct != 9 || total != 39 {
		t.Errorf("countDependencies = (%d, %d), want (9, 39)", direct, total)
	}
}
