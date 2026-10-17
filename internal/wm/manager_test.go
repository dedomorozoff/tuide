package wm

import (
	"testing"

	"charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/geom"
)

type fakeApp struct{ closed bool }

func (a *fakeApp) Init() tea.Cmd                        { return nil }
func (a *fakeApp) Update(tea.Msg) (appsdk.App, tea.Cmd) { return a, nil }
func (a *fakeApp) View(appsdk.Context) tea.View         { return tea.View{} }
func (a *fakeApp) Close() tea.Cmd                       { a.closed = true; return nil }

func spec(id string) appsdk.Spec {
	return appsdk.Spec{
		ID:            id,
		Name:          id,
		DefaultWidth:  20,
		DefaultHeight: 10,
		MinWidth:      5,
		MinHeight:     3,
		Launch:        func() appsdk.App { return &fakeApp{} },
	}
}

func newManager() *Manager {
	return New(geom.Rect{Width: 80, Height: 24})
}

func TestOpenAddsAndFocuses(t *testing.T) {
	m := newManager()

	w, err := m.Open(spec("a"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if m.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", m.Count())
	}
	if m.Focused() != w {
		t.Errorf("Focused() = %v, want the new window", m.Focused())
	}
	if w.Rect.Empty() {
		t.Error("new window has empty geometry")
	}
}

func TestOpenRejectsDuplicateID(t *testing.T) {
	m := newManager()
	if _, err := m.Open(spec("a")); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := m.Open(spec("a")); err == nil {
		t.Fatal("second Open with the same id should fail")
	}
}

func TestOpenRejectsInvalidSpec(t *testing.T) {
	m := newManager()
	bad := spec("b")
	bad.Launch = nil

	if _, err := m.Open(bad); err == nil {
		t.Fatal("Open without a launch function should fail")
	}
}

func TestFocusRaisesWindow(t *testing.T) {
	m := newManager()
	a, _ := m.Open(spec("a"))
	b, _ := m.Open(spec("b"))

	if err := m.Focus(a.ID); err != nil {
		t.Fatalf("Focus: %v", err)
	}

	windows := m.Windows()
	if windows[len(windows)-1] != a {
		t.Error("focused window should be on top")
	}
	if m.Focused() != a || m.Focused() == b {
		t.Error("Focused() should return the raised window")
	}
}

func TestFocusUnknownWindow(t *testing.T) {
	m := newManager()
	if err := m.Focus("nope"); err == nil {
		t.Fatal("Focus on unknown id should fail")
	}
}

func TestCycleWrapsAround(t *testing.T) {
	m := newManager()
	a, _ := m.Open(spec("a"))
	_, _ = m.Open(spec("b"))
	_, _ = m.Open(spec("c"))

	m.Cycle(1)
	if got := m.Focused().ID; got != "a" {
		t.Fatalf("Cycle forward wrapped to %q, want a", got)
	}

	m.Cycle(1)
	if got := m.Focused().ID; got != "b" {
		t.Fatalf("Cycle forward focused %q, want b", got)
	}

	m.Cycle(-1)
	if got := m.Focused().ID; got != "a" {
		t.Fatalf("Cycle backward focused %q, want a", got)
	}
	if m.Find(a.ID) == nil {
		t.Fatal("Cycle should not lose windows")
	}
}

func TestCloseFallsBackToWindowBelow(t *testing.T) {
	m := newManager()
	a, _ := m.Open(spec("a"))
	top, _ := m.Open(spec("b"))

	cmd := m.Close(top.ID)
	if cmd != nil {
		t.Error("Close should return the app teardown command")
	}
	if !top.App.(*fakeApp).closed {
		t.Error("Close should call App.Close")
	}
	if m.Focused() != a {
		t.Errorf("focus after close = %q, want a", m.Focused().ID)
	}
}

func TestCloseLastWindowClearsFocus(t *testing.T) {
	m := newManager()
	w, _ := m.Open(spec("a"))
	m.Close(w.ID)

	if m.Focused() != nil {
		t.Error("focus should be empty when no windows remain")
	}
}

func TestHitTestReturnsTopmost(t *testing.T) {
	m := newManager()
	_, _ = m.Open(spec("back"))
	front, _ := m.Open(spec("front"))

	if got := m.HitTest(front.Rect.X+1, front.Rect.Y+1); got != front {
		t.Errorf("HitTest returned %q, want the topmost window %q", got.ID, front.ID)
	}
}

func TestHitTestMissesBackground(t *testing.T) {
	m := newManager()
	if _, err := m.Open(spec("a")); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := m.HitTest(0, 23); got != nil {
		t.Errorf("HitTest outside any window = %v, want nil", got)
	}
}

func TestMoveClampsIntoWorkArea(t *testing.T) {
	m := newManager()
	w, _ := m.Open(spec("a"))

	if err := m.Move(w.ID, geom.Rect{X: 70, Y: 20, Width: 40, Height: 20}); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if !m.WorkArea().ContainsRect(w.Rect) {
		t.Errorf("window rect %+v escapes the work area %+v", w.Rect, m.WorkArea())
	}
}

func TestMoveRespectsMinimumSize(t *testing.T) {
	m := newManager()
	w, _ := m.Open(spec("a"))

	if err := m.Move(w.ID, geom.Rect{Width: 1, Height: 1}); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if w.Rect.Width != 5 || w.Rect.Height != 3 {
		t.Errorf("window size = %dx%d, want at least 5x3", w.Rect.Width, w.Rect.Height)
	}
}

func TestToggleMaximizedRoundTrip(t *testing.T) {
	m := newManager()
	w, _ := m.Open(spec("a"))
	restored := w.Rect

	if err := m.ToggleMaximized(w.ID); err != nil {
		t.Fatalf("ToggleMaximized: %v", err)
	}
	if w.Rect != m.WorkArea() {
		t.Errorf("maximized rect = %+v, want the work area %+v", w.Rect, m.WorkArea())
	}

	if err := m.ToggleMaximized(w.ID); err != nil {
		t.Fatalf("ToggleMaximized: %v", err)
	}
	if w.Rect != restored {
		t.Errorf("restored rect = %+v, want %+v", w.Rect, restored)
	}
}

func TestToggleMinimizedHidesFromHitTest(t *testing.T) {
	m := newManager()
	w, _ := m.Open(spec("a"))

	if err := m.ToggleMinimized(w.ID); err != nil {
		t.Fatalf("ToggleMinimized: %v", err)
	}
	if got := m.HitTest(w.Rect.X+1, w.Rect.Y+1); got != nil {
		t.Error("minimized window should not be hit-testable")
	}
	if m.Focused() != nil {
		t.Error("focus should move away from a minimized window")
	}

	if err := m.ToggleMinimized(w.ID); err != nil {
		t.Fatalf("ToggleMinimized: %v", err)
	}
	if m.Focused() != w {
		t.Error("restoring a minimized window should focus it")
	}
}

func TestSetWorkAreaKeepsWindowsVisible(t *testing.T) {
	m := newManager()
	w, _ := m.Open(spec("a"))

	m.SetWorkArea(geom.Rect{Width: 30, Height: 10})

	if !m.WorkArea().ContainsRect(w.Rect) {
		t.Errorf("window rect %+v escapes the new work area %+v", w.Rect, m.WorkArea())
	}
}

func TestContentRectLeavesRoomForChrome(t *testing.T) {
	w := &Window{Rect: geom.Rect{X: 5, Y: 5, Width: 20, Height: 10}}

	want := geom.Rect{X: 6, Y: 6, Width: 18, Height: 8}
	if got := w.ContentRect(); got != want {
		t.Errorf("ContentRect() = %+v, want %+v", got, want)
	}
}
