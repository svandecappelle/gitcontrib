package stats

import (
	"testing"
	"time"
)

func TestAggregate(t *testing.T) {
	begin := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC) // a Monday
	end := begin.AddDate(0, 0, 6)

	mk := func(folder string) *StatsResult {
		r := &StatsResult{
			Options:          StatsOptions{},
			BeginOfScan:      begin,
			EndOfScan:        end,
			DurationInDays:   7,
			Folder:           folder,
			Commits:          map[int]int{1: 2, 2: 1},
			AuthorsEditions:  map[string]map[string]int{},
			LanguageEditions: map[string]map[string]int{},
			CommitTypes:      map[string]int{},
			DayEditions:      map[int][2]int{},
		}
		return r
	}

	r1 := mk("repoA")
	r1.HoursCommits[9] = 2
	r1.HoursCommits[14] = 1
	r1.DayCommits[1] = 3 // Monday (Go weekday 1)
	r1.Punchcard[1][9] = 2
	r1.Punchcard[1][14] = 1
	r1.AuthorsEditions["Alice"+authorIDSep+"a@e"] = map[string]int{"additions": 10, "deletions": 2}
	r1.LanguageEditions["Go"] = map[string]int{"additions": 8, "deletions": 1}
	r1.CommitTypes["feat"] = 2
	r1.CommitTypes["fix"] = 1

	r2 := mk("repoB")
	r2.HoursCommits[9] = 1
	r2.DayCommits[3] = 1 // Wednesday
	r2.Punchcard[3][9] = 1
	r2.AuthorsEditions["Alice"+authorIDSep+"a@e"] = map[string]int{"additions": 5, "deletions": 0}
	r2.LanguageEditions["Go"] = map[string]int{"additions": 4, "deletions": 0}
	r2.CommitTypes["feat"] = 1

	agg := Aggregate([]*StatsResult{r1, r2})

	if agg.TotalCommits != 6 { // (2+1) per repo
		t.Errorf("TotalCommits = %d, want 6", agg.TotalCommits)
	}
	if agg.AnalyzedRepos != 2 {
		t.Errorf("AnalyzedRepos = %d, want 2", agg.AnalyzedRepos)
	}

	// Punchcard row/column sums must match the 1D projections.
	for d := 0; d < 7; d++ {
		sum := 0
		for h := 0; h < 24; h++ {
			sum += agg.Punchcard[d][h]
		}
		if sum != agg.CommitsByWeekday[d] {
			t.Errorf("punchcard row %d sum = %d, CommitsByWeekday = %d", d, sum, agg.CommitsByWeekday[d])
		}
	}
	for h := 0; h < 24; h++ {
		sum := 0
		for d := 0; d < 7; d++ {
			sum += agg.Punchcard[d][h]
		}
		if sum != agg.CommitsByHour[h] {
			t.Errorf("punchcard col %d sum = %d, CommitsByHour = %d", h, sum, agg.CommitsByHour[h])
		}
	}

	// Contributor merged across the two repos.
	if len(agg.Contributors) != 1 {
		t.Fatalf("want 1 contributor, got %d", len(agg.Contributors))
	}
	if c := agg.Contributors[0]; c.Author != "Alice" || c.Additions != 15 || c.Deletions != 2 || c.Total != 17 {
		t.Errorf("contributor = %+v, want Alice +15 -2 =17", c)
	}

	// Languages merged.
	if len(agg.Languages) != 1 || agg.Languages[0].Name != "Go" || agg.Languages[0].Total != 13 {
		t.Errorf("languages = %+v, want Go total 13", agg.Languages)
	}

	// Commit types merged and sorted (feat=3 before fix=1).
	if len(agg.CommitTypes) != 2 || agg.CommitTypes[0].Type != "feat" || agg.CommitTypes[0].Count != 3 {
		t.Errorf("commit types = %+v, want feat=3 first", agg.CommitTypes)
	}
}
