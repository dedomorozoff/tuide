package term

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/sys"
	"github.com/dedomorozoff/tuide/internal/theme"
)

const liveMarker = "tuide-live-ok"

// liveShell is the child the end to end test drives: the system command
// processor on Windows, a plain shell elsewhere.
func liveShell(t *testing.T) []string {
	t.Helper()

	if runtime.GOOS == "windows" {
		shell := os.Getenv("COMSPEC")
		if shell == "" {
			shell = `C:\Windows\System32\cmd.exe`
		}
		if _, err := os.Stat(shell); err != nil {
			t.Skipf("command processor unavailable: %v", err)
		}
		return []string{shell}
	}

	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skipf("no shell available: %v", err)
	}
	return []string{"/bin/sh"}
}

// TestLiveChildOutputReachesTheScreen drives a real process over the whole path
// the desktop uses: the read pump, the decoder and the grid the window paints
// from. Every other test in this package runs against a fake terminal, so a
// break anywhere between the process and the screen would not show up in them.
func TestLiveChildOutputReachesTheScreen(t *testing.T) {
	app := New(theme.Get(theme.DefaultName))

	terminal, err := sys.StartPTY(sys.PTYOptions{
		Command: liveShell(t),
		Width:   app.width,
		Height:  app.height,
	})
	if err != nil {
		t.Skipf("no terminal available: %v", err)
	}
	defer func() { _ = terminal.Close() }()

	app.pty = terminal
	app.out = make(chan []byte, readQueue)
	app.done = make(chan struct{})
	defer close(app.done)

	go app.readPump()

	if _, err := terminal.Write([]byte("echo " + liveMarker + "\r\n")); err != nil {
		t.Fatalf("write to child: %v", err)
	}

	deadline := time.After(30 * time.Second)
	for {
		select {
		case chunk, ok := <-app.out:
			if !ok {
				t.Fatalf("the child stopped writing before answering, screen:\n%s", screenText(app))
			}
			app.Update(outputMsg(chunk))
			if strings.Contains(screenText(app), liveMarker) {
				return
			}
		case <-deadline:
			t.Fatalf("the marker never reached the screen, last screen:\n%s", screenText(app))
		}
	}
}

// TestLiveChildSurvivesAResize checks the other half of the live path: the
// window manager resizes a terminal window as soon as it opens, and a transport
// that cannot take it must not take the terminal down with it.
func TestLiveChildSurvivesAResize(t *testing.T) {
	app := New(theme.Get(theme.DefaultName))

	terminal, err := sys.StartPTY(sys.PTYOptions{
		Command: liveShell(t),
		Width:   80,
		Height:  24,
	})
	if err != nil {
		t.Skipf("no terminal available: %v", err)
	}
	defer func() { _ = terminal.Close() }()

	app.pty = terminal
	app.out = make(chan []byte, readQueue)
	app.done = make(chan struct{})
	defer close(app.done)

	app.Update(appsdk.Frame{Width: 60, Height: 18})

	if app.width != 60 || app.height != 18 {
		t.Fatalf("frame %dx%d was not applied, app is %dx%d",
			60, 18, app.width, app.height)
	}
	if !sys.ChildAlive(terminal) {
		t.Error("a resize must not kill the child process")
	}
}

func screenText(app *App) string {
	_, height := app.term.Size()
	lines := make([]string, 0, height)
	for y := 0; y < height; y++ {
		lines = append(lines, app.term.Line(y))
	}
	return strings.Join(lines, "\n")
}
