package compositor

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/dedomorozoff/tuide/internal/geom"
)

func TestScreenBlitPlainText(t *testing.T) {
	s := New(10, 3)
	s.Blit(geom.Rect{Width: 10, Height: 2}, "ab\ncd")

	assertRow(t, s, 0, "ab")
	assertRow(t, s, 1, "cd")
	assertRow(t, s, 2, "")
}

func TestScreenBlitTruncatesToRect(t *testing.T) {
	s := New(10, 2)
	s.Blit(geom.Rect{Width: 4, Height: 1}, "abcdefgh")

	assertRow(t, s, 0, "abcd")
}

func TestScreenBlitDropsOverflowRows(t *testing.T) {
	s := New(6, 2)
	s.Blit(geom.Rect{Width: 6, Height: 1}, "one\ntwo\nthree")

	assertRow(t, s, 0, "one")
	assertRow(t, s, 1, "")
}

func TestScreenBlitClipsToScreen(t *testing.T) {
	s := New(5, 1)
	s.Blit(geom.Rect{X: 3, Y: 0, Width: 10, Height: 1}, "xy")

	assertRow(t, s, 0, "   xy")
}

func TestScreenBlitIgnoresOutOfBounds(t *testing.T) {
	s := New(3, 1)
	s.Blit(geom.Rect{X: 10, Y: 10, Width: 4, Height: 1}, "abc")

	assertRow(t, s, 0, "")
}

func TestScreenOverwriteDropsPreviousStyle(t *testing.T) {
	s := New(6, 1)
	s.Blit(geom.Rect{Width: 6, Height: 1}, "\x1b[31mred")
	s.Blit(geom.Rect{Width: 3, Height: 1}, "abc")

	assertRow(t, s, 0, "abc")
}

func TestScreenKeepsStylePerCell(t *testing.T) {
	s := New(3, 1)
	s.Blit(geom.Rect{Width: 3, Height: 1}, "\x1b[31ma\x1b[32mb\x1b[0mc")

	row := s.Row(0)
	if got := s.At(0, 0).Style; got != "\x1b[31m" {
		t.Errorf("cell 0 style = %q, want %q", got, "\x1b[31m")
	}
	if got := s.At(1, 0).Style; got != "\x1b[32m" {
		t.Errorf("cell 1 style = %q, want %q", got, "\x1b[32m")
	}
	if got := s.At(2, 0).Style; got != "" {
		t.Errorf("cell 2 style = %q, want empty after reset", got)
	}
	if !strings.Contains(row, "\x1b[0m") {
		t.Errorf("row %q should reset the style at the end", row)
	}
}

func TestScreenFillAppliesStyle(t *testing.T) {
	s := New(2, 1)
	s.Fill(geom.Rect{Width: 2, Height: 1}, "\x1b[41m")

	if got := s.At(0, 0).Background; got != "\x1b[41m" {
		t.Errorf("cell background = %q, want the fill background", got)
	}
}

func TestScreenBlitKeepsTheBackgroundUnderTheText(t *testing.T) {
	s := New(6, 1)
	s.Fill(geom.Rect{Width: 6, Height: 1}, "\x1b[44m")
	s.Blit(geom.Rect{Width: 3, Height: 1}, "abc")

	for x := range 6 {
		if got := s.At(x, 0).Background; got != "\x1b[44m" {
			t.Fatalf("cell %d background = %q, want the fill background", x, got)
		}
	}
}

func TestScreenBlitKeepsTheBackgroundOfStyledPadding(t *testing.T) {
	s := New(6, 1)
	s.Fill(geom.Rect{Width: 6, Height: 1}, "\x1b[44m")
	s.Blit(geom.Rect{Width: 6, Height: 1}, "\x1b[31ma\x1b[0m  ")

	if got := s.At(1, 0).Background; got != "\x1b[44m" {
		t.Errorf("padding after a reset kept background %q, want the fill background", got)
	}
}

func TestScreenBlitLetsACellNameItsOwnBackground(t *testing.T) {
	s := New(3, 1)
	s.Fill(geom.Rect{Width: 3, Height: 1}, "\x1b[44m")
	s.Blit(geom.Rect{Width: 3, Height: 1}, "\x1b[41mabc")

	for x := range 3 {
		if got := s.At(x, 0).Background; got != "\x1b[41m" {
			t.Fatalf("cell %d background = %q, want the blitted one", x, got)
		}
	}
}

func TestScreenWideRunesOccupyTwoCells(t *testing.T) {
	s := New(4, 1)
	s.Blit(geom.Rect{Width: 4, Height: 1}, "日本")

	if got := s.At(0, 0).Width; got != 2 {
		t.Errorf("cell 0 width = %d, want 2", got)
	}
	if got := s.At(1, 0).Width; got != 0 {
		t.Errorf("cell 1 width = %d, want 0 for a wide continuation", got)
	}
	if got := s.At(2, 0).Width; got != 2 {
		t.Errorf("cell 2 width = %d, want 2", got)
	}
}

func TestScreenWideRuneTruncation(t *testing.T) {
	s := New(3, 1)
	s.Blit(geom.Rect{Width: 3, Height: 1}, "日本")

	assertRow(t, s, 0, "日")
}

func TestScreenResizeClearsContent(t *testing.T) {
	s := New(4, 1)
	s.Blit(geom.Rect{Width: 4, Height: 1}, "abcd")
	s.Resize(4, 2)

	assertRow(t, s, 0, "")
	if got := s.Height(); got != 2 {
		t.Errorf("Height() = %d, want 2", got)
	}
}

func TestScreenAtOutOfBounds(t *testing.T) {
	s := New(2, 2)
	if got := s.At(9, 9); got.Text != " " || got.Width != 1 {
		t.Errorf("At(9,9) = %+v, want Blank", got)
	}
}

func TestScreenBounds(t *testing.T) {
	s := New(7, 3)
	want := geom.Rect{Width: 7, Height: 3}
	if got := s.Bounds(); got != want {
		t.Errorf("Bounds() = %+v, want %+v", got, want)
	}
}

func assertRow(t *testing.T, s *Screen, y int, want string) {
	t.Helper()
	got := s.Row(y)
	want = padTo(want, s.Width())
	if got != want {
		t.Errorf("Row(%d) = %q, want %q", y, got, want)
	}
}

func padTo(s string, width int) string {
	if gap := width - ansi.StringWidth(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}
