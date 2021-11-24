package main

import (
	"fmt"
	"log"
	"sort"
    "strconv"
	"strings"
	"time"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

    "github.com/fatih/color"
)

const outOfRange = 99999

var durationInDays = 365
var durationInWeeks = 52

type column []int

type Color struct {
    Foreground color.Attribute
    Background color.Attribute
}

var (
    Today = Color{color.FgWhite, color.BgMagenta}
    ValueLow = Color{color.FgBlack, color.BgWhite}
    ValueMiddle = Color{color.FgBlack, color.BgYellow}
    ValueHigh = Color{color.FgBlack, color.BgGreen}
    Empty = Color{color.FgWhite, color.BgBlack}
    Message = Color{color.FgGreen, color.BgBlack}
	Error = Color{color.FgRed, color.Bold}
	Header = Color{color.FgMagenta, color.Bold}
)

func colorize(c Color, s string) {
    color.New(c.Foreground, c.Background).PrintfFunc()(s)
}

func isRepo(path string) bool {
	_, err := git.PlainOpen(path)
	return err == nil
}

// Stats calculates and prints the stats.
func Stats(emailOrUsername string, durationParamInWeeks *int, folder *string, delta string) {
    if folder != nil {
		colorize(Header, *folder)
		fmt.Println()
	}

    nowDate := getBeginningOfDay(time.Now())
    end := nowDate
    switch {
        case strings.Contains(delta, "y"):
            value, err := strconv.Atoi(strings.Split(delta, "y")[0])
            if err != nil {
                panic("Error delta is not a number")
            }
            if value > 0 {
                value = -value
            }
            end = nowDate.AddDate(value, 0, 0)
        case strings.Contains(delta, "m"):
            value, err := strconv.Atoi(strings.Split(delta, "m")[0])
            if err != nil {
                panic("Error delta is not a number")
            }
            if value > 0 {
                value = -value
            }
            end = nowDate.AddDate(0, value, 0)
        case strings.Contains(delta, "w"):
            value, err := strconv.Atoi(strings.Split(delta, "w")[0])
            if err != nil {
                panic("Error delta is not a number")
            }
            if value > 0 {
                value = -value
            }
            end = nowDate.AddDate(0, 0, value * 7)
        case strings.Contains(delta, "d"):
            value, err := strconv.Atoi(strings.Split(delta, "d")[0])
            if err != nil {
                panic("Error delta is not a number")
            }
            if value > 0 {
                value = -value
            }
            end = nowDate.AddDate(0, 0, value)
    }

	if durationParamInWeeks != nil && *durationParamInWeeks > 0 {
		durationInDays = *durationParamInWeeks * 7
		durationInWeeks = *durationParamInWeeks
	}
    start := end
    start = start.AddDate(0, 0, *durationParamInWeeks / 7)
    fmt.Printf("Scanning for ")
    colorize(Message, emailOrUsername)
    fmt.Printf(" contributions from ")
    colorize(Message, fmt.Sprintf("%s", start))
    fmt.Printf(" to ")
    colorize(Message, fmt.Sprintf("%s", end))
	fmt.Println()
	fmt.Println()

	commits, err := processRepositories(emailOrUsername, folder, end)
	if err != nil {
		colorize(Error, fmt.Sprintf("%s", err))
		fmt.Println()
	} else {
		printCommitsStats(commits, end)
	}
}

// getBeginningOfDay given a time.Time calculates the start time of that day
func getBeginningOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	startOfDay := time.Date(year, month, day, 0, 0, 0, 0, t.Location())
	return startOfDay
}

// countDaysSinceDate counts how many days passed since the passed `date`
func countDaysSinceDate(date time.Time, end time.Time) int {
	days := 0
    endDate := getBeginningOfDay(end)
    if !date.Before(endDate) && !date.Equal(endDate) {
        return outOfRange
    } else if date.Equal(endDate) {
        return days
    }
	for date.Before(endDate) {
		date = date.Add(time.Hour * 24)
		days++
		if days > durationInDays {
			return outOfRange
		}
	}
	return days
}

