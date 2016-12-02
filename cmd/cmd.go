package cmd

import (
	"fmt"
	"github.com/op/go-logging"
	"syscall"

	"bufio"
	"context"
	"net/http"
	_ "net/http/pprof"

	"os"
	"os/signal"
	"strings"

	"github.com/bogdanovich/dns_resolver"
	"github.com/fatih/color"
	"github.com/minio/cli"

	"github.com/dutchcoders/anam/config"
	"github.com/dutchcoders/anam/scanner"
)

var format = logging.MustStringFormatter(
	"%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}",
)

var log = logging.MustGetLogger("anam/client")

var globalFlags = []cli.Flag{
	cli.IntFlag{
		Name:  "port, p",
		Usage: "port to scan",
		Value: 80,
	},
	cli.IntFlag{
		Name:  "threads",
		Usage: "amount of similar threads",
		Value: 50,
	},
	cli.IntFlag{
		Name:  "timeout, t",
		Usage: "amount of seconds to wait for connection",
		Value: 5,
	},
	cli.StringFlag{
		Name:  "interface, i",
		Usage: "network interface to use",
		Value: "eth0",
	},
	cli.StringFlag{
		Name:  "prefixes",
		Usage: "",
		Value: "www",
	},
	// do we want to save the output of the hits?

	// where shoud we look at (eg. starts with?)
	cli.StringFlag{
		Name:  "resolvers",
		Usage: "",
		Value: "",
	},
	cli.StringFlag{
		Name:  "user-agent",
		Usage: "",
		Value: "anam mass scanner",
	},
	cli.BoolFlag{
		Name:  "profiler",
		Usage: "enable profiler",
	},
	cli.BoolFlag{
		Name:  "tls",
		Usage: "enable tls",
	},
	cli.BoolFlag{
		Name:  "help, h",
		Usage: "show help.",
	},
}

var (
	Version = "1.0"
)

var helpTemplate = `NAME:
{{.Name}} - {{.Usage}}

DESCRIPTION:
{{.Description}}

USAGE:
{{.Name}} {{if .Flags}}[flags] {{end}}command{{if .Flags}}{{end}} [arguments...]

COMMANDS:
{{range .Commands}}{{join .Names ", "}}{{ "\t" }}{{.Usage}}
{{end}}{{if .Flags}}
FLAGS:
{{range .Flags}}{{.}}
{{end}}{{end}}
VERSION:
` + Version +
	`{{ "\n"}}`

type App struct {
	*cli.App
}

func New() *App {
	// Set up app.
	app := cli.NewApp()
	app.Name = "anam"
	app.Author = "Remco Verhoef"
	app.Usage = `anam "/.git/head" "/.svn/entries"`
	app.Description = `ANAM: blabla`
	app.Flags = globalFlags
	app.CustomAppHelpTemplate = helpTemplate
	app.Commands = []cli.Command{}

	app.Before = func(c *cli.Context) error {
		return nil
	}

	app.Action = run

	return &App{
		app,
	}
}

func run(c *cli.Context) {
	cfg := config.LoadFromContext(c)

	if len(c.Args()) == 0 {
		// help()
		os.Exit(1)
	}

	cfg.Paths = c.Args()

	color.Green("ANAM: Mass http(s) scanner. (c) Dutchcoders")
	color.Green("Using interface: %s.", cfg.Interface)

	if c.GlobalBool("profiler") {
		go func() {
			fmt.Println(color.YellowString("Starting profiler on :6060."))
			if err := http.ListenAndServe(":6060", nil); err != nil {
				panic(err)
			}
		}()
	}

	var anam *scanner.Scanner
	if a, err := scanner.New(cfg); err != nil {
		panic(err)
	} else {
		anam = a
	}

	if servers := c.GlobalString("resolvers"); servers == "" {
	} else if resolver := dns_resolver.New(strings.Split(servers, ",")); resolver == nil {
	} else {
		anam.SetResolver(resolver)
	}

	// reading input from stdin
	fi, err := os.Stdin.Stat()
	if err != nil {
		panic(err)
	}

	if fi.Mode()&os.ModeNamedPipe == 0 {
		fmt.Println(color.RedString(fmt.Sprintf("Could not read hosts from stdin.")))
		return
	}

	go func() {
		scanner := bufio.NewScanner(os.Stdin)

		feeder := anam.Feed()
		for scanner.Scan() {
			feeder <- scanner.Text()
		}

		if err := scanner.Err(); err != nil {
			panic(err)
		}

		close(feeder)
	}()

	ctx, cancelFn := context.WithCancel(context.Background())

	go func() {
		s := make(chan os.Signal, 1)
		signal.Notify(s, os.Interrupt)
		signal.Notify(s, syscall.SIGTERM)

		select {
		case <-s:
			fmt.Println(color.YellowString(fmt.Sprintf("Aborting scan.")))
			cancelFn()
		}
	}()

	anam.Scan(ctx)
}
