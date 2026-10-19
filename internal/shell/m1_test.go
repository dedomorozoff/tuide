package shell

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/theme"
	"github.com/dedomorozoff/tuide/internal/wm"
)

// stubApp is a window body that renders nothing, so M1 tests only exercise the
// window manager and the shell's input handling.
type stubApp struct{}

func (stubApp) Init() tea.Cmd                            { return nil }
func (app stubApp) Update(tea.Msg) (appsdk.App, tea.Cmd) { return app, nil }
func (stubApp) View(appsdk.Context) tea.View             { return tea.NewView("body") }
func (stubApp) Close() tea.Cmd                           { return nil }

func stubSpec(id string) appsdk.Spec {
	return appsdk.Spec{
		ID:            id,
		Name:          id,
		DefaultWidth:  24,
		DefaultHeight: 10,
		MinWidth:      6,
		MinHeight:     4,
		Launch:        func() appsdk.App { return stubApp{} },
	}
}

func stubMultiSpec(id string) appsdk.Spec {
	s := stubSpec(id)
	s.Multi = true
	return s
}

// newMultiDesktop builds a desktop with the named stub windows already open.
func newMultiDesktop(ids ...string) *Model {
	specs := make([]appsdk.Spec, 0, len(ids))
	for _, id := range ids {
		specs = append(specs, stubSpec(id))
	}
	return New(theme.Get(theme.DefaultName), specs, Options{
		Width:   80,
		Height:  24,
		Taskbar: true,
		Mouse:   true,
		Launch:  ids,
	})
}

func TestTitleDragMovesWindow(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	w := m.windows.Focused()
	origin := w.Rect
	title := w.TitleRect()
	titleX, titleY := title.X+1, title.Y

	m.Update(tea.MouseClickMsg(tea.Mouse{X: titleX, Y: titleY, Button: tea.MouseLeft}))
	m.Update(tea.MouseMotionMsg(tea.Mouse{X: titleX + 5, Y: titleY + 3, Button: tea.MouseLeft}))
	m.Update(tea.MouseReleaseMsg(tea.Mouse{X: titleX + 5, Y: titleY + 3, Button: tea.MouseLeft}))

	if w.Rect.X != origin.X+5 || w.Rect.Y != origin.Y+3 {
		t.Errorf("window = %+v, want moved by 5,3 from %+v", w.Rect, origin)
	}
	if m.drag.kind != dragNone {
		t.Error("the drag should be finished after release")
	}
}

// doubleClickTitle sends a click pair on the window's title bar.
func doubleClickTitle(t *testing.T, m *Model, w *wm.Window) {
	t.Helper()
	x, y := w.TitleRect().X+1, w.TitleRect().Y
	m.Update(tea.MouseClickMsg(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft}))
	m.Update(tea.MouseReleaseMsg(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft}))
	_, _ = m.Update(tea.MouseClickMsg(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft}))
	m.Update(tea.MouseReleaseMsg(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft}))
	// Break the pair, so the next gesture starts from a clean click.
	m.memo = lastClick{}
}

func TestDoubleClickTitleMaximizesAndRestores(t *testing.T) {
	m := newMultiDesktop("a", "b")
	_ = m.Init()

	w := m.windows.Find("a")
	if w == nil {
		t.Fatal("window a not found")
	}
	m.focus("a")
	origin := w.Rect

	doubleClickTitle(t, m, w)

	if w.State != wm.Maximized {
		t.Fatalf("state after the double click = %v, want maximized", w.State)
	}
	if w.Rect != m.windows.WorkArea() {
		t.Errorf("rect = %+v, want the work area %+v", w.Rect, m.windows.WorkArea())
	}

	doubleClickTitle(t, m, w)

	if w.State != wm.Normal || w.Rect != origin {
		t.Errorf("after the second double click state = %v, rect = %+v, want normal %+v", w.State, w.Rect, origin)
	}
}

