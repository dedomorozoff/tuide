package wm

import (
	"testing"

	"github.com/dedomorozoff/tuide/internal/geom"
)

func TestWorkspacesStartOnTheFirstOne(t *testing.T) {
	m := newManager()

	if got := m.Active(); got != 0 {
		t.Errorf("Active() = %d, want 0", got)
	}
	if got := Workspaces; got < 2 {
		t.Fatalf("Workspaces = %d, want at least 2 to be useful", got)
	}
}

func TestSetWorkspaceHidesOtherWindows(t *testing.T) {
	m := newManager()
	if _, err := m.Open(spec("a")); err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := m.SetWorkspace(1); err != nil {
		t.Fatalf("SetWorkspace: %v", err)
	}
	if m.Count() != 0 {
		t.Errorf("Count() on an empty workspace = %d, want 0", m.Count())
	}
	if m.Focused() != nil {
		t.Error("an empty workspace should have no focused window")
	}
	if got := m.HitTest(5, 5); got != nil {
		t.Error("windows from other workspaces must not be hit-testable")
	}

	if err := m.SetWorkspace(0); err != nil {
		t.Fatalf("SetWorkspace: %v", err)
	}
	if m.Count() != 1 {
		t.Errorf("Count() back on the first workspace = %d, want 1", m.Count())
	}
	if m.Focused() == nil {
		t.Error("returning to a workspace should focus its top window")
	}
}

func TestSetWorkspaceRejectsOutOfRange(t *testing.T) {
	m := newManager()

	if err := m.SetWorkspace(-1); err == nil {
		t.Error("negative workspace should fail")
	}
	if err := m.SetWorkspace(Workspaces); err == nil {
		t.Error("workspace past the last one should fail")
	}
}

func TestMoveToWorkspaceKeepsFocusLocal(t *testing.T) {
	m := newManager()
	a, _ := m.Open(spec("a"))
	b, _ := m.Open(spec("b"))

	if err := m.MoveToWorkspace(a.ID, 2); err != nil {
		t.Fatalf("MoveToWorkspace: %v", err)
	}
	if a.Workspace != 2 {
		t.Errorf("window workspace = %d, want 2", a.Workspace)
	}
	if m.Count() != 1 {
		t.Errorf("Count() = %d, want the one remaining window", m.Count())
	}
	if m.Focused() != b {
		t.Errorf("focus = %v, want the window that stayed", m.Focused())
	}
}

func TestFindSeesEveryWorkspace(t *testing.T) {
	m := newManager()
	a, _ := m.Open(spec("a"))
	if err := m.MoveToWorkspace(a.ID, 3); err != nil {
		t.Fatalf("MoveToWorkspace: %v", err)
	}

	if m.Find(a.ID) != a {
		t.Error("Find should look across all workspaces")
	}
}

func TestRevealFollowsTheWindowToItsWorkspace(t *testing.T) {
	m := newManager()
	a, _ := m.Open(spec("a"))
	b, _ := m.Open(spec("b"))
	_ = b

	if err := m.MoveToWorkspace(a.ID, 4); err != nil {
		t.Fatalf("MoveToWorkspace: %v", err)
	}
	if m.Active() != 0 {
		t.Fatalf("Active() = %d, want the manager to stay on the first workspace", m.Active())
	}

	if err := m.Reveal(a.ID); err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	if m.Active() != 4 {
		t.Errorf("Active() = %d, want the revealed window's workspace", m.Active())
	}
	if m.Focused() != a {
		t.Error("Reveal should focus the window it was asked for")
	}
}

func TestRevealStaysPutWhenTheWindowIsAlreadyHere(t *testing.T) {
	m := newManager()
	a, _ := m.Open(spec("a"))
	if _, err := m.Open(spec("b")); err != nil {
		t.Fatalf("Open b: %v", err)
	}

	if err := m.Reveal(a.ID); err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	if m.Active() != 0 {
		t.Errorf("Active() = %d, want the workspace unchanged", m.Active())
	}
	if m.Focused() != a {
		t.Errorf("focus = %v, want %v", m.Focused(), a)
	}
}

func TestRevealRejectsUnknownWindow(t *testing.T) {
	m := newManager()
	if err := m.Reveal("nope"); err == nil {
		t.Error("revealing a window that does not exist should fail")
	}
}

func TestToggleMinimizedSkipsToAnotherWorkspaceWindow(t *testing.T) {
	m := newManager()
	a, _ := m.Open(spec("a"))
	b, _ := m.Open(spec("b"))

	if err := m.ToggleMinimized(b.ID); err != nil {
		t.Fatalf("ToggleMinimized: %v", err)
	}
	if m.Focused() != a {
		t.Error("focus should fall back to the other visible window")
	}
}

