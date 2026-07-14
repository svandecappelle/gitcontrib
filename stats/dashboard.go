package stats

import (
	"fmt"
	"log"
	"math"
	"sort"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"golang.org/x/term"
)

// pieColors is the color palette used both for the contributors list and, in
// the same order, for the committers pie chart.
var pieColors = []string{"red", "green", "yellow", "blue", "magenta", "cyan", "white"}

type Contributions struct {
	Author    string
	Additions int
	Deletions int
}

func (c Contributions) Total() int {
	return c.Additions + c.Deletions
}
func (c Contributions) Str(color string) string {
	return fmt.Sprintf(
		"[%s](fg:%s): [+%d](fg:green):[-%d](fg:red)",
		c.Author,
		color,
		c.Additions,
		c.Deletions,
	)
}

// dashboardData holds the values aggregated across every scanned repository,
// ready to be fed to the dashboard widgets.
type dashboardData struct {
	nbCommits  int
	nbAnalyzed int
	nbErrors   int
	hoursData  []float64
	daysData   []float64
	authors    map[string][]float64 // author -> [additions, deletions]
	repoLines  []string             // "folder: commitCount" for repos with commits
	merged     StatsResult          // all repositories merged into a single result
}

// aggregateResults sums the per-repository results into a single dashboardData.
func aggregateResults(results []*StatsResult) dashboardData {
	d := dashboardData{
		hoursData: make([]float64, 24),
		daysData:  make([]float64, 7),
		authors:   make(map[string][]float64),
		merged: StatsResult{
			Options:        results[0].Options,
			BeginOfScan:    results[0].BeginOfScan,
			EndOfScan:      results[0].EndOfScan,
			DurationInDays: results[0].DurationInDays,
			Commits:        make(map[int]int),
		},
	}

	for _, l := range results {
		if l.Error != nil {
			d.nbErrors++
			continue
		}
		d.nbAnalyzed++

		commitsByRepo := 0
		for i, commit := range l.Commits {
			d.nbCommits += commit
			commitsByRepo += commit
			d.merged.Commits[i] += commit
		}
		if commitsByRepo > 0 {
			d.repoLines = append(d.repoLines, fmt.Sprintf("%s: %d", l.Folder, commitsByRepo))
		}

		for i, v := range l.DayCommits {
			d.daysData[(i+6)%7] += float64(v)
		}
		for i, v := range l.HoursCommits {
			d.hoursData[i] += float64(v)
		}
		for author, c := range l.AuthorsEditions {
			if d.authors[author] == nil {
				d.authors[author] = make([]float64, 2)
			}
			d.authors[author][0] += float64(c["additions"])
			d.authors[author][1] += float64(c["deletions"])
		}
	}
	return d
}

// buildContributions returns the pie-chart data and the colored contributor
// lines for the list widget. Contributor lines are sorted by total
// contribution (descending); the pie data keeps the map iteration order.
func buildContributions(authors map[string][]float64) (pieData []float64, lines []string) {
	var sorted []Contributions
	for author, c := range authors {
		pieData = append(pieData, c[0]+c[1])
		sorted = append(sorted, Contributions{
			Author:    author,
			Additions: int(c[0]),
			Deletions: int(c[1]),
		})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Total() > sorted[j].Total()
	})
	for colorIdx, a := range sorted {
		lines = append(lines, a.Str(pieColors[colorIdx%len(pieColors)]))
	}
	return pieData, lines
}

