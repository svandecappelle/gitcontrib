package stats

import (
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/schollz/progressbar/v3"
)

const outOfRange = 99999

var DefaultDurationInDays = 365

type LaunchOptions struct {
	User             *string
	DurationInWeeks  int
	Folders          []string
	Merge            bool
	Delta            string
	Dashboard        bool
	PatternToExclude []string
	PatternToInclude []string
}

type StatsResult struct {
	Options          StatsOptions
	BeginOfScan      time.Time
	EndOfScan        time.Time
	DurationInDays   int
	Folder           string
	Commits          map[int]int
	HoursCommits     [24]int
	DayCommits       [7]int
	AuthorsEditions  map[string]map[string]int
	LanguageEditions map[string]map[string]int
	CommitTypes      map[string]int
	Error            error
}

type StatsOptions struct {
	EmailOrUsername      *string
	DurationParamInWeeks int
	Folders              []string
	Delta                string
	Silent               bool
	PatternToExclude     []string
	PatternToInclude     []string
}

func isRepo(path string) bool {
	_, err := git.PlainOpen(path)
	return err == nil
}

// compilePatterns compiles every pattern in the slice, returning an error as
// soon as one of them is not a valid regular expression.
func compilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
		}
		compiled = append(compiled, re)
	}
	return compiled, nil
}

// TODO use an interface object in order to refacto in same place the statistic run logic and then print results

func Launch(opts LaunchOptions) []*StatsResult {
	results := []*StatsResult{}
	var wg sync.WaitGroup
	bar := newProgressBar(opts.Dashboard)

	// When merging, every folder is scanned into a single result; otherwise
	// each folder is scanned independently.
	var folderSets [][]string
	if opts.Merge {
		folderSets = [][]string{opts.Folders}
	} else {
		for _, folder := range opts.Folders {
			folderSets = append(folderSets, []string{folder})
		}
	}

	for _, folders := range folderSets {
		r := &StatsResult{
			Options: StatsOptions{
				EmailOrUsername:      opts.User,
				DurationParamInWeeks: opts.DurationInWeeks,
				Folders:              folders,
				Delta:                opts.Delta,
				Silent:               opts.Dashboard,
				PatternToExclude:     opts.PatternToExclude,
				PatternToInclude:     opts.PatternToInclude,
			},
		}
		populateDurationInDays(opts, r)
		results = append(results, r)
		wg.Add(1)
		go Stats(r, &wg, bar)
	}
	wg.Wait()
	_ = bar.Finish()

	for _, r := range results {
		if !opts.Dashboard {
			fmt.Println()
			PrintResult(r)
		}
	}

	return results
}

// newProgressBar returns the "Analyzing commits" progress bar. When silent
// (dashboard and web modes) it writes nowhere, so it does not clutter the
// terminal on background refreshes.
func newProgressBar(silent bool) *progressbar.ProgressBar {
	if silent {
		return progressbar.NewOptions(-1, progressbar.OptionSetWriter(io.Discard))
	}
	return progressbar.Default(-1, "Analyzing commits")
}

// parseDelta interprets a delta string of the form "<int>[y|m|w|d]" and returns
// `from` shifted back by that amount (years, months, weeks or days). An empty
// delta returns `from` unchanged. The sign of the amount is ignored: the shift
// always goes into the past.
func parseDelta(delta string, from time.Time) (time.Time, error) {
	if delta == "" {
		return from, nil
	}

	unit := delta[len(delta)-1]
	switch unit {
	case 'y', 'm', 'w', 'd':
		// recognised unit, value is parsed below
	default:
		return from, errors.New("invalid delta value use the format: <int>[y/m/w/d]")
	}

	value, err := strconv.Atoi(delta[:len(delta)-1])
	if err != nil {
		return from, errors.New("error delta is not a number")
	}
	if value > 0 {
		value = -value
	}

	switch unit {
	case 'y':
		return from.AddDate(value, 0, 0), nil
	case 'm':
		return from.AddDate(0, value, 0), nil
	case 'w':
		return from.AddDate(0, 0, value*7), nil
	default: // 'd'
		return from.AddDate(0, 0, value), nil
	}
}