func TestDoubleClickTitleZoomsATiledWindow(t *testing.T) {
	m := newMultiDesktop("a", "b")
	_ = m.Init()
	m.windows.SetLayout(wm.LayoutTiled)

	w := m.windows.Find("a")
	doubleClickTitle(t, m, w)

	if w.State != wm.Maximized || w.Rect != m.windows.WorkArea() {
		t.Fatalf("zoom = state %v rect %+v, want maximized over %+v", w.State, w.Rect, m.windows.WorkArea())
	}

	doubleClickTitle(t, m, w)

	if w.State != wm.Normal || !m.windows.WorkArea().ContainsRect(w.Rect) {
		t.Errorf("unzoom = state %v rect %+v, want the window back inside the tiling", w.State, w.Rect)
	}
}

func TestSingleClicksDoNotMaximize(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	w := m.windows.Focused()
	titleX, titleY := w.TitleRect().X+1, w.TitleRect().Y

	m.Update(tea.MouseClickMsg(tea.Mouse{X: titleX, Y: titleY, Button: tea.MouseLeft}))
	m.Update(tea.MouseReleaseMsg(tea.Mouse{X: titleX, Y: titleY, Button: tea.MouseLeft}))
	// The first click now looks long enough in the past to break the pair.
	m.memo.at = m.memo.at.Add(-2 * doubleClickGap)
	m.Update(tea.MouseClickMsg(tea.Mouse{X: titleX, Y: titleY, Button: tea.MouseLeft}))
	m.Update(tea.MouseReleaseMsg(tea.Mouse{X: titleX, Y: titleY, Button: tea.MouseLeft}))

	if w.State == wm.Maximized {
		t.Error("two well spaced clicks must not read as a double click")
	}
}

func TestCornerDragResizesWindow(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	w := m.windows.Focused()
	origin := w.Rect
	x, y := w.Rect.Right()-1, w.Rect.Bottom()-1

	m.Update(tea.MouseClickMsg(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft}))
	m.Update(tea.MouseMotionMsg(tea.Mouse{X: x + 4, Y: y + 2, Button: tea.MouseLeft}))
	m.Update(tea.MouseReleaseMsg(tea.Mouse{X: x + 4, Y: y + 2, Button: tea.MouseLeft}))

	if w.Rect.Width != origin.Width+4 || w.Rect.Height != origin.Height+2 {
		t.Errorf("window = %+v, want resized from %+v", w.Rect, origin)
	}
	if w.Rect.X != origin.X || w.Rect.Y != origin.Y {
		t.Errorf("resizing the bottom-right corner moved the origin: %+v", w.Rect)
	}
}

func TestReleaseNearEdgeSnapsWindow(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	w := m.windows.Focused()
	title := w.TitleRect()
	titleX, titleY := title.X+1, title.Y

	m.Update(tea.MouseClickMsg(tea.Mouse{X: titleX, Y: titleY, Button: tea.MouseLeft}))
	m.Update(tea.MouseMotionMsg(tea.Mouse{X: 1, Y: titleY, Button: tea.MouseLeft}))
	m.Update(tea.MouseReleaseMsg(tea.Mouse{X: 1, Y: titleY, Button: tea.MouseLeft}))

	want := wm.ZoneLeft.Rect(m.windows.WorkArea())
	if w.Rect != want {
		t.Errorf("window = %+v, want snapped to %+v", w.Rect, want)
	}
}

func TestReleaseInTheMiddleKeepsWindowFloating(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	w := m.windows.Focused()
	title := w.TitleRect()
	titleX, titleY := title.X+1, title.Y
	middle := m.windows.WorkArea().Width / 2

	m.Update(tea.MouseClickMsg(tea.Mouse{X: titleX, Y: titleY, Button: tea.MouseLeft}))
	m.Update(tea.MouseMotionMsg(tea.Mouse{X: middle, Y: titleY, Button: tea.MouseLeft}))
	m.Update(tea.MouseReleaseMsg(tea.Mouse{X: middle, Y: titleY, Button: tea.MouseLeft}))

	if w.Rect.Width != 24 || w.Rect.Height != 10 {
		t.Errorf("window should keep its size when dropped away from an edge: %+v", w.Rect)
	}
}

