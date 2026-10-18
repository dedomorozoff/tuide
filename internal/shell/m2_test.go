package shell

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dedomorozoff/tuide/internal/apps/settings"
	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/config"
	"github.com/dedomorozoff/tuide/internal/input"
	"github.com/dedomorozoff/tuide/internal/theme"
	"github.com/dedomorozoff/tuide/internal/wm"
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

func TestBareCtrlPKeyOpensOverlay(t *testing.T) {
	m := launcherDesktop()

	press(t, m, tea.Key{Code: 'p', Mod: tea.ModCtrl})
	if !m.launcher.open || m.launcher.mode != launcherApps {
		t.Fatalf("ctrl+p should open the app launcher, got open=%v mode=%v",
			m.launcher.open, m.launcher.mode)
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

func TestPaletteListsTheCalendar(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()
	m.openLauncher(launcherActions)

	for _, r := range "cal" {
		press(t, m, tea.Key{Code: r, Text: string(r)})
	}

	row := launcherRowWithLabel(t, m, "Calendar")
	if cmd := m.runLauncherRow(row); cmd != nil {
		_ = cmd()
	}

	if !m.calendar.open {
		t.Error("the palette's calendar row should open the month view")
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

// TestLauncherRowsMatchTheRenderedOverlay pins the click targets to what is on
// screen. The row rectangles were laid out as if the panel padded vertically as
// well as horizontally, so every target sat one line below the label it belonged
// to: a click ran the row under the pointer or missed the list entirely.
func TestLauncherRowsMatchTheRenderedOverlay(t *testing.T) {
	m := launcherDesktop()
	_ = m.Init()
	m.openLauncher(launcherApps)

	layout := m.launcherLayout()
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")

	for _, row := range layout.rows {
		if row.rect.Y < 0 || row.rect.Y >= len(lines) {
			t.Fatalf("row %q targets y=%d, outside a %d line frame", row.label, row.rect.Y, len(lines))
		}
		if line := lines[row.rect.Y]; !strings.Contains(line, row.label) {
			t.Errorf("row %q targets line %d, which shows %q", row.label, row.rect.Y, line)
		}
	}
}

func TestLauncherClickRunsTheRowUnderThePointer(t *testing.T) {
	m := launcherDesktop()

	press(t, m, tea.Key{Code: tea.KeySpace, Mod: tea.ModCtrl | tea.ModAlt})

	row := launcherRowWithLabel(t, m, "bravo")
	m.Update(tea.MouseClickMsg(tea.Mouse{
		X:      row.rect.X + 1,
		Y:      row.rect.Y,
		Button: tea.MouseLeft,
	}))

	if m.launcher.open {
		t.Error("running a row should close the launcher")
	}
	if m.windows.Find("bravo") == nil {
		t.Fatal("clicking a row should open that app's window")
	}
	if m.windows.Focused() == nil {
		t.Error("the launched window should take the focus")
	}
}

func TestLauncherClickOffTheRowsClosesWithoutLaunching(t *testing.T) {
	m := launcherDesktop()

	press(t, m, tea.Key{Code: tea.KeySpace, Mod: tea.ModCtrl | tea.ModAlt})
	m.Update(tea.MouseClickMsg(tea.Mouse{
		X:      1,
		Y:      1,
		Button: tea.MouseLeft,
	}))

	if m.launcher.open {
		t.Error("a click that misses every row should close the launcher")
	}
	if m.windows.Count() != 0 {
		t.Error("a click off the rows must not start an app")
	}
}

// launcherRowWithLabel finds a row the launcher is currently showing.
func launcherRowWithLabel(t *testing.T, m *Model, label string) launcherRow {
	t.Helper()
	for _, row := range m.launcherLayout().rows {
		if row.label == label {
			return row
		}
	}
	t.Fatalf("the launcher is not showing a %q row", label)
	return launcherRow{}
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

// TestLauncherOpensAboveTheStartButton pins the start menu to the corner the
// button opened it from: left edge, bottom row just above the taskbar.
func TestLauncherOpensAboveTheStartButton(t *testing.T) {
	m := launcherDesktop()
	_ = m.Init()
	m.openLauncher(launcherApps)

	layout := m.launcherLayout()
	bottom := m.windows.WorkArea().Bottom()

	if layout.rect.X != 0 {
		t.Errorf("launcher X = %d, want the left edge", layout.rect.X)
	}
	if layout.rect.Bottom() != bottom {
		t.Errorf("launcher bottom = %d, want just above the taskbar at %d",
			layout.rect.Bottom(), bottom)
	}
}

// TestStartButtonPaintsEveryCellOfItsOwn pins the trap the mark is built for: a
// styled string nested inside another one loses its background where the inner
// reset lands, and a taskbar cell without a background hands the host
// terminal's colours through. The button is two styled panes plus two padding
// cells, so each of the four has to name the chip's own background.
func TestStartButtonPaintsEveryCellOfItsOwn(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()

	styles := theme.NewStyles(theme.Get(theme.DefaultName))
	cases := []struct {
		name       string
		open       bool
		background string
	}{
		{"closed", false, theme.Background(styles.Theme.Accent)},
		{"open", true, theme.Background(styles.Theme.TextStrong)},
	}

	for _, tc := range cases {
		if tc.open {
			m.openLauncher(launcherApps)
		} else {
			m.launcher.open = false
		}
		_ = m.View()

		rect := m.startRect()
		for x := rect.X; x < rect.Right(); x++ {
			if got := m.screen.At(x, rect.Y).Background; got != tc.background {
				t.Errorf("%s: cell %d background = %q, want %q",
					tc.name, x, got, tc.background)
			}
		}
	}
}

func TestStartButtonLabelComesFromTheConfig(t *testing.T) {
	m := launcherDesktop()
	m.applyConfig(config.Config{Theme: theme.DefaultName, Taskbar: true, Mouse: true, StartLabel: "menu"})

	button := m.startButton()
	if !strings.Contains(button, "menu") {
		t.Errorf("button = %q, want the configured label", button)
	}
	if strings.Contains(button, startPaneLit) {
		t.Errorf("button = %q, want the built-in mark replaced by the label", button)
	}

	m.applyConfig(config.Config{Theme: theme.DefaultName, Taskbar: true, Mouse: true})
	if !strings.Contains(m.startButton(), startPaneLit) {
		t.Error("an empty label should fall back to the built-in mark")
	}
}

func TestStartButtonLabelCannotEatTheBar(t *testing.T) {
	m := launcherDesktop()
	m.applyConfig(config.Config{
		Theme:      theme.DefaultName,
		Taskbar:    true,
		Mouse:      true,
		StartLabel: strings.Repeat("m", 3*startLabelMax),
	})

	if got := m.startRect().Width; got > startLabelMax+2 {
		t.Errorf("button width = %d, want at most %d", got, startLabelMax+2)
	}
}

func TestStartButtonShowsItsOpenState(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()

	closed := m.startButton()
	m.openLauncher(launcherApps)
	if open := m.startButton(); open == closed {
		t.Error("an open launcher should change how the button looks")
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

func TestTaskbarMiddleClickClosesWindow(t *testing.T) {
	m := newMultiDesktop("a", "b")
	_ = m.Init()

	target := m.windows.Windows()[0]
	m.Update(tea.MouseClickMsg(tea.Mouse{
		X:      taskbarButtonX(t, m, target.ID),
		Y:      m.height - TaskbarHeight,
		Button: tea.MouseMiddle,
	}))

	if m.windows.Find(target.ID) != nil {
		t.Error("a middle click on a window button should close that window")
	}
	if m.windows.Count() != 1 {
		t.Errorf("the other window must survive, got %d windows", m.windows.Count())
	}
}

func TestTaskbarMiddleClickOnStartDoesNothing(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	m.Update(tea.MouseClickMsg(tea.Mouse{
		X:      m.startRect().X + 1,
		Y:      m.height - TaskbarHeight,
		Button: tea.MouseMiddle,
	}))

	if m.windows.Count() != 1 {
		t.Errorf("the middle button must not close a window, got %d", m.windows.Count())
	}
	if m.launcher.open {
		t.Error("only the left button opens the launcher")
	}
}

func TestTaskbarMiddleClickOnTheBarMissesWindows(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	gap, ok := taskbarGapX(t, m)
	if !ok {
		t.Skip("the taskbar has no bare column at this width")
	}

	m.Update(tea.MouseClickMsg(tea.Mouse{
		X:      gap,
		Y:      m.height - TaskbarHeight,
		Button: tea.MouseMiddle,
	}))

	if m.windows.Count() != 1 {
		t.Errorf("a click on bare taskbar must not close anything, got %d", m.windows.Count())
	}
}

// TestWindowControlsAreVisibleOnTheTitleRow pins what TestWindowTitleButtons
// cannot see. The click targets were right all along while lipgloss wrapped the
// title and pushed the controls onto a second row, under the app's content, so
// the window looked like it had no buttons and a click went to the wrong place.
func TestWindowControlsAreVisibleOnTheTitleRow(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()

	w := m.windows.Focused()
	if w == nil {
		t.Fatal("no focused window")
	}

	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	titleRow := w.TitleRect().Y
	if titleRow >= len(lines) {
		t.Fatalf("title row %d is outside a %d line frame", titleRow, len(lines))
	}
	if !strings.Contains(lines[titleRow], "[_]") || !strings.Contains(lines[titleRow], "[■]") || !strings.Contains(lines[titleRow], "[x]") {
		t.Errorf("the title row does not show the window controls: %q", lines[titleRow])
	}
}

// taskbarButtonX is the middle of one taskbar element, so a test does not have
// to guess which column the layout put it in.
func taskbarButtonX(t *testing.T, m *Model, id string) int {
	t.Helper()
	for _, item := range m.taskbarItems() {
		if item.id == id {
			return item.rect.X + item.rect.Width/2
		}
	}
	t.Fatalf("the taskbar has no %q button", id)
	return 0
}

// taskbarGapX finds a column of the taskbar row that belongs to no element.
func taskbarGapX(t *testing.T, m *Model) (int, bool) {
	t.Helper()

	items := m.taskbarItems()
	for x := 0; x < m.width; x++ {
		covered := false
		for _, item := range items {
			if item.rect.Contains(x, m.height-TaskbarHeight) {
				covered = true
				break
			}
		}
		if !covered {
			return x, true
		}
	}
	return 0, false
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

func TestNetworkLightOnlyShowsWhenTheProbeFailed(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()

	if got := m.statusIndicators(); got != "" {
		t.Errorf("status = %q, want silence before the first probe came back", got)
	}

	m.Update(netMsg(true))
	if got := m.statusIndicators(); got != "" {
		t.Errorf("status = %q, want silence while the network is up", got)
	}

	m.Update(netMsg(false))
	if got := m.statusIndicators(); !strings.Contains(got, "offline") {
		t.Errorf("status = %q, want a warning once a probe has failed", got)
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
	action, ok = keymap.Lookup(tea.KeyPressMsg(tea.Key{Code: 'p', Mod: tea.ModCtrl}))
	if !ok || action != input.ActionOpenLauncher {
		t.Errorf("ctrl+p = %q, %v, want open-launcher", action, ok)
	}
}

type recordingConfigApp struct {
	lastState config.Config
	savedErr  error
	received  bool
}

func (r *recordingConfigApp) Init() tea.Cmd { return nil }
func (r *recordingConfigApp) Update(msg tea.Msg) (appsdk.App, tea.Cmd) {
	switch m := msg.(type) {
	case appsdk.ConfigState:
		r.lastState = m.Config
		r.received = true
	case appsdk.ConfigSaved:
		r.savedErr = m.Err
		r.received = true
	}
	return r, nil
}
func (r *recordingConfigApp) View(appsdk.Context) tea.View { return tea.NewView("config app") }
func (r *recordingConfigApp) Close() tea.Cmd               { return nil }

func TestConfigRequestAndSave(t *testing.T) {
	recApp := &recordingConfigApp{}
	spec := appsdk.Spec{
		ID:            "settings",
		Name:          "Settings",
		DefaultWidth:  30,
		DefaultHeight: 15,
		Launch:        func() appsdk.App { return recApp },
	}

	m := New(theme.Get("midnight"), []appsdk.Spec{spec}, Options{
		Width:   80,
		Height:  24,
		Taskbar: true,
		Mouse:   true,
		Launch:  []string{"settings"},
	})
	_ = m.Init()

	// 1. RequestConfig
	_, cmd := m.Update(appsdk.RequestConfig{})
	if cmd != nil {
		_ = cmd()
	}
	if !recApp.received {
		t.Fatal("expected recording app to receive ConfigState")
	}
	if recApp.lastState.Theme != "midnight" {
		t.Errorf("expected theme midnight, got %q", recApp.lastState.Theme)
	}

	// 2. SaveConfig
	newCfg := config.Config{
		Theme:   "nord",
		Taskbar: false,
		Mouse:   false,
	}
	_, cmd = m.Update(appsdk.SaveConfig{Config: newCfg})
	if m.styles.Theme.Name != "nord" {
		t.Errorf("expected nord theme applied immediately, got %q", m.styles.Theme.Name)
	}
	if m.options.Taskbar != false {
		t.Errorf("expected taskbar disabled, got %v", m.options.Taskbar)
	}
	if cmd != nil {
		savedMsg := cmd()
		_, replyCmd := m.Update(savedMsg)
		if replyCmd != nil {
			_ = replyCmd()
		}
	}
}

// TestSettingsLabelRowPaintsEveryCellOfItsOwn pins the trap the row is built
// for: the cursor cell is styled apart from the selection around it, so the row
// has to be assembled from separately styled pieces. A single string wrapped in
// the selection style would lose its background where the cursor's reset lands,
// and the window body would show through the rest of the line.
func TestSettingsLabelRowPaintsEveryCellOfItsOwn(t *testing.T) {
	palette := theme.Get(theme.DefaultName)
	m := New(palette, []appsdk.Spec{settings.Spec(palette)}, Options{
		Width:   80,
		Height:  24,
		Taskbar: true,
		Mouse:   true,
		Launch:  []string{"settings"},
	})
	m.now = time.Date(2026, time.October, 11, 21, 45, 0, 0, time.UTC)
	_ = m.Init()

	w := m.windows.Find("settings")
	if w == nil {
		t.Fatal("the settings window did not open")
	}
	if _, cmd := m.Update(appsdk.RequestConfig{}); cmd != nil {
		_ = cmd()
	}

	// Into the label row, and one letter into the draft: the row on screen ends
	// up wider than the label it started from.
	for range 3 {
		press(t, m, tea.Key{Code: tea.KeyDown})
	}
	press(t, m, tea.Key{Code: tea.KeyEnter})
	press(t, m, tea.Key{Code: 'g', Text: "g"})

	_ = m.View()

	styles := theme.NewStyles(palette)
	selection := theme.Background(styles.Theme.Selection)
	active := theme.Background(styles.Theme.Cursor)

	cursorX, cursorY, ok := cursorCell(m, w)
	if !ok {
		t.Fatal("the focused window shows no cursor: the label is not in edit mode")
	}

	row := ""
	for x := cursorX; x < w.Rect.Right()-1; x++ {
		cell := m.screen.At(x, cursorY)
		if cell.Background != selection && cell.Background != active {
			break
		}
		row += cell.Text
	}
	for x := cursorX - 1; x > w.Rect.X; x-- {
		cell := m.screen.At(x, cursorY)
		if cell.Background != selection {
			break
		}
		row = cell.Text + row
	}

	if want := "> Start:     [g ]"; row != want {
		t.Errorf("the label row reads %q, want %q: a hole in the row means a cell lost its background", row, want)
	}
}

// cursorCell finds the cell a window's app painted in the cursor colour.
func cursorCell(m *Model, w *wm.Window) (int, int, bool) {
	cursor := theme.Background(theme.NewStyles(m.styles.Theme).Theme.Cursor)

	for y := w.Rect.Y; y < w.Rect.Bottom(); y++ {
		for x := w.Rect.X; x < w.Rect.Right(); x++ {
			if m.screen.At(x, y).Background == cursor {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

func TestWindowTitleButtons(t *testing.T) {
	m := newMultiDesktop("btnwin")
	_ = m.Init()

	w := m.windows.Find("btnwin")
	if w == nil {
		t.Fatal("window btnwin not found")
	}

	// Window starts normal.
	// Click on minimize [_]:
	// The controls live on the title bar, at w.TitleRect().Y.
	// They start at w.Rect.Right() - 1 - 9.
	ctlStart := w.Rect.Right() - 1 - 9
	minX := ctlStart + 1
	titleY := w.TitleRect().Y

	m.Update(tea.MouseClickMsg(tea.Mouse{X: minX, Y: titleY, Button: tea.MouseLeft}))
	if w.State != wm.Minimized {
		t.Errorf("expected minimized after [_] click, got %v", w.State)
	}

	// Restore via taskbar click:
	m.Update(tea.MouseClickMsg(tea.Mouse{X: 10, Y: 23, Button: tea.MouseLeft}))
	if w.State != wm.Normal {
		t.Errorf("expected normal after restoring, got %v", w.State)
	}

	// Click on maximize [^]:
	maxX := ctlStart + 4
	m.Update(tea.MouseClickMsg(tea.Mouse{X: maxX, Y: titleY, Button: tea.MouseLeft}))
	if w.State != wm.Maximized {
		t.Errorf("expected maximized after [^] click, got %v", w.State)
	}

	// Click on close [x]:
	closeX := w.Rect.Right() - 1 - 9 + 7
	closeY := w.TitleRect().Y
	m.Update(tea.MouseClickMsg(tea.Mouse{X: closeX, Y: closeY, Button: tea.MouseLeft}))
	if m.windows.Find("btnwin") != nil {
		t.Error("expected window closed after [x] click")
	}
}
