package stats

import (
	"fmt"
	"log"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"golang.org/x/term"
)

func OpenDashboard(opts LaunchOptions) {
	// folders, _ := GetFolders()
	results := []string{}
	width, _, _ := term.GetSize(0)

	barchartData := []float64{3, 2, 5, 3, 9, 5, 3}
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

	p := widgets.NewParagraph()
	p.Title = "Global statistics"
	p.Text = fmt.Sprintf("BeginDate: %s\nEndDate: %s\nCommits: %d\n", rLaunch[0].BeginOfScan, rLaunch[0].EndOfScan, nbCommits)
	p.SetRect(0, 0, width/3, 15)

	ui.Render(p)

	bc := widgets.NewBarChart()
	bc.Title = "Commits on weekday"
	bc.SetRect(width/3, 0, width, 35)
	bc.Labels = []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
	bc.Data = barchartData
	bc.BarWidth = int(width / 3 * 2 / 7)
	// bc.BarColors[0] = ui.ColorGreen
	// bc.NumStyles[0] = ui.NewStyle(ui.ColorBlack)
	ui.Render(bc)

	for e := range ui.PollEvents() {
		if e.Type == ui.KeyboardEvent {
			break
		}
	}
}
