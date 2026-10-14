// Package vt is the virtual terminal behind the terminal window: a VT100 and
// xterm-256color state machine over a grid of styled cells that the compositor
// blits directly.
package vt

import (
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/dedomorozoff/tuide/internal/compositor"
	"github.com/dedomorozoff/tuide/internal/geom"
)

// CursorShape is how the cursor is drawn over its cell.
type CursorShape int

const (
	// CursorBlock is a filled reverse-video block.
	CursorBlock CursorShape = iota
	// CursorUnderline is a line under the cell.
	CursorUnderline
	// CursorBar is a thin line at the left edge.
	CursorBar
)

// DefaultScrollback is how many lines of history a terminal keeps.
const DefaultScrollback = 5000

// BlinkPeriod is how often a blinking cursor toggles.
const BlinkPeriod = 500 * time.Millisecond

// Cell is one character cell of the screen. Style is the SGR prefix that was
// active when the cell was written, which is exactly what the compositor wants,
// so nothing has to be re-rendered on the way out.
type Cell struct {
	Text  string
	Style string
	Width int
}

// blank is a cleared cell in the default style.
func blank() Cell { return Cell{Text: " ", Width: 1} }

// Cursor is where the next character goes and how it is marked.
type Cursor struct {
	X       int
	Y       int
	Shape   CursorShape
	Visible bool
	Blink   bool
	On      bool
}

// Buffer is one screen grid: either the normal buffer with its scrollback or
// the alternate buffer full-screen programs draw into.
type Buffer struct {
	cells  []Cell
	width  int
	height int

	// top and bottom bound the scrolling region, inclusive.
	top    int
	bottom int

	cursor Cursor
	saved  Cursor
}

// MouseMode is how the program wants mouse reports encoded.
type MouseMode int

const (
	// MouseOff means the program does not want mouse reports.
	MouseOff MouseMode = iota
	// MouseX10 reports presses only, in the legacy X10 encoding.
	MouseX10
	// MouseButton reports presses and releases.
	MouseButton
	// MouseAny reports presses, releases and motion while dragging.
	MouseAny
	// MouseSGR is MouseAny with the unambiguous SGR encoding.
	MouseSGR
	// MouseURXVT is the urxvt encoding of the same events.
	MouseURXVT
)

// Modes is the terminal state a program changes with SM and RM. The host needs
// it to encode keys and mouse events the way the program expects.
type Modes struct {
	AppCursor      bool
	AppKeypad      bool
	BracketedPaste bool
	AltScreen      bool
	Mouse          MouseMode
	FocusEvents    bool
	Origin         bool
	Insert         bool
	AutoWrap       bool
	ReverseVideo   bool
}

// VT is the emulator. It is not safe for concurrent use: the terminal app owns
// one and drives it from the Bubble Tea update loop.
type VT struct {
	normal *Buffer
	alt    *Buffer
	active *Buffer

	width  int
	height int

	history *history
	offset  int

	title       string
	modes       Modes
	sgr         rendition
	sgrCache    string
	sel         Selection
	pendingWrap bool

	// lastRune is the character most recently printed, for the REP control.
	lastRune  rune
	lastWidth int

	parser parser

	// replies are bytes the emulator owes the program: device attributes,
	// cursor reports, colour queries.
	replies []byte

	clipboard string

	// scratch is the compositor cell row reused by Blit so a frame costs no
	// allocations.
	scratch []compositor.Cell

	// OnBell fires on the BEL character.
	OnBell func()
	// OnTitle fires when the program sets the window title.
	OnTitle func(string)
	// OnClipboard fires when the program writes the clipboard with OSC 52.
	OnClipboard func(string)
}

// New returns a terminal of the given size in cells.
func New(width, height int) *VT {
	width, height = max(1, width), max(1, height)

	v := &VT{
		normal:  newBuffer(width, height),
		alt:     newBuffer(width, height),
		history: newHistory(DefaultScrollback),
		width:   width,
		height:  height,
	}
	v.active = v.normal
	v.modes.AutoWrap = true
	v.resetSGR()
	return v
}

func newBuffer(width, height int) *Buffer {
	b := &Buffer{
		cells:  make([]Cell, width*height),
		width:  width,
		height: height,
		top:    0,
		bottom: height - 1,
		cursor: Cursor{Shape: CursorBlock, Visible: true, On: true},
	}
	b.clear()
	return b
}

// Size is the terminal size in cells.
func (v *VT) Size() (width, height int) { return v.width, v.height }