func TestDraggingTheTitleUnfoldsAMaximizedWindow(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	w := m.windows.Focused()
	restored := w.Rect
	if err := m.windows.ToggleMaximized(w.ID); err != nil {
		t.Fatalf("ToggleMaximized: %v", err)
	}
	titleX, titleY := w.TitleRect().X+3, w.TitleRect().Y

	m.Update(tea.MouseClickMsg(tea.Mouse{X: titleX, Y: titleY, Button: tea.MouseLeft}))
	m.Update(tea.MouseMotionMsg(tea.Mouse{X: titleX + 1, Y: titleY, Button: tea.MouseLeft}))
	m.Update(tea.MouseMotionMsg(tea.Mouse{X: titleX + 6, Y: titleY + 5, Button: tea.MouseLeft}))
	m.Update(tea.MouseReleaseMsg(tea.Mouse{X: titleX + 6, Y: titleY + 5, Button: tea.MouseLeft}))

	if w.State != wm.Normal {
		t.Fatalf("state during the drag = %v, want normal", w.State)
	}
	if w.Rect == m.windows.WorkArea() {
		t.Fatalf("rect = %+v, the drag should have moved it off the work area", w.Rect)
	}
	if w.Rect.Width != restored.Width || w.Rect.Height != restored.Height {
		t.Errorf("rect = %+v, want the restored size from %+v", w.Rect, restored)
	}
}

func TestGrabbingTheZoomedTitleEndsTheTiling(t *testing.T) {
	m := newMultiDesktop("a", "b")
	_ = m.Init()
	m.windows.SetLayout(wm.LayoutTiled)

	w := m.windows.Find("a")
	if err := m.windows.ToggleMaximized(w.ID); err != nil {
		t.Fatalf("ToggleMaximized: %v", err)
	}
	zoom := w.Rect
	titleX, titleY := w.TitleRect().X+3, w.TitleRect().Y

	m.Update(tea.MouseClickMsg(tea.Mouse{X: titleX, Y: titleY, Button: tea.MouseLeft}))
	m.Update(tea.MouseMotionMsg(tea.Mouse{X: titleX + 2, Y: titleY, Button: tea.MouseLeft}))
	m.Update(tea.MouseReleaseMsg(tea.Mouse{X: titleX + 2, Y: titleY, Button: tea.MouseLeft}))

	if m.windows.Layout() != wm.LayoutFloating {
		t.Errorf("layout = %v, want floating after a manual pull", m.windows.Layout())
	}
	if w.State != wm.Normal {
		t.Errorf("state = %v, want the zoom off", w.State)
	}
	if w.Rect == zoom || w.Rect == m.windows.WorkArea() {
		t.Errorf("rect = %+v, want the restored geometry under the cursor", w.Rect)
	}

	other := m.windows.Find("b")
	if other.State != wm.Normal {
		t.Errorf("the window left behind = %v, want normal slotted", other.State)
	}
}

func TestClickOnAMaximizedTitleWithoutMotionKeepsTheZoom(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	w := m.windows.Focused()
	m.windows.ToggleMaximized(w.ID)
	titleX, titleY := w.TitleRect().X+3, w.TitleRect().Y

	m.Update(tea.MouseClickMsg(tea.Mouse{X: titleX, Y: titleY, Button: tea.MouseLeft}))
	m.Update(tea.MouseReleaseMsg(tea.Mouse{X: titleX, Y: titleY, Button: tea.MouseLeft}))

	if w.State != wm.Maximized {
		t.Errorf("state = %v, want maximized after a press without motion", w.State)
	}
}

