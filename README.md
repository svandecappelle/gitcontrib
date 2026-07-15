# gitcontrib

Command-line tool to explore git contributions — as a terminal calendar
heatmap, an interactive terminal dashboard, or a web interface backed by a JSON
API.

## Features

- **Calendar heatmap** of commits (GitHub-style) in the terminal.
- **Interactive TUI dashboard**: heatmap, commits per weekday and per hour,
  contributors and repositories.
- **Web interface + JSON API**: highlights, commit calendar, commits-over-time
  and lines-changed (additions/deletions) trends, commits by weekday/hour and a
  weekday×hour punchcard, contributors ranking with a contribution-share donut,
  breakdown by language / file type and by Conventional Commits type, JSON
  export, per-contributor drill-down, and live re-parametrization.
- Filter by user, date range, and file patterns; scan one or many repositories.
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
otherwise as a user (name or `email`, comma-separated for several). With no
folder argument, the current repository (or the saved list) is scanned.

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
    "cacheFile": "/tmp/gitcontrib-cache.json"
  }
}
```

```sh
gitcontribution web --config ./gitcontrib.json
gitcontribution stat --weeks 4   # --weeks overrides the config's "weeks"
```

Path fields (`folders`, `web.cacheFile`) expand environment variables and a
leading `~`, e.g. `"$HOME/wd"` or `"~/wd"`. A folder that is not a repository is
expanded to its direct repository subfolders (see above).

## Web interface

```sh
gitcontribution web --addr :8080
# gitcontrib web interface listening on http://localhost:8080
```

Open http://localhost:8080. The page shows at-a-glance highlights
(longest/current streak, most active day and hour, busiest day, average commit
size, top contributor share), the commit calendar, weekly commits-over-time and
lines-changed trends, commits by weekday and hour plus a weekday×hour
punchcard, the contributors ranking with a contribution-share donut, a
breakdown by language / file type, and a breakdown by Conventional Commits type.

- A **parameters form** re-runs the analysis on the fly (weeks, delta, user or
  all users, repository, include/exclude patterns).
- **Clicking a contributor** filters the whole view to that person (all their
  identities).
- **Export JSON** downloads the current statistics.

The set of scanned folders is fixed when the server starts and cannot be
changed from the UI (only a repository already scanned can be selected).

### Web flags

- `--addr` — listen address (default `:8080`).
- `--ttl` — cache lifetime before a background refresh, e.g. `30s`, `5m`, `1h`;
  `0` disables auto-refresh (default `5m`).
- `--cache-file` — JSON cache path (default `<home>/.gitcontrib-cache.json`).

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
| `POST /api/refresh` | Trigger a background refresh (returns `202`). |

Both API endpoints accept the analysis parameters as query string:

| Query param | Meaning |
| --- | --- |
| `weeks` | number of weeks |
| `delta` | window shift, `<n>[y\|m\|w\|d]` |
| `user` | name or email filter (comma-separated) |
| `countAll` | `true`/`false` — analyze everyone |
| `merge` | `true`/`false` — merge folders |
| `repo` | restrict to one of the configured folders |
| `include` / `exclude` | comma-separated file-pattern regexes |

Example: `GET /api/stats?weeks=8&user=someone@example.com`.

The `/api/stats` response (top-level fields) includes: `user`, `beginOfScan`,
`endOfScan`, `durationInDays`, `totalCommits`, `analyzedRepos`, `errors`,
`commitsByHour` (24), `commitsByWeekday` (7, Monday-first), `punchcard`
(`[7][24]`, Monday-first × hour), `repositories`, `contributors` (with merged
`identities`), `languages`, `commitTypes`, `calendar` (per-day `count`,
`additions`, `deletions`), plus the applied `params`, `availableRepos`, and the
cache metadata `updatedAt` / `stale` / `refreshing` / `ttlSeconds`.

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
