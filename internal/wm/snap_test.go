package wm

import (
	"testing"

	"github.com/dedomorozoff/tuide/internal/geom"
)

func TestZoneHalvesCoverWorkArea(t *testing.T) {
	area := geom.Rect{X: 0, Y: 0, Width: 80, Height: 24}

	left, right := ZoneLeft.Rect(area), ZoneRight.Rect(area)
	if left.Width+right.Width != area.Width {
		t.Errorf("halves width = %d, want %d", left.Width+right.Width, area.Width)
	}
	if left.Height != area.Height || right.Height != area.Height {
		t.Error("halves should keep the full height")
	}
	if right.X != left.Right() {
		t.Error("halves should be adjacent")
	}

	top, bottom := ZoneTop.Rect(area), ZoneBottom.Rect(area)
	if top.Height+bottom.Height != area.Height {
		t.Errorf("halves height = %d, want %d", top.Height+bottom.Height, area.Height)
	}
	if bottom.Y != top.Bottom() {
		t.Error("horizontal halves should be adjacent")
	}
}

func TestZoneQuartersTileTheWorkArea(t *testing.T) {
	area := geom.Rect{X: 0, Y: 0, Width: 80, Height: 24}

	quarters := []Zone{ZoneTopLeft, ZoneTopRight, ZoneBottomLeft, ZoneBottomRight}
	total := 0
	for _, zone := range quarters {
		rect := zone.Rect(area)
		if !area.ContainsRect(rect) {
			t.Errorf("%s escapes the work area: %+v", zone, rect)
		}
		total += rect.Area()
	}

	if total != area.Area() {
		t.Errorf("quarters cover %d cells, want %d", total, area.Area())
	}
}

func TestZoneRespectsWorkAreaOrigin(t *testing.T) {
	area := geom.Rect{X: 10, Y: 5, Width: 60, Height: 20}

	rect := ZoneTopLeft.Rect(area)
	if rect.X != area.X || rect.Y != area.Y {
		t.Errorf("ZoneTopLeft = %+v, want origin %d,%d", rect, area.X, area.Y)
	}

	rect = ZoneBottomRight.Rect(area)
	if rect.Right() != area.Right() || rect.Bottom() != area.Bottom() {
		t.Errorf("ZoneBottomRight = %+v, want to end at %d,%d", rect, area.Right(), area.Bottom())
	}
}

func TestZoneCenterIsSmallerThanWorkArea(t *testing.T) {
	area := geom.Rect{X: 0, Y: 0, Width: 80, Height: 24}

	rect := ZoneCenter.Rect(area)
	if rect.Area() >= area.Area() {
		t.Error("center should be smaller than the work area")
	}
	if !area.ContainsRect(rect) {
		t.Errorf("center escapes the work area: %+v", rect)
	}
}

func TestSnapMovesWindowIntoZone(t *testing.T) {
	m := newManager()
	w, _ := m.Open(spec("a"))

	if err := m.Snap(w.ID, ZoneRight); err != nil {
		t.Fatalf("Snap: %v", err)
	}

	want := ZoneRight.Rect(m.WorkArea())
	if w.Rect != want {
		t.Errorf("snapped rect = %+v, want %+v", w.Rect, want)
	}
	if w.State != Normal {
		t.Errorf("state after snap = %v, want Normal", w.State)
	}
}

func TestSnapIsIgnoredWhenTiled(t *testing.T) {
	m := newManager()
	w, _ := m.Open(spec("a"))
	m.SetLayout(LayoutTiled)
	tiled := w.Rect

	if err := m.Snap(w.ID, ZoneLeft); err != nil {
		t.Fatalf("Snap: %v", err)
	}
	if w.Rect != tiled {
		t.Errorf("snapping should not fight the tiling layout")
	}
}

func TestSnapZoneNoneKeepsGeometry(t *testing.T) {
	m := newManager()
	w, _ := m.Open(spec("a"))
	before := w.Rect

	if err := m.Snap(w.ID, ZoneNone); err != nil {
		t.Fatalf("Snap: %v", err)
	}
	if w.Rect != before {
		t.Errorf("ZoneNone changed the geometry: %+v", w.Rect)
	}
}

func TestSnapUnknownWindow(t *testing.T) {
	m := newManager()
	if err := m.Snap("nope", ZoneLeft); err == nil {
		t.Error("snapping an unknown window should fail")
	}
}

func TestDropZoneEdges(t *testing.T) {
	area := geom.Rect{X: 0, Y: 0, Width: 80, Height: 24}
	const margin = 2

	cases := []struct {
		x, y int
		want Zone
	}{
		{1, 12, ZoneLeft},
		{78, 12, ZoneRight},
		{40, 1, ZoneTop},
		{40, 22, ZoneBottom},
		{1, 1, ZoneTopLeft},
		{78, 1, ZoneTopRight},
		{1, 22, ZoneBottomLeft},
		{78, 22, ZoneBottomRight},
		{40, 12, ZoneNone},
		{-1, 12, ZoneNone},
		{200, 12, ZoneNone},
	}

	for _, c := range cases {
		if got := DropZone(c.x, c.y, area, margin); got != c.want {
			t.Errorf("DropZone(%d, %d) = %s, want %s", c.x, c.y, got, c.want)
		}
	}
}

func TestDropZoneWithoutMargin(t *testing.T) {
	area := geom.Rect{X: 0, Y: 0, Width: 80, Height: 24}
	if got := DropZone(0, 12, area, 0); got != ZoneNone {
		t.Errorf("DropZone with no margin = %s, want none", got)
	}
}

func TestZoneCombine(t *testing.T) {
	cases := []struct {
		name  string
		first Zone
		next  Zone
		want  Zone
	}{
		{"left then top", ZoneLeft, ZoneTop, ZoneTopLeft},
		{"top then left", ZoneTop, ZoneLeft, ZoneTopLeft},
		{"right then top", ZoneRight, ZoneTop, ZoneTopRight},
		{"left then bottom", ZoneLeft, ZoneBottom, ZoneBottomLeft},
		{"bottom then right", ZoneBottom, ZoneRight, ZoneBottomRight},
		{"same half twice", ZoneLeft, ZoneLeft, ZoneLeft},
		{"quarter replaced by half", ZoneTopLeft, ZoneRight, ZoneRight},
		{"half replaced by quarter", ZoneLeft, ZoneTopRight, ZoneTopRight},
		{"center wins", ZoneLeft, ZoneCenter, ZoneCenter},
		{"center is kept", ZoneCenter, ZoneNone, ZoneCenter},
		{"nothing yet", ZoneNone, ZoneTop, ZoneTop},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.first.Combine(c.next); got != c.want {
				t.Errorf("%s.Combine(%s) = %s, want %s", c.first, c.next, got, c.want)
			}
		})
	}
}
