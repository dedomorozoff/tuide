package sys

import "errors"

// PTYOptions describes the child process a pseudo terminal runs and the size
// it starts at.
type PTYOptions struct {
	// Command is the program and its arguments. An empty list starts the
	// platform default shell.
	Command []string
	// Env replaces the child environment when it is not nil.
	Env []string
	// Dir is the working directory. An empty string keeps the current one.
	Dir string
	// Width and Height are the initial terminal size in cells.
	Width  int
	Height int
}

// PTY is a pseudo terminal: a bidirectional byte stream wired to a child
// process that believes it is talking to a real terminal. Read blocks until
// the child writes or the terminal is closed.
type PTY interface {
	// Read returns bytes the child wrote. io.EOF means the child is gone.
	Read(p []byte) (int, error)
	// Write sends bytes to the child, as if typed on its terminal.
	Write(p []byte) (int, error)
	// Resize tells the child its window changed size.
	Resize(width, height int) error
	// Close shuts the terminal down and terminates the child.
	Close() error
}

// ErrNoCommand reports that a pseudo terminal was started without a program.
var ErrNoCommand = errors.New("sys: pty: no command")

// StartPTY spawns a child process attached to a fresh pseudo terminal. The
// caller owns the returned terminal and must close it.
func StartPTY(options PTYOptions) (PTY, error) {
	command := options.Command
	if len(command) == 0 {
		command = DefaultShell()
	}
	if len(command) == 0 {
		return nil, ErrNoCommand
	}

	opts := options
	opts.Command = command
	opts.Width = max(1, options.Width)
	opts.Height = max(1, options.Height)
	return startPTY(opts)
}

// ChildAlive reports whether the child process behind the terminal is still
// running. It is false once the process exited, even before the output pipe
// is drained.
func ChildAlive(terminal PTY) bool {
	if terminal == nil {
		return false
	}
	if probe, ok := terminal.(interface{ Alive() bool }); ok {
		return probe.Alive()
	}
	return true
}
