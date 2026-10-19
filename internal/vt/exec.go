package vt

import (
	"strconv"
	"strings"
)

// param returns the nth parameter with the ECMA-48 default applied. A missing
// or empty parameter reads as -1, which callers treat as zero unless the
// control has its own default.
func param(params []int, n, fallback int) int {
	if n >= len(params) || params[n] < 0 {
		return fallback
	}
	return params[n]
}

// executeControl handles a C0 or C1 byte.
func (v *VT) executeControl(b byte) {
	switch b {
	case 0x07:
		if v.OnBell != nil {
			v.OnBell()
		}
	case 0x08:
		v.backspace()
	case 0x09:
		v.tab(tabStops(v.width))
	case 0x0a, 0x0b, 0x0c:
		v.lineFeed()
	case 0x0d:
		v.carriageReturn()
	case 0x0e:
		v.g1 = true
	case 0x0f:
		v.g1 = false
	case 0x84:
		v.lineFeed()
	case 0x85:
		v.nextLine()
	case 0x88:
		v.active.cursor.X = 0
		v.tab(tabStops(v.width))
	case 0x8d:
		v.reverseIndex()
	}
}

// tabStops are the default tab stops every eight columns.
func tabStops(width int) []int {
	stops := make([]int, 0, width/8)
	for x := 8; x < width; x += 8 {
		stops = append(stops, x)
	}
	return stops
}

// executeCSI dispatches a complete CSI sequence.
func (v *VT) executeCSI(final byte, params []int, intermediate byte, private byte) {
	if intermediate == ' ' && final == 'q' {
		shape, blink := cursorStyle(param(params, 0, 1))
		v.active.cursor.Shape = shape
		v.active.cursor.Blink = blink
		v.active.cursor.On = true
		return
	}
	if intermediate != 0 {
		return
	}

	switch private {
	case '?':
		v.executePrivateCSI(final, params)
		return
	case '>', '<', '=':
		return
	}

	switch final {
	case '@':
		v.insertBlanks(param(params, 0, 1))
	case 'A':
		v.moveCursor(0, -param(params, 0, 1))
	case 'B', 'e':
		v.moveCursor(0, param(params, 0, 1))
	case 'C', 'a':
		v.moveCursor(param(params, 0, 1), 0)
	case 'D':
		v.moveCursor(-param(params, 0, 1), 0)
	case 'E':
		v.active.cursor.X = 0
		v.moveCursor(0, param(params, 0, 1))
	case 'F':
		v.active.cursor.X = 0
		v.moveCursor(0, -param(params, 0, 1))
	case 'G', '`':
		v.moveToColumn(param(params, 0, 1) - 1)
	case 'H', 'f':
		// CUP takes the row first and the column second.
		v.moveTo(param(params, 1, 1)-1, param(params, 0, 1)-1)
	case 'I':
		v.moveCursor(param(params, 0, 1), 0)
	case 'J':
		v.eraseInDisplay(param(params, 0, 0))
	case 'K':
		v.eraseInLine(param(params, 0, 0))
	case 'L':
		v.insertLines(param(params, 0, 1))
	case 'M':
		v.deleteLines(param(params, 0, 1))
	case 'P':
		v.deleteChars(param(params, 0, 1))
	case 'S':
		for range param(params, 0, 1) {
			v.scrollUp(1)
		}
	case 'T':
		for range param(params, 0, 1) {
			v.scrollDown(1)
		}
	case 'X':
		v.eraseCells(param(params, 0, 1))
	case 'Z':
		v.backTab(param(params, 0, 1))
	case 'b':
		v.repeatLast(param(params, 0, 1))
	case 'c':
		v.replies = append(v.replies, primaryDeviceAttributes...)
	case 'd':
		v.moveToRow(param(params, 0, 1) - 1)
	case 'g':
		v.clearTabStops(param(params, 0, 0))
	case 'h':
		v.setMode(params, true)
	case 'l':
		v.setMode(params, false)
	case 'm':
		v.applySGR(params)
	case 'n':
		v.deviceStatus(param(params, 0, 0))
	case 'r':
		v.setRegion(param(params, 0, 1)-1, param(params, 1, v.height)-1)
	case 's':
		v.saveCursor()
	case 'u':
		v.restoreCursor()
	case 't':
		v.windowOperation(param(params, 0, 0))
	}
}

// primaryDeviceAttributes identifies us as a VT220 with the usual extensions.
var primaryDeviceAttributes = []byte("\x1b[?62;1;2;6;9;15;22;29c")

// moveTo places the cursor in absolute cell coordinates.
func (v *VT) moveTo(x, y int) {
	b := v.active
	b.cursor.X = min(max(x, 0), b.width-1)
	b.cursor.Y = v.originTo(y)
	v.pendingWrap = false
}