// Title is the last title the program set.
func (v *VT) Title() string { return v.title }

// Clipboard is the last payload the program put on the clipboard.
func (v *VT) Clipboard() string { return v.clipboard }

// Modes returns the current mode flags.
func (v *VT) Modes() Modes { return v.modes }

// CursorAt returns the cursor position and shape.
func (v *VT) CursorAt() (x, y int, shape CursorShape, visible bool) {
	c := v.active.cursor
	return c.X, c.Y, c.Shape, c.Visible
}

// AdvanceBlink flips both cursors between the two halves of a blink cycle.
func (v *VT) AdvanceBlink(on bool) {
	v.normal.cursor.On = on
	v.alt.cursor.On = on
}

// NormalCursorOn reports which half of the blink cycle the cursor is in.
func (v *VT) NormalCursorOn() bool { return v.normal.cursor.On }

// ScrollbackLen is how many lines of history are stored.
func (v *VT) ScrollbackLen() int { return v.history.len() }

// Offset is how far the viewport is scrolled up from the live screen.
func (v *VT) Offset() int { return v.offset }

// AtBottom reports whether the viewport shows the live screen.
func (v *VT) AtBottom() bool { return v.offset == 0 }

// ScrollUp moves the viewport into the history, stopping at the oldest line.
func (v *VT) ScrollUp(lines int) {
	if lines > 0 {
		v.offset = min(v.offset+lines, v.history.len())
	}
}

// ScrollDown moves the viewport back towards the live screen.
func (v *VT) ScrollDown(lines int) {
	if lines > 0 {
		v.offset = max(0, v.offset-lines)
	}
}

// ScrollToBottom snaps the viewport back to the live screen.
func (v *VT) ScrollToBottom() { v.offset = 0 }

// TakeReplies returns the bytes the emulator owes the program and clears them.
func (v *VT) TakeReplies() []byte {
	if len(v.replies) == 0 {
		return nil
	}
	out := v.replies
	v.replies = nil
	return out
}

// Write feeds output from the child process into the emulator. New output
// invalidates a selection: what was highlighted is no longer what is on screen.
func (v *VT) Write(data []byte) {
	v.sel = Selection{}
	for _, b := range data {
		v.parseByte(b)
	}
}

// Resize changes the terminal size. Lines that no longer fit move into the
// scrollback instead of disappearing.
func (v *VT) Resize(width, height int) {
	width, height = max(1, width), max(1, height)
	if width == v.width && height == v.height {
		return
	}
	if height < v.height {
		v.trimTop(v.height-height, false)
	}
	v.width, v.height = width, height

	v.resizeBuffer(v.normal)
	v.resizeBuffer(v.alt)
	v.offset = 0
}

// trimTop archives the topmost rows of the live screen into the history. The
// scrolling region and the cursor are put back where they were: rows leaving
// the top must not shrink the region or move the cursor.
func (v *VT) trimTop(count int, keepCursor bool) {
	count = min(count, v.height)
	for y := range count {
		v.history.push(append([]Cell(nil), v.normal.cells[y*v.width:(y+1)*v.width]...))
	}

	top, bottom := v.normal.top, v.normal.bottom
	anchor := v.normal.cursor.Y
	v.normal.dropTop(count)
	v.normal.top, v.normal.bottom = top, bottom

	if keepCursor {
		v.normal.cursor.Y = min(max(anchor, 0), v.height-1)
	}
}

func (v *VT) resizeBuffer(b *Buffer) {
	width, height := v.width, v.height
	cells := make([]Cell, width*height)
	for i := range cells {
		cells[i] = blank()
	}
	copyWidth := min(b.width, width)
	copyHeight := min(b.height, height)
	for y := range copyHeight {
		copy(cells[y*width:y*width+copyWidth], b.cells[y*b.width:y*b.width+copyWidth])
	}

	b.cells = cells
	b.width = width
	b.height = height
	b.cursor.X = min(b.cursor.X, width-1)
	b.cursor.Y = min(b.cursor.Y, height-1)
	b.top = 0
	b.bottom = height - 1
}

// Line returns the visible text of a screen row.
func (v *VT) Line(y int) string {
	if y < 0 || y >= v.height {
		return ""
	}
	row := v.visibleRow(y)
	if row == nil {
		return ""
	}
	return rowText(row, 0, len(row)-1)
}

