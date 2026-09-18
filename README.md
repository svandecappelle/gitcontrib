# gitcontrib

Command-line tool to explore git contributions — as a terminal calendar
heatmap, an interactive terminal dashboard, or a web interface backed by a JSON
API.

## Features

- **Calendar heatmap** of commits (GitHub-style) in the terminal.
- **Interactive TUI dashboard**: heatmap, commits per weekday and per hour,
  contributors and repositories.
- **Web interface + JSON API**: highlights, **identity cards** (per repository
  and for all of them at once), commit calendar, commits-over-time and
  lines-changed (additions/deletions) trends, commits by weekday/hour and a
  weekday×hour punchcard, contributors ranking with a contribution-share donut,
  breakdown by language / file type and by Conventional Commits type, JSON
  export, per-contributor drill-down, and live re-parametrization.
- Filter by user, date range, and file patterns; scan one or many repositories.
- **Repository multi-selection** in the web UI, reflected in the page URL, with
  named **groups** of repositories saved in a JSON file and edited from the UI.
- **Settings editable from the web UI** — repositories included — guarded by an
  edit token, and applied without restarting the server.
- Author identities grouped by email (like `git shortlog`), with `.mailmap`
  support.

## Installation

```sh
# install the binary (module name: gitcontribution)
go install github.com/svandecappelle/gitcontrib@latest

# or build from source
git clone https://github.com/svandecappelle/gitcontrib
cd gitcontrib
go build -o gitcontrib .
```

Requires Go (see the version in [`go.mod`](go.mod)).

## Quick start

```sh
# from inside a git repository
gitcontribution stat            # your contributions, as a heatmap
gitcontribution dashboard       # interactive terminal dashboard
gitcontribution web             # web UI + API on http://localhost:8080
```

## Commands

| Command | Alias | Description |
| --- | --- | --- |
| `stat [paths\|user]` | `s` | Print the contribution heatmap in the terminal. |
| `dashboard` | | Open the interactive terminal dashboard. |
| `web` | `w` | Start the HTTP server (JSON API + web UI). |
| `add-repository <dir>...` | `ar` | Save repositories to scan by default. |
| `list-repositories` | `lr` | List the saved repositories. |

Positional arguments are interpreted as folders when they exist on disk,
otherwise as a user (name or `email`, comma-separated for several).

With no folder argument, the scanned folders are resolved in this order: the
**current directory when it is a git repository** (the config `folders` are then
ignored), otherwise the config `folders`, otherwise the saved repository list.

When a given folder is not itself a git repository, its immediate
subdirectories (one level deep) that are repositories are scanned instead — so
you can point at a parent directory holding several repositories.

### Common flags (`stat`, `dashboard`, `web`)

