package vt

import (
	"testing"
)

func TestSnapshotRestoresScreenScrollbackAndCursor(t *testing.T) {
	v := New(12, 3)
	v.Write([]byte("\x1b]0;title\x07\x1b[31mred\x1b[0m\r\nplain\r\nlast\r\nscrolled"))

	before := make([]string, 3)
	for y := range before {
		before[y] = v.Line(y)
	}

	restored := New(12, 3)
	restored.Write(v.Snapshot())

	for y, want := range before {
		if got := restored.Line(y); got != want {
			t.Errorf("line %d = %q, want %q", y, got, want)
		}
	}
	if got, want := restored.ScrollbackLen(), v.ScrollbackLen(); got != want {
		t.Errorf("scrollback = %d, want %d", got, want)
	}
	if got := restored.Title(); got != "title" {
		t.Errorf("title = %q, want title", got)
	}
	x1, y1, _, _ := v.CursorAt()
	x2, y2, _, _ := restored.CursorAt()
	if x1 != x2 || y1 != y2 {
		t.Errorf("cursor = %d,%d, want %d,%d", x2, y2, x1, y1)
	}
}

func TestSnapshotKeepsStyles(t *testing.T) {
	v := New(10, 2)
	v.Write([]byte("\x1b[31mred\x1b[0m \x1b[1;34mbold"))

	restored := New(10, 2)
	restored.Write(v.Snapshot())

	for y := range 2 {
		want, got := v.Row(y), restored.Row(y)
		for x := range want {
			if want[x].Text != got[x].Text || want[x].Style != got[x].Style {
				t.Errorf("cell %d,%d = %q/%q, want %q/%q",
					x, y, got[x].Text, got[x].Style, want[x].Text, want[x].Style)
			}
		}
	}
}

func TestSnapshotFlattensTheAlternateScreen(t *testing.T) {
	v := New(10, 2)
	v.Write([]byte("normal"))
	v.Write([]byte("\x1b[?1049hfull"))

	restored := New(10, 2)
	restored.Write(v.Snapshot())

	if restored.Modes().AltScreen {
		t.Error("a restored terminal must not come back in the alternate screen")
	}
	if got := restored.Line(0); got != "full" {
		t.Errorf("Line(0) = %q, want the alternate screen's last frame", got)
	}
}