// visibleRow maps a viewport row to the cells it shows. The alternate buffer
// has no history, so it always shows the live screen.
func (v *VT) visibleRow(y int) []Cell {
	offset := v.offset
	if v.active == v.alt {
		offset = 0
	}
	if y < offset {
		return v.history.row(v.history.len() - offset + y)
	}
	if y-offset >= v.height {
		return nil
	}
	start := (y - offset) * v.width
	return v.active.cells[start : start+v.width]
}

// cell returns one cell of the live screen, clipped to the buffer.
func (v *VT) cell(x, y int) Cell {
	if x < 0 || y < 0 || x >= v.width || y >= v.height {
		return blank()
	}
	return v.active.cells[y*v.width+x]
}

// Blit paints the visible viewport into the compositor. Cells are handed over
// as a prepared row, so no terminal string is ever built for a frame.
func (v *VT) Blit(screen *compositor.Screen, rect geom.Rect) {
	area := rect.Intersect(screen.Bounds())
	if area.Empty() {
		return
	}

	for y := range area.Height {
		cells := v.visibleRow(y)
		if len(cells) == 0 {
			continue
		}
		row := geom.Rect{
			X:      area.X,
			Y:      area.Y + y,
			Width:  min(len(cells), area.Width),
			Height: 1,
		}
		screen.BlitCells(row, v.toCompositor(cells, v.selectionOn(y)))
	}

	v.blitCursor(screen, rect, area)
}

// toCompositor converts one screen row into the compositor's cell type,
// reusing the scratch buffer so a steady-state frame allocates nothing. A
// selected cell is drawn in reverse video, which is how every terminal marks a
// selection without needing a colour of its own.
func (v *VT) toCompositor(cells []Cell, sel rowSpan) []compositor.Cell {
	need := len(cells)
	if cap(v.scratch) < need {
		v.scratch = make([]compositor.Cell, need)
	}
	v.scratch = v.scratch[:need]
	for i, c := range cells {
		cell := compositor.Cell{Style: c.Style, Text: c.Text, Width: c.Width}
		if sel.contains(i) && cell.Width > 0 {
			cell.Style = composeSGR(cell.Style, 7)
		}
		v.scratch[i] = cell
	}
	return v.scratch
}

func (v *VT) blitCursor(screen *compositor.Screen, rect, area geom.Rect) {
	if v.offset != 0 || v.active == v.alt || !v.active.cursor.Visible {
		return
	}
	cursor := v.active.cursor
	if cursor.X >= v.width || cursor.Y >= v.height {
		return
	}
	if cursor.Blink && !cursor.On {
		return
	}
	cell := v.active.cells[cursor.Y*v.width+cursor.X]

	x, y := rect.X+cursor.X, rect.Y+cursor.Y
	switch cursor.Shape {
	case CursorUnderline:
		if !area.Contains(x, y) {
			return
		}
		screen.Set(x, y, compositor.Cell{
			Style: composeSGR(cell.Style, 4),
			Text:  cell.Text,
			Width: 1,
		})
	case CursorBar:
		if cell.Width == 0 && cursor.X > 0 {
			x--
		}
		if !area.Contains(x, y) {
			return
		}
		screen.Set(x, y, compositor.Cell{Style: composeSGR(cell.Style, 7), Text: "", Width: 0})
	default:
		if !area.Contains(x, y) {
			return
		}
		screen.Set(x, y, compositor.Cell{
			Style: composeSGR(cell.Style, 7),
			Text:  cell.Text,
			Width: max(1, cell.Width),
		})
	}
}

// composeSGR merges extra SGR parameters into an existing style string.
func composeSGR(style string, params ...int) string {
	parts := make([]string, 0, len(params)+1)
	if body := strings.TrimSuffix(strings.TrimPrefix(style, "\x1b["), "m"); body != "" {
		parts = append(parts, body)
	}
	for _, p := range params {
		parts = append(parts, strconv.Itoa(p))
	}
	return "\x1b[" + strings.Join(parts, ";") + "m"
}

// runeWidth is how many cells a rune takes. ASCII is the hot path, everything
// else goes through lipgloss, which already knows the East Asian width tables.
func runeWidth(r rune) int {
	switch {
	case r < 0x20:
		return 0
	case r < 0x7f:
		return 1
	case r >= 0x0300 && r <= 0x036f:
		// Combining marks ride along with the previous cell.
		return 0
	}
	w := lipgloss.Width(string(r))
	switch {
	case w < 1:
		return 1
	case w > 2:
		return 2
	default:
		return w
	}
}
