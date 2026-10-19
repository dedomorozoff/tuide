// Package term runs a real shell inside a window: a pseudo terminal with a
// virtual terminal in front of it. Every key the window manager does not claim
// goes to the child process, so ctrl+c, ctrl+w and ctrl+q reach the shell.
package term

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/vt"
)

// modifiers that never belong in what the child process sees.
const ignoredMods = tea.ModCapsLock | tea.ModNumLock | tea.ModScrollLock

// encodeKey turns a key press into the bytes a terminal program expects. The
// modes come from the virtual terminal, because the same key means different
// things depending on what the running program asked for.
func encodeKey(key tea.Key, modes vt.Modes) []byte {
	mod := key.Mod &^ ignoredMods

	// A terminal that reports the alt key as the meta key sends the same key
	// in a different guise, and the child expects the alt form of it.
	if mod&tea.ModMeta != 0 {
		mod &^= tea.ModMeta
		mod |= tea.ModAlt
	}
	param := modParam(mod)

	// Alt is the escape prefix for printable keys; the keys below that have a
	// modifier form of their own use it instead.
	var prefix []byte
	if mod&tea.ModAlt != 0 && mod&tea.ModCtrl == 0 && mod&tea.ModShift == 0 && !hasSpecialForm(key.Code) {
		prefix = []byte{0x1b}
	}

	switch key.Code {
	case tea.KeyEnter:
		return append(prefix, '\r')
	case tea.KeyTab:
		if mod&tea.ModShift != 0 {
			return []byte("\x1b[Z")
		}
		return append(prefix, '\t')
	case tea.KeyEscape:
		return []byte{0x1b}
	case tea.KeyBackspace:
		if mod&tea.ModCtrl != 0 {
			return append(prefix, 0x08)
		}
		return append(prefix, 0x7f)
	case tea.KeySpace:
		if mod&tea.ModCtrl != 0 {
			return append(prefix, 0x00)
		}
		return append(prefix, ' ')
	case tea.KeyUp:
		return cursorKey('A', param, modes.AppCursor || modes.AppKeypad)
	case tea.KeyDown:
		return cursorKey('B', param, modes.AppCursor || modes.AppKeypad)
	case tea.KeyRight:
		return cursorKey('C', param, modes.AppCursor || modes.AppKeypad)
	case tea.KeyLeft:
		return cursorKey('D', param, modes.AppCursor || modes.AppKeypad)
	case tea.KeyHome:
		return cursorKey('H', param, modes.AppCursor || modes.AppKeypad)
	case tea.KeyEnd:
		return cursorKey('F', param, modes.AppCursor || modes.AppKeypad)
	case tea.KeyInsert:
		return tildeKey(2, param)
	case tea.KeyDelete:
		return tildeKey(3, param)
	case tea.KeyPgUp:
		return tildeKey(5, param)
	case tea.KeyPgDown:
		return tildeKey(6, param)
	}

	if sequence, ok := functionKey(key.Code); ok {
		return append(prefix, sequence...)
	}

	// Anything left with text is a printable character. Control turns it into
	// the matching C0 byte, which is how ctrl+c reaches the shell.
	if key.Text == "" {
		return nil
	}
	if mod&tea.ModCtrl == 0 {
		return append(prefix, key.Text...)
	}
	out := prefix
	for _, r := range key.Text {
		out = append(out, ctrlByte(r)...)
	}
	return out
}

// modParam is the xterm modifier parameter: one plus a bit field where shift is
// one, alt is two, ctrl is four and meta is eight.
func modParam(mod tea.KeyMod) int {
	param := 1
	if mod&tea.ModShift != 0 {
		param++
	}
	if mod&tea.ModAlt != 0 {
		param += 2
	}
	if mod&tea.ModCtrl != 0 {
		param += 4
	}
	if mod&tea.ModSuper != 0 {
		param += 8
	}
	return param
}

// ctrlByte is the C0 code a control chord maps onto.
func ctrlByte(r rune) []byte {
	switch {
	case r >= 'a' && r <= 'z':
		return []byte{byte(r-'a') + 1}
	case r >= 'A' && r <= 'Z':
		return []byte{byte(r-'A') + 1}
	case r == '@' || r == ' ':
		return []byte{0}
	case r >= '[' && r <= '_':
		return []byte{byte(r-'[') + 27}
	case r == '?':
		return []byte{0x7f}
	}
	return []byte(string(r))
}

