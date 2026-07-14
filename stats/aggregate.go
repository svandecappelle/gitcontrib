package stats

import (
	"sort"
	"time"
)

// Contributor is a single author's contribution, sorted-friendly and
// JSON-serializable.
type Contributor struct {
	Author    string `json:"author"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Total     int    `json:"total"`
}

// RepositoryStat is the commit count of a single scanned repository.
type RepositoryStat struct {
	Folder  string `json:"folder"`
	Commits int    `json:"commits"`
}

// Language is the amount of changes attributed to a programming language (or
// file type), sorted-friendly and JSON-serializable.
type Language struct {
	Name      string `json:"name"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Total     int    `json:"total"`
}

// CommitTypeCount is the number of commits of a given Conventional Commits type.
type CommitTypeCount struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// DayCount is the number of commits on a single calendar day.
type DayCount struct {
	Date    string `json:"date"`    // YYYY-MM-DD
	Count   int    `json:"count"`   // commits that day
	Weekday int    `json:"weekday"` // 0=Sunday .. 6=Saturday
}

// AggregatedStats is the whole set of statistics merged across every scanned
// repository, ready to be serialized as JSON or fed to the terminal dashboard.
type AggregatedStats struct {
	User             string            `json:"user"`
	BeginOfScan      time.Time         `json:"beginOfScan"`
	EndOfScan        time.Time         `json:"endOfScan"`
	DurationInDays   int               `json:"durationInDays"`
	TotalCommits     int               `json:"totalCommits"`
	AnalyzedRepos    int               `json:"analyzedRepos"`
	Errors           int               `json:"errors"`
	CommitsByHour    [24]int           `json:"commitsByHour"`    // index 0..23
	CommitsByWeekday [7]int            `json:"commitsByWeekday"` // Monday-first
	Repositories     []RepositoryStat  `json:"repositories"`
	Contributors     []Contributor     `json:"contributors"`
	Languages        []Language        `json:"languages"`
	CommitTypes      []CommitTypeCount `json:"commitTypes"`
	Calendar         []DayCount        `json:"calendar"`

	// merged keeps the underlying merged result so the terminal dashboard can
	// reuse the same commit map for its heatmap. Not serialized.
	merged *StatsResult
}

// Aggregate merges the per-repository results into a single AggregatedStats.
func Aggregate(results []*StatsResult) AggregatedStats {
	agg := AggregatedStats{}
	if len(results) == 0 {
		return agg
	}

	first := results[0]
	agg.BeginOfScan = first.BeginOfScan
	agg.EndOfScan = first.EndOfScan
	agg.DurationInDays = first.DurationInDays
	agg.User = "all"
	if first.Options.EmailOrUsername != nil {
		agg.User = *first.Options.EmailOrUsername
	}

	merged := &StatsResult{
		Options:        first.Options,
		BeginOfScan:    first.BeginOfScan,
		EndOfScan:      first.EndOfScan,
		DurationInDays: first.DurationInDays,
		Commits:        make(map[int]int),
	}

	editions := make(map[string][2]int)     // author -> [additions, deletions]
	langEditions := make(map[string][2]int) // language -> [additions, deletions]
	commitTypes := make(map[string]int)     // conventional type -> count

	for _, l := range results {
		if l.Error != nil {
			agg.Errors++
			continue
		}
		agg.AnalyzedRepos++

		commitsByRepo := 0
		for i, commit := range l.Commits {
			agg.TotalCommits += commit
			commitsByRepo += commit
			merged.Commits[i] += commit
		}
		if commitsByRepo > 0 {
			agg.Repositories = append(agg.Repositories, RepositoryStat{
				Folder:  l.Folder,
				Commits: commitsByRepo,
			})
		}

		for i, v := range l.DayCommits {
			agg.CommitsByWeekday[(i+6)%7] += v
		}
		for i, v := range l.HoursCommits {
			agg.CommitsByHour[i] += v
		}
		for author, c := range l.AuthorsEditions {
			e := editions[author]
			e[0] += c["additions"]
			e[1] += c["deletions"]
			editions[author] = e
		}
		for lang, c := range l.LanguageEditions {
			e := langEditions[lang]
			e[0] += c["additions"]
			e[1] += c["deletions"]
			langEditions[lang] = e
		}
		for t, n := range l.CommitTypes {
			commitTypes[t] += n
		}
	}

	// Collapse the per-identity editions into one entry per person.
	agg.Contributors = mergeAuthorAliases(editions)

	for lang, e := range langEditions {
		agg.Languages = append(agg.Languages, Language{
			Name:      lang,
			Additions: e[0],
			Deletions: e[1],
			Total:     e[0] + e[1],
		})
	}
	sort.Slice(agg.Languages, func(i, j int) bool {
		return agg.Languages[i].Total > agg.Languages[j].Total
	})

	for t, n := range commitTypes {
		agg.CommitTypes = append(agg.CommitTypes, CommitTypeCount{Type: t, Count: n})
	}
	sort.Slice(agg.CommitTypes, func(i, j int) bool {
		return agg.CommitTypes[i].Count > agg.CommitTypes[j].Count
	})

	agg.Calendar = buildCalendar(merged)
	agg.merged = merged
	return agg
}

// buildCalendar turns the merged commit map into a chronological list of daily
// counts, using the same index arithmetic as the terminal heatmap so both
// views show identical values.
func buildCalendar(r *StatsResult) []DayCount {
	end := getEndOfDay(r.EndOfScan)
	var days []DayCount
	for d := r.BeginOfScan; d.Before(end); d = d.AddDate(0, 0, 1) {
		idx := int(end.Sub(d).Hours()/24) + 8
		days = append(days, DayCount{
			Date:    d.Format("2006-01-02"),
			Count:   r.Commits[idx],
			Weekday: int(d.Weekday()),
		})
	}
	return days
}
