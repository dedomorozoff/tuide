package sys

import (
	"fmt"
	"io"
)

// resetSequence undoes everything the desktop turns on: styles, mouse
// reporting, bracketed paste, focus reporting and alternate screen.
const resetSequence = "\x1b[0m" +
	"\x1b[?25h" +
	"\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1006l" +
	"\x1b[?2004l" +
	"\x1b[?1004l" +
	"\x1b[?1049l"

// ResetTerminal returns the terminal to a sane state. Bubble Tea already does
// this on a clean exit; this is the safety net for panics and hard kills, so
// the user's shell is never left invisible or in raw mode.
func ResetTerminal(out io.Writer) error {
	if _, err := io.WriteString(out, resetSequence); err != nil {
		return fmt.Errorf("sys: reset terminal: %w", err)
	}
	return nil
}