func TestDragIsIgnoredWhileTiled(t *testing.T) {
	m := newMultiDesktop("a", "b")
	_ = m.Init()
	m.windows.SetLayout(wm.LayoutTiled)

	w := m.windows.Focused()
	before := w.Rect
	title := w.TitleRect()
	titleX, titleY := title.X+1, title.Y

	m.Update(tea.MouseClickMsg(tea.Mouse{X: titleX, Y: titleY, Button: tea.MouseLeft}))
	m.Update(tea.MouseMotionMsg(tea.Mouse{X: titleX + 6, Y: titleY + 2, Button: tea.MouseLeft}))
	m.Update(tea.MouseReleaseMsg(tea.Mouse{X: titleX + 6, Y: titleY + 2, Button: tea.MouseLeft}))

	if w.Rect != before {
		t.Errorf("tiled window moved: %+v, want %+v", w.Rect, before)
	}
}

func TestWorkspaceKeysSwitchAndCount(t *testing.T) {
	m := newMultiDesktop("a", "b")
	_ = m.Init()

	press(t, m, tea.Key{Code: '2', Mod: tea.ModCtrl | tea.ModAlt})
	if m.windows.Active() != 1 {
		t.Fatalf("active workspace = %d, want 1", m.windows.Active())
	}
	if m.windows.Count() != 0 {
		t.Errorf("workspace 2 should be empty, got %d windows", m.windows.Count())
	}

	press(t, m, tea.Key{Code: '1', Mod: tea.ModCtrl | tea.ModAlt})
	if m.windows.Active() != 0 || m.windows.Count() != 2 {
		t.Errorf("back on workspace 1: active %d, count %d", m.windows.Active(), m.windows.Count())
	}
}

func TestMoveToWorkspaceKeyFollowsWindow(t *testing.T) {
	m := newMultiDesktop("a", "b")
	_ = m.Init()

	press(t, m, tea.Key{Code: '3', Mod: tea.ModCtrl | tea.ModAlt | tea.ModShift})

	if m.windows.Active() != 2 {
		t.Fatalf("active workspace = %d, want 2", m.windows.Active())
	}
	if m.windows.Count() != 1 {
		t.Errorf("workspace 3 should hold the moved window, got %d", m.windows.Count())
	}
	if m.windows.Focused() == nil {
		t.Error("focus should follow the moved window")
	}
}

func TestLaunchingAnOpenWindowFollowsItToItsWorkspace(t *testing.T) {
	m := newMultiDesktop("a", "b")
	_ = m.Init()

	if err := m.windows.Focus("a"); err != nil {
		t.Fatalf("Focus: %v", err)
	}
	press(t, m, tea.Key{Code: '2', Mod: tea.ModCtrl | tea.ModAlt | tea.ModShift})
	if m.windows.Active() != 1 {
		t.Fatalf("active workspace = %d, want 1", m.windows.Active())
	}
	press(t, m, tea.Key{Code: '1', Mod: tea.ModCtrl | tea.ModAlt})
	if m.windows.Active() != 0 {
		t.Fatalf("active workspace = %d, want 0", m.windows.Active())
	}

	m.launch("a")

	if m.windows.Active() != 1 {
		t.Errorf("active workspace = %d, want the one holding the window", m.windows.Active())
	}
	if focused := m.windows.Focused(); focused == nil || focused.ID != "a" {
		t.Errorf("focus = %v, want the window that was already open", focused)
	}
	if m.windows.Count() != 1 {
		t.Errorf("launching an open app must not open a second window, got %d", m.windows.Count())
	}
}

func TestMultiAppLaunchesAnotherWindow(t *testing.T) {
	m := newMultiDesktop()
	m.specs["a"] = stubMultiSpec("a")
	m.options.Launch = []string{"a"}
	_ = m.Init()

	if m.windows.Count() != 1 {
		t.Fatalf("count = %d, want one window", m.windows.Count())
	}

	m.launch("a")
	if m.windows.Count() != 2 {
		t.Fatalf("launching a Multi app again = %d windows, want two", m.windows.Count())
	}
	if m.windows.Find("a-2") == nil {
		t.Fatal("the second instance must be named a-2")
	}

	m.launch("a")
	if m.windows.Count() != 3 || m.windows.Find("a-3") == nil {
		t.Fatalf("a third launch gave %d windows, want a-3 among them", m.windows.Count())
	}
}