func OpenDashboard(opts LaunchOptions) {
	width, height, _ := term.GetSize(0)
	rLaunch := Launch(opts)

	data := aggregateResults(rLaunch)

	if data.nbCommits == 0 {
		fmt.Println("\nNo commits found to parse")
		return
	}
	if data.nbErrors == len(rLaunch) {
		panic("Launch has only errors")
	}

	pieData, contribLines := buildContributions(data.authors)

	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	user := "all"
	if rLaunch[0].Options.EmailOrUsername != nil {
		user = *rLaunch[0].Options.EmailOrUsername
	}

	p := widgets.NewList()
	p.Title = "Global statistics"
	p.Rows = []string{
		fmt.Sprintf("BeginDate: %s", rLaunch[0].BeginOfScan),
		fmt.Sprintf("EndDate: %s", rLaunch[0].EndOfScan),
		fmt.Sprintf("Commits: %d", data.nbCommits),
		fmt.Sprintf("Analyzed repos: %d", data.nbAnalyzed),
		fmt.Sprintf("User analyzed: %s", user),
	}
	p.SetRect(0, 0, width/3, height/3)

	bc := widgets.NewBarChart()
	bc.Title = "Commits on weekday"
	bc.SetRect(0, height*2/3, width/4, height)
	bc.Labels = []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
	bc.BarGap = 0
	bc.Data = data.daysData
	bc.BarWidth = int(width / 7 / 4)

	hoursLabels := make([]string, 24)
	for i := 0; i < 24; i++ {
		hoursLabels[i] = fmt.Sprintf("%d", i)
	}
	hoursGraph := widgets.NewBarChart()
	hoursGraph.Title = "Commits on daytime"
	hoursGraph.SetRect(0, height/3, width, height*2/3)
	hoursGraph.BarWidth = int(width / 12 / 2)
	hoursGraph.Data = data.hoursData
	hoursGraph.Labels = hoursLabels
	hoursGraph.BarGap = 0

	contributors := widgets.NewList()
	contributors.Title = "Contributors"
	contributors.Rows = contribLines
	contributors.SetRect(width/3, 0, width/3*2, height/3)

	contribGraph := widgets.NewPieChart()
	contribGraph.Title = "Committers"
	contribGraph.Data = pieData
	contribGraph.Colors = []ui.Color{ui.ColorRed, ui.ColorGreen, ui.ColorYellow, ui.ColorBlue, ui.ColorMagenta, ui.ColorCyan, ui.ColorWhite}
	contribGraph.AngleOffset = -.5 * math.Pi
	contribGraph.LabelFormatter = func(i int, v float64) string {
		return fmt.Sprintf("%d", int(v))
	}
	contribGraph.SetRect(width/3*2, 0, width, height/3)

	foldersStats := widgets.NewList()
	foldersStats.Title = "Repositories"
	foldersStats.Rows = data.repoLines
	foldersStats.SetRect(width/4, height/3*2, width/2, height)

	heatmap := widgets.NewParagraph()
	heatmap.Title = "Heatmap"
	heatmap.SetRect(width/2, height/3*2, width, height)
	// truncate data
	defaultDurationTruncated := (width - 12) / 9
	if defaultDurationTruncated > rLaunch[0].Options.DurationParamInWeeks {
		defaultDurationTruncated = rLaunch[0].Options.DurationParamInWeeks
	}
	heatmap.Text = StatsResultConsolePrinter{Dashboard}.print(&data.merged, defaultDurationTruncated)

	ui.Render(p, bc, hoursGraph, contribGraph, contributors, foldersStats, heatmap)

	runDashboardEventLoop(contributors, foldersStats)
}

// runDashboardEventLoop handles keyboard navigation until the user quits.
// It lets the user scroll the currently selected list (contributors or
// repositories) and switch between them with "n".
func runDashboardEventLoop(contributors, foldersStats *widgets.List) {
	uiEvents := ui.PollEvents()
	selectedList := contributors
	selectedList.BorderStyle.Fg = ui.ColorYellow
	for e := range uiEvents {
		switch e.ID {
		case "q", "<C-c>":
			return
		case "k", "<Up>":
			selectedList.ScrollUp()
		case "j", "<Down>":
			selectedList.ScrollDown()
		case "n":
			selectedList.BorderStyle.Fg = ui.ColorWhite
			if selectedList == contributors {
				selectedList = foldersStats
			} else {
				selectedList = contributors
			}
			selectedList.BorderStyle.Fg = ui.ColorYellow
		}

		ui.Render(contributors, foldersStats)
	}
}