func populateDurationInDays(options LaunchOptions, r *StatsResult) {
	end, err := parseDelta(options.Delta, time.Now())
	if err != nil {
		r.Error = err
		return
	}

	durationInDays := DefaultDurationInDays
	if options.DurationInWeeks > 0 {
		durationInDays = options.DurationInWeeks * 7
	}
	r.DurationInDays = durationInDays
	r.EndOfScan = end
	r.BeginOfScan = end.AddDate(0, 0, -durationInDays)
	if int(r.BeginOfScan.Weekday()) != 1 {
		// Not a monday
		// offset := math.Max(0, float64(6-int(r.BeginOfScan.Weekday())))
		offset := -1 * (int(r.BeginOfScan.Weekday()) - 1)
		r.BeginOfScan = getBeginningOfDay(r.BeginOfScan.AddDate(0, 0, offset))

		r.EndOfScan = getEndOfDay(r.EndOfScan.AddDate(0, 0, offset+6))
		daysBetween := r.EndOfScan.Sub(r.BeginOfScan).Hours() / 24
		r.DurationInDays = int(daysBetween)
	}
}

func daysBetween(begin time.Time, end time.Time) int {
	return int(end.Sub(begin).Hours() / 24)
}

// Stats calculates and prints the stats.
func Stats(r *StatsResult, wg *sync.WaitGroup, bar *progressbar.ProgressBar) {
	defer wg.Done()
	err := processRepositories(r, bar)

	if err != nil {
		r.Error = err
		return
	}

	r.Folder = strings.Join(r.Options.Folders, ",")
}

// getBeginningOfDay given a time.Time calculates the start time of that day
func getBeginningOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	startOfDay := time.Date(year, month, day, 0, 0, 0, 0, t.Location())
	return startOfDay
}

// getEndOfDay given a time.Time calculates the end time of that day
func getEndOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	startOfDay := time.Date(year, month, day, 23, 59, 59, 0, t.Location())
	return startOfDay
}

// countDaysSinceDate counts how many days passed since the passed `date`
func countDaysSinceDate(date time.Time, r *StatsResult) int {
	days := 0
	endDate := getEndOfDay(r.EndOfScan)
	if !date.Before(endDate) && !date.Equal(endDate) {
		return outOfRange
	} else if date.Equal(endDate) {
		return days
	}
	for date.Before(endDate) {
		date = date.Add(time.Hour * 24)
		days++
		if days > r.DurationInDays {
			return outOfRange
		}
	}
	return days
}

