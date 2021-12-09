package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/muja/goconfig"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

func getUserFromGitConfig() (*string, *string, error) {
	user, _ := user.Current()
	// don't forget to handle error!
	gitconfig := filepath.Join(user.HomeDir, ".gitconfig")
	bytes, _ := ioutil.ReadFile(gitconfig)

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
		&cli.Command{
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
		&cli.Command{
			Name:    "list-repositories",
			Aliases: []string{"lr"},
			Usage:   "List repositories to scan for statistic",
			Action: func(c *cli.Context) error {
				return List()
			},
		},
		&cli.Command{
			Name:    "stat",
			Aliases: []string{"s"},
			Usage:   "Email or Name: your@email.com / 'Firstname Name' - show constribution statistics of a user",
			Action: func(c *cli.Context) error {
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
					folders, err = GetFolders()
					if err != nil {
						return err
					}
				}
				if c.Bool("merge") {
					err = launchStats(user, weeks, folders, c.String("delta"))
				} else {
					for _, folder := range folders {
						curErr := launchStats(user, weeks, []string{folder}, c.String("delta"))
						if err == nil {
							err = curErr
						}
					}
				}
				return err
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

func launchStats(email *string, durationInWeeks *int, folders []string, delta string) error {
	width, _, _ := term.GetSize(0)
	if durationInWeeks != nil {
		if width < (4**durationInWeeks)+16 {
			return errors.New("Too much data to display in this terminal width")
		}
	} else {
		defaultDuration := (width - 16) / 4
		durationInWeeks = &defaultDuration
	}
	return Stats(email, *durationInWeeks, folders, delta)
}

func addToScan(folder string) error {
	return Scan(folder)
}

func main() {
	var app = &cli.App{
		Name:     "gitcontribution",
		Version:  "v1.0.0",
		Compiled: time.Now(),
		Commands: commands(),
	}
	err := app.Run(os.Args)
	app.EnableBashCompletion = true
	if err != nil {
		log.Fatal(err)
	}
}