func TestMultiAppIdsDoNotCollideAfterAClose(t *testing.T) {
	m := newMultiDesktop()
	m.specs["a"] = stubMultiSpec("a")
	m.options.Launch = []string{"a"}
	_ = m.Init()

	m.launch("a")
	m.windows.Close("a-2")
	m.launch("a")

	if m.windows.Find("a-2") == nil {
		t.Fatal("a free id must be reused, the next instance should be a-2 again")
	}
}

func TestLayoutKeyTogglesTiling(t *testing.T) {
	m := newMultiDesktop("a", "b", "c")
	_ = m.Init()

	press(t, m, tea.Key{Code: 't', Mod: tea.ModCtrl | tea.ModAlt})
	if m.windows.Layout() != wm.LayoutTiled {
		t.Fatalf("layout = %s, want tiled", m.windows.Layout())
	}

	windows := m.windows.Windows()
	for i, w := range windows {
		if !m.windows.WorkArea().ContainsRect(w.Rect) {
			t.Errorf("window %s escapes the work area: %+v", w.ID, w.Rect)
		}
		for _, other := range windows[i+1:] {
			if w.Rect.Intersects(other.Rect) {
				t.Errorf("%s and %s overlap", w.ID, other.ID)
			}
		}
	}

	press(t, m, tea.Key{Code: 't', Mod: tea.ModCtrl | tea.ModAlt})
	if m.windows.Layout() != wm.LayoutFloating {
		t.Error("the layout key should toggle back to floating")
	}
}

func TestSnapKeysChainHalvesIntoQuarters(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	w := m.windows.Focused()
	area := m.windows.WorkArea()

	press(t, m, tea.Key{Code: 'h', Mod: tea.ModCtrl | tea.ModAlt})
	if want := wm.ZoneLeft.Rect(area); w.Rect != want {
		t.Fatalf("after snap left = %+v, want %+v", w.Rect, want)
	}

	press(t, m, tea.Key{Code: 'k', Mod: tea.ModCtrl | tea.ModAlt})
	if want := wm.ZoneTopLeft.Rect(area); w.Rect != want {
		t.Errorf("after snap top = %+v, want the top-left quarter %+v", w.Rect, want)
	}
}

func TestSnapChainResetsAfterAManualMove(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	w := m.windows.Focused()

	press(t, m, tea.Key{Code: 'h', Mod: tea.ModCtrl | tea.ModAlt})
	press(t, m, tea.Key{Code: tea.KeyRight, Mod: tea.ModCtrl | tea.ModAlt})
	press(t, m, tea.Key{Code: 'k', Mod: tea.ModCtrl | tea.ModAlt})

	if want := wm.ZoneTop.Rect(m.windows.WorkArea()); w.Rect != want {
		t.Errorf("after moving then snapping top = %+v, want the top half %+v", w.Rect, want)
	}
}

func TestInspectorKeyTogglesOverlay(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	press(t, m, tea.Key{Code: 'd', Mod: tea.ModCtrl | tea.ModAlt})
	if !m.inspector {
		t.Fatal("ctrl+alt+d should open the inspector")
	}
	press(t, m, tea.Key{Code: 'd', Mod: tea.ModCtrl | tea.ModAlt})
	if m.inspector {
		t.Error("ctrl+alt+d again should close the inspector")
	}
}

func TestTabHUDTracksAltTabAndClearsOnNextKey(t *testing.T) {
	m := newMultiDesktop("a", "b")
	_ = m.Init()

	press(t, m, tea.Key{Code: tea.KeyTab, Mod: tea.ModAlt})
	if !m.tabbing {
		t.Fatal("alt-tab should show the window list")
	}

	press(t, m, tea.Key{Code: 'x'})
	if m.tabbing {
		t.Error("the next key should dismiss the window list")
	}
}

