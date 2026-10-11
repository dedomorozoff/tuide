package shell

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"tuide/internal/appsdk"
	"tuide/internal/input"
	"tuide/internal/theme"
	"tuide/internal/wm"
)

// launcherDesktop builds a desktop with three stub apps, none of them open, so
// the launcher has something to start.
func launcherDesktop() *Model {
	specs := []appsdk.Spec{stubSpec("alpha"), stubSpec("bravo"), stubSpec("charlie")}
	m := New(theme.Get(theme.DefaultName), specs, Options{
		Width:   80,
		Height:  24,
		Taskbar: true,
		Mouse:   true,
	})
	m.now = time.Date(2026, time.October, 11, 21, 45, 0, 0, time.UTC)
	return m
}

func TestLauncherKeyOpensOverlay(t *testing.T) {
	m := launcherDesktop()

	press(t, m, tea.Key{Code: tea.KeySpace, Mod: tea.ModCtrl | tea.ModAlt})
	if !m.launcher.open {
		t.Fatal("ctrl+alt+space should open the launcher")
	}
	if m.launcher.mode != launcherApps {
		t.Error("ctrl+alt+space should open the app launcher, not the palette")
	}
}

func TestPaletteKeyOpensActions(t *testing.T) {
	m := launcherDesktop()

	press(t, m, tea.Key{Code: 'a', Mod: tea.ModCtrl | tea.ModAlt})
	if !m.launcher.open || m.launcher.mode != launcherActions {
		t.Fatalf("ctrl+alt+a should open the action palette, got open=%v mode=%v",
			m.launcher.open, m.launcher.mode)
	}
}

func TestLauncherFiltersAndEnterStartsApp(t *testing.T) {
	m := launcherDesktop()

	press(t, m, tea.Key{Code: tea.KeySpace, Mod: tea.ModCtrl | tea.ModAlt})
	for _, r := range "bravo" {
		press(t, m, tea.Key{Code: r, Text: string(r)})
	}
	if got := string(m.launcher.query); got != "bravo" {
		t.Fatalf("query = %q, want bravo", got)
	}

	press(t, m, tea.Key{Code: tea.KeyEnter})
	if m.launcher.open {
		t.Error("enter should close the launcher")
	}
	w := m.windows.Find("bravo")
	if w == nil {
		t.Fatal("enter on a filtered app should open its window")
	}
	if m.windows.Focused() != w {
		t.Error("the launched window should take the focus")
	}
}

func TestLauncherEscapeClosesWithoutLaunching(t *testing.T) {
	m := launcherDesktop()

	press(t, m, tea.Key{Code: tea.KeySpace, Mod: tea.ModCtrl | tea.ModAlt})
	press(t, m, tea.Key{Code: tea.KeyEscape})

	if m.launcher.open {
		t.Error("esc should close the launcher")
	}
	if m.windows.Count() != 0 {
		t.Error("esc must not start an app")
	}
}

func TestLauncherFiltersOutUnmatchedApps(t *testing.T) {
	m := launcherDesktop()

	press(t, m, tea.Key{Code: tea.KeySpace, Mod: tea.ModCtrl | tea.ModAlt})
	for _, r := range "zzz" {
		press(t, m, tea.Key{Code: r, Text: string(r)})
	}
	press(t, m, tea.Key{Code: tea.KeyEnter})

	if m.windows.Count() != 0 {
		t.Error("enter with no matches should not open anything")
	}
	if m.launcher.open {
		t.Error("enter should close the launcher even with no matches")
	}
}

func TestLauncherArrowsMoveSelection(t *testing.T) {
	m := launcherDesktop()

	press(t, m, tea.Key{Code: tea.KeySpace, Mod: tea.ModCtrl | tea.ModAlt})
	press(t, m, tea.Key{Code: tea.KeyDown})
	if m.launcher.selected != 1 {
		t.Fatalf("selection after down = %d, want 1", m.launcher.selected)
	}
	press(t, m, tea.Key{Code: tea.KeyDown})
	press(t, m, tea.Key{Code: tea.KeyDown})
	if m.launcher.selected != 0 {
		t.Errorf("selection should wrap, got %d", m.launcher.selected)
	}
	press(t, m, tea.Key{Code: tea.KeyUp})
	if m.launcher.selected != 2 {
		t.Errorf("selection after up from 0 = %d, want 2", m.launcher.selected)
	}
}

func TestPaletteEnterRunsAction(t *testing.T) {
	m := launcherDesktop()
	press(t, m, tea.Key{Code: '1', Mod: tea.ModCtrl | tea.ModAlt})
	if m.windows.Active() != 0 {
		t.Fatal("test setup: should start on workspace 1")
	}

	press(t, m, tea.Key{Code: 'a', Mod: tea.ModCtrl | tea.ModAlt})
	for _, r := range "workspace 2" {
		press(t, m, tea.Key{Code: r, Text: string(r)})
	}
	press(t, m, tea.Key{Code: tea.KeyEnter})

	if m.windows.Active() != 1 {
		t.Errorf("workspace = %d, want the palette ran workspace 2", m.windows.Active())
	}
}

func TestPaletteQuitExits(t *testing.T) {
	m := launcherDesktop()

	press(t, m, tea.Key{Code: 'a', Mod: tea.ModCtrl | tea.ModAlt})
	for _, r := range "quit" {
		press(t, m, tea.Key{Code: r, Text: string(r)})
	}
	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("the quit action should produce a command")
	}
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Errorf("quit action produced %T, want tea.QuitMsg", msg)
	}
}

