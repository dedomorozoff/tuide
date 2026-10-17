package compositor

import (
	"strings"

	"charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/geom"
)

// Screen is a grid of cells that windows, panels and overlays are blitted into.
// Painting order is the caller's z-order: whatever is drawn last wins.
type Screen struct {
	width  int
	height int
	cells  []Cell
}

// New returns an empty screen of the given size.
func New(width, height int) *Screen {
	s := &Screen{}
	s.resize(max(0, width), max(0, height))
	return s
}

func (s *Screen) resize(width, height int) {
	s.width = width
	s.height = height
	s.cells = make([]Cell, width*height)
	for i := range s.cells {
		s.cells[i] = Blank()
	}
}

// Width is the screen width in cells.
func (s *Screen) Width() int { return s.width }

// Height is the screen height in cells.
func (s *Screen) Height() int { return s.height }

// Size is the screen size in cells.
func (s *Screen) Size() (width, height int) { return s.width, s.height }

// Bounds is the full screen rectangle.
func (s *Screen) Bounds() geom.Rect {
	return geom.Rect{X: 0, Y: 0, Width: s.width, Height: s.height}
}

// Resize changes the screen size, dropping all content.
func (s *Screen) Resize(width, height int) {
	s.resize(max(0, width), max(0, height))
}

// Clear resets every cell to blank without reallocating the grid, so the
// per-frame cost stays flat.
func (s *Screen) Clear() {
	blank := Blank()
	for i := range s.cells {
		s.cells[i] = blank
	}
}

// At returns the cell at the given position, or Blank when out of bounds.
func (s *Screen) At(x, y int) Cell {
	if !s.Bounds().Contains(x, y) {
		return Blank()
	}
	return s.cells[y*s.width+x]
}

// Set writes a single cell, clipping to the screen.
func (s *Screen) Set(x, y int, cell Cell) {
	if !s.Bounds().Contains(x, y) {
		return
	}
	s.cells[y*s.width+x] = cell
}

// Fill paints every cell of the rectangle with a blank cell carrying the given
// background, clipping to the screen. Used for window bodies and panel
// backgrounds.
func (s *Screen) Fill(rect geom.Rect, background string) {
	area := rect.Intersect(s.Bounds())
	cell := Cell{Background: background, Text: " ", Width: 1}
	for y := area.Y; y < area.Bottom(); y++ {
		for x := area.X; x < area.Right(); x++ {
			s.cells[y*s.width+x] = cell
		}
	}
}

// Blit draws styled text with its top-left corner at the rectangle origin.
// Content is truncated to the rectangle and clipped to the screen, so callers
// never have to measure anything.
func (s *Screen) Blit(rect geom.Rect, content string) {
	area := rect.Intersect(s.Bounds())
	if area.Empty() {
		return
	}

	for row, cells := range cellRows(content, area.Height) {
		y := rect.Y + row
		if y < area.Y || y >= area.Bottom() {
			continue
		}
		x := rect.X
		for _, cell := range cells {
			if x+cell.Width > area.Right() {
				break
			}
			if x >= area.X {
				s.blit(x, y, cell)
			}
			x += cell.Width
		}
	}
}

// BlitCells paints a prepared row of cells starting at the rectangle origin.
// The terminal uses it to hand over a whole screen row without a string ever
// being built for it, so the rectangle is one row tall. Cells past the right
// edge are dropped; anything clipped off the left is skipped.
func (s *Screen) BlitCells(rect geom.Rect, cells []Cell) {
	if rect.Height <= 0 {
		return
	}
	area := geom.Rect{
		X:      max(rect.X, 0),
		Y:      rect.Y,
		Width:  min(rect.X+rect.Width, s.width) - max(rect.X, 0),
		Height: 1,
	}
	if area.Width <= 0 {
		return
	}

	x := rect.X
	for _, cell := range cells {
		if x >= area.X+area.Width {
			break
		}
		if x >= area.X {
			s.write(x, rect.Y, cell)
		}
		x += max(1, cell.Width)
	}
}

// blit paints one string cell without letting it drop the background beneath.
// A cell that names no background of its own — an app that styles only a
// foreground, or the padding lipgloss leaves unstyled — keeps the background
// already at that cell, so text lands on the window body instead of the host
// terminal's default colour. The caller guarantees the coordinates are on
// screen.
func (s *Screen) blit(x, y int, cell Cell) {
	if cell.Background == "" {
		cell.Background = s.cells[y*s.width+x].Background
	}
	s.write(x, y, cell)
}

// write paints a grapheme and reserves the remaining cells of wide characters.
// A prepared cell replaces the one it covers outright, background included:
// the terminal hands over whole rows and already decided every colour.
func (s *Screen) write(x, y int, cell Cell) {
	s.Set(x, y, cell)
	continuation := Cell{Style: cell.Style, Background: cell.Background, Text: "", Width: 0}
	for offset := 1; offset < cell.Width; offset++ {
		s.Set(x+offset, y, continuation)
	}
}

// BlitView draws a tea.View using only its content.
func (s *Screen) BlitView(rect geom.Rect, view tea.View) {
	s.Blit(rect, view.Content)
}

// Row renders a single screen row.
func (s *Screen) Row(y int) string {
	if y < 0 || y >= s.height {
		return ""
	}
	return joinRow(s.cells[y*s.width : (y+1)*s.width])
}

// String renders the whole screen.
func (s *Screen) String() string {
	rows := make([]string, s.height)
	for y := range rows {
		rows[y] = s.Row(y)
	}
	return strings.Join(rows, "\n")
}
