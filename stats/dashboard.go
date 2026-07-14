package stats

import (
	"fmt"
	"log"
	"math"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"golang.org/x/term"
)

// pieColors is the color palette used both for the contributors list and, in
// the same order, for the committers pie chart.
var pieColors = []string{"red", "green", "yellow", "blue", "magenta", "cyan", "white"}

// contributorLine renders a contributor as a colored termui markup row.
func contributorLine(c Contributor, color string) string {
	return fmt.Sprintf(
		"[%s](fg:%s): [+%d](fg:green):[-%d](fg:red)",
		c.Author,
		color,
		c.Additions,
		c.Deletions,
	)
}

func OpenDashboard(opts LaunchOptions) {
	width, height, _ := term.GetSize(0)
	rLaunch := Launch(opts)

	agg := Aggregate(rLaunch)

	if agg.TotalCommits == 0 {
		fmt.Println("\nNo commits found to parse")
		return
	}
	if agg.Errors == len(rLaunch) {
		panic("Launch has only errors")
	}

	// Pie data and contributor lines share the sorted contributor order, so
	// slice colors line up with the list colors.
	pieData := make([]float64, 0, len(agg.Contributors))
	contribLines := make([]string, 0, len(agg.Contributors))
	for i, c := range agg.Contributors {
		pieData = append(pieData, float64(c.Total))
		contribLines = append(contribLines, contributorLine(c, pieColors[i%len(pieColors)]))
	}

	repoLines := make([]string, 0, len(agg.Repositories))
	for _, r := range agg.Repositories {
		repoLines = append(repoLines, fmt.Sprintf("%s: %d", r.Folder, r.Commits))
	}

	hoursData := make([]float64, 24)
	for i, v := range agg.CommitsByHour {
		hoursData[i] = float64(v)
	}
	daysData := make([]float64, 7)
	for i, v := range agg.CommitsByWeekday {
		daysData[i] = float64(v)
	}

	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	p := widgets.NewList()
	p.Title = "Global statistics"
	p.Rows = []string{
		fmt.Sprintf("BeginDate: %s", agg.BeginOfScan),
		fmt.Sprintf("EndDate: %s", agg.EndOfScan),
		fmt.Sprintf("Commits: %d", agg.TotalCommits),
		fmt.Sprintf("Analyzed repos: %d", agg.AnalyzedRepos),
		fmt.Sprintf("User analyzed: %s", agg.User),
	}
	p.SetRect(0, 0, width/3, height/3)

	bc := widgets.NewBarChart()
	bc.Title = "Commits on weekday"
	bc.SetRect(0, height*2/3, width/4, height)
	bc.Labels = []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
	bc.BarGap = 0
	bc.Data = daysData
	bc.BarWidth = int(width / 7 / 4)

	hoursLabels := make([]string, 24)
	for i := 0; i < 24; i++ {
		hoursLabels[i] = fmt.Sprintf("%d", i)
	}
	hoursGraph := widgets.NewBarChart()
	hoursGraph.Title = "Commits on daytime"
	hoursGraph.SetRect(0, height/3, width, height*2/3)
	hoursGraph.BarWidth = int(width / 12 / 2)
	hoursGraph.Data = hoursData
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
	foldersStats.Rows = repoLines
	foldersStats.SetRect(width/4, height/3*2, width/2, height)

	heatmap := widgets.NewParagraph()
	heatmap.Title = "Heatmap"
	heatmap.SetRect(width/2, height/3*2, width, height)
	// truncate data
	defaultDurationTruncated := (width - 12) / 9
	if defaultDurationTruncated > rLaunch[0].Options.DurationParamInWeeks {
		defaultDurationTruncated = rLaunch[0].Options.DurationParamInWeeks
	}
	heatmap.Text = StatsResultConsolePrinter{Dashboard}.print(agg.merged, defaultDurationTruncated)

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
