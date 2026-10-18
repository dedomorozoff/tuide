// Command tuide is a desktop environment for the terminal.
package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/apps/calc"
	"github.com/dedomorozoff/tuide/internal/apps/clock"
	"github.com/dedomorozoff/tuide/internal/apps/notes"
	"github.com/dedomorozoff/tuide/internal/apps/settings"
	"github.com/dedomorozoff/tuide/internal/apps/term"
	"github.com/dedomorozoff/tuide/internal/apps/welcome"
	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/config"
	"github.com/dedomorozoff/tuide/internal/shell"
	"github.com/dedomorozoff/tuide/internal/sys"
	"github.com/dedomorozoff/tuide/internal/theme"
)

const usage = `tuiDE - a desktop environment for the terminal

Usage:
  tuide [options] [app...]

Options:
  -theme <name>   color palette to start with
  -no-taskbar     start without the taskbar
  -no-mouse       start with mouse reporting disabled
  -list-themes    print the available palettes and exit
  -h, --help      print this help and exit
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tuide:", err)
		os.Exit(1)
	}
}

type flags struct {
	theme    string
	taskbar  bool
	mouse    bool
	listOnly bool
	apps     []string
}

func run(args []string) error {
	opts, err := parseFlags(args)
	if err != nil {
		return err
	}
	if opts == nil {
		return nil
	}
	if opts.listOnly {
		return listThemes()
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if opts.theme != "" {
		cfg.Theme = opts.theme
	}
	cfg.Taskbar = opts.taskbar
	cfg.Mouse = opts.mouse

	launch := opts.apps
	if launch == nil {
		launch = cfg.Startup
	}
	// A first run with no config opens the welcome window; an explicit empty
	// "startup" in the config keeps the desktop empty. Everything else starts
	// from the launcher (ctrl+alt+space).
	if launch == nil {
		launch = []string{"welcome"}
	}

	palette := theme.Get(cfg.Theme)
	model := shell.New(palette, builtinApps(palette), shell.Options{
		Taskbar:    cfg.Taskbar,
		Mouse:      cfg.Mouse,
		Launch:     launch,
		StartLabel: cfg.StartLabel,
	})

	return runProgram(model)
}

// runProgram owns the terminal: Bubble Tea takes it over on Run and must be
// given back before we exit, whatever happens. Resetting also clears the
// screen, so the shutdown notice is printed afterwards to stay visible.
func runProgram(model tea.Model) error {
	var (
		final  tea.Model
		runErr error
	)
	defer func() {
		if err := sys.ResetTerminal(os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "tuide:", err)
		}
		if runErr != nil {
			if desktop, ok := final.(*shell.Model); ok {
				printShutdownNotice(desktop)
			}
		}
	}()

	final, runErr = tea.NewProgram(model).Run()
	return runErr
}

func printShutdownNotice(m *shell.Model) {
	if err := m.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "tuide:", err)
	}
}

func listThemes() error {
	for _, name := range theme.Names() {
		fmt.Println(name)
	}
	return nil
}

func builtinApps(palette theme.Theme) []appsdk.Spec {
	return []appsdk.Spec{
		welcome.Spec(palette),
		clock.Spec(palette),
		notes.Spec(palette),
		calc.Spec(palette),
		settings.Spec(palette),
		term.Spec(palette),
	}
}

func parseFlags(args []string) (*flags, error) {
	opts := &flags{taskbar: true, mouse: true}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			fmt.Print(usage)
			return nil, nil

		case arg == "-list-themes":
			opts.listOnly = true

		case arg == "-no-taskbar":
			opts.taskbar = false

		case arg == "-no-mouse":
			opts.mouse = false

		case arg == "-theme":
			if i+1 >= len(args) {
				return nil, errors.New("-theme needs a palette name")
			}
			i++
			opts.theme = args[i]

		case strings.HasPrefix(arg, "-"):
			return nil, fmt.Errorf("unknown option %q", arg)

		default:
			opts.apps = append(opts.apps, arg)
		}
	}
	return opts, nil
}
