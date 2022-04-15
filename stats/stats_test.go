package stats

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/maxatome/go-testdeep/td"
)

var existingUser = "Steeve Vandecappelle"
var unknownUser = "Mybest Friend"
var currentRepo = []string{".."}
var invalidRepo = []string{"../.."}

func TestFolderIsRepo(t *testing.T) {
	t.Run("Check folder is a repository", func(t *testing.T) {
		if !isRepo(".") {
			t.Errorf("isRepo should be true")
		}
	})
}

func TestStatRepoFromFolder(tt *testing.T) {
	t := td.NewT(tt)
	_, err := Stats(StatsOptions{nil, 4, currentRepo, "", true})
	t.CmpNoError(err)
}

func TestStatForOneUser(tt *testing.T) {
	t := td.NewT(tt)
	_, err := Stats(StatsOptions{&unknownUser, 4, currentRepo, "", true})
	t.CmpNoError(err)

	_, err = Stats(StatsOptions{&existingUser, 4, currentRepo, "", true})
	t.CmpNoError(err)
}

func TestStatWithDelta(tt *testing.T) {
	t := td.NewT(tt)
	_, err := Stats(StatsOptions{nil, 4, currentRepo, "1d", true})
	t.CmpNoError(err)

	_, err = Stats(StatsOptions{nil, 4, currentRepo, "1w", true})
	t.CmpNoError(err)

	_, err = Stats(StatsOptions{nil, 4, currentRepo, "1m", true})
	t.CmpNoError(err)

	_, err = Stats(StatsOptions{nil, 4, currentRepo, "1y", true})
	t.CmpNoError(err)

	_, err = Stats(StatsOptions{nil, 4, currentRepo, "2y", true})
	t.CmpNoError(err)

	_, err = Stats(StatsOptions{nil, 4, currentRepo, "xx", true})
	t.CmpError(err)
	t.Cmp(err.Error(), "invalid delta value use the format: <int>[y/m/w/d]")
}

func TestGetCommitMap(tt *testing.T) {
	t := td.NewT(tt)

	now := time.Now()
	daysNum := 4 * 7

	commits, err := processRepositories(&unknownUser, currentRepo, now, daysNum)
	t.CmpNoError(err)
	t.Cmp(commits, td.Len(daysNum), fmt.Sprintf(
		"Commit map should fill %d days but get %d days",
		daysNum,
		len(commits)),
	)

	aDate := time.Date(2021, time.November, 24, 12, 0, 0, 1, time.UTC)

	commits, err = processRepositories(&existingUser, currentRepo, aDate, daysNum)
	t.CmpNoError(err)
	t.Cmp(commits, td.Len(daysNum), fmt.Sprintf(
		"Commit map should fill %d days but get %d days",
		daysNum,
		len(commits),
	))

	// t.Errorf("Commit map should fill %d days but get %d days", daysNum, len(commits))
	totalCount := 0
	for _, count := range commits {
		totalCount += count
	}
	t.Cmp(totalCount, td.Gt(0), fmt.Sprintf(
		"Commit of this repos should be at %d but it was %d",
		6,
		totalCount,
	))

	commits, err = processRepositories(&unknownUser, invalidRepo, now, daysNum)
	t.CmpError(err)
	t.Cmp(err.Error(), fmt.Sprintf(
		"cannot get stat from folder (not a repository): %s",
		strings.Join(invalidRepo, ","),
	))
	t.Cmp(commits, td.Len(daysNum))
}
