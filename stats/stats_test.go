package stats_test

import (
	"sync"
	"testing"
	"time"

	"github.com/maxatome/go-testdeep/td"
	"github.com/schollz/progressbar/v3"
	"github.com/svandecappelle/gitcontrib/stats"
)

var existingUser = "Steeve Vandecappelle"
var unknownUser = "Mybest Friend"
var currentRepo = []string{".."}
var bar = progressbar.Default(-1, "Analyzing commits")
var wg sync.WaitGroup
var now = time.Now()
var begin = now.AddDate(0, 0, -28)
var days [7]int

var statResults []stats.StatsResult = []stats.StatsResult{
	stats.StatsResult{
		Options: stats.StatsOptions{
			EmailOrUsername:      nil,
			DurationParamInWeeks: 4,
			Folders:              currentRepo,
			Delta:                "",
			Silent:               true,
		},
		BeginOfScan:     begin,
		EndOfScan:       now,
		DurationInDays:  28,
		Folder:          currentRepo[0],
		Commits:         make(map[int]int),
		DayCommits:      days,
		AuthorsEditions: make(map[string]map[string]int),
	},
	stats.StatsResult{
		Options: stats.StatsOptions{
			EmailOrUsername:      &unknownUser,
			DurationParamInWeeks: 4,
			Folders:              currentRepo,
			Delta:                "",
			Silent:               true,
		},
		BeginOfScan:     begin,
		EndOfScan:       now,
		DurationInDays:  28,
		Folder:          currentRepo[0],
		Commits:         make(map[int]int),
		DayCommits:      days,
		AuthorsEditions: make(map[string]map[string]int),
	},
	stats.StatsResult{
		Options: stats.StatsOptions{
			EmailOrUsername:      &existingUser,
			DurationParamInWeeks: 4,
			Folders:              currentRepo,
			Delta:                "",
			Silent:               true,
		},
		BeginOfScan:     begin,
		EndOfScan:       now,
		DurationInDays:  28,
		Folder:          currentRepo[0],
		Commits:         make(map[int]int),
		DayCommits:      days,
		AuthorsEditions: make(map[string]map[string]int),
	},
}

func TestStatForUser(tt *testing.T) {
	for _, r := range statResults {
		t := td.NewT(tt)
		wg.Add(1)
		go stats.Stats(&r, &wg, bar)
		t.CmpNoError(r.Error)
	}
	wg.Wait()
}

func TestStatWithDelta(tt *testing.T) {
	t := td.NewT(tt)

	for _, d := range []string{"1w", "1m", "1y", "2y"} {
		options := stats.LaunchOptions{
			User:            nil,
			DurationInWeeks: 4,
			Folders:         currentRepo,
			Merge:           false,
			Delta:           d,
			Dashboard:       false,
		}
		r := stats.Launch(options)
		t.CmpNoError(r[0].Error)
	}

	options := stats.LaunchOptions{
		User:            nil,
		DurationInWeeks: 4,
		Folders:         currentRepo,
		Merge:           false,
		Delta:           "xx",
		Dashboard:       false,
	}
	r := stats.Launch(options)
	t.CmpError(r[0].Error)
	if r[0].Error != nil {
		t.Cmp(r[0].Error.Error(), "invalid delta value use the format: <int>[y/m/w/d]")
	}
}
