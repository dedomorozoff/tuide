package shell

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dedomorozoff/tuide/internal/apps/term"
	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/theme"
)

const liveMarker = "tuide-shell-ok"

// TestTerminalWindowShowsALiveShell opens the terminal the way a user does and
// types into it through the shell's own key routing, then waits for the answer
// to show up in a composited frame. Every other terminal test stops before this
// point: they either fake the child or check the emulator in isolation, so a
// break between the process and the screen would pass all of them.
func TestTerminalWindowShowsALiveShell(t *testing.T) {
	if os.Getenv("TUIDE_LIVE_TESTS") != "1" {
		t.Skip("set TUIDE_LIVE_TESTS=1 to run tests that spawn a real child")
	}

	palette := theme.Get(theme.DefaultName)
	desktop := New(palette, []appsdk.Spec{term.Spec(palette)}, Options{
		Width:   100,
		Height:  30,
		Taskbar: true,
		Mouse:   true,
		Launch:  []string{"term"},
	})

	pump(t, desktop, desktop.Init())

	// The window owns a live child process and a goroutine parked on its read.
	// Leaving it behind keeps the test binary alive after the test is over.
	t.Cleanup(func() {
		w := desktop.windows.Find("term")
		if w == nil {
			return
		}
		if cmd := desktop.windows.Close(w.ID); cmd != nil {
			cmd()
		}
	})

	waitFor(t, desktop, "the shell to put a prompt on the screen", func(frame string) bool {
		return strings.Contains(frame, "$") || strings.Contains(frame, "PS ")
	})

	// Typing produces the command that hands the encoded key to the child, so
	// every key has to be pumped the way the runtime would run it.
	for _, r := range "echo " + liveMarker {
		_, cmd := desktop.Update(tea.KeyPressMsg(tea.Key{Code: r, Text: string(r)}))
		pump(t, desktop, cmd)
	}
	_, cmd := desktop.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	pump(t, desktop, cmd)

	waitFor(t, desktop, "the echo to appear in a composited frame", func(frame string) bool {
		return strings.Contains(frame, liveMarker)
	})
}

// pump runs commands the way Bubble Tea does: in the background, expanding
// batches and feeding every message a command produces back into the model. The
// read command a terminal issues never returns on its own, so this is the only
// shape that can drive a live desktop from a test.
func pump(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}

	go func() {
		msg := cmd()
		if msg == nil {
			return
		}
		// tea.Batch does not run the commands it carries, it hands the runtime
		// a list to run. A test has to expand that list itself or the terminal
		// never starts and the window stays blank.
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, next := range batch {
				pump(t, m, next)
			}
			return
		}
		model, next := m.Update(msg)
		pump(t, model.(*Model), next)
	}()
}

// waitFor polls until the condition holds for the composited frame or the
// deadline is out, and shows that frame when it never does.
func waitFor(t *testing.T, m *Model, what string, done func(frame string) bool) {
	t.Helper()

	deadline := time.Now().Add(30 * time.Second)
	for {
		frame := ansi.Strip(m.View().Content)
		if done(frame) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s, frame was:\n%s", what, frame)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
