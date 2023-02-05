package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/muja/goconfig"
	"github.com/svandecappelle/gitcontrib/stats"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

func getUserFromGitConfig() (*string, *string, error) {
	user, _ := user.Current()
	// don't forget to handle error!
	gitconfig := filepath.Join(user.HomeDir, ".gitconfig")
	bytes, _ := os.ReadFile(gitconfig)

	config, _, err := goconfig.Parse(bytes)
	if err != nil {
		// Note: config is non-nil and contains successfully parsed values
		log.Fatalf("Error on git config file: %s\n", err)
		return nil, nil, err
	}
	username := config["user.name"]
	usermail := config["user.email"]
	return &username, &usermail, nil
}

func commands() []*cli.Command {
	return []*cli.Command{
		{
			Name:    "add-repository",
			Aliases: []string{"ar"},
			Usage:   "Add folder of git repository to scan for statistics",
			Action: func(c *cli.Context) error {
				if c.NArg() > 0 {
					argNum := 0
					for argNum < c.NArg() {
						arg := c.Args().Get(argNum)
						if _, err := os.Stat(arg); err == nil {
							err := addToScan(arg)
							if err != nil {
								return err
							}
						} else if errors.Is(err, os.ErrNotExist) {
							fmt.Printf("Repository %s does not exists\n", arg)
						}
						argNum += 1
					}
				}

				return nil
			},
		},
		{
			Name:    "list-repositories",
			Aliases: []string{"lr"},
			Usage:   "List repositories to scan for statistic",
			Action: func(c *cli.Context) error {
				return stats.List()
			},
		},
		{
			Name:    "dashboard",
			Aliases: []string{},
			Usage:   "Open a dashboard for print statistics",
			Action: func(c *cli.Context) error {
				return argParse(c, true)
			},
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "delta",
					Value: "",
					Usage: "Delta of starting watch commits",
				},
				&cli.IntFlag{
					Name:  "weeks",
					Value: -1,
					Usage: "Number of weeks to compute",
				},
				&cli.BoolFlag{
					Name:  "merge",
					Value: false,
					Usage: "Merge all scanned repository",
				},
				&cli.BoolFlag{
					Name:  "count-all",
					Value: false,
					Usage: "Force count all users contributions",
				},
			},
		},
		{
			Name:    "stat",
			Aliases: []string{"s"},
			Usage:   "Email or Name: your@email.com / 'Firstname Name' - show constribution statistics of a user",
			Action: func(c *cli.Context) error {
				return argParse(c, false)
			},
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "delta",
					Value: "",
					Usage: "Delta of starting watch commits",
				},
				&cli.IntFlag{
					Name:  "weeks",
					Value: -1,
					Usage: "Number of weeks to compute",
				},
				&cli.BoolFlag{
					Name:  "merge",
					Value: false,
					Usage: "Merge all scanned repository",
				},
				&cli.BoolFlag{
					Name:  "count-all",
					Value: false,
					Usage: "Force count all users contributions",
				},
			},
		},
	}
}

func argParse(c *cli.Context, useDashboard bool) error {
	var folders []string
	var weeks *int = nil
	var user *string = nil
	var err error

	if c.Int("weeks") > 0 {
		weeksParam := c.Int("weeks")
		weeks = &weeksParam
	}

	if c.NArg() > 0 {
		argNum := 0
		for argNum < c.NArg() {
			arg := c.Args().Get(argNum)
			if _, err := os.Stat(arg); err == nil {
				folders = append(folders, arg)
			} else if errors.Is(err, os.ErrNotExist) {
				user = &arg
			}
			argNum += 1
		}
	}
	if user == nil && !c.Bool("count-all") {
		_, gitEmail, err := getUserFromGitConfig()
		if err != nil {
			panic(err)
		}
		user = gitEmail
	}

	if len(folders) == 0 {
		folders, err = stats.GetFolders()
		if err != nil {
			return err
		}
	}

	durationInWeeks := 0
	width, _, _ := term.GetSize(0)

	durationInWeeks = 52
	if weeks != nil {
		if width < (4**weeks)+16 && !useDashboard {
			return errors.New("too much data to display in this terminal width")
		}
		durationInWeeks = *weeks
	} else {
		defaultDuration := (width - 16) / 4
		if !useDashboard {
			durationInWeeks = defaultDuration
		}
	}

	if useDashboard {
		stats.OpenDashboard(stats.LaunchOptions{
			User:            user,
			DurationInWeeks: durationInWeeks,
			Folders:         folders,
			Merge:           false,
			Delta:           c.String("delta"),
			Dashboard:       true,
		})
	} else {
		stats.Launch(stats.LaunchOptions{
			User:            user,
			DurationInWeeks: durationInWeeks,
			Folders:         folders,
			Merge:           c.Bool("merge"),
			Delta:           c.String("delta"),
			Dashboard:       false,
		})
	}

	return err
}

func addToScan(folder string) error {
	return stats.Scan(folder)
}

func main() {
	var app = &cli.App{
		Name:     "gitcontribution",
		Version:  "v1.2.0",
		Compiled: time.Now(),
		Commands: commands(),
		Action: func(c *cli.Context) error {
			return argParse(c, false)
		},
	}
	err := app.Run(os.Args)
	app.EnableBashCompletion = true
	if err != nil {
		log.Fatal(err)
	}
}
