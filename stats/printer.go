package stats

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type OutputType int64

const (
	Console   OutputType = 0
	Dashboard OutputType = 1
)

type StatsResultConsolePrinter struct {
	OutputType OutputType
}

func PrintResult(r *StatsResult) {
	o := r.Options
	start := getBeginningOfDay(r.BeginOfScan)
	end := getEndOfDay(r.EndOfScan)

	if !o.Silent {
		Print(Header, strings.Join(o.Folders, ","))
		fmt.Println()
	}

	fmt.Printf("Scanning for ")
	if o.EmailOrUsername != nil {
		Print(Message, *o.EmailOrUsername)
	} else {
		Print(Message, "all")
	}
	fmt.Printf(" contributions from ")
	Print(Message, start.Format("January 02, 2006 15:04:05"))
	fmt.Printf(" to ")
	Print(Message, end.Format("January 02, 2006 15:04:05"))
	fmt.Println()
	fmt.Println()
	StatsResultConsolePrinter{Console}.print(r, -1)
}

func (p StatsResultConsolePrinter) print(r *StatsResult, limitWeeks int) string {
	switch p.OutputType {
	case Dashboard:
		// nothing
		return p.GetCommitsTable(r, limitWeeks)
	default:
		fmt.Print(p.GetCommitsTable(r, limitWeeks))
	}
	return ""
}

func (p StatsResultConsolePrinter) GetCommitsTable(s *StatsResult, limitWeeks int) string {
	keys := sortMapIntoSlice(s)
	return p.getCells(keys, s, limitWeeks)
}

func (p StatsResultConsolePrinter) colorize(c TermStyle, s string) string {
	return colorize(c, s, p.OutputType)
}

func Print(c TermStyle, s string) {
	fmt.Print(colorize(c, s, Console))
}

// printMonths prints the month names in the first line, determining when the month
// changed between switching weeks
func getMonths(r *StatsResult, limitWeeks int) string {
	week := getBeginningOfDay(r.EndOfScan).Add(-(time.Duration(r.DurationInDays) * time.Hour * 24))
	month := week.Month()
	out := "    "
	i := r.DurationInDays
	for week.Before(r.EndOfScan) {
		if limitWeeks > 0 && i > limitWeeks*7 {
			i -= 7
			week = week.Add(7 * time.Hour * 24)
			continue
		}

		if week.Month() != month {
			out += fmt.Sprintf(" %s", week.Month().String()[:3])
			month = week.Month()
		} else {
			out += "    "
		}
		week = week.Add(7 * time.Hour * 24)
	}
	out += "\n"
	return out
}

// printDayCol given the day number (0 is Sunday) prints the day name,
// alternating the rows (prints just 2,4,6)
func getDayCol(day int) string {
	days := []string{"Su ", "Mo ", "Tu ", "We ", "Th ", "Fr ", "Sa "}
	return days[day]
}

// getCells build a string for the cells of the graph
func (p StatsResultConsolePrinter) getCells(keys []int, r *StatsResult, limitWeeks int) string {
	out := ""
	out += getMonths(r, limitWeeks)
	durationInWeeks := r.DurationInDays / 7

	begin := r.BeginOfScan // .AddDate(0, 0, int(-offset))
	end := getEndOfDay(r.EndOfScan)

	current := begin
	for i := 0; i < 7; i += 1 {
		// Let loop on data with starting column and adds 7 to each cell to print
		// Then start with weekDay gap
		current = begin.AddDate(0, 0, i)
		weekNum := durationInWeeks

		for current.Before(end) {
			if limitWeeks > 0 && weekNum > limitWeeks {
				weekNum -= 1
				current = current.AddDate(0, 0, 7)
				continue
			}
			if daysBetween(r.BeginOfScan, current) < 7 {
				// first week print weekday
				out += getDayCol(int(current.Weekday()))
			}
			day := end.Sub(current).Hours() / 24
			out += p.getCell(r.Commits[int(day+8)], current)

			if daysBetween(current, r.EndOfScan) < 7 {
				// last week return to begin
				out += "\n"
			}

			weekNum -= 1
			current = current.AddDate(0, 0, 7)
		}
	}
	return out
}

// getCell given a cell value prints it with a different format
// based on the value amount, and on the `today` flag.
func (p StatsResultConsolePrinter) getCell(val int, date time.Time) string {
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
	case getBeginningOfDay(date).Equal(getBeginningOfDay(time.Now())):
		// today
		return p.colorize(Today, cellContent)
	case date.Day() == 1:
		// first of month
		return p.colorize(FirstOfMonth, cellContent)
	case val == 0:
		return p.colorize(Empty, "  - ")
	case val > 0 && val < 5:
		return p.colorize(ValueLow, cellContent)
	case val >= 5 && val < 10:
		return p.colorize(ValueMiddle, cellContent)
	default:
		return p.colorize(ValueHigh, cellContent)
	}
}

// sortMapIntoSlice returns a slice of indexes of a map, ordered
func sortMapIntoSlice(r *StatsResult) []int {
	// order map
	// To store the keys in slice in sorted order
	var keys []int
	for k := range r.Commits {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	return keys
}
