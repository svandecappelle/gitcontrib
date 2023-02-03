package stats_test

import (
	"testing"

	"github.com/maxatome/go-testdeep/td"
	"github.com/svandecappelle/gitcontrib/stats"
)

func TestAddRepoToScanFromFolder(tt *testing.T) {
	t := td.NewT(tt)
	folders, err := stats.GetFolders()
	t.CmpNoError(err)
	t.Cmp(folders, td.Len(1))
	t.Cmp(folders[0], ".")
}

func TestUserDotFile(tt *testing.T) {
	t := td.NewT(tt)
	file, err := stats.GetDotFilePath()

	t.CmpNoError(err)
	t.Cmp(file, td.NotNil())
}

func TestIgnoreFolders(tt *testing.T) {
	t := td.NewT(tt)
	t.True(stats.ShouldBeIgnored("node_modules"), "node_modules should be ignored from stats")
	t.True(stats.ShouldBeIgnored("venv"), "venv should be ignored from stats")
	t.True(stats.ShouldBeIgnored("vendor"), "vendor folder should be ignored from stats")
	t.False(stats.ShouldBeIgnored("tests"), "tests folder should not be ignored from stats")
	t.False(stats.ShouldBeIgnored("src"), "src folder and others should not be ignored from stats")
}

func TestListReposNoConfig(tt *testing.T) {
	t := td.NewT(tt)

	err := stats.List()
	t.CmpNoError(err)
}

func TestLunchStatsNoConfig(tt *testing.T) {
	t := td.NewT(tt)

	folders, err := stats.ScanGitFolders([]string{}, "..")
	t.CmpNoError(err)
	t.Cmp(folders, td.Len(1))
}
