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
Then open http://localhost:8080 in a browser. The page shows the commit
calendar, commits by weekday and by hour, the contributors ranking and a
contribution-share chart. The raw data is available as JSON on
`http://localhost:8080/api/stats`. The `web` command accepts the same flags as
`dashboard` (`--weeks`, `--delta`, `--count-all`, `--merge`,
`--file-include-pattern`, `--file-exclude-pattern`).

You can also add multiple repositories to scan each time you launch the command `gitcontribution stat` and you are not in a repository folder with
`gitcontribution add-repository <dir>`

See `gitcontribution` to show help
