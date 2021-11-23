package main

import (
    "errors"
    "fmt"
    "io/ioutil"
	"log"
	"os"
    "os/user"
    "path/filepath"
	"strconv"
    "strings"
    "time"
    "golang.org/x/crypto/ssh/terminal"

	"github.com/urfave/cli/v2"
    "github.com/muja/goconfig"
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
				folder := c.Args().Get(0)
				addToScan(folder)
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
                email := c.Args().Get(0)
                shift := 0
                if !strings.Contains(email, "@") {
                    shift = 1
                    _, gitEmail, err := getUserFromGitConfig()
                    if err != nil {
                        panic(err)
                    }
                    email = *gitEmail
                }
                var folder *string = nil
                var weeks *int = nil

                if c.NArg() > 1 - shift {
                    weekNumber, err := strconv.Atoi(c.Args().Get(1 - shift))
                    if err != nil {
                        // panic(fmt.Sprintf("Weeks parameter is not a number: %s", err))
                        shift = shift + 1
                    } else {
                        weeks = &weekNumber
                    }

                    if c.NArg() > 2 - shift {
                        folderToScan := c.Args().Get(2 - shift)
                        folder = &folderToScan
                    }
                }
                return launchStats(email, weeks, folder)
			},
		},
    }
}

func launchStats(email string, durationInWeeks *int, folder *string) error {
    fmt.Printf("\nScanning for %s contributions", email)
    width, _, _ := terminal.GetSize(0)
    if durationInWeeks != nil {
        fmt.Printf(" over %d weeks\n", *durationInWeeks)
        if width < (4 * *durationInWeeks) + 16 {
            return errors.New("Too much data to display in this terminal width")
        }
    } else {
        defaultDuration := (width - 16) / 4
        durationInWeeks = &defaultDuration
    }
    fmt.Println()
	Stats(email, durationInWeeks, folder)
    return nil
}

func addToScan(folder string) {
	Scan(folder)
}

func main() {
    var app = &cli.App{
        Name: "gitcontribution",
        Version: "v1.0.0",
        Compiled: time.Now(),
        Commands: commands(),
    }
	err := app.Run(os.Args)
	app.EnableBashCompletion = true
	if err != nil {
		log.Fatal(err)
	}
}
