package vt

import (
	"fmt"
	"strings"
)

// snapshotScrollback bounds how much history a snapshot carries: enough that a
// restored terminal reads like the one that was left behind, without writing
// megabytes of scrollback on every session save.
const snapshotScrollback = 1000

// Snapshot renders the terminal as an ANSI stream. Written into a fresh
// terminal of the same size it brings back the scrollback, the screen, the
// cursor and the title.
//
// Modes are left out on purpose: the program that set them is gone, and the
// shell that takes the window over asks for its own. The alternate screen is
// flattened the same way, so a restored full-screen program shows its last
// frame on the normal screen instead of trapping the new shell in the
// alternate one.
func (v *VT) Snapshot() []byte {
	var b strings.Builder
	b.WriteString("\x1b[0m\x1b[2J\x1b[H")

	rows := make([][]Cell, 0, v.height+snapshotScrollback)
	if v.active == v.normal {
		rows = append(rows, v.historyTail(snapshotScrollback)...)
	}
	for y := range v.height {
		rows = append(rows, v.liveRow(y))
	}

	for i, row := range rows {
		if i > 0 {
			b.WriteString("\r\n")
		}
		writeSnapshotRow(&b, row)
	}

	x, y, _, _ := v.CursorAt()
	fmt.Fprintf(&b, "\x1b[%d;%dH", min(y+1, v.height), min(x+1, v.width))
	if v.title != "" {
		fmt.Fprintf(&b, "\x1b]0;%s\x07", cleanTitle(v.title))
	}
	return []byte(b.String())
}

// liveRow returns the cells of the live screen row y, regardless of where the
// viewport is scrolled.
func (v *VT) liveRow(y int) []Cell {
	if y < 0 || y >= v.height {
		return nil
	}
	start := y * v.width
	return v.active.cells[start : start+v.width]
}

// historyTail returns the newest n scrollback lines, oldest first.
func (v *VT) historyTail(n int) [][]Cell {
	total := v.history.len()
	start := max(0, total-n)
	out := make([][]Cell, 0, total-start)
	for i := start; i < total; i++ {
		out = append(out, v.history.row(i))
	}
	return out
}

// writeSnapshotRow emits one row as styled text, trimming the trailing blanks
// a clear screen would show anyway.
func writeSnapshotRow(b *strings.Builder, row []Cell) {
	last := -1
	for i, cell := range row {
		if cell.Style != "" || (cell.Text != "" && cell.Text != " ") {
			last = i
		}
	}

	style := ""
	for i := 0; i <= last; i++ {
		cell := row[i]
		if cell.Text == "" {
			continue
		}
		if cell.Style != style {
			if cell.Style == "" {
				b.WriteString("\x1b[0m")
			} else {
				b.WriteString(cell.Style)
			}
			style = cell.Style
		}
		b.WriteString(cell.Text)
	}
	if style != "" {
		b.WriteString("\x1b[0m")
	}
}

// cleanTitle drops the control bytes a title could smuggle into the stream.
func cleanTitle(title string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, title)
}
