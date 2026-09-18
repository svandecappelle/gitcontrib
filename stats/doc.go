// Package stats computes and presents git contribution statistics.
//
// The core entry point is [Launch], which scans one or more repositories with a
// [LaunchOptions] and returns a [StatsResult] per scanned unit. [Aggregate]
// merges those results into a single [AggregatedStats] — the shape consumed by
// both the terminal dashboard and the web API — containing the commit calendar,
// per-weekday/per-hour counts and the weekday×hour punchcard, the contributors
// ranking, the language and Conventional Commits breakdowns, and per-day
// additions/deletions.
//
// Alongside the window-based statistics, [BuildIdentity] reads a repository's
// identity card: what the repository is (remote, branch, HEAD, tags), what its
// whole history says about contributions (commits, contributors, age, activity)
// and how much code its current version holds (files, lines, packages,
// languages and declared dependencies). [AggregateIdentities] merges the cards
// of several repositories into one overview.
//
// Three presentations build on this:
//
//   - the console heatmap (PrintResult),
//   - the interactive terminal dashboard (OpenDashboard),
//   - the HTTP server exposing a JSON API and a single-page web UI (Serve),
//     which caches results per parameter set with a TTL and background refresh,
//     serves the identity cards on /api/identity, and lets a client restrict
//     every view to a selection of the scanned repositories — spelled out, or
//     named through a saved [RepoGroup] (/api/groups). The analysis config
//     itself, the scanned repositories included, can be edited through
//     /api/config and applied without a restart ([SaveConfig], [ApplyConfig]);
//     every endpoint that writes needs the edit token of [ServeOptions].
//
// Author identities are grouped by email (like git shortlog); a repository
// .mailmap is honored to unify or remap identities (see Mailmap).
package stats
