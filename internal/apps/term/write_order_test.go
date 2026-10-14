package term

import (
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/sys"
	"github.com/dedomorozoff/tuide/internal/theme"
)

// recordingPTY is a terminal that keeps whatever it was asked to write, so a
// test can look at the order the bytes arrived in.
type recordingPTY struct {
	mu     sync.Mutex
	writes []byte
}

func (p *recordingPTY) Read(b []byte) (int, error) { return 0, nil }

func (p *recordingPTY) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.writes = append(p.writes, b...)
	return len(b), nil
}

func (p *recordingPTY) Resize(int, int) error { return nil }

func (p *recordingPTY) Close() error { return nil }

func (p *recordingPTY) text() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return string(p.writes)
}

func newRecordingApp(pty sys.PTY) *App {
	app := New(theme.Get(theme.DefaultName))
	app.pty = pty
	app.done = make(chan struct{})
	return app
}

// TestKeystrokesReachTheChildInOrder is the regression test for the bug where
// fast typing reached the shell scrambled. Every key press used to be handed to
// the child by its own command, and Bubble Tea runs commands concurrently, so
// the bytes raced each other into the channel and arrived in any order.
func TestKeystrokesReachTheChildInOrder(t *testing.T) {
	pty := &recordingPTY{}
	app := newRecordingApp(pty)

	const phrase = "echo ordered\r\n"
	cmds := make([]tea.Cmd, 0, len(phrase))
	for _, r := range phrase {
		_, cmd := app.Update(tea.KeyPressMsg(tea.Key{Code: r, Text: string(r)}))
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// The runtime starts every command at once, so run them the same way.
	var wg sync.WaitGroup
	for _, cmd := range cmds {
		wg.Add(1)
		go func(c tea.Cmd) {
			defer wg.Done()
			c()
		}(cmd)
	}
	wg.Wait()

	if got := pty.text(); got != phrase {
		t.Errorf("child received %q, want %q", got, phrase)
	}
}

// TestAFailedWriteLetsTheNextKeyThrough checks the writer hands its role back
// when a write fails, otherwise one dead child would swallow every keystroke
// after it for the rest of the window's life.
func TestAFailedWriteLetsTheNextKeyThrough(t *testing.T) {
	pty := &failingPTY{}
	app := newRecordingApp(pty)

	if cmd := app.send([]byte("x")); cmd != nil {
		if msg := cmd(); msg == nil {
			t.Fatal("a failed write must report why")
		}
	}

	app.mu.Lock()
	idle := !app.writing
	app.mu.Unlock()
	if !idle {
		t.Error("the writer must be released after a failure")
	}

	pty.fail = false
	if cmd := app.send([]byte("y")); cmd == nil {
		t.Error("the next key must start a fresh writer")
	}
}

// failingPTY refuses every write until fail is cleared.
type failingPTY struct {
	recordingPTY
	fail bool
}

func (p *failingPTY) Write(b []byte) (int, error) {
	if p.fail {
		return 0, errWriteFailed
	}
	return p.recordingPTY.Write(b)
}

var errWriteFailed = &writeError{}

type writeError struct{}

func (*writeError) Error() string { return "write failed" }
