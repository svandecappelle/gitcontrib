package stats

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// GetFolders returns all the folders needs to be scanned saved in dotfile
func GetFolders() ([]string, error) {
	var repos []string
	filePath, err := getDotFilePath()
	if err != nil {
		return []string{}, err
	}
	repos = parseFileLinesToSlice(*filePath)
	if len(repos) == 0 || isRepo(".") {
		repos = []string{"."}
	}

	return repos, nil
}

// getDotFilePath returns the dot file for the repos list.
// Creates it and the enclosing folder if it does not exist.
func getDotFilePath() (*string, error) {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	dotFile := usr.HomeDir + "/.gogitstats"

	return &dotFile, nil
}

// openFile opens the file located at `filePath`. Creates it if not existing.
func openFile(filePath string) *os.File {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_RDWR, 0755)
	if err != nil {
		if os.IsNotExist(err) {
			// file does not exist
			_, err = os.Create(filePath)
			if err != nil {
				panic(err)
			}
			f, err = os.OpenFile(filePath, os.O_APPEND|os.O_RDWR, 0755)
			if err != nil {
				panic(err)
			}
		} else {
			// other error
			panic(err)
		}
	}

	return f
}

// parseFileLinesToSlice given a file path string, gets the content
// of each line and parses it to a slice of strings.
func parseFileLinesToSlice(configFilePath string) []string {
	f := openFile(configFilePath)
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		if err != io.EOF {
			panic(err)
		}
	}

	return lines
}

// sliceContains returns true if `slice` contains `value`
func sliceContains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// joinSlices adds the element of the `new` slice
// into the `existing` slice, only if not already there
func joinSlices(new []string, existing []string) []string {
	for _, i := range new {
		if !sliceContains(existing, i) {
			existing = append(existing, i)
		}
	}
	return existing
}

// dumpStringsSliceToFile writes content to the file in path `filePath` (overwriting existing content)
func dumpStringsSliceToFile(repos []string, filePath string) error {
	content := strings.Join(repos, "\n")
	return ioutil.WriteFile(filePath, []byte(content), 0755)
}

// addNewSliceElementsToFile given a slice of strings representing paths, stores them
// to the filesystem
func addNewSliceElementsToFile(filePath string, newRepos []string) error {
	existingRepos := parseFileLinesToSlice(filePath)
	repos := joinSlices(newRepos, existingRepos)
	return dumpStringsSliceToFile(repos, filePath)
}

// recursiveScanFolder starts the recursive search of git repositories
// living in the `folder` subtree
func recursiveScanFolder(folder string) ([]string, error) {
	return scanGitFolders(make([]string, 0), folder)
}

// Scan scans a new folder for Git repositories
func Scan(folder string) error {
	repositories, err := recursiveScanFolder(folder)
	if err != nil {
		return err
	}
	filePath, err := getDotFilePath()
	if err != nil {
		return err
	}
	return addNewSliceElementsToFile(*filePath, repositories)
}

// List list all repositories wich saved to scan
func List() error {
	configFilePath, err := getDotFilePath()
	if err != nil {
		return err
	}
	repositories := parseFileLinesToSlice(*configFilePath)
	fmt.Printf("Git folders:\n\n")
	for _, repository := range repositories {
		fmt.Printf("- %s\n", repository)
	}
	return nil
}

func shouldBeIgnored(folderName string) bool {
	return folderName == "vendor" || folderName == "node_modules" || folderName == "venv"
}

// scanGitFolders returns a list of subfolders of `folder` ending with `.git`.
// Returns the base folder of the repo, the .git folder parent.
// Recursively searches in the subfolders by passing an existing `folders` slice.
func scanGitFolders(folders []string, folder string) ([]string, error) {
	// trim the last `/`
	folder = strings.TrimSuffix(folder, "/")

	f, err := os.Open(folder)
	if err != nil {
		log.Fatal(err)
		return folders, err
	}

	pathFrom, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
		return folders, err
	}
	pathFrom = filepath.Join(pathFrom, folder)
	files, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		log.Fatal(err)
		return folders, err
	}
	for _, file := range files {
		if file.IsDir() {
			pathRelative := folder + "/" + file.Name()
			pathAbsolute := filepath.Join(pathFrom, file.Name())
			if file.Name() == ".git" {
				pathAbsolute = strings.TrimSuffix(pathAbsolute, "/.git")
				fmt.Printf("Folder %s added to scan list\n", pathAbsolute)
				folders = append(folders, pathAbsolute)
				continue
			}
			if shouldBeIgnored(file.Name()) {
				continue
			}
			folders, err = scanGitFolders(folders, pathRelative)
			if err != nil {
				return folders, err
			}
		}
	}

	return folders, nil
}
