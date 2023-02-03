package stats

import (
	"fmt"
	"log"
	"math"
	"strconv"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"golang.org/x/term"
)

func OpenDashboard(opts LaunchOptions) {
	folders, _ := GetFolders()
	results := []string{}
	width, height, _ := term.GetSize(0)

	rLaunch := Launch(opts)
	output := ""
	nbCommits := 0
	for _, r := range rLaunch {
		line := fmt.Sprintf("%s: %d", r.Folder, len(r.Commits))
		for _, commit := range r.Commits {
			// calculate week day
			nbCommits += commit
		}
		output += line
		results = append(results, line)
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
	p.Rows = []string{
		fmt.Sprintf("BeginDate: %s", rLaunch[0].BeginOfScan),
		fmt.Sprintf("EndDate: %s", rLaunch[0].EndOfScan),
		fmt.Sprintf("Commits: %d", nbCommits),
		fmt.Sprintf("Analyzed repos: %d", len(folders)),
		fmt.Sprintf("User analyzed: %s", user),
	}
	p.SetRect(0, 0, width/3, height/3)

	ui.Render(p)

	var daysData []float64
	for _, v := range rLaunch[0].DayCommits {
		daysData = append(daysData, float64(v))
	}

	bc := widgets.NewBarChart()
	bc.Title = "Commits on weekday"
	bc.SetRect(0, height*2/3, width/4, height)
	bc.Labels = []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
	bc.BarGap = 0
	bc.Data = daysData
	bc.BarWidth = int(width / 7 / 4)
	ui.Render(bc)

	var hoursData []float64
	var hoursLabels []string
	for i, v := range rLaunch[0].HoursCommits {
		hoursData = append(hoursData, float64(v))
		hoursLabels = append(hoursLabels, strconv.Itoa(i+1))
	}

	hoursGraph := widgets.NewBarChart()
	hoursGraph.Title = "Commits on daytime"
	hoursGraph.SetRect(0, height/3, width, height*2/3)
	hoursGraph.BarWidth = int(width / 12 / 2)
	hoursGraph.Data = hoursData
	hoursGraph.Labels = hoursLabels
	hoursGraph.BarGap = 0
	ui.Render(hoursGraph)

	contribs := []string{}
	contributorsMap := make(map[int]string)
	allContributions := make([]float64, len(rLaunch[0].AuthorsEditions))
	i := 0
	colors := []string{"red", "green", "yellow", "blue", "magenta", "cyan", "white"}
	for author, c := range rLaunch[0].AuthorsEditions {
		contribs = append(
			contribs,
			fmt.Sprintf(
				"[%s](fg:%s): [+%d](fg:green):[-%d](fg:red)",
				author,
				colors[i%len(colors)],
				c["additions"],
				c["deletions"],
			),
		)
		allContributions[i] = float64(c["additions"] + c["deletions"])
		contributorsMap[i] = author
		i += 1
	}
	contributors := widgets.NewList()
	contributors.Title = "Contributors"
	contributors.Rows = contribs
	contributors.SetRect(width/3, 0, width/3*2, height/3)
	ui.Render(contributors)

	contribGraph := widgets.NewPieChart()
	contribGraph.Title = "Committers"
	contribGraph.Data = allContributions
	contribGraph.AngleOffset = -.5 * math.Pi
	contribGraph.LabelFormatter = func(i int, v float64) string {
		return fmt.Sprintf("%d", int(v))
	}
	contribGraph.SetRect(width/3*2, 0, width, height/3)
	ui.Render(contribGraph)

	foldersStats := widgets.NewList()
	foldersStats.Title = "Repositories"
	foldersStats.Rows = results
	foldersStats.SetRect(width/4, height/3*2, width/2, height)
	ui.Render(foldersStats)

	heatmap := widgets.NewParagraph()
	heatmap.Title = "Heatmap"
	heatmap.SetRect(width/2, height/3*2, width, height)
	// truncate data
	defaultDurationTruncated := (width - 12) / 9
	if defaultDurationTruncated > rLaunch[0].Options.DurationParamInWeeks {
		defaultDurationTruncated = rLaunch[0].Options.DurationParamInWeeks
	}
	heatmap.Text = StatsResultConsolePrinter{Dashboard}.print(rLaunch[0], defaultDurationTruncated)
	ui.Render(heatmap)

	uiEvents := ui.PollEvents()
	for e := range uiEvents {
		switch e.ID {
		case "q", "<C-c>":
			return
		}
	}
}
