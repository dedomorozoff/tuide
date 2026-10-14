package compositor

import (
	"strings"
	"testing"

	"github.com/dedomorozoff/tuide/internal/geom"
)

func TestBlitCellsPaintsAPreparedRow(t *testing.T) {
	s := New(6, 2)
	cells := []Cell{
		{Text: "h", Width: 1},
		{Text: "i", Width: 1},
		{Text: "!", Style: "\x1b[31m", Width: 1},
	}
	s.BlitCells(geom.Rect{Width: 6, Height: 1}, cells)

	for x, want := range []string{"h", "i", "!"} {
		if got := s.At(x, 0).Text; got != want {
			t.Errorf("column %d = %q, want %q", x, got, want)
		}
	}
	if style := s.At(2, 0).Style; style != "\x1b[31m" {
		t.Fatalf("style at column 2 = %q, want the red one", style)
	}
	if s.Row(1) != strings.Repeat(" ", 6) {
		t.Fatalf("the row below = %q, want it untouched", s.Row(1))
	}
}

func TestBlitCellsStopsAtTheRightEdge(t *testing.T) {
	s := New(4, 1)
	cells := []Cell{
		{Text: "a", Width: 1},
		{Text: "b", Width: 1},
		{Text: "c", Width: 1},
		{Text: "d", Width: 1},
		{Text: "e", Width: 1},
	}
	s.BlitCells(geom.Rect{Width: 4, Height: 1}, cells)

	assertRow(t, s, 0, "abcd")
}

func TestBlitCellsSkipsTheClippedLeftEdge(t *testing.T) {
	s := New(6, 1)
	cells := []Cell{
		{Text: "x", Width: 1},
		{Text: "a", Width: 1},
		{Text: "b", Width: 1},
	}
	s.BlitCells(geom.Rect{X: -1, Width: 6, Height: 1}, cells)

	assertRow(t, s, 0, "ab")
}

func TestBlitCellsReservesWideCharacters(t *testing.T) {
	s := New(6, 1)
	cells := []Cell{
		{Text: "界", Width: 2},
		{Text: "x", Width: 1},
	}
	s.BlitCells(geom.Rect{Width: 6, Height: 1}, cells)

	if cell := s.At(1, 0); cell.Width != 0 || cell.Text != "" {
		t.Fatalf("continuation cell = %+v, want an empty cell of width 0", cell)
	}
	if cell := s.At(2, 0); cell.Text != "x" {
		t.Fatalf("cell after a wide rune = %+v, want x", cell)
	}
}

func TestBlitCellsClipsToTheScreen(t *testing.T) {
	s := New(4, 2)
	cells := []Cell{
		{Text: "a", Width: 1},
		{Text: "b", Width: 1},
	}
	s.BlitCells(geom.Rect{Y: 5, Width: 4, Height: 1}, cells)

	assertRow(t, s, 0, "")
	assertRow(t, s, 1, "")
}

func TestBlitCellsIgnoresAnEmptyRectangle(t *testing.T) {
	s := New(4, 1)
	s.BlitCells(geom.Rect{Width: 0, Height: 1}, []Cell{{Text: "a", Width: 1}})

	assertRow(t, s, 0, "")
}
