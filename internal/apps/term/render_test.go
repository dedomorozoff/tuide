package term

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/dedomorozoff/tuide/internal/appsdk"

	"github.com/dedomorozoff/tuide/internal/compositor"
	"github.com/dedomorozoff/tuide/internal/geom"
	"github.com/dedomorozoff/tuide/internal/theme"
)

// session is what a shell writes before anything takes the screen: a prompt
// with colours, a directory listing with styles, and a command's output.
const session = "\x1b[?25l" +
	"\x1b[1;32muser@host\x1b[0m \x1b[1;34m~/src/tuide\x1b[0m$ ls internal\r\n" +
	"\r\n" +
	"\x1b[1;34mapps\x1b[0m  \x1b[1;34mcompositor\x1b[0m  \x1b[1;34mshell\x1b[0m  \x1b[1;34mvt\x1b[0m\r\n" +
	"\x1b[1;34mwm\x1b[0m\r\n" +
	"\r\n" +
	"\x1b[1;32muser@host\x1b[0m \x1b[1;34m~/src/tuide\x1b[0m$ \x1b[1;31mgit status\x1b[0m\r\n" +
	"On branch \x1b[1;32mmain\x1b[0m\r\n" +
	"nothing to commit, working tree clean\r\n" +
	"\r\n" +
	"\x1b[1;32muser@host\x1b[0m \x1b[1;34m~/src/tuide\x1b[0m$ \x1b[7m less README.md \x1b[0m\r\n"

// pager is what a full screen program writes once it owns the screen.
const pager = "\x1b[?1049h\x1b[?25h\x1b[2J\x1b[H" +
	"\x1b[7m README.md \x1b[0m\r\n" +
	"\x1b[1m# tuiDE\x1b[0m\r\n" +
	"A desktop environment inside your terminal.\x1b[3K\r\n" +
	"\x1b[7mdocs  help  quit\x1b[0m"

// painted is the frame a terminal window shows, as plain text.
func painted(t *testing.T, a *App, width, height int) string {
	t.Helper()

	screen := compositor.New(width, height)
	a.Paint(screen, geom.Rect{Width: width, Height: height})

	rows := make([]string, height)
	for y := range height {
		rows[y] = strings.TrimRight(ansi.Strip(screen.Row(y)), " ")
	}
	return strings.Join(rows, "\n")
}

func feed(t *testing.T, a *App, data string) {
	t.Helper()

	_, cmd := a.Update(outputMsg(data))
	if cmd == nil {
		t.Fatal("feeding output should schedule the next read")
	}
}

func newPainted(t *testing.T, width, height int, data string) string {
	t.Helper()

	a := New(theme.Get(theme.DefaultName))
	_, _ = a.Update(appsdk.Frame{Width: width, Height: height})
	feed(t, a, data)
	return painted(t, a, width, height)
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", name+".golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("create testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got+"\n"), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}

	golden, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (rerun with UPDATE_GOLDEN=1)", path, err)
	}
	// Git may check the files out with CRLF endings, so the comparison runs on
	// line endings the working tree agreed to.
	want := strings.ReplaceAll(string(golden), "\r\n", "\n")
	if got+"\n" != want {
		t.Errorf("frame mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestTerminalFrame is the whole path a terminal frame takes without an OS
// pipe: bytes in, cells painted, window sized to fit.
func TestTerminalFrame(t *testing.T) {
	assertGolden(t, "session", newPainted(t, 46, 12, session))
	assertGolden(t, "pager", newPainted(t, 46, 12, session+pager))
}

// TestAlternateScreenTakesOver checks the transition a full screen program
// makes and the moment it gives the screen back.
func TestAlternateScreenTakesOver(t *testing.T) {
	a := New(theme.Get(theme.DefaultName))
	_, _ = a.Update(appsdk.Frame{Width: 24, Height: 6})
	feed(t, a, "before\r\n\x1b[?1049h\x1b[2J\x1b[Hinside")

	if got := painted(t, a, 24, 6); !strings.HasPrefix(got, "inside") {
		t.Fatalf("frame =\n%s\nwant the alternate screen to start with 'inside'", got)
	}

	feed(t, a, "\x1b[?1049l")
	if got := painted(t, a, 24, 6); !strings.HasPrefix(got, "before") {
		t.Fatalf("frame =\n%s\nwant the primary screen back", got)
	}
}

// TestResizeKeepsTheScrollback shows what a user sees after shrinking a window.
func TestResizeKeepsTheScrollback(t *testing.T) {
	a := New(theme.Get(theme.DefaultName))
	_, _ = a.Update(appsdk.Frame{Width: 20, Height: 6})
	feed(t, a, "one\r\ntwo\r\nthree\r\nfour\r\nfive\r\nsix\r\nseven")

	_, _ = a.Update(appsdk.Frame{Width: 20, Height: 3})
	got := painted(t, a, 20, 3)
	if !strings.Contains(got, "seven") {
		t.Fatalf("frame =\n%s\nwant the newest line still visible", got)
	}
	if strings.Contains(got, "one") {
		t.Fatalf("frame =\n%s\nwant the lines that scrolled off gone from the screen", got)
	}
	if a.term.ScrollbackLen() == 0 {
		t.Error("the lines that no longer fit should be in the scrollback, not lost")
	}
}

// TestStylesSurviveThePaint checks that colour from the program reaches the
// compositor as a style prefix rather than as text. Basic colours are
// normalised to their indexed form, so downstream only ever sees one encoding.
func TestStylesSurviveThePaint(t *testing.T) {
	a := New(theme.Get(theme.DefaultName))
	_, _ = a.Update(appsdk.Frame{Width: 12, Height: 1})
	feed(t, a, "\x1b[31mred\x1b[0m plain")

	screen := compositor.New(12, 1)
	a.Paint(screen, geom.Rect{Width: 12, Height: 1})

	if style := screen.At(0, 0).Style; style != "\x1b[38;5;1m" {
		t.Errorf("first cell style = %q, want the indexed red", style)
	}
	if style := screen.At(4, 0).Style; style != "" {
		t.Errorf("cell after the reset = %q, want the default", style)
	}
	if got := screen.At(0, 0).Text; got != "r" {
		t.Errorf("cell text = %q, want the character, not the escape sequence", got)
	}
}
