package server

import (
	"fmt"

	"github.com/dedomorozoff/tuide/internal/vt"
)

// replayLocked builds the byte stream that recreates the current screen on a
// terminal that has seen nothing yet: modes first, then the cells, then the
// cursor. The mirror lock must be held.
func (m *mirror) replayLocked() []byte {
	width, height := m.screen.Size()
	out := make([]byte, 0, width*height)

	out = append(out, "\x1b[0m\x1b[?25l"...)
	out = append(out, modeSequences(m.screen.Modes())...)

	// The client attaches to whatever its terminal already holds, and the
	// replay only overdraws the cells the desktop uses. Clearing first keeps
	// the shell output around the frame from bleeding through. The alternate
	// screen is entered above, so this clears the buffer the desktop draws on
	// and leaves the primary screen underneath it alone.
	out = append(out, "\x1b[2J\x1b[H"...)

	style := ""
	for y := range height {
		row := m.screen.Row(y)
		out = append(out, fmt.Sprintf("\x1b[%d;1H", y+1)...)
		for x := range min(len(row), lastPrinted(row)+1) {
			cell := row[x]
			if cell.Text == "" {
				continue
			}
			if cell.Style != style {
				if cell.Style == "" {
					out = append(out, "\x1b[0m"...)
				} else {
					out = append(out, cell.Style...)
				}
				style = cell.Style
			}
			out = append(out, cell.Text...)
		}
	}

	x, y, shape, visible := m.screen.CursorAt()
	out = append(out, fmt.Sprintf("\x1b[%d;%dH\x1b[%d q", y+1, x+1, cursorShapeCode(shape))...)
	if visible {
		out = append(out, "\x1b[?25h"...)
	} else {
		out = append(out, "\x1b[?25l"...)
	}
	return out
}

// lastPrinted is the index of the last cell worth drawing: trailing blanks in
// the default style carry nothing a clear screen does not already show.
func lastPrinted(row []vt.Cell) int {
	last := -1
	for i, cell := range row {
		if cell.Style != "" || (cell.Text != "" && cell.Text != " ") {
			last = i
		}
	}
	return last
}

// modeSequences re-enables everything the program asked the terminal for:
// cursor keys, mouse reports, paste wrapping and the alternate screen. Without
// them a fresh client would type into the wrong decoder.
func modeSequences(modes vt.Modes) []byte {
	var out []byte
	if modes.AppCursor {
		out = append(out, "\x1b[?1h"...)
	} else {
		out = append(out, "\x1b[?1l"...)
	}
	if !modes.AutoWrap {
		out = append(out, "\x1b[?7l"...)
	}
	if modes.AltScreen {
		out = append(out, "\x1b[?1049h"...)
	}
	if modes.BracketedPaste {
		out = append(out, "\x1b[?2004h"...)
	}
	if modes.FocusEvents {
		out = append(out, "\x1b[?1004h"...)
	}
	switch modes.Mouse {
	case vt.MouseX10:
		out = append(out, "\x1b[?9h"...)
	case vt.MouseButton:
		out = append(out, "\x1b[?1000h"...)
	case vt.MouseAny:
		out = append(out, "\x1b[?1002h\x1b[?1003h"...)
	case vt.MouseSGR:
		out = append(out, "\x1b[?1002h\x1b[?1003h\x1b[?1006h"...)
	case vt.MouseURXVT:
		out = append(out, "\x1b[?1002h\x1b[?1003h\x1b[?1015h"...)
	}
	return out
}

// cursorShapeCode maps the emulator's cursor shapes to DECSCUSR numbers.
func cursorShapeCode(shape vt.CursorShape) int {
	switch shape {
	case vt.CursorUnderline:
		return 4
	case vt.CursorBar:
		return 6
	default:
		return 2
	}
}
