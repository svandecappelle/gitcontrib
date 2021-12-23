package main

import (
	"testing"

	"github.com/maxatome/go-testdeep/td"
)

func TestAddRepoToScanFromFolder(tt *testing.T) {
	t := td.NewT(tt)
	folders, err := GetFolders()
	t.CmpNoError(err)
	t.Cmp(folders, td.Len(1))
	t.Cmp(folders[0], ".")
}

func TestUserDotFile(tt *testing.T) {
	t := td.NewT(tt)
	file, err := getDotFilePath()

	t.CmpNoError(err)
	t.Cmp(file, td.NotNil())
}

func TestIgnoreFolders(tt *testing.T) {
	t := td.NewT(tt)
	t.True(shouldBeIgnored("node_modules"), "node_modules should be ignored from stats")
	t.True(shouldBeIgnored("venv"), "venv should be ignored from stats")
	t.True(shouldBeIgnored("vendor"), "vendor folder should be ignored from stats")
	t.False(shouldBeIgnored("tests"), "tests folder should not be ignored from stats")
	t.False(shouldBeIgnored("src"), "src folder and others should not be ignored from stats")
}

func TestListReposNoConfig(tt *testing.T) {
	t := td.NewT(tt)

	err := List()
	t.CmpNoError(err)
}

func TestLunchStatsNoConfig(tt *testing.T) {
	t := td.NewT(tt)

	folders, err := scanGitFolders([]string{}, ".")
	t.CmpNoError(err)
	t.Cmp(folders, td.Len(1))
}