func TestFocusLastReturnsToPreviousWindow(t *testing.T) {
	m := newManager()
	a, _ := m.Open(spec("a"))
	b, _ := m.Open(spec("b"))

	if err := m.Focus(a.ID); err != nil {
		t.Fatalf("Focus: %v", err)
	}
	if !m.FocusLast() {
		t.Fatal("FocusLast returned false with a previous window available")
	}
	if m.Focused() != b {
		t.Errorf("focus = %v, want the previously focused window", m.Focused())
	}
}

func TestFocusLastIgnoresOtherWorkspaces(t *testing.T) {
	m := newManager()
	a, _ := m.Open(spec("a"))
	b, _ := m.Open(spec("b"))

	if err := m.Focus(a.ID); err != nil {
		t.Fatalf("Focus: %v", err)
	}
	if err := m.MoveToWorkspace(b.ID, 1); err != nil {
		t.Fatalf("MoveToWorkspace: %v", err)
	}

	if m.FocusLast() {
		t.Error("FocusLast should not jump to another workspace")
	}
}

func TestFocusHistoryForgetsClosedWindows(t *testing.T) {
	m := newManager()
	a, _ := m.Open(spec("a"))
	b, _ := m.Open(spec("b"))
	_ = b

	if err := m.Focus(a.ID); err != nil {
		t.Fatalf("Focus: %v", err)
	}
	_ = m.Close(b.ID)

	if m.FocusLast() {
		t.Error("a closed window must not stay in the focus history")
	}
}

func TestCycleStaysInsideWorkspace(t *testing.T) {
	m := newManager()
	a, _ := m.Open(spec("a"))
	b, _ := m.Open(spec("b"))
	_ = b

	if err := m.MoveToWorkspace(a.ID, 1); err != nil {
		t.Fatalf("MoveToWorkspace: %v", err)
	}
	m.Cycle(1)

	if m.Focused() != b {
		t.Error("cycle should only walk the active workspace")
	}
}

func TestTiledLayoutFillsWorkAreaWithoutOverlap(t *testing.T) {
	m := newManager()
	for _, id := range []string{"a", "b", "c"} {
		if _, err := m.Open(spec(id)); err != nil {
			t.Fatalf("Open %s: %v", id, err)
		}
	}
	m.SetLayout(LayoutTiled)

	windows := m.Windows()
	if len(windows) != 3 {
		t.Fatalf("windows = %d, want 3", len(windows))
	}

	total := 0
	for i, w := range windows {
		if !m.WorkArea().ContainsRect(w.Rect) {
			t.Errorf("window %s escapes the work area: %+v", w.ID, w.Rect)
		}
		total += w.Rect.Area()
		for _, other := range windows[i+1:] {
			if w.Rect.Intersects(other.Rect) {
				t.Errorf("%s and %s overlap in tiled mode", w.ID, other.ID)
			}
		}
	}
	if total != m.WorkArea().Area() {
		t.Errorf("tiling covers %d cells, want %d", total, m.WorkArea().Area())
	}
}

func TestTiledLayoutKeepsMasterStableWhenRaising(t *testing.T) {
	m := newManager()
	a, _ := m.Open(spec("a"))
	b, _ := m.Open(spec("b"))
	if _, err := m.Open(spec("c")); err != nil {
		t.Fatalf("Open: %v", err)
	}
	m.SetLayout(LayoutTiled)
	master := a.Rect

	if err := m.Focus(b.ID); err != nil {
		t.Fatalf("Focus: %v", err)
	}
	if a.Rect != master {
		t.Errorf("master moved after raising another window: %+v", a.Rect)
	}
}

func TestLeavingTiledLayoutRestoresFloatingGeometry(t *testing.T) {
	m := newManager()
	a, _ := m.Open(spec("a"))
	floating := a.Rect

	m.SetLayout(LayoutTiled)
	if a.Rect == floating {
		t.Fatal("tiled layout should have changed the geometry")
	}

	m.SetLayout(LayoutFloating)
	if a.Rect != floating {
		t.Errorf("geometry after tiling = %+v, want %+v", a.Rect, floating)
	}
}

func TestTiledModeIgnoresManualMove(t *testing.T) {
	m := newManager()
	w, _ := m.Open(spec("a"))
	m.SetLayout(LayoutTiled)
	tiled := w.Rect

	if err := m.Move(w.ID, geom.Rect{X: 40, Y: 10, Width: 20, Height: 10}); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if w.Rect != tiled {
		t.Error("tiled windows should not be moved by hand")
	}
}

func TestTiledLayoutRearrangesWhenWindowCloses(t *testing.T) {
	m := newManager()
	a, _ := m.Open(spec("a"))
	b, _ := m.Open(spec("b"))
	m.SetLayout(LayoutTiled)

	_ = m.Close(b.ID)

	if a.Rect != m.WorkArea() {
		t.Errorf("the last window should fill the work area, got %+v", a.Rect)
	}
}

