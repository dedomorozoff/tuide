package wm

import (
	"sort"

	"github.com/dedomorozoff/tuide/internal/geom"
)

// Layout is how a workspace arranges its windows.
type Layout int

const (
	// LayoutFloating lets windows keep the geometry the user gave them.
	LayoutFloating Layout = iota
	// LayoutTiled keeps one master window beside a stack of the rest.
	LayoutTiled
)

func (l Layout) String() string {
	if l == LayoutTiled {
		return "tiled"
	}
	return "floating"
}

// Next returns the other layout.
func (l Layout) Next() Layout {
	if l == LayoutTiled {
		return LayoutFloating
	}
	return LayoutTiled
}

// arrange lays every visible window of the active workspace out according to
// the current layout.
func (m *Manager) arrange() {
	if m.layout != LayoutTiled {
		return
	}

	// A zoomed window folds out of the grid and takes the whole work area;
	// the windows behind it keep tiling the space they had.
	var zoomed, slotted []*Window
	for _, w := range tilingOrder(m.visible()) {
		if w.State == Maximized {
			zoomed = append(zoomed, w)
		} else {
			slotted = append(slotted, w)
		}
	}

	rects := mintLayout(m.workArea, len(slotted))
	for i, w := range slotted {
		if i >= len(rects) {
			break
		}
		w.Rect = rects[i]
		w.State = Normal
	}
	for _, w := range zoomed {
		w.Rect = m.workArea
	}
}

// tilingOrder returns windows sorted by the order they were opened, which is
// stable: raising a window must not shuffle the tiling.
func tilingOrder(windows []*Window) []*Window {
	ordered := make([]*Window, len(windows))
	copy(ordered, windows)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Opened < ordered[j].Opened
	})
	return ordered
}

// mintLayout computes the master-stack geometry for n windows inside area.
// The master keeps the left half, the stack shares the right half.
func mintLayout(area geom.Rect, count int) []geom.Rect {
	if count <= 0 || area.Empty() {
		return nil
	}
	if count == 1 {
		return []geom.Rect{area}
	}

	half := area.Width / 2
	master := geom.Rect{X: area.X, Y: area.Y, Width: half, Height: area.Height}
	stackArea := geom.Rect{
		X:      area.X + half,
		Y:      area.Y,
		Width:  area.Width - half,
		Height: area.Height,
	}

	rects := make([]geom.Rect, 0, count)
	rects = append(rects, master)

	stackCount := count - 1
	base := stackArea.Height / stackCount
	extra := stackArea.Height % stackCount

	y := stackArea.Y
	for i := 0; i < stackCount; i++ {
		height := base
		if i < extra {
			height++
		}
		rects = append(rects, geom.Rect{
			X:      stackArea.X,
			Y:      y,
			Width:  stackArea.Width,
			Height: height,
		})
		y += height
	}

	return rects
}
