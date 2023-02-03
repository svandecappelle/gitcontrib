package stats

import (
	"fmt"
	"sort"
	"time"

	"github.com/fatih/color"
)

type Color struct {
	Foreground color.Attribute
	Background color.Attribute
}

type OutputType int64

const (
	Console   OutputType = 0
	Dashboard OutputType = 1
)

var (
	Today       = Color{color.FgWhite, color.BgMagenta}
	ValueLow    = Color{color.FgBlack, color.BgWhite}
	ValueMiddle = Color{color.FgBlack, color.BgYellow}
	ValueHigh   = Color{color.FgBlack, color.BgGreen}
	Empty       = Color{color.FgWhite, color.BgBlack}
	Message     = Color{color.FgGreen, color.BgBlack}
	Error       = Color{color.FgRed, color.Bold}
	Header      = Color{color.FgMagenta, color.Bold}
)

type StatsResultConsolePrinter struct {
	OutputType OutputType
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

func (p StatsResultConsolePrinter) colorize(c Color, s string) string {
	return colorize(c, s, p.OutputType)
}

func colorize(c Color, s string, oType OutputType) string {
	switch oType {
	case Dashboard:

		var cFormat string
		switch c.Background {
		case color.BgGreen:
			cFormat = "green"
		case color.BgMagenta:
			cFormat = "magenta"
		case color.BgRed:
			cFormat = "red"
		case color.BgYellow:
			cFormat = "red"
		default:
			cFormat = "black"
		}
		return fmt.Sprintf("[%s](bg:%s)", s, cFormat)

	default:
		return color.New(c.Foreground, c.Background).SprintfFunc()(s)
	}
}

func Print(c Color, s string) {
	fmt.Print(colorize(c, s, Console))
}

// printMonths prints the month names in the first line, determining when the month
// changed between switching weeks
func getMonths(r *StatsResult, limitWeeks int) string {
	week := getBeginningOfDay(r.EndOfScan).Add(-(time.Duration(r.DurationInDays) * time.Hour * 24))
	month := week.Month()
	out := "         "
	i := r.Options.DurationParamInWeeks
	for {
		if i > limitWeeks {
			i -= 1
			week = week.Add(7 * time.Hour * 24)
			continue
		}

		if week.Month() != month {
			out += fmt.Sprintf("%s ", week.Month().String()[:3])
			month = week.Month()
		} else {
			out += "    "
		}

		week = week.Add(7 * time.Hour * 24)
		if week.After(r.EndOfScan) {
			break
		}
	}
	out += "\n"
	return out
}

// printDayCol given the day number (0 is Sunday) prints the day name,
// alternating the rows (prints just 2,4,6)
func getDayCol(day int) string {
	out := "     "
	switch day {
	case 1:
		out = " Mon "
	case 3:
		out = " Wed "
	case 5:
		out = " Fri "
	}

	return out
}

// getCells build a string for the cells of the graph
func (p StatsResultConsolePrinter) getCells(keys []int, r *StatsResult, limitWeeks int) string {
	cols := buildCols(keys, r, limitWeeks)
	out := ""
	out += getMonths(r, limitWeeks)
	durationInWeeks := r.DurationInDays / 7
	for j := 6; j >= 0; j-- {
		for i := durationInWeeks + 1; i >= 0; i-- {
			if i == durationInWeeks+1 {
				out += getDayCol(j)
			}
			if limitWeeks > 0 && limitWeeks-i < 0 {
				continue
			}

			if col, ok := cols[i]; ok {
				// special case today
				if getEndOfDay(time.Now()).Equal(getEndOfDay(r.EndOfScan)) && i == 0 && j == calcOffset(getEndOfDay(time.Now())) {
					out += p.getCell(col[j], true)
					continue
				} else {
					if len(col) > j {
						out += p.getCell(col[j], false)
						continue
					}
				}
			}
			out += p.getCell(0, false)
		}
		out += "\n"
	}
	return out
}

// getCell given a cell value prints it with a different format
// based on the value amount, and on the `today` flag.
func (p StatsResultConsolePrinter) getCell(val int, today bool) string {
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
		return p.colorize(Today, cellContent)
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

// buildCols generates a map with rows and columns ready to be printed to screen
func buildCols(keys []int, r *StatsResult, limitWeeks int) map[int]column {
	cols := make(map[int]column)
	col := column{}
	for _, k := range keys {
		week := int(k / 7) // 26,25...1
		dayinweek := k % 7 // 0,1,2,3,4,5,6

		if dayinweek == 0 { //reset
			col = column{}
			wInverted := (len(keys) / 7) - (k / 7)
			if limitWeeks > 0 && limitWeeks-wInverted <= 0 {
				continue
			}
		}

		col = append(col, r.Commits[k])

		if dayinweek == 6 {
			cols[week] = col
		}
	}

	return cols
}
