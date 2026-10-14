package vt

import "strings"

// CellPos is a position in the visible window: row zero is the top row on
// screen, counting back into the scrollback when the viewport has been moved.
type CellPos struct {
	X int
	Y int
}

// Selection is the range the user dragged over. The anchor is where the press
// landed and the head is where the pointer is now; the two may be in any
// order, and an equal pair means a plain click rather than a selection.
type Selection struct {
	anchor CellPos
	head   CellPos
	active bool
}

// BeginSelection starts a drag at a cell.
func (v *VT) BeginSelection(x, y int) {
	v.sel = Selection{anchor: CellPos{X: x, Y: y}, active: true}
}

// ExtendSelection moves the end of a drag. A click on another window or on
// empty space is ignored so a stray motion cannot wipe the selection.
func (v *VT) ExtendSelection(x, y int) {
	if !v.sel.active {
		return
	}
	v.sel.head = CellPos{X: x, Y: y}
}

// ClearSelection drops the selection and forgets that a drag was in flight.
func (v *VT) ClearSelection() { v.sel = Selection{} }

// Selecting reports whether a drag is in flight right now.
func (v *VT) Selecting() bool { return v.sel.active }

// HasSelection reports whether the drag actually covers any cells.
func (v *VT) HasSelection() bool {
	if !v.sel.active {
		return false
	}
	return v.sel.anchor != v.sel.head
}

// spans returns the selection as an ordered pair of positions. The end is
// inclusive, so a selection of one cell has equal ends.
func (v *VT) spans() (from, to CellPos) {
	from, to = v.sel.anchor, v.sel.head
	if from.Y > to.Y || (from.Y == to.Y && from.X > to.X) {
		from, to = to, from
	}
	return from, to
}

// rowSpan is which cells of one viewport row are selected. An empty end past
// the last row means the row is not part of the selection at all.
type rowSpan struct {
	from int
	to   int
	set  bool
}

func (s rowSpan) contains(index int) bool {
	return s.set && index >= s.from && index <= s.to
}

// selectionOn reports the selected cells of viewport row y.
func (v *VT) selectionOn(y int) rowSpan {
	if !v.HasSelection() {
		return rowSpan{}
	}
	from, to := v.spans()
	if y < from.Y || y > to.Y {
		return rowSpan{}
	}

	span := rowSpan{from: 0, to: v.width - 1, set: true}
	if y == from.Y {
		span.from = max(0, from.X)
	}
	if y == to.Y {
		span.to = min(v.width-1, to.X)
	}
	return span
}

// SelectedText is the text the selection covers, one line per screen row, with
// trailing blanks dropped. A selection that runs past the end of a row keeps
// the line break, which is what makes copying a command line work.
func (v *VT) SelectedText() string {
	if !v.HasSelection() {
		return ""
	}

	from, to := v.spans()
	lines := make([]string, 0, to.Y-from.Y+1)
	for y := from.Y; y <= to.Y; y++ {
		span := v.selectionOn(y)
		cells := v.visibleRow(y)
		if len(cells) == 0 {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, rowText(cells, span.from, min(span.to, len(cells)-1)))
	}

	// A drag that ends past the last column swallowed the trailing blanks of
	// every row but the last; drop the empty tail they leave behind.
	for len(lines) > 1 && lines[len(lines)-1] == "" {
		last := len(lines) - 1
		span := v.selectionOn(from.Y + last)
		if span.to < v.width-1 {
			break
		}
		lines = lines[:last]
	}
	return strings.Join(lines, "\n")
}

// rowText renders one row of cells between two columns, trimming the blanks a
// terminal keeps after the last visible character.
func rowText(cells []Cell, from, to int) string {
	from = max(0, from)
	to = min(to, len(cells)-1)
	if from > to {
		return ""
	}

	var b strings.Builder
	for _, cell := range cells[from : to+1] {
		switch {
		case cell.Width == 0:
		case cell.Text == "":
			b.WriteByte(' ')
		default:
			b.WriteString(cell.Text)
		}
	}
	return strings.TrimRight(b.String(), " ")
}
