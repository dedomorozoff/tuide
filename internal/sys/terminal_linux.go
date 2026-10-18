//go:build linux

package sys

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// TerminalSize reports the size of the terminal behind f in cells.
func TerminalSize(f *os.File) (int, int, error) {
	size, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, fmt.Errorf("sys: terminal size: %w", err)
	}
	return int(size.Col), int(size.Row), nil
}

// MakeRaw puts the terminal into raw mode. The returned function restores the
// previous mode.
func MakeRaw(f *os.File) (func() error, error) {
	previous, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	if err != nil {
		return nil, fmt.Errorf("sys: read terminal mode: %w", err)
	}
	raw := *previous
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP |
		unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(int(f.Fd()), unix.TCSETS, &raw); err != nil {
		return nil, fmt.Errorf("sys: set raw mode: %w", err)
	}
	restore := func() error {
		if err := unix.IoctlSetTermios(int(f.Fd()), unix.TCSETS, previous); err != nil {
			return fmt.Errorf("sys: restore terminal mode: %w", err)
		}
		return nil
	}
	return restore, nil
}

// EnableVTProcessing turns on ANSI handling for the console output f. Unix
// terminals speak ANSI already, so this is a no-op.
func EnableVTProcessing(f *os.File) (func() error, error) {
	return func() error { return nil }, nil
}
