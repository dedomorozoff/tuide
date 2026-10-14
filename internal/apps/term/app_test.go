package term

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/compositor"
	"github.com/dedomorozoff/tuide/internal/geom"
	"github.com/dedomorozoff/tuide/internal/theme"
	"github.com/dedomorozoff/tuide/internal/vt"
)

func TestPlainTextPassesThrough(t *testing.T) {
	tests := []struct {
		name string
		key  tea.Key
		want string
	}{
		{"letter", tea.Key{Code: 'a', Text: "a"}, "a"},
		{"space", tea.Key{Code: tea.KeySpace, Text: " "}, " "},
		{"uppercase", tea.Key{Code: 'A', Text: "A"}, "A"},
		{"utf8", tea.Key{Code: 'ж', Text: "ж"}, "ж"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(encodeKey(tc.key, vt.Modes{})); got != tc.want {
				t.Fatalf("encodeKey = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestControlChordsReachTheShell(t *testing.T) {
	tests := []struct {
		name string
		key  tea.Key
		want string
	}{
		{"ctrl+c", tea.Key{Code: 'c', Text: "c", Mod: tea.ModCtrl}, "\x03"},
		{"ctrl+w", tea.Key{Code: 'w', Text: "w", Mod: tea.ModCtrl}, "\x17"},
		{"ctrl+q", tea.Key{Code: 'q', Text: "q", Mod: tea.ModCtrl}, "\x11"},
		{"ctrl+d", tea.Key{Code: 'd', Text: "d", Mod: tea.ModCtrl}, "\x04"},
		{"ctrl+space", tea.Key{Code: tea.KeySpace, Mod: tea.ModCtrl}, "\x00"},
		{"ctrl+left", tea.Key{Code: tea.KeyLeft, Mod: tea.ModCtrl}, "\x1b[1;5D"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(encodeKey(tc.key, vt.Modes{})); got != tc.want {
				t.Fatalf("encodeKey = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSpecialKeys(t *testing.T) {
	tests := []struct {
		name string
		key  tea.Key
		want string
	}{
		{"enter", tea.Key{Code: tea.KeyEnter}, "\r"},
		{"tab", tea.Key{Code: tea.KeyTab}, "\t"},
		{"shift+tab", tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}, "\x1b[Z"},
		{"escape", tea.Key{Code: tea.KeyEscape}, "\x1b"},
		{"backspace", tea.Key{Code: tea.KeyBackspace}, "\x7f"},
		{"ctrl+backspace", tea.Key{Code: tea.KeyBackspace, Mod: tea.ModCtrl}, "\x08"},
		{"up", tea.Key{Code: tea.KeyUp}, "\x1b[A"},
		{"down", tea.Key{Code: tea.KeyDown}, "\x1b[B"},
		{"right", tea.Key{Code: tea.KeyRight}, "\x1b[C"},
		{"left", tea.Key{Code: tea.KeyLeft}, "\x1b[D"},
		{"home", tea.Key{Code: tea.KeyHome}, "\x1b[H"},
		{"end", tea.Key{Code: tea.KeyEnd}, "\x1b[F"},
		{"delete", tea.Key{Code: tea.KeyDelete}, "\x1b[3~"},
		{"page up", tea.Key{Code: tea.KeyPgUp}, "\x1b[5~"},
		{"page down", tea.Key{Code: tea.KeyPgDown}, "\x1b[6~"},
		{"f1", tea.Key{Code: tea.KeyF1}, "\x1bOP"},
		{"f5", tea.Key{Code: tea.KeyF5}, "\x1b[15~"},
		{"f12", tea.Key{Code: tea.KeyF12}, "\x1b[24~"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(encodeKey(tc.key, vt.Modes{})); got != tc.want {
				t.Fatalf("encodeKey = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestApplicationCursorModeSwitchesTheArrowKeys(t *testing.T) {
	modes := vt.Modes{AppCursor: true}
	if got := string(encodeKey(tea.Key{Code: tea.KeyUp}, modes)); got != "\x1bOA" {
		t.Fatalf("up = %q, want \\x1bOA", got)
	}
	if got := string(encodeKey(tea.Key{Code: tea.KeyHome}, modes)); got != "\x1bOH" {
		t.Fatalf("home = %q, want \\x1bOH", got)
	}
}

func TestAltIsTheEscapePrefix(t *testing.T) {
	if got := string(encodeKey(tea.Key{Code: 'b', Text: "b", Mod: tea.ModAlt}, vt.Modes{})); got != "\x1bb" {
		t.Fatalf("alt+b = %q, want \\x1bb", got)
	}
	if got := string(encodeKey(tea.Key{Code: tea.KeyLeft, Mod: tea.ModAlt}, vt.Modes{})); got != "\x1b[1;3D" {
		t.Fatalf("alt+left = %q, want the modifier form", got)
	}
}

func TestUnmappedKeyProducesNothing(t *testing.T) {
	if got := encodeKey(tea.Key{Code: tea.KeyCapsLock, Mod: tea.ModCapsLock}, vt.Modes{}); got != nil {
		t.Fatalf("caps lock = %q, want nothing", got)
	}
}

func TestMouseIsNotEncodedWhenTheProgramWantsNone(t *testing.T) {
	event := mouseEvent{button: 0, x: 4, y: 9, press: true}
	if got := encodeMouse(event, vt.MouseOff); got != nil {
		t.Fatalf("encodeMouse with mouse off = %q, want nothing", got)
	}
}

func TestSGRMouseEncoding(t *testing.T) {
	tests := []struct {
		name  string
		event mouseEvent
		want  string
	}{
		{
			"press",
			mouseEvent{button: 0, x: 4, y: 9, press: true},
			"\x1b[<0;5;10M",
		},
		{
			"release",
			mouseEvent{button: 0, x: 4, y: 9},
			"\x1b[<0;5;10m",
		},
		{
			"motion",
			mouseEvent{button: 32, x: 0, y: 0, motion: true},
			"\x1b[<32;1;1M",
		},
		{
			"wheel up",
			mouseEvent{button: 64, x: 1, y: 2, press: true},
			"\x1b[<64;2;3M",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(encodeMouse(tc.event, vt.MouseSGR)); got != tc.want {
				t.Fatalf("encodeMouse = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLegacyMouseEncoding(t *testing.T) {
	press := mouseEvent{button: 0, x: 0, y: 0, press: true}
	if got := string(encodeMouse(press, vt.MouseButton)); got != "\x1b[M\x20\x21\x21" {
		t.Fatalf("press = %q, want \\x1b[M then 0x20 0x21 0x21", got)
	}
	release := mouseEvent{button: 0, x: 0, y: 0}
	if got := string(encodeMouse(release, vt.MouseButton)); got != "\x1b[M\x23\x21\x21" {
		t.Fatalf("release = %q, want the no-button code", got)
	}
}

func TestBracketedPasteWrapsThePayload(t *testing.T) {
	on := vt.Modes{BracketedPaste: true}
	if got := string(paste("hello", on)); got != "\x1b[200~hello\x1b[201~" {
		t.Fatalf("paste = %q, want the bracketed form", got)
	}
	if got := string(paste("hello", vt.Modes{})); got != "hello" {
		t.Fatalf("paste without the mode = %q, want the raw text", got)
	}
}

func TestPasteNormalisesLineEndings(t *testing.T) {
	got := string(paste("a\r\nb", vt.Modes{}))
	if got != "a\nb" {
		t.Fatalf("paste = %q, want a\\nb", got)
	}
}

func TestMouseButtonCodes(t *testing.T) {
	tests := map[tea.MouseButton]int{
		tea.MouseLeft:     0,
		tea.MouseMiddle:   1,
		tea.MouseRight:    2,
		tea.MouseBackward: 3,
		tea.MouseForward:  4,
	}
	for button, want := range tests {
		if got := mouseButtonCode(button); got != want {
			t.Errorf("mouseButtonCode(%v) = %d, want %d", button, got, want)
		}
	}
	if got := mouseButtonCode(tea.MouseNone); got != -1 {
		t.Errorf("mouseButtonCode(none) = %d, want -1", got)
	}
}

func TestTheTerminalAlwaysOwnsItsBody(t *testing.T) {
	a := New(theme.Get(theme.DefaultName))
	if !a.OwnsPointer() {
		t.Error("a terminal must take pointer input in its body: a drag is a selection")
	}
	if a.programWantsMouse() {
		t.Error("a fresh terminal has no program asking for the mouse")
	}

	a.term.Write([]byte("\x1b[?1000h\x1b[?1006h"))
	if !a.programWantsMouse() {
		t.Error("the terminal should notice that a program asked for the mouse")
	}
}

func TestShiftClickSelectsInsteadOfReporting(t *testing.T) {
	pty := &recordingPTY{}
	a := newRecordingApp(pty)
	a.term.Write([]byte("\x1b[?1000h\x1b[?1006h"))
	a.term.Write([]byte("hello world"))
	a.rect = geom.Rect{Width: 20, Height: 5}

	a.onMouse(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft, Mod: tea.ModShift}, true)
	if !a.term.Selecting() {
		t.Fatal("shift click should start a host selection")
	}

	a.onMotion(tea.Mouse{X: 4, Y: 0})
	if got := a.term.SelectedText(); got != "hello" {
		t.Fatalf("selection = %q, want the dragged text", got)
	}

	a.onMouse(tea.Mouse{X: 4, Y: 0, Button: tea.MouseLeft}, false)
	if pty.text() != "" {
		t.Errorf("shift click sent mouse reports to the program: %q", pty.text())
	}
	if a.term.Selecting() {
		t.Error("releasing the button should end the selection")
	}
}

func TestProgramMouseReportsStillReachTheChild(t *testing.T) {
	pty := &recordingPTY{}
	a := newRecordingApp(pty)
	a.term.Write([]byte("\x1b[?1000h\x1b[?1006h"))
	a.rect = geom.Rect{Width: 20, Height: 5}

	cmd := a.onMouse(tea.Mouse{X: 1, Y: 1, Button: tea.MouseLeft}, true)
	if cmd == nil {
		t.Fatal("a program tracking the mouse should get a report")
	}
	_ = cmd()
	if got := pty.text(); got != "\x1b[<0;2;2M" {
		t.Fatalf("press report = %q, want the SGR press", got)
	}
}

func TestTitleFallsBackToTheDefault(t *testing.T) {
	a := New(theme.Get(theme.DefaultName))
	if a.Title() != "Terminal" {
		t.Fatalf("Title = %q, want Terminal", a.Title())
	}
	a.term.Write([]byte("\x1b]0;vim README\x07"))
	if a.Title() != "vim README" {
		t.Fatalf("Title = %q, want the name the program set", a.Title())
	}
}

func TestResizeFollowsTheFrameMessage(t *testing.T) {
	a := New(theme.Get(theme.DefaultName))
	if cmd := a.onFrame(appsdk.Frame{Width: 80, Height: 24}); cmd != nil {
		t.Error("a frame of the same size should do nothing")
	}
	if width, height := a.term.Size(); width != 80 || height != 24 {
		t.Fatalf("size = %dx%d, want 80x24", width, height)
	}

	a.onFrame(appsdk.Frame{Width: 40, Height: 10})
	if width, height := a.term.Size(); width != 40 || height != 10 {
		t.Fatalf("size = %dx%d, want 40x10", width, height)
	}
}

func TestWheelScrollsTheViewport(t *testing.T) {
	a := New(theme.Get(theme.DefaultName))
	a.term.Resize(40, 3)
	a.term.Write([]byte("one\r\ntwo\r\nthree\r\nfour"))

	if a.term.ScrollbackLen() == 0 {
		t.Fatal("writing four lines into a three row terminal should leave history")
	}
	a.onWheel(tea.Mouse{Button: tea.MouseWheelUp})
	if a.term.AtBottom() {
		t.Fatal("one wheel up should scroll into the history")
	}
	a.onWheel(tea.Mouse{Button: tea.MouseWheelDown})
	if !a.term.AtBottom() {
		t.Fatal("one wheel down should come back to the live screen")
	}
}

func TestStatusIsEmptyWhileTheShellLives(t *testing.T) {
	a := New(theme.Get(theme.DefaultName))
	if a.status() != "" {
		t.Fatalf("status = %q, want nothing while the child is running", a.status())
	}
}

func TestASuccessfulResizeDoesNotKillTheTerminal(t *testing.T) {
	a := New(theme.Get(theme.DefaultName))

	// This is what onFrame sends back after a resize that worked.
	_, _ = a.Update(failureMsg{err: nil})

	if a.exited {
		t.Error("a resize that succeeded must not mark the terminal as dead")
	}
	if a.status() != "" {
		t.Errorf("status = %q, want nothing", a.status())
	}
}

func TestAFailedResizeIsReportedButKeepsTheScreen(t *testing.T) {
	a := New(theme.Get(theme.DefaultName))

	_, _ = a.Update(failureMsg{err: errors.New("resize refused")})

	if a.exited {
		t.Error("a failed resize leaves the terminal alive")
	}
	if !strings.Contains(a.status(), "resize refused") {
		t.Errorf("status = %q, want the reason to be visible", a.status())
	}
}

func TestStatusNamesTheFailure(t *testing.T) {
	a := New(theme.Get(theme.DefaultName))
	_, _ = a.Update(startedMsg{err: errors.New("no shell here")})

	status := a.status()
	if !strings.Contains(status, "no shell here") {
		t.Fatalf("status = %q, want the reason the child never started", status)
	}
	if got := lastLine(status); !strings.Contains(got, "no shell here") {
		t.Fatalf("status row = %q, want the message on it", got)
	}
}

func TestStatusReportsAnExitedProcess(t *testing.T) {
	a := New(theme.Get(theme.DefaultName))
	_, _ = a.Update(exitedMsg{})

	if got := lastLine(a.status()); !strings.Contains(got, "process exited") {
		t.Fatalf("status = %q, want it to say the process is gone", a.status())
	}
}

func TestPaintedScreenShowsTheStatusOnTheLastRow(t *testing.T) {
	a := New(theme.Get(theme.DefaultName))
	_, _ = a.Update(exitedMsg{})

	screen := compositor.New(20, 4)
	a.Paint(screen, geom.Rect{Width: 20, Height: 4})

	if got := ansi.Strip(screen.Row(3)); !strings.Contains(got, "process exited") {
		t.Fatalf("row 3 = %q, want the status on the last row", got)
	}
	if got := ansi.Strip(screen.Row(2)); strings.Contains(got, "process exited") {
		t.Fatalf("row 2 = %q, want the screen above the status untouched", got)
	}
}

func TestOSC52ReachesTheClipboard(t *testing.T) {
	a := New(theme.Get(theme.DefaultName))
	a.term.Write([]byte("\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte("copied")) + "\x07"))

	if a.lastClipboard != "" {
		t.Fatal("a clipboard should only be pushed when a new payload arrives")
	}
	if cmd := a.takeClipboard(); cmd == nil {
		t.Fatal("an OSC 52 payload should produce a clipboard command")
	}
	if again := a.takeClipboard(); again != nil {
		t.Error("the same payload should not be pushed twice")
	}
}

func TestOSC52WithoutAPayloadDoesNothing(t *testing.T) {
	a := New(theme.Get(theme.DefaultName))
	a.term.Write([]byte("\x1b]52;c;\x07"))

	if cmd := a.takeClipboard(); cmd != nil {
		t.Error("an empty payload should not touch the clipboard")
	}
}

func TestDecodeClipboardStripsGuards(t *testing.T) {
	guarded := "\x07" + base64.StdEncoding.EncodeToString([]byte("guarded")) + "\x07"
	got, err := decodeClipboard(guarded)
	if err != nil {
		t.Fatalf("decodeClipboard: %v", err)
	}
	if got != "guarded" {
		t.Fatalf("decoded = %q, want guarded", got)
	}

	if _, err := decodeClipboard(""); err == nil {
		t.Error("an empty payload should be reported")
	}
	if _, err := decodeClipboard("not base64!"); err == nil {
		t.Error("a payload that is not base64 should be reported")
	}
}

// lastLine is the visible text of a styled status string.
func lastLine(status string) string {
	return strings.TrimSpace(ansi.Strip(status))
}
