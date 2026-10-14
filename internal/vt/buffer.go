package vt

// clear blanks every cell without reallocating the grid.
func (b *Buffer) clear() {
	cell := blank()
	for i := range b.cells {
		b.cells[i] = cell
	}
	b.top = 0
	b.bottom = b.height - 1
	b.cursor = Cursor{Shape: CursorBlock, Visible: true, On: true}
	b.saved = b.cursor
}

// blankTail empties the last rows after the grid has been shifted upwards.
func (b *Buffer) blankTail(rows int) {
	if rows <= 0 {
		return
	}
	cell := blank()
	for y := b.height - rows; y < b.height; y++ {
		for x := range b.width {
			b.cells[y*b.width+x] = cell
		}
	}
}

// dropTop removes rows from the top, shifting the rest up.
func (b *Buffer) dropTop(rows int) {
	if rows <= 0 {
		return
	}
	rows = min(rows, b.height)
	copy(b.cells, b.cells[rows*b.width:])
	b.blankTail(rows)
	b.cursor.Y = max(0, b.cursor.Y-rows)
	b.saved.Y = max(0, b.saved.Y-rows)
	b.top = max(0, b.top-rows)
	b.bottom = max(0, b.bottom-rows)
}

// index moves the cursor down one line, scrolling the region at its bottom.
func (v *VT) index() {
	b := v.active
	if b.cursor.Y == b.bottom {
		v.scrollUp(1)
		return
	}
	if b.cursor.Y < b.height-1 {
		b.cursor.Y++
	}
	v.pendingWrap = false
}

// reverseIndex moves the cursor up one line, scrolling the region at its top.
func (v *VT) reverseIndex() {
	b := v.active
	if b.cursor.Y == b.top {
		v.scrollDown(1)
		return
	}
	if b.cursor.Y > 0 {
		b.cursor.Y--
	}
	v.pendingWrap = false
}

// nextLine is carriage return plus index.
func (v *VT) nextLine() {
	v.active.cursor.X = 0
	v.index()
}

// carriageReturn moves the cursor to the left margin.
func (v *VT) carriageReturn() {
	v.active.cursor.X = 0
	v.pendingWrap = false
}

// lineFeed moves the cursor down, keeping the column.
func (v *VT) lineFeed() { v.index() }

// backspace moves the cursor left without erasing.
func (v *VT) backspace() {
	c := &v.active.cursor
	if c.X > 0 {
		c.X--
	}
	v.pendingWrap = false
}

// tab moves the cursor to the next tab stop.
func (v *VT) tab(stops []int) {
	c := &v.active.cursor
	for _, stop := range stops {
		if stop > c.X {
			c.X = stop
			return
		}
	}
	c.X = v.active.width - 1
	v.pendingWrap = false
}

// scrollUp pushes the scrolling region towards its bottom. Lines leaving the
// top of the live screen land in the scrollback.
func (v *VT) scrollUp(count int) {
	b := v.active
	if count <= 0 || b.top > b.bottom {
		return
	}
	count = min(count, b.bottom-b.top+1)

	if b == v.normal && b.top == 0 && b.bottom == b.height-1 {
		v.trimTop(count, true)
		return
	}

	for i := b.top; i+count <= b.bottom; i++ {
		copy(b.cells[i*b.width:(i+1)*b.width], b.cells[(i+count)*b.width:(i+count+1)*b.width])
	}
	b.blankRows(b.bottom-count+1, count)
}

func (b *Buffer) blankRows(from, count int) {
	cell := blank()
	for y := from; y < from+count && y < b.height; y++ {
		for x := range b.width {
			b.cells[y*b.width+x] = cell
		}
	}
}

// scrollDown pulls the scrolling region towards its top.
func (v *VT) scrollDown(count int) {
	b := v.active
	if count <= 0 || b.top > b.bottom {
		return
	}
	count = min(count, b.bottom-b.top+1)

	for i := b.bottom; i-count >= b.top; i-- {
		copy(b.cells[i*b.width:(i+1)*b.width], b.cells[(i-count)*b.width:(i-count+1)*b.width])
	}
	b.blankRows(b.top, count)
}

// setRegion installs a new scrolling region and parks the cursor home.
func (v *VT) setRegion(top, bottom int) {
	b := v.active
	if top < 0 {
		top = 0
	}
	if bottom >= b.height || bottom < 0 {
		bottom = b.height - 1
	}
	if bottom <= top {
		top, bottom = 0, b.height-1
	}
	b.top, b.bottom = top, bottom
	b.cursor.X = 0
	b.cursor.Y = 0
	if v.modes.Origin {
		b.cursor.Y = top
	}
}

// originTo converts an absolute row into a cursor row, honouring origin mode
// and the scrolling region.
func (v *VT) originTo(row int) int {
	b := v.active
	if v.modes.Origin {
		return min(max(b.top+row, b.top), b.bottom)
	}
	return min(max(row, 0), b.height-1)
}