// fillCommits given a repository found in `path`, gets the commits and
// puts them in the `commits` map, returning it when completed
func fillCommits(r *StatsResult, emailOrUsername *string, path string, bar *progressbar.ProgressBar) error {
	// instantiate a git repo object from path
	repo, err := git.PlainOpen(path)
	if err != nil {
		// log.Fatalf("Cannot get stat from folder (not a repository): %s", path)
		return fmt.Errorf("cannot get stat from folder (not a repository): %s", path)
	}
	// Remove one day to end date to be sure parse today date
	// trueEndDateParse := endDate.AddDate(0, 0, 1)
	// get the commits history until endDate is not reached
	iterator, err := repo.Log(&git.LogOptions{Since: &r.BeginOfScan, Until: &r.EndOfScan})
	if err != nil {
		log.Fatalf("Cannot get repository history: %s", err)
		return err
	}
	// Compile the include/exclude patterns once, up front, rather than for
	// every commit stat as we iterate.
	excludeRegexps, err := compilePatterns(r.Options.PatternToExclude)
	if err != nil {
		return err
	}
	includeRegexps, err := compilePatterns(r.Options.PatternToInclude)
	if err != nil {
		return err
	}

	// iterate the commits
	offset := calcOffset(r.EndOfScan)
	err = iterator.ForEach(func(c *object.Commit) error {
		daysAgo := countDaysSinceDate(c.Author.When, r) + offset
		hour := c.Author.When.Hour()
		day := int(c.Author.When.Weekday())
		if daysAgo == outOfRange {
			return nil
		}

		if emailOrUsername != nil {
			users := strings.Split(*emailOrUsername, ",")
			var found bool
			for _, u := range users {
				if strings.Contains(u, "@") && c.Author.Email == u {
					found = true
					break
				} else if c.Author.Name == u {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		}

		// TODO find a solution for improve perf
		stats, _ := c.Stats()
		for _, stat := range stats {
			ignore := false
			for _, re := range excludeRegexps {
				if re.MatchString(stat.Name) {
					ignore = true
					break
				}
			}
			for _, re := range includeRegexps {
				if !re.MatchString(stat.Name) {
					ignore = true
					continue
				} else {
					ignore = false
					break
				}
			}

			if ignore {
				continue
			}
			authorKey := c.Author.Name + authorIDSep + c.Author.Email
			if r.AuthorsEditions[authorKey] == nil {
				r.AuthorsEditions[authorKey] = make(map[string]int, 2)
			}
			r.AuthorsEditions[authorKey]["additions"] = r.AuthorsEditions[authorKey]["additions"] + stat.Addition
			r.AuthorsEditions[authorKey]["deletions"] = r.AuthorsEditions[authorKey]["deletions"] + stat.Deletion

			lang := languageForFile(stat.Name)
			if r.LanguageEditions[lang] == nil {
				r.LanguageEditions[lang] = make(map[string]int, 2)
			}
			r.LanguageEditions[lang]["additions"] = r.LanguageEditions[lang]["additions"] + stat.Addition
			r.LanguageEditions[lang]["deletions"] = r.LanguageEditions[lang]["deletions"] + stat.Deletion
		}

		if daysAgo <= r.DurationInDays {
			r.Commits[daysAgo] = r.Commits[daysAgo] + 1
			r.HoursCommits[hour] = r.HoursCommits[hour] + 1
			r.DayCommits[day] = r.DayCommits[day] + 1
			r.CommitTypes[commitType(c.Message)]++
		}
		_ = bar.Add(1)
		return nil
	})
	if err != nil {
		log.Fatalf("Error on git-log iterate: %s", err)
		return err
	}

	return nil
}

// processRepositories given an user email, returns the
// commits made in the last 6 months
func processRepositories(r *StatsResult, bar *progressbar.ProgressBar) error {
	daysInMap := r.DurationInDays

	r.Commits = make(map[int]int, daysInMap)
	r.AuthorsEditions = make(map[string]map[string]int)
	r.LanguageEditions = make(map[string]map[string]int)
	r.CommitTypes = make(map[string]int)
	var errReturn error
	for i := daysInMap; i > 0; i-- {
		r.Commits[i] = 0
	}

	for _, path := range r.Options.Folders {
		err := fillCommits(r, r.Options.EmailOrUsername, path, bar)
		if err != nil {
			// continue for other folders
			Print(Error, fmt.Sprintf("\nError scanning folder repository %s: %s\n", path, err))
			errReturn = err
			continue
		}
	}
	return errReturn
}

// calcOffset determines and returns the amount of days missing to fill
// the last row of the stats graph
func calcOffset(endDate time.Time) int {
	var offset int
	weekday := endDate.Weekday()

	switch weekday {
	case time.Sunday:
		offset = 7
	case time.Monday:
		offset = 6
	case time.Tuesday:
		offset = 5
	case time.Wednesday:
		offset = 4
	case time.Thursday:
		offset = 3
	case time.Friday:
		offset = 2
	case time.Saturday:
		offset = 1
	}

	return offset
}
