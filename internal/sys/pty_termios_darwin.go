//go:build darwin

package sys

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// disableFlowControl turns the terminal's own flow control off. With it on,
// ctrl+s stops the output and only ctrl+q starts it again, and the desktop
// keeps ctrl+q for itself: a session that could be frozen with no way to
// thaw it would be worse than one without flow control at all.
func disableFlowControl(tty *os.File) error {
	settings, err := unix.IoctlGetTermios(int(tty.Fd()), unix.TIOCGETA)
	if err != nil {
		return fmt.Errorf("sys: pty: read terminal mode: %w", err)
	}
	settings.Iflag &^= unix.IXON
	if err := unix.IoctlSetTermios(int(tty.Fd()), unix.TIOCSETA, settings); err != nil {
		return fmt.Errorf("sys: pty: set terminal mode: %w", err)
	}
	return nil
}
