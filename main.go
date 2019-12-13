package main

import (
	"log"
	"os"
	"strconv"

	"github.com/urfave/cli"
)

func info(app *cli.App) {
	app.Name = "Simple git contribution scanner CLI"
	app.Usage = ""
	app.Author = "Steeve Vandecappelle"
	app.Version = "1.0.0"
}

func commands(app *cli.App) {
	app.Commands = []cli.Command{
		{
			Name:    "add-repository",
			Aliases: []string{"ar"},
			Usage:   "Add folder of git repository to scan for statistics",
			Action: func(c *cli.Context) {
				folder := c.Args().Get(0)
				addToScan(folder)
			},
		},
		{
			Name:    "list-repositories",
			Aliases: []string{"lr"},
			Usage:   "List repositories to scan for statistic",
			Action: func(c *cli.Context) {
				List()
			},
		},
		{
			Name:    "stat",
			Aliases: []string{"s"},
			Usage:   "Email: your@email.com - show constribution statistics of a user",
			Action: func(c *cli.Context) {
				email := c.Args().Get(0)
				if n, err := strconv.Atoi(c.Args().Get(1)); err == nil {
					launchStats(email, &n)
				} else {
					launchStats(email, nil)
				}
			},
		},
	}
}

func launchStats(email string, durationInWeeks *int) {
	Stats(email, durationInWeeks)
}

func addToScan(folder string) {
	Scan(folder)
}

func main() {
	var app = cli.NewApp()
	commands(app)
	err := app.Run(os.Args)
	app.EnableBashCompletion = true
	if err != nil {
		log.Fatal(err)
	}
}
