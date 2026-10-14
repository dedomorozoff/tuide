package shell

import (
	"testing"

	tea "charm.land/bubbletea/v2"

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
	titleX, titleY := w.Rect.X+2, w.Rect.Y+1

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
	titleX, titleY := w.Rect.X+2, w.Rect.Y+1

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
	titleX, titleY := w.Rect.X+2, w.Rect.Y+1
	middle := m.windows.WorkArea().Width / 2

	m.Update(tea.MouseClickMsg(tea.Mouse{X: titleX, Y: titleY, Button: tea.MouseLeft}))
	m.Update(tea.MouseMotionMsg(tea.Mouse{X: middle, Y: titleY, Button: tea.MouseLeft}))
	m.Update(tea.MouseReleaseMsg(tea.Mouse{X: middle, Y: titleY, Button: tea.MouseLeft}))

	if w.Rect.Width != 24 || w.Rect.Height != 10 {
		t.Errorf("window should keep its size when dropped away from an edge: %+v", w.Rect)
	}
}

func TestDragIsIgnoredWhileTiled(t *testing.T) {
	m := newMultiDesktop("a", "b")
	_ = m.Init()
	m.windows.SetLayout(wm.LayoutTiled)

	w := m.windows.Focused()
	before := w.Rect
	titleX, titleY := w.Rect.X+2, w.Rect.Y+1

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

// press sends a key press and fails the test if the shell panics on it.
func press(t *testing.T, m *Model, key tea.Key) {
	t.Helper()
	model, _ := m.Update(tea.KeyPressMsg(key))
	if model == nil {
		t.Fatalf("Update(%v) returned a nil model", key)
	}
}
