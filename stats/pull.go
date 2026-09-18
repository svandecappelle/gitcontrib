package stats

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// statusTimeout caps a local git call; pullTimeout a networked one.
	statusTimeout = 30 * time.Second
	pullTimeout   = 3 * time.Minute
	// pullParallelism limits how many repositories are fetched at once, so
	// updating twenty of them does not open twenty connections to a server.
	pullParallelism = 4
)

// RepositoryStatus is the working state of a repository: what its worktree
// holds that its history does not, and how it stands against its upstream. It
// is what tells whether the repository can be updated.
type RepositoryStatus struct {
	Folder    string `json:"folder"`
	Name      string `json:"name"`
	Branch    string `json:"branch"`
	Upstream  string `json:"upstream,omitempty"`
	Ahead     int    `json:"ahead"`
	Behind    int    `json:"behind"`
	Changes   int    `json:"changes"`   // tracked files added, modified or deleted
	Untracked int    `json:"untracked"` // files git does not follow
	Clean     bool   `json:"clean"`     // no tracked change at all
	Pullable  bool   `json:"pullable"`  // clean, and following an upstream branch
	Reason    string `json:"reason,omitempty"`
	Error     string `json:"error,omitempty"`
}

// PullResult is what became of one repository asked to update.
type PullResult struct {
	Folder  string `json:"folder"`
	Name    string `json:"name"`
	Updated bool   `json:"updated"` // its HEAD moved
	Skipped bool   `json:"skipped"` // nothing was attempted, Reason says why
	Reason  string `json:"reason,omitempty"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

// gitAvailable reports whether the git command can be run at all. Updating a
// repository goes through git itself rather than through a library, so that it
// uses the very credentials, SSH agent and configuration the person already
// pulls with from their terminal.
func gitAvailable() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("the git command is required to update repositories: %w", err)
	}
	return nil
}

// runGit runs a git command inside a repository. It never lets git ask a
// question: a server waiting on a passphrase prompt would hang for good.
func runGit(folder string, timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", folder}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "GCM_INTERACTIVE=never")
	if os.Getenv("GIT_SSH_COMMAND") == "" {
		cmd.Env = append(cmd.Env, "GIT_SSH_COMMAND=ssh -oBatchMode=yes")
	}

	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if ctx.Err() != nil {
		return output, fmt.Errorf("git %s timed out after %s", args[0], timeout)
	}
	if err != nil {
		if output == "" {
			return "", err
		}
		return output, fmt.Errorf("%s", firstLines(output, 3))
	}
	return output, nil
}

// firstLines keeps the beginning of a git message, which is where it says what
// went wrong.
func firstLines(output string, n int) string {
	lines := strings.Split(output, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "; ")
}

// StatusOf reads the working state of one repository.
func StatusOf(folder string) RepositoryStatus {
	status := RepositoryStatus{Folder: folder, Name: repoName(folder)}
	if err := gitAvailable(); err != nil {
		status.Error = err.Error()
		return status
	}

	out, err := runGit(folder, statusTimeout, "status", "--porcelain=v1", "--branch", "--untracked-files=normal")
	if err != nil {
		status.Error = err.Error()
		return status
	}

	for _, line := range strings.Split(out, "\n") {
		switch {
		case line == "":
		case strings.HasPrefix(line, "## "):
			parseBranchLine(strings.TrimPrefix(line, "## "), &status)
		case strings.HasPrefix(line, "?? "):
			status.Untracked++
		default:
			status.Changes++
		}
	}

	status.Clean = status.Changes == 0
	switch {
	case !status.Clean:
		status.Reason = fmt.Sprintf("%d local change%s", status.Changes, plural(status.Changes))
	case status.Branch == "":
		status.Reason = "no branch checked out"
	case status.Upstream == "":
		status.Reason = "no upstream branch"
	default:
		status.Pullable = true
	}
	return status
}

// parseBranchLine reads the "## branch...upstream [ahead 1, behind 2]" header
// of a porcelain status.
func parseBranchLine(line string, status *RepositoryStatus) {
	if strings.HasPrefix(line, "HEAD (no branch)") {
		return // detached: there is nothing to pull into
	}
	tracking := line
	if open := strings.Index(line, " ["); open >= 0 {
		tracking = line[:open]
		for _, part := range strings.Split(strings.Trim(line[open+2:], "[]"), ", ") {
			name, value, ok := strings.Cut(part, " ")
			if !ok {
				continue
			}
			n, err := strconv.Atoi(value)
			if err != nil {
				continue
			}
			switch name {
			case "ahead":
				status.Ahead = n
			case "behind":
				status.Behind = n
			}
		}
	}
	branch, upstream, _ := strings.Cut(tracking, "...")
	status.Branch = strings.TrimSpace(branch)
	status.Upstream = strings.TrimSpace(upstream)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// RepositoryStatuses reads the working state of several repositories at once.
func RepositoryStatuses(folders []string) []RepositoryStatus {
	statuses := make([]RepositoryStatus, len(folders))
	var wg sync.WaitGroup
	for i, folder := range folders {
		wg.Add(1)
		go func(i int, folder string) {
			defer wg.Done()
			statuses[i] = StatusOf(folder)
		}(i, folder)
	}
	wg.Wait()
	return statuses
}

// PullRepository brings one repository up to date, and only ever does so by
// fast-forward: a repository holding local changes, or whose branch has
// diverged from its upstream, is left exactly as it was.
func PullRepository(folder string) PullResult {
	result := PullResult{Folder: folder, Name: repoName(folder)}

	status := StatusOf(folder)
	if status.Error != "" {
		result.Error = status.Error
		return result
	}
	if !status.Pullable {
		result.Skipped = true
		result.Reason = status.Reason
		return result
	}

	before, _ := runGit(folder, statusTimeout, "rev-parse", "HEAD")
	out, err := runGit(folder, pullTimeout, "pull", "--ff-only")
	if err != nil {
		result.Error = pullFailure(out, err)
		return result
	}
	after, _ := runGit(folder, statusTimeout, "rev-parse", "HEAD")

	result.Output = firstLines(out, 2)
	result.Updated = before != after
	return result
}

// pullFailure turns git's report of a failed pull into the one line worth
// showing, naming the few causes that have a plain explanation.
func pullFailure(output string, err error) string {
	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "diverg"), strings.Contains(lower, "not possible to fast-forward"):
		return "the branch and its upstream have diverged: it needs a merge or a rebase"
	case strings.Contains(lower, "would be overwritten"):
		return "local files would be overwritten: move them aside first"
	case strings.Contains(lower, "authentication failed"),
		strings.Contains(lower, "could not read username"),
		strings.Contains(lower, "permission denied"),
		strings.Contains(lower, "host key verification failed"):
		return "the remote refused the connection: gitcontrib never asks for a passphrase, so the credentials must work unattended"
	case strings.Contains(lower, "could not resolve host"),
		strings.Contains(lower, "connection timed out"),
		strings.Contains(lower, "network is unreachable"),
		strings.Contains(lower, "connection refused"):
		return "the remote could not be reached"
	}
	if summary := meaningfulLine(output); summary != "" {
		return summary
	}
	return err.Error()
}

// meaningfulLine skips what git says about the fetch itself — where it fetched
// from and which refs moved — to reach the line explaining the failure.
func meaningfulLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "",
			strings.HasPrefix(line, "From "),
			strings.HasPrefix(line, "*"),
			strings.HasPrefix(line, "+"),
			strings.HasPrefix(line, "="),
			strings.HasPrefix(line, "remote:"),
			strings.Contains(line, "->"):
			continue
		}
		return line
	}
	return ""
}

// PullRepositories updates several repositories, a few at a time, and reports
// what happened to each of them in the order they were given.
func PullRepositories(folders []string) []PullResult {
	results := make([]PullResult, len(folders))
	tokens := make(chan struct{}, pullParallelism)
	var wg sync.WaitGroup

	for i, folder := range folders {
		wg.Add(1)
		go func(i int, folder string) {
			defer wg.Done()
			tokens <- struct{}{}
			defer func() { <-tokens }()
			results[i] = PullRepository(folder)
		}(i, folder)
	}
	wg.Wait()
	return results
}
