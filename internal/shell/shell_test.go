package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dedomorozoff/tuide/internal/apps/welcome"
	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/theme"
)

const (
	goldenWidth  = 60
	goldenHeight = 18
)

func newTestDesktop(launch ...string) *Model {
	palette := theme.Get(theme.DefaultName)
	if len(launch) == 0 {
		launch = []string{"welcome"}
	}
	m := New(palette, []appsdk.Spec{welcome.Spec(palette)}, Options{
		Width:   goldenWidth,
		Height:  goldenHeight,
		Taskbar: true,
		Mouse:   true,
		Launch:  launch,
	})
	m.now = time.Date(2026, time.October, 11, 21, 45, 0, 0, time.UTC)
	return m
}

func render(t *testing.T, m *Model) string {
	t.Helper()
	_ = m.Init()
	return viewFrame(m)
}

// viewFrame strips the model's current frame to plain text. It is for tests
// that have already brought the desktop up, so rendering does not run Init a
// second time and open the launch list twice.
func viewFrame(m *Model) string {
	content := ansi.Strip(m.View().Content)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
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

func TestDesktopFrame(t *testing.T) {
	assertGolden(t, "desktop", render(t, newTestDesktop()))
}

func TestDesktopWithoutTaskbar(t *testing.T) {
	palette := theme.Get(theme.DefaultName)
	m := New(palette, []appsdk.Spec{welcome.Spec(palette)}, Options{
		Width:  goldenWidth,
		Height: goldenHeight,
		Launch: []string{"welcome"},
	})
	m.now = time.Date(2026, time.October, 11, 21, 45, 0, 0, time.UTC)
	assertGolden(t, "desktop-no-taskbar", render(t, m))
}

func TestDesktopResizes(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()

	if _, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40}); cmd != nil {
		t.Error("resizing should not need a command")
	}
	if got := m.windows.WorkArea(); got.Width != 100 || got.Height != 39 {
		t.Errorf("work area after resize = %+v, want 100x39", got)
	}
}

func TestQuitIsBound(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()

	model, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Mod: tea.ModCtrl | tea.ModAlt}))
	if model == nil {
		t.Fatal("Update returned a nil model")
	}
	if cmd == nil {
		t.Error("ctrl+alt+q should produce a command")
	}
}

func TestUnknownAppIsReported(t *testing.T) {
	m := newTestDesktop("does-not-exist")
	_ = m.Init()

	if m.Err() == nil {
		t.Error("launching an unknown app should be reported")
	}
	if m.windows.Count() != 0 {
		t.Error("a failed launch should not open a window")
	}
}

func TestEmptyLaunchListOpensNoWindows(t *testing.T) {
	palette := theme.Get(theme.DefaultName)
	m := New(palette, []appsdk.Spec{welcome.Spec(palette)}, Options{
		Width:   goldenWidth,
		Height:  goldenHeight,
		Taskbar: true,
	})
	_ = m.Init()

	if m.windows.Count() != 0 {
		t.Errorf("windows = %d, want an empty desktop when nothing is launched", m.windows.Count())
	}
}

func TestClickFocusesWindow(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()

	w := m.windows.Windows()[0]
	if _, cmd := m.Update(tea.MouseClickMsg(tea.Mouse{
		X: w.Rect.X + 1,
		Y: w.Rect.Y + 1,
	})); cmd != nil {
		t.Error("focusing should not need a command")
	}
	if m.windows.Focused() != w {
		t.Error("clicking a window should focus it")
	}
}

func TestWorkAreaExcludesTaskbar(t *testing.T) {
	withBar := workArea(goldenWidth, goldenHeight, true)
	if withBar.Height != goldenHeight-TaskbarHeight {
		t.Errorf("work area height = %d, want %d", withBar.Height, goldenHeight-TaskbarHeight)
	}

	without := workArea(goldenWidth, goldenHeight, false)
	if without.Height != goldenHeight {
		t.Errorf("work area height = %d, want %d", without.Height, goldenHeight)
	}
}