- `--weeks <n>` — number of weeks to analyze (console default fits the terminal width).
- `--delta <n>[y|m|w|d]` — shift the analyzed window into the past, e.g. `1y`, `6m`, `2w`.
- `--count-all` — analyze every user instead of just the git-config user.
- `--merge` — merge all scanned folders into a single result.
- `--config <path>` — JSON config file with default values (see [Configuration file](#configuration-file)).

`dashboard` and `web` additionally accept `--file-include-pattern` and
`--file-exclude-pattern` (regular expressions, repeatable) to restrict which
files count toward the statistics.

## Examples

```sh
gitcontribution stat                       # current repo, your commits
gitcontribution stat dir1 dir2             # several repositories
gitcontribution stat "Firstname Name,me@example.com"   # specific users
gitcontribution stat --merge $(ls)         # merge all sub-folders
gitcontribution stat --weeks 4             # last 4 weeks
gitcontribution stat --delta 1y            # shifted back one year
gitcontribution stat --count-all           # all contributors
```

Save repositories to scan when you are not inside a repository folder:

```sh
gitcontribution add-repository /path/to/repo
gitcontribution list-repositories
```

## Configuration file

Default analysis values can be stored in a JSON config file, read from
`<home>/.gitcontrib.json` by default (override with `--config <path>`). A
command-line flag always wins over the config, which wins over the built-in
default. Every field is optional:

```json
{
  "weeks": 12,
  "delta": "6m",
  "user": "me@example.com",
  "countAll": false,
  "merge": false,
  "folders": ["/path/to/repoA", "/path/to/repoB"],
  "includePatterns": ["\\.go$"],
  "excludePatterns": ["vendor/", "_test\\.go$"],
  "web": {
    "addr": ":9000",
    "ttl": "10m",
    "cacheFile": "/tmp/gitcontrib-cache.json",
    "groupsFile": "~/.gitcontrib-groups.json",
    "editToken": "a-long-random-string"
  }
}
```

```sh
gitcontribution web --config ./gitcontrib.json
gitcontribution stat --weeks 4   # --weeks overrides the config's "weeks"
```

Path fields (`folders`, `web.cacheFile`, `web.groupsFile`) expand environment
variables and a
leading `~`, e.g. `"$HOME/wd"` or `"~/wd"`. A folder that is not a repository is
expanded to its direct repository subfolders (see above). The config `folders`
are ignored when the command runs inside a git repository (the current
repository is analyzed instead).

## Web interface

```sh
gitcontribution web --addr :8080
# gitcontrib web interface listening on http://localhost:8080
```

Open http://localhost:8080. The page shows at-a-glance highlights
(longest/current streak, most active day and hour, busiest day, average commit
size, top contributor share), the **identity cards** (see below), the commit
calendar, weekly commits-over-time and lines-changed trends, commits by weekday
and hour plus a weekday×hour punchcard, the contributors ranking with a
contribution-share donut, a breakdown by language / file type, and a breakdown
by Conventional Commits type.

- A **repository picker** selects any number of the scanned repositories, with
  a filter box, "All"/"None", and one-click **groups** (see below).
- A **Settings** dialog edits the configuration — the scanned repositories
  first of all — once unlocked with the edit token (see below).
- **Clicking a repository's card name** narrows the whole analysis to it.
- A **parameters form** re-runs the analysis on the fly (weeks, delta, user or
  all users, repositories, include/exclude patterns).
- **Clicking a contributor** filters the whole view to that person (all their
  identities).
- **Export JSON** downloads the current statistics.

The repositories the server scans are those it was started with, until they are
changed from the **Settings** dialog (see below); the parameters form itself
only selects among them.

### Repository selection, groups and shareable links

Every parameter — the repository selection included — is mirrored in the page
URL, so the address bar always describes what is displayed and can be shared or
bookmarked as is:

```
http://localhost:8080/?repo=/wd/api&repo=/wd/ui   # these two repositories
http://localhost:8080/?group=Frontend             # a saved group
http://localhost:8080/?group=Frontend&weeks=8&countAll=true
```

A selection that reproduces a saved group travels as `?group=<name>` rather
than as a list of paths, so the link keeps working when the group's content
changes. Opening such a link restores the selection in the picker.

Groups are named selections, created and deleted from the picker itself:

1. tick the repositories (the filter box narrows a long list),
2. click **Save as group…**, type a name, **Save** — an existing name is
   overwritten, so a group is updated the same way it is created,
3. click a group chip to recall its selection, or its **×** to delete it.

They are stored in a JSON file of their own — `<home>/.gitcontrib-groups.json`
by default, `--groups-file` or the config's `web.groupsFile` to move it — kept
apart from the analysis config so that saving from the UI cannot disturb what
you hand-wrote there:

```json
{
  "groups": [
    { "name": "Frontend", "repositories": ["/wd/agora-ui", "/wd/catalog-ui"] },
    { "name": "Backend", "repositories": ["/wd/agora-api", "/wd/cart"] }
  ]
}
```

The file can also be written by hand (the server reads it at startup). A group
may name a repository that is not currently scanned: such an entry is simply
ignored when the group is applied, so the same file can be shared between
setups. A group that ends up with no scanned repository is reported as an
error rather than silently analyzing everything.

### Identity cards

Every repository gets an identity card, and all of them together get a
combined one. Unlike the rest of the page, a card does not depend on the
analysis window: it describes the repository itself.

- **What it is** — name and path, remote URL (credentials stripped), current
  branch, HEAD commit (hash, subject, date), number of tags and branches.
- **Contributions (all time)** — commits and merge commits over the whole
  history reachable from HEAD, contributors (grouped by email, `.mailmap`
  honored), top authors with their commit counts, first and last commit, age,
  distinct active days, commits per week, plus how many of those commits fall
  in the window currently analyzed.
- **Code (current version)** — the files tracked by HEAD: lines of code and
  total lines, main language and per-language shares, files, size, packages
  (directories holding at least one source file), directories, and the
  dependencies declared by the manifests found at the root or up to two
  directories deep (`go.mod`, `package.json`, `requirements.txt`,
  `pyproject.toml`, `Cargo.toml`, `composer.json`, `Gemfile`, `pom.xml`,
  `build.gradle[.kts]`), counted as direct / indirect / dev.

Vendored directories (`vendor`, `node_modules`, `venv`) are left out of the
code figures — they are dependencies, not the repository's own code — and
binary files count toward the size but not toward the lines. Lines of code
count the source languages only, leaving out documentation, data and
configuration files, which the "all lines" figure includes.

Cards are computed per repository and kept in memory (they are not written to
the cache file), warmed at startup and rebuilt once older than the TTL or when
a refresh is requested. The first build of a large repository walks its whole
history, so it can take a moment; the page shows the cards as soon as they are
ready.

### Settings and the edit token

The **Settings** dialog writes to the config file and applies the result at
once — no restart:

- **Repositories** — add one by path (a folder that is not a repository is
  replaced by the repositories it holds, `~` and `$VARS` expanded), or remove
  one. Saving re-points the server: the picker, the statistics and the identity
  cards follow immediately.
- **Default analysis** — weeks, delta, user, count-all, merge, include/exclude
  patterns. An emptied field drops out of the file, bringing back the value the
  server was started with.
- **Server** — the cache lifetime, applied on save. The listen address and the
  cache/groups paths are shown for reference only: they follow the command line
  and need a restart.

Every change made from the UI — the settings, the repositories and the
repository groups — requires an **edit token**, set in the config file:

```json
{ "web": { "editToken": "a-long-random-string" } }
```

Without it the server refuses every write (`403`) and the UI stays read-only,
which is what a server started with no token is meant to be. With it, the UI
asks for the token once ("Unlock"), keeps it in the browser's local storage,
and sends it as an `X-Gitcontrib-Token` header; a wrong token is a `401`. The
token is never sent back by the API, and a save from the UI preserves it — as
well as every other key of the config file it does not edit.

The token is only as private as the config file and the network the server
listens on: keep the file readable by you alone (it is a shared secret, not a
password), and prefer binding to `localhost` when the token is not set.

### Web flags

- `--addr` — listen address (default `:8080`).
- `--ttl` — cache lifetime before a background refresh, e.g. `30s`, `5m`, `1h`;
  `0` disables auto-refresh (default `5m`).
- `--cache-file` — JSON cache path (default `<home>/.gitcontrib-cache.json`).
- `--groups-file` — JSON repository-groups path (default
  `<home>/.gitcontrib-groups.json`).

Plus all the common/filtering flags listed above.

### Caching

Statistics are scanned at startup and cached to a JSON file. Every request is
served from the cache; once an entry is older than the TTL it is still served
immediately while a refresh runs in the background (stale-while-revalidate). A
refresh can also be forced from the UI or via `POST /api/refresh`. Each distinct
parameter set is cached independently.

## HTTP API

| Method & path | Description |
| --- | --- |
| `GET /` | The single-page web UI. |
| `GET /api/stats` | Aggregated statistics as JSON. |
| `GET /api/identity` | Repository identity cards as JSON. |
| `GET /api/groups` | The saved repository groups. |
| `PUT /api/groups` | Replace the saved groups (`POST` also accepted). 🔒 |
| `GET /api/config` | The editable config, without the token. |
| `PUT /api/config` | Save the config and apply it (`POST` also accepted). 🔒 |
| `POST /api/repositories/scan` | Resolve a path into the repositories it holds. 🔒 |
| `POST /api/refresh` | Trigger a background refresh (returns `202`). |

Both API endpoints accept the analysis parameters as query string:

| Query param | Meaning |
| --- | --- |
| `weeks` | number of weeks |
| `delta` | window shift, `<n>[y\|m\|w\|d]` |
| `user` | name or email filter (comma-separated) |
| `countAll` | `true`/`false` — analyze everyone |
| `merge` | `true`/`false` — merge folders |
| `repo` | restrict to the given folders — repeat it, or separate them with commas |
| `group` | restrict to a saved group of repositories (used when `repo` is absent) |
| `include` / `exclude` | comma-separated file-pattern regexes |

Example: `GET /api/stats?weeks=8&user=someone@example.com`.

`/api/identity` only reads the `repo` and `group` parameters — the cards do not
depend on the analysis window. Its response holds `repositories` (one card per scanned
folder, each with `history` and `code`), `all` (the same figures merged, with
contributors and active days de-duplicated across repositories) and the cache
metadata `updatedAt` / `stale` / `refreshing` / `ttlSeconds`.

The `/api/stats` response (top-level fields) includes: `user`, `beginOfScan`,
`endOfScan`, `durationInDays`, `totalCommits`, `analyzedRepos`, `errors`,
`commitsByHour` (24), `commitsByWeekday` (7, Monday-first), `punchcard`
(`[7][24]`, Monday-first × hour), `repositories`, `contributors` (with merged
`identities`), `languages`, `commitTypes`, `calendar` (per-day `count`,
`additions`, `deletions`), plus the applied `params` (including the
selected `repos` and the `group` they reproduce), `availableRepos`, and the
cache metadata `updatedAt` / `stale` / `refreshing` / `ttlSeconds`.

🔒 marks the endpoints that need the `X-Gitcontrib-Token` header: they answer
`403` while no `web.editToken` is configured, and `401` when the token does not
match.

`PUT /api/config` takes `{"config": {…}}` with the editable fields (`weeks`,
`delta`, `user`, `countAll`, `merge`, `folders`, `includePatterns`,
`excludePatterns`, `ttl`); a `null` field removes it from the file. The answer
is the same shape as `GET /api/config`: the stored config, the folders now
scanned, the read-only `web` settings, and the `editable` / `authorized` flags.
Invalid edits — a bad delta or regex, a folder holding no repository, an empty
repository list — are a `400`, and change nothing.

`POST /api/repositories/scan` takes `{"path": "…"}` and answers
`{"path": …, "folders": [...]}`: what the path resolves to, without saving
anything. It is how the UI turns a typed path into repositories.

`PUT /api/groups` takes the whole list — `{"groups":[{"name":…,"repositories":[…]}]}`
— and answers with what was stored. A group needs a name and at least one
repository, and two groups cannot share a name (case-insensitively); anything
else is a `400`.

## Author identities & .mailmap

Identities are grouped by email (like `git shortlog`), so one person committing
under a stable email with several name spellings is counted once; the displayed
name is the spelling with the most changes. Grouping by email — rather than
name — avoids letting a bot that authors commits under a human's email bridge
unrelated people together.

To unify a person's several emails, or to remap bot/old identities, add a
repository [`.mailmap`](https://git-scm.com/docs/gitmailmap) file; all standard
forms are honored.

## Development

```sh
go build ./...                 # build
go test ./...                  # run the test suite
go test -tags safe ./...       # run with the "safe" build tag (as CI does)
golangci-lint run ./...        # lint (golangci-lint v2)
```

CI (GitHub Actions, [`.github/workflows/go.yml`](.github/workflows/go.yml))
runs the linter and tests on Linux and macOS, using the Go version declared in
`go.mod`.
