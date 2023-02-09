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

func OpenDashboard(opts LaunchOptions) {
	folders := opts.Folders
	results := []string{}
	width, height, _ := term.GetSize(0)
	rLaunch := Launch(opts)
	nbCommits := 0
	nbAnalyzed := 0

	var hoursData []float64 = make([]float64, 24)
	var hoursLabels []string = make([]string, 24)
	var authors map[string][]float64 = make(map[string][]float64)
	var daysData []float64 = make([]float64, 7)
	colors := []string{"red", "green", "yellow", "blue", "magenta", "cyan", "white"}
	var contribs []string

	mergedValues := StatsResult{
		Options:        rLaunch[0].Options,
		BeginOfScan:    rLaunch[0].BeginOfScan,
		EndOfScan:      rLaunch[0].EndOfScan,
		DurationInDays: rLaunch[0].DurationInDays,
		Folder:         "",
		Commits:        make(map[int]int),
		Error:          nil,
	}

	var nbErrors int
	for i := 0; i < 24; i++ {
		hoursLabels[i] = fmt.Sprintf("%d", i)
	}

	for _, l := range rLaunch {
		if l.Error != nil {
			nbErrors += 1
			continue
		}
		nbAnalyzed += 1
		commitsByRepo := 0
		for i, commit := range l.Commits {
			// calculate week day
			nbCommits += commit
			commitsByRepo += commit
			mergedValues.Commits[i] += commit
		}
		if commitsByRepo > 0 {
			line := fmt.Sprintf("%s: %d", l.Folder, commitsByRepo)
			results = append(results, line)
		}
		for i, v := range l.DayCommits {
			daysData[(i+6)%7] += float64(v)
		}
		for i, v := range l.HoursCommits {
			hoursData[i] += float64(v)
		}

		for author, c := range l.AuthorsEditions {
			if authors[author] == nil {
				authors[author] = make([]float64, 2)
			}
			authors[author][0] += float64(c["additions"])
			authors[author][1] += float64(c["deletions"])
		}
	}

	if nbCommits == 0 {
		fmt.Println("\nNo commits found to parse")
		return
	}

	if nbErrors == len(rLaunch) {
		panic("Launch has only errors")
	}

	var allContributions []float64
	var sAuthors []Contributions
	for a, c := range authors {
		additionsAndDeletions := c[0] + c[1]
		allContributions = append(allContributions, additionsAndDeletions)
		sAuthors = append(sAuthors, Contributions{
			Author:    a,
			Additions: int(c[0]),
			Deletions: int(c[1]),
		})
	}
	sort.Slice(sAuthors, func(i, j int) bool {
		return sAuthors[i].Total() > sAuthors[j].Total()
	})
	for colorIdx, a := range sAuthors {
		contribs = append(
			contribs,
			a.Str(colors[colorIdx%len(colors)]),
		)
	}

	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	p := widgets.NewList()
	p.Title = "Global statistics"
	user := "all"
	if rLaunch[0].Options.EmailOrUsername != nil {
		user = *rLaunch[0].Options.EmailOrUsername
	}

	analyzed := len(folders)
	if analyzed != nbAnalyzed {
		analyzed = nbAnalyzed
	}

	p.Rows = []string{
		fmt.Sprintf("BeginDate: %s", rLaunch[0].BeginOfScan),
		fmt.Sprintf("EndDate: %s", rLaunch[0].EndOfScan),
		fmt.Sprintf("Commits: %d", nbCommits),
		fmt.Sprintf("Analyzed repos: %d", analyzed),
		fmt.Sprintf("User analyzed: %s", user),
	}
	p.SetRect(0, 0, width/3, height/3)

	bc := widgets.NewBarChart()
	bc.Title = "Commits on weekday"
	bc.SetRect(0, height*2/3, width/4, height)
	bc.Labels = []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
	bc.BarGap = 0
	bc.Data = daysData
	bc.BarWidth = int(width / 7 / 4)

	hoursGraph := widgets.NewBarChart()
	hoursGraph.Title = "Commits on daytime"
	hoursGraph.SetRect(0, height/3, width, height*2/3)
	hoursGraph.BarWidth = int(width / 12 / 2)
	hoursGraph.Data = hoursData
	hoursGraph.Labels = hoursLabels
	hoursGraph.BarGap = 0

	contributors := widgets.NewList()
	contributors.Title = "Contributors"
	contributors.Rows = contribs
	contributors.SetRect(width/3, 0, width/3*2, height/3)

	contribGraph := widgets.NewPieChart()
	contribGraph.Title = "Committers"
	contribGraph.Data = allContributions
	contribGraph.Colors = []ui.Color{ui.ColorRed, ui.ColorGreen, ui.ColorYellow, ui.ColorBlue, ui.ColorMagenta, ui.ColorCyan, ui.ColorWhite}
	contribGraph.AngleOffset = -.5 * math.Pi
	contribGraph.LabelFormatter = func(i int, v float64) string {
		return fmt.Sprintf("%d", int(v))
	}
	contribGraph.SetRect(width/3*2, 0, width, height/3)

	foldersStats := widgets.NewList()
	foldersStats.Title = "Repositories"
	foldersStats.Rows = results
	foldersStats.SetRect(width/4, height/3*2, width/2, height)

	heatmap := widgets.NewParagraph()
	heatmap.Title = "Heatmap"
	heatmap.SetRect(width/2, height/3*2, width, height)
	// truncate data
	defaultDurationTruncated := (width - 12) / 9
	if defaultDurationTruncated > rLaunch[0].Options.DurationParamInWeeks {
		defaultDurationTruncated = rLaunch[0].Options.DurationParamInWeeks
	}
	heatmap.Text = StatsResultConsolePrinter{Dashboard}.print(&mergedValues, defaultDurationTruncated)

	ui.Render(p, bc, hoursGraph, contribGraph, contributors, foldersStats, heatmap)

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
