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
	usr, err := user.Current()
	if err != nil {
		return nil, nil, err
	}
	gitconfig := filepath.Join(usr.HomeDir, ".gitconfig")
	bytes, _ := os.ReadFile(gitconfig)

	config, _, err := goconfig.Parse(bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("error on git config file: %w", err)
	}
	username := config["user.name"]
	usermail := config["user.email"]
	return &username, &usermail, nil
}

// statFlags returns the flags shared by the "stat" and "dashboard" commands.
func statFlags() []cli.Flag {
	return []cli.Flag{
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
	}
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
				return runDashboard(c)
			},
			Flags: append(statFlags(), patternFlags()...),
		},
		{
			Name:    "web",
			Aliases: []string{"w"},
			Usage:   "Start a web server exposing the statistics through an API and a UI",
			Action: func(c *cli.Context) error {
				return runWeb(c)
			},
			Flags: append(append(statFlags(), patternFlags()...),
				&cli.StringFlag{
					Name:  "addr",
					Value: ":8080",
					Usage: "Address the web server listens on (host:port)",
				},
				&cli.StringFlag{
					Name:  "ttl",
					Value: "5m",
					Usage: "Cache time-to-live before a background refresh (e.g. 30s, 5m, 1h; 0 disables auto-refresh)",
				},
				&cli.StringFlag{
					Name:  "cache-file",
					Value: "",
					Usage: "Path to the JSON cache file (default: <home>/.gitcontrib-cache.json)",
				},
			),
		},
		{
			Name:    "stat",
			Aliases: []string{"s"},
			Usage:   "Email or Name: your@email.com / 'Firstname Name' - show constribution statistics of a user",
			Action: func(c *cli.Context) error {
				return runStat(c)
			},
			Flags: statFlags(),
		},
	}
}

// patternFlags returns the include/exclude file-pattern flags shared by the
// commands that filter contributions.
func patternFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "file-exclude-pattern",
			Usage: "File pattern to exclude of contributions statistics",
		},
		&cli.StringSliceFlag{
			Name:  "file-include-pattern",
			Usage: "File pattern to include of contributions statistics",
		},
	}
}

// buildLaunchOptions resolves the folders, user and scan duration shared by the
// stat, dashboard and web commands. When console is true the scan window is
// derived from (and validated against) the terminal width; otherwise it
// defaults to one year, or the --weeks flag when provided.
func buildLaunchOptions(c *cli.Context, console bool) (stats.LaunchOptions, error) {
	var folders []string
	var user *string

	for i := 0; i < c.NArg(); i++ {
		arg := c.Args().Get(i)
		if _, err := os.Stat(arg); err == nil {
			folders = append(folders, arg)
		} else if errors.Is(err, os.ErrNotExist) {
			value := arg
			user = &value
		}
	}

	if user == nil && !c.Bool("count-all") {
		_, gitEmail, err := getUserFromGitConfig()
		if err != nil {
			return stats.LaunchOptions{}, err
		}
		user = gitEmail
	}

	if len(folders) == 0 {
		found, err := stats.GetFolders()
		if err != nil {
			return stats.LaunchOptions{}, err
		}
		folders = found
	}

	width, _, _ := term.GetSize(0)
	durationInWeeks := 52
	if weeks := c.Int("weeks"); weeks > 0 {
		if console && width < (4*weeks)+16 {
			return stats.LaunchOptions{}, errors.New("too much data to display in this terminal width")
		}
		durationInWeeks = weeks
	} else if console {
		durationInWeeks = (width - 16) / 4
	}

	return stats.LaunchOptions{
		User:             user,
		DurationInWeeks:  durationInWeeks,
		Folders:          folders,
		Delta:            c.String("delta"),
		PatternToExclude: c.StringSlice("file-exclude-pattern"),
		PatternToInclude: c.StringSlice("file-include-pattern"),
	}, nil
}

func runStat(c *cli.Context) error {
	opts, err := buildLaunchOptions(c, true)
	if err != nil {
		return err
	}
	opts.Merge = c.Bool("merge")
	stats.Launch(opts)
	return nil
}

func runDashboard(c *cli.Context) error {
	opts, err := buildLaunchOptions(c, false)
	if err != nil {
		return err
	}
	opts.Dashboard = true
	stats.OpenDashboard(opts)
	return nil
}

func runWeb(c *cli.Context) error {
	opts, err := buildLaunchOptions(c, false)
	if err != nil {
		return err
	}
	opts.Merge = c.Bool("merge")

	ttl, err := time.ParseDuration(c.String("ttl"))
	if err != nil {
		return fmt.Errorf("invalid --ttl value: %w", err)
	}

	cacheFile := c.String("cache-file")
	if cacheFile == "" {
		cacheFile = defaultCacheFile()
	}

	return stats.Serve(opts, c.String("addr"), ttl, cacheFile)
}

// defaultCacheFile returns the default web cache path, in the user's home
// directory, falling back to the current directory if the home is unknown.
func defaultCacheFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "gitcontrib-cache.json"
	}
	return filepath.Join(home, ".gitcontrib-cache.json")
}

func addToScan(folder string) error {
	return stats.Scan(folder)
}

func main() {
	var app = &cli.App{
		Name:                 "gitcontribution",
		Version:              "v1.5.0",
		Compiled:             time.Now(),
		Commands:             commands(),
		EnableBashCompletion: true,
		Action: func(c *cli.Context) error {
			return runStat(c)
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