// TestTabHUDNumbersWindowsLikeTheTaskbar pins the numbers a user reads in the
// alt-tab list to the numbers alt+1..9 take: the slot is fixed when the window
// opens, so minimizing a window must not renumber the ones after it.
func TestTabHUDNumbersWindowsLikeTheTaskbar(t *testing.T) {
	m := newMultiDesktop("a", "b", "c")
	_ = m.Init()

	windows := m.taskbarWindows()
	m.toggleMinimized(windows[1].ID)
	m.tabbing = true

	frame := ansi.Strip(m.View().Content)
	if strings.Contains(frame, "2  b") {
		t.Errorf("a minimized window must not be listed:\n%s", frame)
	}
	if !strings.Contains(frame, "1  a") || !strings.Contains(frame, "3  c") {
		t.Errorf("the HUD should keep the taskbar slots:\n%s", frame)
	}
}

func TestWindowSlotKeyActivatesAndMinimizes(t *testing.T) {
	m := newMultiDesktop("a", "b")
	_ = m.Init()

	windows := m.taskbarWindows()
	first, second := windows[0], windows[1]
	m.focus(second.ID)

	press(t, m, tea.Key{Code: '1', Mod: tea.ModAlt})
	if m.windows.Focused() != first {
		t.Fatalf("alt+1 focused %v, want the first window", m.windows.Focused())
	}

	press(t, m, tea.Key{Code: '1', Mod: tea.ModAlt})
	if first.State != wm.Minimized {
		t.Error("alt+1 on the focused window should minimize it")
	}
}

func TestWindowSlotKeyRevealsMinimizedWindow(t *testing.T) {
	m := newMultiDesktop("a", "b")
	_ = m.Init()

	first := m.taskbarWindows()[0]
	m.toggleMinimized(first.ID)

	press(t, m, tea.Key{Code: '1', Mod: tea.ModAlt})
	if m.windows.Focused() != first {
		t.Errorf("focus = %v, want the minimized window", m.windows.Focused())
	}
	if first.State == wm.Minimized {
		t.Error("the window behind the slot should come back up")
	}
}

func TestWindowSlotKeyCountsWithinTheWorkspace(t *testing.T) {
	m := newMultiDesktop("a", "b")
	_ = m.Init()

	second := m.taskbarWindows()[1]
	if err := m.windows.MoveToWorkspace(second.ID, 1); err != nil {
		t.Fatalf("move to workspace: %v", err)
	}
	if err := m.windows.SetWorkspace(0); err != nil {
		t.Fatalf("switch workspace: %v", err)
	}

	focused := m.windows.Focused()
	press(t, m, tea.Key{Code: '2', Mod: tea.ModAlt})
	if m.windows.Focused() != focused {
		t.Error("the other workspace's window must not answer for a slot on this one")
	}

	// The window that left is slot 1 of its own workspace, and alt+1 brings it
	// back from there.
	if err := m.windows.SetWorkspace(1); err != nil {
		t.Fatalf("switch workspace: %v", err)
	}
	m.toggleMinimized(second.ID)

	press(t, m, tea.Key{Code: '1', Mod: tea.ModAlt})
	if m.windows.Focused() != second {
		t.Errorf("focus = %v, want the only window of this workspace", m.windows.Focused())
	}
}

func TestWindowSlotKeyIgnoresEmptySlots(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	focused := m.windows.Focused()
	press(t, m, tea.Key{Code: '7', Mod: tea.ModAlt})
	if m.windows.Focused() != focused {
		t.Error("a slot with no window behind it should do nothing")
	}
}

func TestFocusLastKeyReturnsToPreviousWindow(t *testing.T) {
	m := newMultiDesktop("a", "b")
	_ = m.Init()

	windows := m.windows.Windows()
	first, second := windows[0], windows[1]
	m.focus(first.ID)

	press(t, m, tea.Key{Code: 'p', Mod: tea.ModCtrl | tea.ModAlt})
	if m.windows.Focused() != second {
		t.Errorf("focus = %v, want the previously focused window", m.windows.Focused())
	}
}

