package stats

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const outOfRange = 99999

var DefaultDurationInDays = 365

type column []int

type LaunchOptions struct {
	User            *string
	DurationInWeeks int
	Folders         []string
	Merge           bool
	Delta           string
	Dashboard       bool
}

type StatsResult struct {
	Options         StatsOptions
	BeginOfScan     time.Time
	EndOfScan       time.Time
	DurationInDays  int
	Folder          string
	Commits         map[int]int
	HoursCommits    [24]int
	DayCommits      [7]int
	AuthorsEditions map[string]map[string]int
	error           error
}

type StatsOptions struct {
	EmailOrUsername      *string
	DurationParamInWeeks int
	Folders              []string
	Delta                string
	Silent               bool
}

func isRepo(path string) bool {
	_, err := git.PlainOpen(path)
	return err == nil
}

// TODO use an interface object in order to refacto in same place the statistic run logic and then print results

func Launch(opts LaunchOptions) []*StatsResult {
	var results []*StatsResult = []*StatsResult{}
	var wg sync.WaitGroup

	if opts.Merge {
		options := StatsOptions{
			EmailOrUsername:      opts.User,
			DurationParamInWeeks: opts.DurationInWeeks,
			Folders:              opts.Folders,
			Delta:                opts.Delta,
			Silent:               opts.Dashboard,
		}

		r := &StatsResult{
			Options: options,
		}
		populateDurationInDays(opts, r)
		results = append(results, r)
		wg.Add(1)
		go Stats(r, &wg)
	} else {
		for _, folder := range opts.Folders {
			options := StatsOptions{
				EmailOrUsername:      opts.User,
				DurationParamInWeeks: opts.DurationInWeeks,
				Folders:              []string{folder},
				Delta:                opts.Delta,
				Silent:               opts.Dashboard,
			}
			r := &StatsResult{
				Options: options,
			}
			populateDurationInDays(opts, r)
			results = append(results, r)
			wg.Add(1)
			go Stats(r, &wg)
		}
	}
	wg.Wait()

	for _, r := range results {
		if !opts.Dashboard {
			PrintResult(r)
		}
	}

	return results
}

func PrintResult(r *StatsResult) {
	o := r.Options
	start := r.EndOfScan
	start = getBeginningOfDay(start.AddDate(0, 0, -o.DurationParamInWeeks*7))
	end := getEndOfDay(r.EndOfScan)

	fmt.Printf("Scanning for ")
	if o.EmailOrUsername != nil {
		Print(Message, *o.EmailOrUsername)
	} else {
		Print(Message, "all")
	}
	fmt.Printf(" contributions from ")
	Print(Message, start.Format(time.RFC1123))
	fmt.Printf(" to ")
	Print(Message, end.Format(time.RFC1123))
	fmt.Println()
	fmt.Println()
	StatsResultConsolePrinter{Console}.print(r)
}

func populateDurationInDays(options LaunchOptions, r *StatsResult) {
	nowDate := time.Now()
	end := nowDate

	delta := options.Delta
	switch {
	case strings.Contains(delta, "y"):
		value, err := strconv.Atoi(strings.Split(delta, "y")[0])
		if err != nil {
			r.error = errors.New("error delta is not a number")
			return
		}
		if value > 0 {
			value = -value
		}
		end = nowDate.AddDate(value, 0, 0)
	case strings.Contains(delta, "m"):
		value, err := strconv.Atoi(strings.Split(delta, "m")[0])
		if err != nil {
			r.error = errors.New("error delta is not a number")
			return
		}
		if value > 0 {
			value = -value
		}
		end = nowDate.AddDate(0, value, 0)
	case strings.Contains(delta, "w"):
		value, err := strconv.Atoi(strings.Split(delta, "w")[0])
		if err != nil {
			r.error = errors.New("error delta is not a number")
			return
		}
		if value > 0 {
			value = -value
		}
		end = nowDate.AddDate(0, 0, value*7)
	case strings.Contains(delta, "d"):
		value, err := strconv.Atoi(strings.Split(delta, "d")[0])
		if err != nil {
			r.error = errors.New("error delta is not a number")
			return
		}
		if value > 0 {
			value = -value
		}
		end = nowDate.AddDate(0, 0, value)
	default:
		if delta != "" {
			r.error = errors.New("invalid delta value use the format: <int>[y/m/w/d]")
			return
		}
	}
	durationInDays := DefaultDurationInDays
	if options.DurationInWeeks > 0 {
		durationInDays = options.DurationInWeeks * 7
	}
	r.DurationInDays = durationInDays
	r.EndOfScan = end
	r.BeginOfScan = end.AddDate(0, 0, -durationInDays)
}

// Stats calculates and prints the stats.
func Stats(r *StatsResult, wg *sync.WaitGroup) {
	defer wg.Done()
	o := r.Options
	if !o.Silent {
		Print(Header, strings.Join(o.Folders, ","))
		fmt.Println()
	}
	err := processRepositories(r, o.EmailOrUsername, o.Folders)

	if err != nil {
		r.error = err
		return
	}

	r.Folder = strings.Join(o.Folders, ",")
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
func fillCommits(r *StatsResult, emailOrUsername *string, path string) error {
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
	// iterate the commits
	offset := calcOffset(r.EndOfScan)
	err = iterator.ForEach(func(c *object.Commit) error {
		daysAgo := countDaysSinceDate(c.Author.When, r) + offset
		hour := c.Author.When.Hour()
		day := c.Author.When.Weekday() - 1
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
			// fmt.Printf("+%d, -%d %s", stat.Addition, stat.Deletion, stat.Name)
			if r.AuthorsEditions[c.Author.Name] == nil {
				r.AuthorsEditions[c.Author.Name] = make(map[string]int, 2)
			}
			r.AuthorsEditions[c.Author.Name]["additions"] = r.AuthorsEditions[c.Author.Name]["additions"] + stat.Addition
			r.AuthorsEditions[c.Author.Name]["deletions"] = r.AuthorsEditions[c.Author.Name]["deletions"] + stat.Deletion
		}

		if daysAgo <= r.DurationInDays {
			r.Commits[daysAgo] = r.Commits[daysAgo] + 1
			r.HoursCommits[hour] = r.HoursCommits[hour] + 1
			r.DayCommits[day] = r.DayCommits[day] + 1
		}

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
func processRepositories(r *StatsResult, emailOrUsername *string, folders []string) error {
	daysInMap := r.DurationInDays

	r.Commits = make(map[int]int, daysInMap)
	r.AuthorsEditions = make(map[string]map[string]int)
	var errReturn error
	for i := daysInMap; i > 0; i-- {
		r.Commits[i] = 0
	}

	for _, path := range folders {
		var err error
		err = fillCommits(r, emailOrUsername, path)
		if err != nil {
			// continue for other folders
			Print(Error, fmt.Sprintf("Error scanning folder repository %s: %s\n", path, err))
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
