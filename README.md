Command line tool to display the git contributions with a calendar heatmap

Show contribution on a repository
Default values:
  * The weeks computed fit the terminal width
  * The user computed is parsed from .gitconfig file if exists
```
gitcontribution stat
.
Scanning for steeve.vandecappelle@corp.ovh.com contributions from 2021-08-16 00:00:00 +0200 CEST to 2021-11-29 00:00:00 +0100 CET

                     Sep             Oct             Nov
       -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -
 Fri   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   1   -
       -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   2   -
 Wed   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   3   -
       -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -
 Mon   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   2   -
       -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -
```

Show contribution multiple repositories
```
gitcontribution stat dir1 dir2
dir1
Scanning for steeve.vandecappelle@corp.ovh.com contributions from 2021-08-16 00:00:00 +0200 CEST to 2021-11-29 00:00:00 +0100 CET

                     Sep             Oct             Nov
       -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -
 Fri   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   1   -
       -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   2   -
 Wed   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   3   -
       -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -
 Mon   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   2   -
       -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -
dir2
Scanning for steeve.vandecappelle@corp.ovh.com contributions from 2021-08-16 00:00:00 +0200 CEST to 2021-11-29 00:00:00 +0100 CET

                     Sep             Oct             Nov
       -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -
 Fri   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   1   -
       -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   2   -
 Wed   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   3   -
       -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -
 Mon   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   2   -
       -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -
```

Show contribution of specific users
```
gitcontribution stat "Firstname Name,myfriend@email.com"
```

Show merged configuration for multiple folders
```
gitcontribution stat --merge $(ls)
gitcontribution,gitcontribution2
Scanning for steeve.vandecappelle@corp.ovh.com contributions from 2021-08-16 00:00:00 +0200 CEST to 2021-11-29 00:00:00 +0100 CET

                     Sep             Oct             Nov
       -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -
 Fri   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   2   -
       -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   4   -
 Wed   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   6   -
       -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -
 Mon   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   4   -
       -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -   -
```

Show 4 last week contributions
```
gitcontribution stat --weeks 4
```

Show last year contribution
```
gitcontribution stat --delta 1y
```

Show all users contributions of repository
```
gitcontribution stat --count-all
```

Open an interactive terminal dashboard (heatmap, per-weekday and per-hour
charts, contributors and repositories)
```
gitcontribution dashboard
```

Share the statistics over HTTP through a JSON API and a web UI
```
gitcontribution web --addr :8080
gitcontrib web interface listening on http://localhost:8080
```
Then open http://localhost:8080 in a browser. The page shows at-a-glance
highlights (longest/current streak, most active day and hour, busiest day,
average commit size, top contributor share), the commit
calendar, weekly commits-over-time and lines-changed (additions/deletions)
trends, commits by weekday and by hour plus a weekday×hour punchcard, the
contributors ranking with a
contribution-share chart, a breakdown of changes by language / file type, and a
breakdown of commits by Conventional Commits type (feat, fix, …). The raw data
is available as JSON on `http://localhost:8080/api/stats`. Author identities
that share a name are merged, so one person committing under one name with
several emails is counted once (merging by email as well is intentionally
avoided, as bot/CI commits authored under a human's email would otherwise
bridge unrelated people together). Clicking a contributor filters
the whole view down to that person (across all their identities). An "Export
JSON" button downloads the current statistics as a JSON file.

The UI has a parameters form to re-run the analysis on the fly (number of
weeks, delta, a specific user or all users, merge, include/exclude patterns).
The scanned folders are fixed when the server starts and cannot be changed from
the UI. Each parameter set is scanned on first use and cached independently; the
API accepts the same parameters as query string, e.g.
`GET /api/stats?weeks=8&user=someone@example.com`.

The statistics are scanned at startup and cached to a JSON file. Each request
is served from the cache; once an entry is older than the TTL it is still
served immediately while a refresh runs in the background
(stale-while-revalidate). A refresh can also be forced with the "Refresh"
button in the UI or by calling `POST /api/refresh` (with the same parameters).

```
gitcontribution web --ttl 10m --cache-file /path/to/cache.json
```

- `--addr` address to listen on (default `:8080`)
- `--ttl` cache lifetime before a background refresh, e.g. `30s`, `5m`, `1h`; `0` disables auto-refresh (default `5m`)
- `--cache-file` JSON cache path (default `<home>/.gitcontrib-cache.json`)

The `web` command also accepts the same filtering flags as `dashboard`
(`--weeks`, `--delta`, `--count-all`, `--merge`, `--file-include-pattern`,
`--file-exclude-pattern`).

You can also add multiple repositories to scan each time you launch the command `gitcontribution stat` and you are not in a repository folder with
`gitcontribution add-repository <dir>`

See `gitcontribution` to show help