// putRune prints one rune, honouring insert mode, wrapping and combining marks.
func (v *VT) putRune(r rune) {
	width := runeWidth(r)
	if width == 0 {
		v.appendToLast(r)
		return
	}

	if v.pendingWrap && v.modes.AutoWrap {
		v.active.cursor.X = 0
		v.index()
		v.pendingWrap = false
	}

	if v.active.cursor.X+width > v.active.width {
		if v.modes.AutoWrap {
			v.active.cursor.X = 0
			v.index()
		} else {
			v.active.cursor.X = max(0, v.active.width-width)
		}
	}

	b := v.active
	x := b.cursor.X
	if v.modes.Insert {
		v.insertBlanks(width)
	}

	v.write(x, b.cursor.Y, Cell{Text: string(r), Style: v.styleString(), Width: width})
	if width == 2 {
		v.write(x+1, b.cursor.Y, Cell{Text: "", Style: v.styleString(), Width: 0})
	}

	v.lastRune, v.lastWidth = r, width
	b.cursor.X = x + width
	if b.cursor.X >= b.width {
		b.cursor.X = b.width - 1
		v.pendingWrap = v.modes.AutoWrap
	}
}

// appendToLast glues a combining mark onto the rune before the cursor.
func (v *VT) appendToLast(r rune) {
	b := v.active
	x := b.cursor.X - 1
	if v.pendingWrap {
		x = b.width - 1
	}
	if x < 0 || b.cursor.Y >= b.height {
		return
	}
	index := b.cursor.Y*b.width + x
	for x > 0 && b.cells[index].Width == 0 {
		x--
		index--
	}
	if b.cells[index].Width == 0 {
		return
	}
	b.cells[index].Text += string(r)
}

// write puts one cell, clipping to the buffer.
func (v *VT) write(x, y int, cell Cell) {
	b := v.active
	if x < 0 || y < 0 || x >= b.width || y >= b.height {
		return
	}
	b.cells[y*b.width+x] = cell
}

// insertBlanks opens count cells on the cursor row, pushing the rest right.
func (v *VT) insertBlanks(count int) {
	b := v.active
	if b.cursor.Y < b.top || b.cursor.Y > b.bottom {
		return
	}
	row := b.cursor.Y
	start := row*b.width + b.cursor.X
	end := (row + 1) * b.width
	if count >= end-start {
		b.blankRows(row, 1)
		return
	}
	copy(b.cells[start+count:end], b.cells[start:end-count])
	for i := start; i < start+count; i++ {
		b.cells[i] = blank()
	}
}

// deleteChars removes count cells at the cursor, pulling the rest left.
func (v *VT) deleteChars(count int) {
	b := v.active
	if b.cursor.Y < b.top || b.cursor.Y > b.bottom {
		return
	}
	row := b.cursor.Y
	start := row*b.width + b.cursor.X
	end := (row + 1) * b.width
	if count >= end-start {
		b.blankRows(row, 1)
		return
	}
	copy(b.cells[start:end-count], b.cells[start+count:end])
	for i := end - count; i < end; i++ {
		b.cells[i] = blank()
	}
}

// eraseCells blanks count cells from the cursor to the right.
func (v *VT) eraseCells(count int) {
	b := v.active
	if b.cursor.Y < b.top || b.cursor.Y > b.bottom {
		return
	}
	row := b.cursor.Y
	cell := Cell{Text: " ", Style: v.styleString(), Width: 1}
	for i := range count {
		x := b.cursor.X + i
		if x >= b.width {
			break
		}
		b.cells[row*b.width+x] = cell
	}
}

// insertLines opens count blank lines at the cursor row inside the region.
func (v *VT) insertLines(count int) {
	b := v.active
	if b.cursor.Y < b.top || b.cursor.Y > b.bottom {
		return
	}
	v.reverseIndex()
	v.scrollDown(count)
	v.active.cursor.Y = max(b.top, b.cursor.Y-1)
}

// deleteLines removes count lines at the cursor row inside the region.
func (v *VT) deleteLines(count int) {
	b := v.active
	if b.cursor.Y < b.top || b.cursor.Y > b.bottom {
		return
	}
	v.index()
	v.scrollUp(count)
	v.active.cursor.Y = max(b.top, b.cursor.Y-1)
}

// history is the scrollback ring: fixed-size rows dropped from the oldest end.
type history struct {
	rows   [][]Cell
	cursor int
	limit  int
}

func newHistory(limit int) *history {
	return &history{rows: make([][]Cell, 0, 64), limit: limit}
}

func (h *history) len() int { return len(h.rows) }

// row returns the nth oldest line, or nil when it is out of range.
func (h *history) row(n int) []Cell {
	if n < 0 || n >= len(h.rows) {
		return nil
	}
	return h.rows[(h.cursor+n)%len(h.rows)]
}

func (h *history) push(row []Cell) {
	if h.limit <= 0 {
		return
	}
	if len(h.rows) < h.limit {
		h.rows = append(h.rows, row)
		return
	}
	// Full: overwrite the oldest slot and advance the head.
	h.rows[h.cursor] = row
	h.cursor = (h.cursor + 1) % h.limit
}

// clear drops the whole scrollback.
func (h *history) clear() {
	h.rows = h.rows[:0]
	h.cursor = 0
}