// cursorKey encodes an arrow or a home key. With no modifiers it is the plain
// sequence, or the application mode one when the program asked for it.
func cursorKey(final byte, param int, appMode bool) []byte {
	if param > 1 {
		return []byte{'\x1b', '[', '1', ';', byte('0' + param), final}
	}
	if appMode {
		return []byte{0x1b, 'O', final}
	}
	return []byte{'\x1b', '[', final}
}

// tildeKey encodes the keys that use the CSI number tilde form.
func tildeKey(code, param int) []byte {
	digits := strconv.Itoa(code)
	if param <= 1 {
		return []byte("\x1b[" + digits + "~")
	}
	return []byte("\x1b[" + digits + ";" + strconv.Itoa(param) + "~")
}

// hasSpecialForm reports whether a key has an escape sequence of its own, in
// which case Alt belongs in the modifier parameter instead of an escape prefix.
func hasSpecialForm(code rune) bool {
	switch code {
	case tea.KeyUp, tea.KeyDown, tea.KeyLeft, tea.KeyRight,
		tea.KeyHome, tea.KeyEnd, tea.KeyInsert, tea.KeyDelete,
		tea.KeyPgUp, tea.KeyPgDown:
		return true
	}
	_, ok := functionKey(code)
	return ok
}

// functionKey encodes the function keys, which have three historical layouts.
func functionKey(code rune) (string, bool) {
	switch code {
	case tea.KeyF1:
		return "\x1bOP", true
	case tea.KeyF2:
		return "\x1bOQ", true
	case tea.KeyF3:
		return "\x1bOR", true
	case tea.KeyF4:
		return "\x1bOS", true
	case tea.KeyF5:
		return "\x1b[15~", true
	case tea.KeyF6:
		return "\x1b[17~", true
	case tea.KeyF7:
		return "\x1b[18~", true
	case tea.KeyF8:
		return "\x1b[19~", true
	case tea.KeyF9:
		return "\x1b[20~", true
	case tea.KeyF10:
		return "\x1b[21~", true
	case tea.KeyF11:
		return "\x1b[23~", true
	case tea.KeyF12:
		return "\x1b[24~", true
	}
	return "", false
}

// mouseEvent is one pointer report in the shape the terminal encodes it in.
type mouseEvent struct {
	button int
	x, y   int
	press  bool
	motion bool
}

// encodeMouse turns a pointer event into the report the running program asked
// for. Modes says whether it wants the legacy encoding or the unambiguous SGR
// one.
func encodeMouse(event mouseEvent, mode vt.MouseMode) []byte {
	if mode == vt.MouseOff {
		return nil
	}

	if mode == vt.MouseSGR {
		final := byte('m')
		if event.press || event.motion {
			final = 'M'
		}
		return []byte("\x1b[<" +
			strconv.Itoa(event.button) + ";" +
			strconv.Itoa(event.x+1) + ";" +
			strconv.Itoa(event.y+1) +
			string(final))
	}

	if event.button < 3 && !event.press {
		// The legacy encoding has no release report; 3 is the "no button"
		// value a program reads as a release.
		event.button = 3
	}
	return []byte{0x1b, '[', 'M',
		byte(32 + event.button),
		byte(32 + event.x + 1),
		byte(32 + event.y + 1)}
}

// mouseButtonCode maps a Bubble Tea button onto the xterm numbering.
func mouseButtonCode(button tea.MouseButton) int {
	switch button {
	case tea.MouseLeft:
		return 0
	case tea.MouseMiddle:
		return 1
	case tea.MouseRight:
		return 2
	case tea.MouseBackward:
		return 3
	case tea.MouseForward:
		return 4
	}
	return -1
}

// paste wraps clipboard content for a program that asked for bracketed paste.
func paste(text string, modes vt.Modes) []byte {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if !modes.BracketedPaste {
		return []byte(text)
	}
	return []byte("\x1b[200~" + text + "\x1b[201~")
}

// decodeClipboard turns an OSC 52 payload into plain text. The payload is the
// base64 the program sent; some programs wrap it in guard characters that are
// not part of the data.
func decodeClipboard(payload string) (string, error) {
	if payload == "" {
		return "", errors.New("term: empty clipboard payload")
	}

	var clean strings.Builder
	for _, r := range payload {
		if r < 0x20 || r == 0x7f {
			continue
		}
		clean.WriteRune(r)
	}

	raw, err := base64.StdEncoding.DecodeString(clean.String())
	if err != nil {
		return "", fmt.Errorf("term: clipboard: %w", err)
	}
	return string(raw), nil
}