func TestInspectorOverlayFrame(t *testing.T) {
	m := newTestDesktop()
	m.inspector = true
	assertGolden(t, "desktop-inspector", render(t, m))
}

func TestTabHUDOverlayFrame(t *testing.T) {
	m := newTestDesktop()
	m.tabbing = true
	assertGolden(t, "desktop-tab-hud", render(t, m))
}

// hoverPoint returns a cell on the window's left frame, clear of the corners.
func hoverPoint(w *wm.Window) (int, int) {
	return w.Rect.X, w.Rect.Y + w.Rect.Height/2
}

func TestHoverLightsUpTheResizeEdge(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	w := m.windows.Focused()
	if w == nil {
		t.Fatal("no window to hover")
	}
	x, y := hoverPoint(w)

	m.Update(tea.MouseMotionMsg(tea.Mouse{X: x, Y: y}))

	if m.hover.id != w.ID {
		t.Fatalf("hover = %+v, want it to mark %q", m.hover, w.ID)
	}
	if !m.hover.edges.Has(wm.EdgeLeft) || m.hover.edges.Has(wm.EdgeRight) {
		t.Errorf("hover edges = %v, want the left side only", m.hover.edges)
	}

	_ = m.View()
	accent := theme.Background(m.styles.Theme.Accent)
	if got := m.screen.At(x, y).Background; got != accent {
		t.Errorf("handle background = %q, want the accent %q", got, accent)
	}
	if got := m.screen.At(x, y).Text; got != "↔" {
		t.Errorf("handle glyph = %q, want the horizontal arrow", got)
	}
	foreground := theme.Foreground(m.styles.Theme.Accent)
	if got := m.screen.At(x, w.Rect.Y+1).Style; !strings.Contains(got, foreground) {
		t.Errorf("edge style = %q, want it to carry the accent foreground %q", got, foreground)
	}
}

func TestHoverClearsAwayFromFrames(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	w := m.windows.Focused()
	x, y := hoverPoint(w)

	m.Update(tea.MouseMotionMsg(tea.Mouse{X: x, Y: y}))
	if m.hover.id == "" {
		t.Fatal("expected a hover on the frame")
	}

	m.Update(tea.MouseMotionMsg(tea.Mouse{X: w.Rect.X + 3, Y: w.Rect.Y + 3}))
	if m.hover.id != "" {
		t.Errorf("hover = %+v, want it cleared over the body", m.hover)
	}
}

func TestHoverSkipsTheTitleBar(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	w := m.windows.Focused()
	title := w.TitleRect()

	m.Update(tea.MouseMotionMsg(tea.Mouse{X: title.X + 1, Y: title.Y}))
	if m.hover.id != "" {
		t.Errorf("hover = %+v, want none on the title bar", m.hover)
	}
}

func TestHoverSkipsDesktopOwnedGeometry(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	w := m.windows.Focused()
	m.toggleMaximized(w.ID)

	m.Update(tea.MouseMotionMsg(tea.Mouse{X: w.Rect.X, Y: w.Rect.Y + 2}))
	if m.hover.id != "" {
		t.Errorf("hover = %+v, want none on a maximized window", m.hover)
	}
}

func TestResizeHoverFrame(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()

	w := m.windows.Focused()
	x, y := hoverPoint(w)
	m.Update(tea.MouseMotionMsg(tea.Mouse{X: x, Y: y}))

	assertGolden(t, "desktop-resize-hover", viewFrame(m))
}

func TestDesktopReportsAllMouseMotion(t *testing.T) {
	m := newMultiDesktop("a")
	_ = m.Init()

	if got := m.View().MouseMode; got != tea.MouseModeAllMotion {
		t.Errorf("mouse mode = %v, want all motion so frames light up on hover", got)
	}
}

// press sends a key press and fails the test if the shell panics on it.
func press(t *testing.T, m *Model, key tea.Key) {
	t.Helper()
	model, _ := m.Update(tea.KeyPressMsg(key))
	if model == nil {
		t.Fatalf("Update(%v) returned a nil model", key)
	}
}
