// Command tuide is a desktop environment for the terminal.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/apps/calc"
	"github.com/dedomorozoff/tuide/internal/apps/clock"
	"github.com/dedomorozoff/tuide/internal/apps/notes"
	"github.com/dedomorozoff/tuide/internal/apps/settings"
	"github.com/dedomorozoff/tuide/internal/apps/term"
	"github.com/dedomorozoff/tuide/internal/apps/welcome"
	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/client"
	"github.com/dedomorozoff/tuide/internal/config"
	"github.com/dedomorozoff/tuide/internal/server"
	"github.com/dedomorozoff/tuide/internal/session"
	"github.com/dedomorozoff/tuide/internal/shell"
	"github.com/dedomorozoff/tuide/internal/sys"
	"github.com/dedomorozoff/tuide/internal/theme"
	"github.com/dedomorozoff/tuide/internal/wire"
)

const usage = `tuiDE - a desktop environment for the terminal

Usage:
  tuide [options] [app...]   start a session, or attach to the one already running
  tuide attach               attach to the running session
  tuide ls                   show the running session
  tuide kill-server          end the session for everyone

Options:
  -d               start the session without attaching to it
  -local           run without a server, directly in this terminal
  -theme <name>    color palette to start with
  -no-taskbar      start without the taskbar
  -no-mouse        start with mouse reporting disabled
  -list-themes     print the available palettes and exit
  -h, --help       print this help and exit

While attached, ctrl+\ detaches: the session keeps running and "tuide attach"
brings it back, even from another terminal or after ssh disconnects.
`

// attachWait is how long a fresh client waits for its server to come up.
const attachWait = 10 * time.Second

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
	serve    bool
	detach   bool
	local    bool
	command  string
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

	switch {
	case opts.command != "":
		return runCommand(opts.command)
	case opts.serve:
		return runServe()
	case opts.local:
		return runLocal(opts)
	default:
		return runSession(opts)
	}
}

// runSession is the default: attach to the running session, or start a server
// in the background and attach to the new one.
func runSession(opts *flags) error {
	if state, err := wire.ReadState(); err == nil && state != nil {
		if _, err := client.List(state.Addr, state.Token); err == nil {
			if opts.detach {
				return nil
			}
			return client.Attach(state.Addr, state.Token)
		}
	}
	if err := sys.StartBackground(serverArgs()); err != nil {
		return err
	}
	if opts.detach {
		return nil
	}
	return waitAndAttach()
}

// serverArgs is this executable re-run as the detached server, with the
// user's own flags so the first launch defines the session.
func serverArgs() []string {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	return append(append([]string{exe}, os.Args[1:]...), "--serve")
}

// waitAndAttach gives a fresh server the time to publish its state file.
func waitAndAttach() error {
	deadline := time.Now().Add(attachWait)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		state, err := wire.ReadState()
		if err != nil || state == nil {
			continue
		}
		if _, err := client.List(state.Addr, state.Token); err != nil {
			continue
		}
		return client.Attach(state.Addr, state.Token)
	}
	return errors.New("the session did not start")
}

// runCommand handles attach, ls and kill-server against the running server.
func runCommand(name string) error {
	state, err := wire.ReadState()
	if err != nil {
		return err
	}
	if state == nil {
		return errors.New("no tuide session is running")
	}
	switch name {
	case "attach":
		return client.Attach(state.Addr, state.Token)
	case "ls":
		info, err := client.List(state.Addr, state.Token)
		if err != nil {
			return err
		}
		fmt.Printf("session: pid %d, %d client(s)\n", info.Pid, info.Clients)
		return nil
	case "kill-server":
		return client.Shutdown(state.Addr, state.Token)
	}
	return fmt.Errorf("unknown command %q", name)
}

// runServe is the detached server: the desktop runs headless and its screen
// reaches clients over the wire. It never touches a controlling terminal.
func runServe() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	launch := cfg.Startup
	if launch == nil {
		launch = []string{"welcome"}
	}

	palette := theme.Get(cfg.Theme)
	model := shell.New(palette, builtinApps(palette), shell.Options{
		Taskbar:    cfg.Taskbar,
		Mouse:      cfg.Mouse,
		Launch:     launch,
		StartLabel: cfg.StartLabel,
		Persist:    true,
	})
	if snap, err := session.Load(); err == nil && !snap.Empty() {
		model.WithRestore(snap)
	}

	logw := serverLog()
	srv := server.New(model, logw)
	runErr := srv.Run()
	if runErr != nil {
		fmt.Fprintln(logw, "tuide: server run:", runErr)
	}

	if err := session.Save(srv.Model().Snapshot()); err != nil {
		fmt.Fprintln(logw, "tuide: save session:", err)
	}
	return runErr
}

// runLocal keeps the single-process behaviour: no server, no sessions, the
// desktop owns this terminal and exits with it.
func runLocal(opts *flags) error {
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

// serverLog opens the server's log file in the cache directory. A server has
// no terminal to complain on, so this is the only place a failure can show.
func serverLog() io.Writer {
	dir, err := sys.CacheDir()
	if err != nil {
		return io.Discard
	}
	if err := sys.EnsureDir(dir); err != nil {
		return io.Discard
	}
	file, err := os.OpenFile(filepath.Join(dir, "server.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return io.Discard
	}
	return file
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

	if len(args) > 0 {
		switch args[0] {
		case "attach", "ls", "kill-server":
			opts.command = args[0]
			args = args[1:]
		}
	}
	if opts.command != "" && len(args) > 0 {
		return nil, fmt.Errorf("unexpected arguments after %q", opts.command)
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			fmt.Print(usage)
			return nil, nil

		case arg == "-list-themes":
			opts.listOnly = true

		case arg == "--serve":
			opts.serve = true

		case arg == "-d":
			opts.detach = true

		case arg == "-local":
			opts.local = true

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
