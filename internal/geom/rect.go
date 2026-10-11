package geom

// Rect is a rectangle in terminal cells. Zero Width or Height means empty.
type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Empty reports whether the rectangle covers no cells.
func (r Rect) Empty() bool {
	return r.Width <= 0 || r.Height <= 0
}

// Right is the exclusive right edge.
func (r Rect) Right() int {
	return r.X + r.Width
}

// Bottom is the exclusive bottom edge.
func (r Rect) Bottom() int {
	return r.Y + r.Height
}

// Area is the number of cells covered.
func (r Rect) Area() int {
	if r.Empty() {
		return 0
	}
	return r.Width * r.Height
}

// Contains reports whether the cell is inside the rectangle.
func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.Right() && y >= r.Y && y < r.Bottom()
}

// Intersects reports whether the two rectangles share at least one cell.
func (r Rect) Intersects(other Rect) bool {
	if r.Empty() || other.Empty() {
		return false
	}
	return r.X < other.Right() && other.X < r.Right() &&
		r.Y < other.Bottom() && other.Y < r.Bottom()
}

// Intersect returns the overlapping region, empty if they do not overlap.
func (r Rect) Intersect(other Rect) Rect {
	x := max(r.X, other.X)
	y := max(r.Y, other.Y)
	right := min(r.Right(), other.Right())
	bottom := min(r.Bottom(), other.Bottom())
	if right <= x || bottom <= y {
		return Rect{}
	}
	return Rect{X: x, Y: y, Width: right - x, Height: bottom - y}
}

// ContainsRect reports whether other fits entirely inside r.
func (r Rect) ContainsRect(other Rect) bool {
	if other.Empty() {
		return true
	}
	if r.Empty() {
		return false
	}
	return other.X >= r.X && other.Y >= r.Y &&
		other.Right() <= r.Right() && other.Bottom() <= r.Bottom()
}

// ClampTo moves and shrinks the rectangle so it fits entirely inside bounds,
// never violating minWidth and minHeight where possible.
func (r Rect) ClampTo(bounds Rect, minWidth, minHeight int) Rect {
	clamped := r

	if clamped.Width < minWidth {
		clamped.Width = minWidth
	}
	if clamped.Height < minHeight {
		clamped.Height = minHeight
	}
	if clamped.Width > bounds.Width {
		clamped.Width = bounds.Width
	}
	if clamped.Height > bounds.Height {
		clamped.Height = bounds.Height
	}
	if clamped.Width < minWidth {
		clamped.Width = min(minWidth, bounds.Width)
	}
	if clamped.Height < minHeight {
		clamped.Height = min(minHeight, bounds.Height)
	}

	if bounds.Width <= 0 || bounds.Height <= 0 {
		return Rect{}
	}

	if clamped.Right() > bounds.Right() {
		clamped.X = bounds.Right() - clamped.Width
	}
	if clamped.Bottom() > bounds.Bottom() {
		clamped.Y = bounds.Bottom() - clamped.Height
	}

	clamped.X = max(clamped.X, bounds.X)
	clamped.Y = max(clamped.Y, bounds.Y)

	return clamped
}

// WithSize returns a rectangle with the same origin and the given size.
func (r Rect) WithSize(width, height int) Rect {
	return Rect{X: r.X, Y: r.Y, Width: width, Height: height}
}

// Inset shrinks the rectangle by the given amount on every side.
func (r Rect) Inset(dx, dy int) Rect {
	return Rect{
		X:      r.X + dx,
		Y:      r.Y + dy,
		Width:  r.Width - 2*dx,
		Height: r.Height - 2*dy,
	}
}
