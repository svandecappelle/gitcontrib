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
// Three presentations build on this:
//
//   - the console heatmap (PrintResult),
//   - the interactive terminal dashboard (OpenDashboard),
//   - the HTTP server exposing a JSON API and a single-page web UI (Serve),
//     which caches results per parameter set with a TTL and background refresh.
//
// Author identities are grouped by email (like git shortlog); a repository
// .mailmap is honored to unify or remap identities (see Mailmap).
package stats