// fillCommits given a repository found in `path`, gets the commits and
// puts them in the `commits` map, returning it when completed
func fillCommits(emailOrUsername string, path string, commits map[int]int, endDate time.Time) (map[int]int, error) {
	// instantiate a git repo object from path
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	// get the HEAD reference
	ref, err := repo.Head()
	if err != nil {
		log.Fatalf("Cannot get HEAD from repository: %s", path)
		return nil, err
	}
	// get the commits history starting from HEAD
	iterator, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, err
	}
	// iterate the commits
	offset := calcOffset(endDate)
	err = iterator.ForEach(func(c *object.Commit) error {
		daysAgo := countDaysSinceDate(c.Author.When, endDate) + offset

		if strings.Contains(emailOrUsername, "@") {
			if c.Author.Email != emailOrUsername {
				return nil
			}
		} else {
			if c.Author.Name != emailOrUsername {
				return nil
			}
		}

		if daysAgo != outOfRange {
			commits[daysAgo]++
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return commits, nil
}

// processRepositories given an user email, returns the
// commits made in the last 6 months
func processRepositories(emailOrUsername string, folder *string, endDate time.Time) (map[int]int, error) {
    var repos []string
	if folder == nil {
        filePath := getDotFilePath()
	    repos = parseFileLinesToSlice(filePath)
    } else {
        repos = []string{*folder}
    }
    if len(repos) == 0 || isRepo(".") {
        repos = []string{"."}
    }
	daysInMap := durationInDays

	commits := make(map[int]int, daysInMap)
	var err error = nil
	for i := daysInMap; i > 0; i-- {
		commits[i] = 0
	}

	for _, path := range repos {
		commits, err = fillCommits(emailOrUsername, path, commits, endDate)
	}

	return commits, err
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

// printCell given a cell value prints it with a different format
// based on the value amount, and on the `today` flag.
func printCell(val int, today bool) {
	str := "  %d "
	switch {
        case val == 0:
            str = "  - "
    	case val >= 10:
	    	str = " %d "
    	case val >= 100:
	    	str = "%d "
	}

    cellContent := str
    if val > 0 {
        cellContent = fmt.Sprintf(str, val)
    }
    switch {
        case today:
            colorize(Today, cellContent)
        case val == 0:
            colorize(Empty, "  - ")
        case val > 0 && val < 5:
            colorize(ValueLow, cellContent)
        case val >= 5 && val < 10:
            colorize(ValueMiddle, cellContent)
        default:
            colorize(ValueHigh, cellContent)
    }
}

// printCommitsStats prints the commits stats
func printCommitsStats(commits map[int]int, endDate time.Time) {
	keys := sortMapIntoSlice(commits)
	cols := buildCols(keys, commits)
	printCells(cols, endDate)
}

// sortMapIntoSlice returns a slice of indexes of a map, ordered
func sortMapIntoSlice(m map[int]int) []int {
	// order map
	// To store the keys in slice in sorted order
	var keys []int
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	return keys
}

// buildCols generates a map with rows and columns ready to be printed to screen
func buildCols(keys []int, commits map[int]int) map[int]column {
	cols := make(map[int]column)
	col := column{}

	for _, k := range keys {
		week := int(k / 7) // 26,25...1
		dayinweek := k % 7 // 0,1,2,3,4,5,6

		if dayinweek == 0 { //reset
			col = column{}
		}

		col = append(col, commits[k])

		if dayinweek == 6 {
			cols[week] = col
		}
	}

	return cols
}

// printCells prints the cells of the graph
func printCells(cols map[int]column, endDate time.Time) {
	printMonths(endDate)
	for j := 6; j >= 0; j-- {
		for i := durationInWeeks + 1; i >= 0; i-- {
			if i == durationInWeeks+1 {
				printDayCol(j)
			}
			if col, ok := cols[i]; ok {
				//special case today
				if time.Now().Before(endDate) && i == 0 && j == calcOffset(time.Now())-1 {
                    printCell(col[j], true)
					continue
				} else {
					if len(col) > j {
						printCell(col[j], false)
						continue
					}
				}
			}
			printCell(0, false)
		}
		fmt.Printf("\n")
	}
}

// printMonths prints the month names in the first line, determining when the month
// changed between switching weeks
func printMonths(endDate time.Time) {
	week := getBeginningOfDay(endDate).Add(-(time.Duration(durationInDays) * time.Hour * 24))
	month := week.Month()
	fmt.Printf("         ")
	for {
		if week.Month() != month {
			fmt.Printf("%s ", week.Month().String()[:3])
			month = week.Month()
		} else {
			fmt.Printf("    ")
		}

		week = week.Add(7 * time.Hour * 24)
		if week.After(endDate) {
			break
		}
	}
	fmt.Printf("\n")
}

// printDayCol given the day number (0 is Sunday) prints the day name,
// alternating the rows (prints just 2,4,6)
func printDayCol(day int) {
	out := "     "
	switch day {
	case 1:
		out = " Mon "
	case 3:
		out = " Wed "
	case 5:
		out = " Fri "
	}

	fmt.Printf(out)
}
