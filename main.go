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

	"golang.org/x/crypto/ssh/terminal"

	"github.com/muja/goconfig"
	"github.com/urfave/cli/v2"
)

func getUserFromGitConfig() (*string, *string, error) {
	user, _ := user.Current()
	// don't forget to handle error!
	gitconfig := filepath.Join(user.HomeDir, ".gitconfig")
	bytes, _ := ioutil.ReadFile(gitconfig)

	config, _, err := goconfig.Parse(bytes)
	if err != nil {
		// Note: config is non-nil and contains successfully parsed values
		log.Fatalf("Error on line %d: %v.\n", err)
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
							addToScan(arg)
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
				List()
				return nil
			},
		},
		&cli.Command{
			Name:    "stat",
			Aliases: []string{"s"},
			Usage:   "Email: your@email.com - show constribution statistics of a user",
			Action: func(c *cli.Context) error {
				var folders []*string = make([]*string, 0)
				var weeks *int = nil
				var user *string = nil

				if c.Int("weeks") > 0 {
					weeksParam := c.Int("weeks")
					weeks = &weeksParam
				}

				if c.NArg() > 0 {
					argNum := 0
					for argNum < c.NArg() {
						arg := c.Args().Get(argNum)
						if _, err := os.Stat(arg); err == nil {
							folders = append(folders, &arg)
						} else if errors.Is(err, os.ErrNotExist) {
							user = &arg
						}
						argNum += 1
					}
				}
				if user == nil {
					_, gitEmail, err := getUserFromGitConfig()
					if err != nil {
						panic(err)
					}
					user = gitEmail
				}

				if len(folders) == 0 {
					folders = []*string{nil}
				}

				for _, folder := range folders {
					launchStats(*user, weeks, folder, c.String("delta"))
				}
				return nil
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
			},
		},
	}
}

func launchStats(email string, durationInWeeks *int, folder *string, delta string) error {
	width, _, _ := terminal.GetSize(0)
	if durationInWeeks != nil {
		if width < (4**durationInWeeks)+16 {
			return errors.New("Too much data to display in this terminal width")
		}
	} else {
		defaultDuration := (width - 16) / 4
		durationInWeeks = &defaultDuration
	}
	Stats(email, durationInWeeks, folder, delta)
	return nil
}

func addToScan(folder string) {
	Scan(folder)
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
