package wm

import (
	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/geom"
)

// State is where a window sits in the stack.
type State int

const (
	// Normal is a regular floating or tiled window.
	Normal State = iota
	// Maximized fills the work area.
	Maximized
	// Minimized is hidden from the work area but still listed in the taskbar.
	Minimized
)

// Window is one application plus the geometry the window manager gives it.
type Window struct {
	ID   string
	Spec appsdk.Spec
	App  appsdk.App

	// Rect is where the window is right now.
	Rect geom.Rect
	// Restore is the geometry to go back to when unmaximizing.
	Restore geom.Rect
	// Floating is the geometry to go back to when tiling is switched off.
	Floating geom.Rect

	State       State
	Workspace   int
	Opened      int
	TitleHeight int
}

// ContentRect is the area an app is allowed to draw into: inside the border,
// below the title bar.
func (w *Window) ContentRect() geom.Rect {
	return geom.Rect{
		X:      w.Rect.X + 1,
		Y:      w.Rect.Y + 1 + w.TitleHeight,
		Width:  w.Rect.Width - 2,
		Height: w.Rect.Height - 2 - w.TitleHeight,
	}
}

// TitleRect is the row the window manager draws the title on.
func (w *Window) TitleRect() geom.Rect {
	return geom.Rect{
		X:      w.Rect.X + 1,
		Y:      w.Rect.Y + 1,
		Width:  max(0, w.Rect.Width-2),
		Height: max(0, w.TitleHeight),
	}
}

// Context builds the frame context for the current geometry.
func (w *Window) Context(focused bool) appsdk.Context {
	return appsdk.Context{Rect: w.ContentRect(), Focused: focused}
}

// Region is which part of a window a cell belongs to.
type Region int

const (
	// RegionNone means the cell is outside the window.
	RegionNone Region = iota
	// RegionTitle is the title bar, which drags the window.
	RegionTitle
	// RegionBorder is the frame, which resizes the window.
	RegionBorder
	// RegionContent is the body, which belongs to the app.
	RegionContent
)

// RegionAt reports which part of the window the cell falls into.
func (w *Window) RegionAt(x, y int) Region {
	if !w.Rect.Contains(x, y) {
		return RegionNone
	}
	if x == w.Rect.X || x == w.Rect.Right()-1 ||
		y == w.Rect.Y || y == w.Rect.Bottom()-1 {
		return RegionBorder
	}
	if y >= w.Rect.Y+1 && y < w.Rect.Y+1+w.TitleHeight {
		return RegionTitle
	}
	return RegionContent
}

// Edges is a bit set of the sides of a window.
type Edges uint8

const (
	// EdgeLeft is the left side.
	EdgeLeft Edges = 1 << iota
	// EdgeRight is the right side.
	EdgeRight
	// EdgeTop is the top side.
	EdgeTop
	// EdgeBottom is the bottom side.
	EdgeBottom
)

// Empty reports whether no side is set.
func (e Edges) Empty() bool { return e == 0 }

// Has reports whether a side is part of the set.
func (e Edges) Has(edge Edges) bool { return e&edge != 0 }

// EdgesAt returns the sides that should resize when dragging the cell.
// Corners set two edges at once. Zero means the cell is not a resize handle.
func (w *Window) EdgesAt(x, y int) Edges {
	if !w.Rect.Contains(x, y) {
		return 0
	}

	var edges Edges
	switch x {
	case w.Rect.X:
		edges |= EdgeLeft
	case w.Rect.Right() - 1:
		edges |= EdgeRight
	}
	switch y {
	case w.Rect.Y:
		edges |= EdgeTop
	case w.Rect.Bottom() - 1:
		edges |= EdgeBottom
	}
	return edges
}