func TestBackspaceEditsTheQuery(t *testing.T) {
	m := launcherDesktop()

	press(t, m, tea.Key{Code: tea.KeySpace, Mod: tea.ModCtrl | tea.ModAlt})
	for _, r := range "brav" {
		press(t, m, tea.Key{Code: r, Text: string(r)})
	}
	press(t, m, tea.Key{Code: tea.KeyBackspace})
	if got := string(m.launcher.query); got != "bra" {
		t.Errorf("query after backspace = %q, want bra", got)
	}
}

func TestLauncherDoesNotLeakKeysToWindows(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	w := m.windows.Focused()
	rect := w.Rect

	press(t, m, tea.Key{Code: tea.KeySpace, Mod: tea.ModCtrl | tea.ModAlt})
	for _, r := range "hello" {
		press(t, m, tea.Key{Code: r, Text: string(r)})
	}
	press(t, m, tea.Key{Code: tea.KeyEscape})

	if w.Rect != rect {
		t.Errorf("typing in the launcher moved the window: %+v", w.Rect)
	}
}

func TestTaskbarStartButtonOpensLauncher(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	rect := m.startRect()
	m.Update(tea.MouseClickMsg(tea.Mouse{
		X:      rect.X + 1,
		Y:      rect.Y,
		Button: tea.MouseLeft,
	}))
	if !m.launcher.open {
		t.Error("clicking start should open the launcher")
	}
}

func TestTaskbarButtonClickFocusesAndMinimizes(t *testing.T) {
	m := newMultiDesktop("a", "b")
	_ = m.Init()

	focused := m.windows.Focused()
	other := m.windows.Windows()[0]
	if other == focused {
		other = m.windows.Windows()[1]
	}

	barY := m.height - TaskbarHeight
	buttonX := m.startRect().Right() + 1
	m.Update(tea.MouseClickMsg(tea.Mouse{X: buttonX, Y: barY, Button: tea.MouseLeft}))
	if m.windows.Focused() != other {
		t.Fatalf("taskbar click should focus the first window button, got %v", m.windows.Focused())
	}

	m.Update(tea.MouseClickMsg(tea.Mouse{X: buttonX, Y: barY, Button: tea.MouseLeft}))
	if other.State != wm.Minimized {
		t.Error("clicking the focused window's button should minimize it")
	}
}

func TestClickOutsideTaskbarIsNotATaskbarClick(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	m.Update(tea.MouseClickMsg(tea.Mouse{
		X:      m.startRect().X + 1,
		Y:      m.startRect().Y - 1,
		Button: tea.MouseLeft,
	}))
	if m.launcher.open {
		t.Error("a click above the taskbar must not open the launcher")
	}
}

// tickRecorder is an app that remembers the last clock tick it was told about.
type tickRecorder struct {
	last time.Time
}

func (a *tickRecorder) Init() tea.Cmd { return nil }

func (a *tickRecorder) Update(msg tea.Msg) (appsdk.App, tea.Cmd) {
	if tick, ok := msg.(appsdk.Tick); ok {
		a.last = tick.Time
	}
	return a, nil
}

func (a *tickRecorder) View(appsdk.Context) tea.View { return tea.NewView("") }

func (a *tickRecorder) Close() tea.Cmd { return nil }

func TestTickBroadcastsClockToApps(t *testing.T) {
	recorder := &tickRecorder{}
	spec := appsdk.Spec{
		ID:            "recorder",
		Name:          "Recorder",
		DefaultWidth:  20,
		DefaultHeight: 6,
		MinWidth:      8,
		MinHeight:     3,
		Launch:        func() appsdk.App { return recorder },
	}
	m := New(theme.Get(theme.DefaultName), []appsdk.Spec{spec}, Options{
		Width:  40,
		Height: 12,
		Launch: []string{"recorder"},
	})
	_ = m.Init()

	at := time.Date(2026, time.October, 11, 21, 45, 0, 0, time.UTC)
	_, cmd := m.Update(tickMsg(at))

	if m.now != at {
		t.Errorf("now = %v, want the tick time %v", m.now, at)
	}
	if cmd == nil {
		t.Fatal("a tick should schedule the next one")
	}
	if recorder.last.IsZero() {
		t.Fatal("the tick should reach open apps as appsdk.Tick")
	}
	if recorder.last != at {
		t.Errorf("app got tick %v, want %v", recorder.last, at)
	}
}

func TestTaskbarClockShowsTickTime(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	m.Update(tickMsg(time.Date(2026, time.October, 11, 9, 5, 0, 0, time.UTC)))
	if got := m.clockText(); got != " 09:05 " {
		t.Errorf("clockText = %q, want %q", got, " 09:05 ")
	}
}

func TestLauncherOverlayFrame(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()
	m.openLauncher(launcherApps)
	assertGolden(t, "desktop-launcher", render(t, m))
}

func TestPaletteOverlayFrame(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()
	m.openLauncher(launcherActions)
	assertGolden(t, "desktop-palette", render(t, m))
}

func TestDefaultsIncludeLauncherKeys(t *testing.T) {
	keymap := input.DefaultKeymap()

	action, ok := keymap.Lookup(tea.KeyPressMsg(tea.Key{Code: tea.KeySpace, Mod: tea.ModCtrl | tea.ModAlt}))
	if !ok || action != input.ActionOpenLauncher {
		t.Errorf("ctrl+alt+space = %q, %v, want open-launcher", action, ok)
	}
	action, ok = keymap.Lookup(tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl | tea.ModAlt}))
	if !ok || action != input.ActionOpenPalette {
		t.Errorf("ctrl+alt+a = %q, %v, want open-palette", action, ok)
	}
}
