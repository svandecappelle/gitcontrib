package stats

import (
	"fmt"
	"log"
	"strconv"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"golang.org/x/term"
)

func OpenDashboard(opts LaunchOptions) {
	folders, _ := GetFolders()
	results := []string{}
	width, _, _ := term.GetSize(0)

	rLaunch := Launch(opts)
	output := ""
	nbCommits := 0
	for _, r := range rLaunch {
		line := fmt.Sprintf("%s:\n%d", r.Folder, len(r.Commits))
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
	p.SetRect(0, 0, width/3, 15)

	ui.Render(p)

	var daysData []float64
	for _, v := range rLaunch[0].DayCommits {
		daysData = append(daysData, float64(v))
	}

	bc := widgets.NewBarChart()
	bc.Title = "Commits on weekday"
	bc.SetRect(0, 15, width/3, 35)
	bc.Labels = []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
	bc.Data = daysData
	bc.BarWidth = int(width / 8 / 3)
	// bc.BarColors[0] = ui.ColorGreen
	// bc.NumStyles[0] = ui.NewStyle(ui.ColorBlack)
	ui.Render(bc)

	var hoursData []float64
	var hoursLabels []string
	for i, v := range rLaunch[0].HoursCommits {
		hoursData = append(hoursData, float64(v))
		hoursLabels = append(hoursLabels, strconv.Itoa(i+1))
	}
	// hours.Labels = []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}

	hoursGraph := widgets.NewBarChart()
	hoursGraph.Title = "Commits on daytime"
	hoursGraph.SetRect(width/3, 0, width, 15)
	hoursGraph.Data = hoursData
	hoursGraph.Labels = hoursLabels
	ui.Render(hoursGraph)

	contribs := []string{}
	for author, c := range rLaunch[0].AuthorsEditions {
		contribs = append(contribs, fmt.Sprintf("%s: +%d:-%d", author, c["additions"], c["deletions"]))
	}
	contributors := widgets.NewList()
	contributors.Title = "Contributors"
	contributors.Rows = contribs
	contributors.SetRect(0, 45, width/3, 35)

	ui.Render(contributors)

	for e := range ui.PollEvents() {
		if e.Type == ui.KeyboardEvent {
			break
		}
	}
}