func (v *VT) moveToColumn(x int) {
	b := v.active
	b.cursor.X = min(max(x, 0), b.width-1)
	v.pendingWrap = false
}

func (v *VT) moveToRow(y int) {
	v.active.cursor.Y = v.originTo(y)
	v.pendingWrap = false
}

// moveCursor shifts the cursor, staying inside the screen.
func (v *VT) moveCursor(dx, dy int) {
	b := v.active
	b.cursor.X = min(max(b.cursor.X+dx, 0), b.width-1)
	b.cursor.Y = min(max(b.cursor.Y+dy, 0), b.height-1)
	v.pendingWrap = false
}

// backTab moves the cursor left across tab stops.
func (v *VT) backTab(count int) {
	for range count {
		x := v.active.cursor.X
		for x > 0 && x%8 != 0 {
			x--
		}
		if x == v.active.cursor.X && x > 0 {
			x -= 8
		}
		v.active.cursor.X = max(0, x)
	}
	v.pendingWrap = false
}

// clearTabStops is accepted and ignored: the emulator only ever uses the
// default eight column stops.
func (v *VT) clearTabStops(mode int) {}

// repeatLast prints the previous rune count more times.
func (v *VT) repeatLast(count int) {
	if v.lastWidth == 0 {
		return
	}
	for range count {
		v.putRune(v.lastRune)
	}
}

// eraseInDisplay clears from the cursor to the end of the screen, to the
// start of it, the whole screen, or the scrollback on mode 3.
func (v *VT) eraseInDisplay(mode int) {
	b := v.active
	switch mode {
	case 0:
		v.eraseCells(b.width - b.cursor.X)
		for y := b.cursor.Y + 1; y < b.height; y++ {
			v.eraseRow(y, 0, b.width)
		}
	case 1:
		for y := 0; y < b.cursor.Y; y++ {
			v.eraseRow(y, 0, b.width)
		}
		v.eraseRow(b.cursor.Y, 0, b.cursor.X+1)
	case 2:
		for y := range b.height {
			v.eraseRow(y, 0, b.width)
		}
		b.top = 0
		b.bottom = b.height - 1
	case 3:
		if b == v.normal {
			v.history.clear()
			v.offset = 0
		}
	}
	v.pendingWrap = false
}

// eraseInLine clears part of the cursor row.
func (v *VT) eraseInLine(mode int) {
	b := v.active
	switch mode {
	case 0:
		v.eraseCells(b.width - b.cursor.X)
	case 1:
		v.eraseRow(b.cursor.Y, 0, b.cursor.X+1)
	case 2:
		v.eraseRow(b.cursor.Y, 0, b.width)
	}
	v.pendingWrap = false
}

func (v *VT) eraseRow(y, from, to int) {
	if y < 0 || y >= v.active.height {
		return
	}
	start := y*v.active.width + from
	cell := Cell{Text: " ", Style: v.styleString(), Width: 1}
	for x := from; x < to && x < v.active.width; x++ {
		v.active.cells[start+x-from] = cell
	}
}

// deviceStatus answers the two status requests programs actually make.
func (v *VT) deviceStatus(mode int) {
	switch mode {
	case 5:
		v.replies = append(v.replies, "\x1b[0n"...)
	case 6:
		c := v.active.cursor
		v.replies = append(v.replies, "\x1b["+strconv.Itoa(c.Y+1)+";"+strconv.Itoa(c.X+1)+"R"...)
	}
}

// windowOperation answers the two size queries programs ask for.
func (v *VT) windowOperation(mode int) {
	switch mode {
	case 14:
		v.replies = append(v.replies, "\x1b[4;"+strconv.Itoa(v.height)+";"+strconv.Itoa(v.width)+"t"...)
	case 16:
		v.replies = append(v.replies, "\x1b[6;"+strconv.Itoa(v.height)+";"+strconv.Itoa(v.width)+"t"...)
	case 18:
		v.replies = append(v.replies, "\x1b[8;"+strconv.Itoa(v.height)+";"+strconv.Itoa(v.width)+"t"...)
	case 21:
		v.replies = append(v.replies, "\x1b]l"+v.title+"\x1b\\"...)
	}
}

// setMode applies SM and RM, both for ANSI and private DEC modes.
func (v *VT) setMode(params []int, on bool) {
	for _, code := range params {
		switch {
		case code < 0:
		case code == 4:
			v.modes.Insert = on
		case code == 20:
			v.modes.Insert = !on
		case code >= 80 && code <= 90:
			v.modes.ReverseVideo = on
		default:
			v.setPrivateMode([]int{code}, on)
		}
	}
}

