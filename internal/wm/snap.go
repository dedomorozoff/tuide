package wm

import "github.com/dedomorozoff/tuide/internal/geom"

// Zone is a place a window can be snapped to. Halves split along one axis,
// quarters along both, center keeps a comfortable floating size.
type Zone int

const (
	ZoneNone Zone = iota
	ZoneLeft
	ZoneRight
	ZoneTop
	ZoneBottom
	ZoneTopLeft
	ZoneTopRight
	ZoneBottomLeft
	ZoneBottomRight
	ZoneCenter
)

// Zones lists every usable snap target in a stable order.
func Zones() []Zone {
	return []Zone{
		ZoneLeft, ZoneRight, ZoneTop, ZoneBottom,
		ZoneTopLeft, ZoneTopRight, ZoneBottomLeft, ZoneBottomRight,
		ZoneCenter,
	}
}

func (z Zone) String() string {
	switch z {
	case ZoneLeft:
		return "left"
	case ZoneRight:
		return "right"
	case ZoneTop:
		return "top"
	case ZoneBottom:
		return "bottom"
	case ZoneTopLeft:
		return "top-left"
	case ZoneTopRight:
		return "top-right"
	case ZoneBottomLeft:
		return "bottom-left"
	case ZoneBottomRight:
		return "bottom-right"
	case ZoneCenter:
		return "center"
	}
	return "none"
}

// Rect returns the geometry a zone occupies inside area.
func (z Zone) Rect(area geom.Rect) geom.Rect {
	if area.Empty() {
		return geom.Rect{}
	}

	halfWidth := area.Width / 2
	halfHeight := area.Height / 2

	left := geom.Rect{X: area.X, Y: area.Y, Width: halfWidth, Height: area.Height}
	right := geom.Rect{X: area.X + halfWidth, Y: area.Y, Width: area.Width - halfWidth, Height: area.Height}
	top := geom.Rect{X: area.X, Y: area.Y, Width: area.Width, Height: halfHeight}
	bottom := geom.Rect{X: area.X, Y: area.Y + halfHeight, Width: area.Width, Height: area.Height - halfHeight}

	topLeft := geom.Rect{X: area.X, Y: area.Y, Width: halfWidth, Height: halfHeight}
	topRight := geom.Rect{X: area.X + halfWidth, Y: area.Y, Width: area.Width - halfWidth, Height: halfHeight}
	bottomLeft := geom.Rect{X: area.X, Y: area.Y + halfHeight, Width: halfWidth, Height: area.Height - halfHeight}
	bottomRight := geom.Rect{
		X:      area.X + halfWidth,
		Y:      area.Y + halfHeight,
		Width:  area.Width - halfWidth,
		Height: area.Height - halfHeight,
	}

	switch z {
	case ZoneLeft:
		return left
	case ZoneRight:
		return right
	case ZoneTop:
		return top
	case ZoneBottom:
		return bottom
	case ZoneTopLeft:
		return topLeft
	case ZoneTopRight:
		return topRight
	case ZoneBottomLeft:
		return bottomLeft
	case ZoneBottomRight:
		return bottomRight
	case ZoneCenter:
		return centered(area, 2, 3)
	}
	return area
}

// centered builds a rectangle holding num/den of the area, centred.
func centered(area geom.Rect, num, den int) geom.Rect {
	width := area.Width * num / den
	height := area.Height * num / den
	return geom.Rect{
		X:      area.X + (area.Width-width)/2,
		Y:      area.Y + (area.Height-height)/2,
		Width:  width,
		Height: height,
	}
}

func horizontal(z Zone) (left bool, ok bool) {
	switch z {
	case ZoneLeft:
		return true, true
	case ZoneRight:
		return false, true
	}
	return false, false
}

func vertical(z Zone) (top bool, ok bool) {
	switch z {
	case ZoneTop:
		return true, true
	case ZoneBottom:
		return false, true
	}
	return false, false
}

func corner(left, top bool) Zone {
	switch {
	case left && top:
		return ZoneTopLeft
	case !left && top:
		return ZoneTopRight
	case left && !top:
		return ZoneBottomLeft
	}
	return ZoneBottomRight
}

// Combine returns the zone a window ends up in when it is snapped to z and
// then to other. Two perpendicular halves produce the quarter where they meet;
// anything else simply replaces the previous zone.
func (z Zone) Combine(other Zone) Zone {
	if other == ZoneNone {
		return z
	}
	if z == ZoneNone || z == other {
		return other
	}

	left, zHorizontal := horizontal(z)
	top, zVertical := vertical(z)

	if otherLeft, ok := horizontal(other); zVertical && ok {
		return corner(otherLeft, top)
	}
	if otherTop, ok := vertical(other); zHorizontal && ok {
		return corner(left, otherTop)
	}

	return other
}

// DropZone maps a cell to the zone a dragged window would land in when
// released. Only the margin cells nearest an edge count, so the middle of the
// screen keeps windows floating.
func DropZone(x, y int, area geom.Rect, margin int) Zone {
	if area.Empty() || !area.Contains(x, y) || margin <= 0 {
		return ZoneNone
	}

	nearLeft := x < area.X+margin
	nearRight := x >= area.Right()-margin
	nearTop := y < area.Y+margin
	nearBottom := y >= area.Bottom()-margin

	switch {
	case nearLeft && nearTop:
		return ZoneTopLeft
	case nearRight && nearTop:
		return ZoneTopRight
	case nearLeft && nearBottom:
		return ZoneBottomLeft
	case nearRight && nearBottom:
		return ZoneBottomRight
	case nearLeft:
		return ZoneLeft
	case nearRight:
		return ZoneRight
	case nearTop:
		return ZoneTop
	case nearBottom:
		return ZoneBottom
	}
	return ZoneNone
}
