//go:build windows

package sys

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// TerminalSize reports the size of the console behind f in cells.
func TerminalSize(f *os.File) (int, int, error) {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(f.Fd()), &info); err != nil {
		return 0, 0, fmt.Errorf("sys: terminal size: %w", err)
	}
	width := int(info.Window.Right - info.Window.Left + 1)
	height := int(info.Window.Bottom - info.Window.Top + 1)
	return width, height, nil
}

// MakeRaw puts the console input into raw mode with VT key encoding, so arrow
// keys and function keys arrive as escape sequences the way every terminal
// sends them. The returned function restores the previous mode.
func MakeRaw(f *os.File) (func() error, error) {
	handle := windows.Handle(f.Fd())
	var previous uint32
	if err := windows.GetConsoleMode(handle, &previous); err != nil {
		return nil, fmt.Errorf("sys: read console mode: %w", err)
	}
	raw := previous
	raw &^= windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT | windows.ENABLE_PROCESSED_INPUT
	raw |= windows.ENABLE_VIRTUAL_TERMINAL_INPUT
	if err := windows.SetConsoleMode(handle, raw); err != nil {
		return nil, fmt.Errorf("sys: set raw mode: %w", err)
	}
	restore := func() error {
		if err := windows.SetConsoleMode(handle, previous); err != nil {
			return fmt.Errorf("sys: restore console mode: %w", err)
		}
		return nil
	}
	return restore, nil
}

// EnableVTProcessing turns on ANSI handling for the console output f. The
// returned function restores the previous mode.
func EnableVTProcessing(f *os.File) (func() error, error) {
	handle := windows.Handle(f.Fd())
	var previous uint32
	if err := windows.GetConsoleMode(handle, &previous); err != nil {
		return nil, fmt.Errorf("sys: read console mode: %w", err)
	}
	if err := windows.SetConsoleMode(handle, previous|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return nil, fmt.Errorf("sys: enable vt processing: %w", err)
	}
	restore := func() error {
		if err := windows.SetConsoleMode(handle, previous); err != nil {
			return fmt.Errorf("sys: restore console mode: %w", err)
		}
		return nil
	}
	return restore, nil
}