// executePrivateCSI handles the DEC modes reached through CSI ? Ps h/l.
func (v *VT) executePrivateCSI(final byte, params []int) {
	if final != 'h' && final != 'l' {
		return
	}
	v.setPrivateMode(params, final == 'h')
}

func (v *VT) setPrivateMode(params []int, on bool) {
	for _, code := range params {
		switch code {
		case 1:
			v.modes.AppCursor = on
		case 3:
			// DECCOLM clears the screen and homes the cursor.
			if on {
				v.eraseInDisplay(2)
				v.moveTo(0, 0)
			}
		case 5:
			v.modes.ReverseVideo = on
		case 6:
			v.modes.Origin = on
			v.moveTo(v.active.cursor.X, 0)
		case 7:
			v.modes.AutoWrap = on
		case 12:
			v.active.cursor.Blink = on
		case 25:
			v.active.cursor.Visible = on
		case 47, 1047, 1049:
			v.switchAltScreen(on)
		case 1000:
			v.setMouse(MouseX10, on)
		case 1002:
			v.setMouse(MouseButton, on)
		case 1003:
			v.setMouse(MouseAny, on)
		case 1005:
			v.modes.Mouse = mouseEncoding(v.modes.Mouse, true)
		case 1006:
			v.modes.Mouse = mouseEncoding(v.modes.Mouse, false)
		case 1015:
			v.modes.Mouse = mouseEncoding(v.modes.Mouse, false)
		case 1004:
			v.modes.FocusEvents = on
		case 2004:
			v.modes.BracketedPaste = on
		}
	}
}

func (v *VT) setMouse(mode MouseMode, on bool) {
	if on {
		v.modes.Mouse = mode
		return
	}
	v.modes.Mouse = MouseOff
}

// mouseEncoding switches between the legacy and the SGR encodings.
func mouseEncoding(mode MouseMode, legacy bool) MouseMode {
	if mode == MouseOff {
		return MouseOff
	}
	if legacy {
		if mode == MouseSGR {
			// Nothing legacy carries the SGR encoding, so the closest is the
			// X10 one.
			return MouseAny
		}
		return mode
	}
	switch mode {
	case MouseX10, MouseButton, MouseAny, MouseURXVT:
		return MouseSGR
	}
	return mode
}

// switchAltScreen moves between the primary and the alternate buffer. Mode 1049
// also parks the cursor so the program comes back where it left off.
func (v *VT) switchAltScreen(on bool) {
	if on == (v.active == v.alt) {
		return
	}
	if on {
		v.normal.saved = v.normal.cursor
		v.active = v.alt
		v.modes.AltScreen = true
		v.alt.clear()
		v.offset = 0
		return
	}
	v.active = v.normal
	v.modes.AltScreen = false
	v.normal.cursor = v.normal.saved
	v.normal.top = 0
	v.normal.bottom = v.normal.height - 1
}

func (v *VT) saveCursor() {
	b := v.active
	b.saved = b.cursor
	b.saved.X = min(b.saved.X, b.width-1)
	b.saved.Y = min(b.saved.Y, b.height-1)
}

func (v *VT) restoreCursor() {
	b := v.active
	b.cursor = b.saved
	b.cursor.X = min(max(b.cursor.X, 0), b.width-1)
	b.cursor.Y = min(max(b.cursor.Y, 0), b.height-1)
	v.pendingWrap = false
}

// executeOSC handles a complete operating system command.
func (v *VT) executeOSC(payload []byte) {
	body, code, ok := strings.Cut(string(payload), ";")
	if !ok {
		return
	}
	number, err := strconv.Atoi(body)
	if err != nil {
		return
	}

	switch number {
	case 0, 2:
		v.setTitle(code)
	case 1, 3, 4, 5, 6, 7, 9, 10, 11, 12, 104, 110, 111, 112:
		// Icon names, palettes, dynamic colours and key definitions are
		// accepted and ignored.
	case 52:
		if kind, data, ok := strings.Cut(code, ";"); ok && kind == "c" {
			v.clipboard = data
			if v.OnClipboard != nil {
				v.OnClipboard(data)
			}
		}
	case 8:
		// Hyperlinks are dropped: the compositor draws plain text.
	}
}

func (v *VT) setTitle(title string) {
	v.title = title
	if v.OnTitle != nil {
		v.OnTitle(title)
	}
}

// cursorStyle maps a DECSCUSR parameter onto a shape and blink flag.
func cursorStyle(param int) (shape CursorShape, blink bool) {
	switch param {
	case 2:
		return CursorBlock, true
	case 3:
		return CursorUnderline, false
	case 4:
		return CursorUnderline, true
	case 5:
		return CursorBar, false
	case 6:
		return CursorBar, true
	default:
		return CursorBlock, false
	}
}