func TestTiledLayoutSkipsMinimizedWindows(t *testing.T) {
	m := newManager()
	a, _ := m.Open(spec("a"))
	b, _ := m.Open(spec("b"))
	m.SetLayout(LayoutTiled)

	if err := m.ToggleMinimized(b.ID); err != nil {
		t.Fatalf("ToggleMinimized: %v", err)
	}
	if a.Rect != m.WorkArea() {
		t.Errorf("minimized windows must not hold a tile: %+v", a.Rect)
	}
}

func TestWindowOpenedWhileTiledGetsFloatingGeometryBack(t *testing.T) {
	m := newManager()
	if _, err := m.Open(spec("a")); err != nil {
		t.Fatalf("Open a: %v", err)
	}
	m.SetLayout(LayoutTiled)

	b, err := m.Open(spec("b"))
	if err != nil {
		t.Fatalf("Open b: %v", err)
	}
	if b.Floating.Empty() {
		t.Fatal("a window opened while tiled should still remember a floating rect")
	}

	m.SetLayout(LayoutFloating)
	if b.Rect != b.Floating {
		t.Errorf("floating rect after untile = %+v, want %+v", b.Rect, b.Floating)
	}
	if b.Rect.Empty() {
		t.Error("untiling should not collapse the window")
	}
}

func TestMinimizeInTiledModeClearsMaximizedState(t *testing.T) {
	m := newManager()
	w, _ := m.Open(spec("a"))
	w.State = Maximized

	if err := m.ToggleMinimized(w.ID); err != nil {
		t.Fatalf("ToggleMinimized: %v", err)
	}
	if w.State != Minimized {
		t.Errorf("state = %v, want Minimized", w.State)
	}
}

func TestMintLayoutSingleWindowFillsArea(t *testing.T) {
	area := geom.Rect{X: 2, Y: 3, Width: 40, Height: 20}
	rects := mintLayout(area, 1)

	if len(rects) != 1 || rects[0] != area {
		t.Errorf("mintLayout(area, 1) = %+v, want the area itself", rects)
	}
}

func TestMintLayoutCoversAreaExactly(t *testing.T) {
	area := geom.Rect{Width: 81, Height: 23}

	for count := 1; count <= 6; count++ {
		rects := mintLayout(area, count)
		if len(rects) != count {
			t.Fatalf("mintLayout(area, %d) returned %d rects", count, len(rects))
		}

		total := 0
		seen := make(map[geom.Rect]bool)
		for _, rect := range rects {
			if rect.Empty() {
				t.Fatalf("count %d produced an empty rect", count)
			}
			if !area.ContainsRect(rect) {
				t.Fatalf("count %d produced %+v outside %+v", count, rect, area)
			}
			if seen[rect] {
				t.Fatalf("count %d produced duplicate rect %+v", count, rect)
			}
			seen[rect] = true
			total += rect.Area()
		}
		if total != area.Area() {
			t.Errorf("count %d covers %d cells, want %d", count, total, area.Area())
		}
	}
}

func TestRegionAt(t *testing.T) {
	w := &Window{
		Rect: geom.Rect{X: 5, Y: 5, Width: 20, Height: 10},
	}

	cases := []struct {
		x, y int
		want Region
	}{
		{5, 5, RegionBorder},
		{24, 5, RegionBorder},
		{5, 14, RegionBorder},
		{24, 14, RegionBorder},
		{10, 5, RegionTitle},
		{10, 6, RegionContent},
		{6, 6, RegionContent},
		{10, 7, RegionContent},
		{4, 5, RegionNone},
		{200, 200, RegionNone},
	}

	for _, c := range cases {
		if got := w.RegionAt(c.x, c.y); got != c.want {
			t.Errorf("RegionAt(%d, %d) = %d, want %d", c.x, c.y, got, c.want)
		}
	}
}

func TestEdgesAt(t *testing.T) {
	w := &Window{Rect: geom.Rect{X: 5, Y: 5, Width: 20, Height: 10}}

	cases := []struct {
		name string
		x, y int
		want Edges
	}{
		{"left", 5, 8, EdgeLeft},
		{"right", 24, 8, EdgeRight},
		{"top", 10, 5, EdgeTop},
		{"bottom", 10, 14, EdgeBottom},
		{"top-left corner", 5, 5, EdgeLeft | EdgeTop},
		{"bottom-right corner", 24, 14, EdgeRight | EdgeBottom},
		{"inside", 10, 8, 0},
		{"outside", 0, 0, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := w.EdgesAt(c.x, c.y); got != c.want {
				t.Errorf("EdgesAt(%d, %d) = %04b, want %04b", c.x, c.y, got, c.want)
			}
		})
	}
}

func TestTitleRect(t *testing.T) {
	w := &Window{Rect: geom.Rect{X: 5, Y: 5, Width: 20, Height: 10}}

	want := geom.Rect{X: 6, Y: 5, Width: 18, Height: 1}
	if got := w.TitleRect(); got != want {
		t.Errorf("TitleRect() = %+v, want %+v", got, want)
	}
}
