//go:build !windows

package sys

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	creack "github.com/creack/pty"
)

// unixPTY is a POSIX pseudo terminal: the master file plus the child holding
// the slave open.
type unixPTY struct {
	master *os.File
	cmd    *exec.Cmd
	closed bool
}

// startPTY forks the command onto a fresh pseudo terminal.
func startPTY(options PTYOptions) (PTY, error) {
	cmd := exec.Command(options.Command[0], options.Command[1:]...)
	cmd.Dir = options.Dir
	if options.Env != nil {
		cmd.Env = options.Env
	}
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}

	size := &creack.Winsize{
		Rows: uint16(options.Height),
		Cols: uint16(options.Width),
	}
	master, err := creack.StartWithAttrs(cmd, size, cmd.SysProcAttr)
	if err != nil {
		return nil, fmt.Errorf("sys: pty: start %s: %w", options.Command[0], err)
	}

	return &unixPTY{master: master, cmd: cmd}, nil
}

// Read implements PTY.
func (p *unixPTY) Read(b []byte) (int, error) { return p.master.Read(b) }

// Write implements PTY.
func (p *unixPTY) Write(b []byte) (int, error) { return p.master.Write(b) }

// Resize implements PTY.
func (p *unixPTY) Resize(width, height int) error {
	size := &creack.Winsize{
		Rows: uint16(max(1, height)),
		Cols: uint16(max(1, width)),
	}
	if err := creack.Setsize(p.master, size); err != nil {
		return fmt.Errorf("sys: pty: resize %dx%d: %w", width, height, err)
	}
	return nil
}

// Alive reports whether the child process is still running.
func (p *unixPTY) Alive() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	process, err := os.FindProcess(p.cmd.Process.Pid)
	if err != nil {
		return true
	}
	// Signal 0 asks the kernel without delivering anything; once the child is
	// a zombie it still exists, so cmd.Wait is what decides in the end.
	return process.Signal(syscall.Signal(0)) == nil
}

// Close implements PTY.
func (p *unixPTY) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true

	// Closing the master makes the kernel hang up the slave, which is what
	// lets an interactive shell exit; the signal is the backstop for children
	// that ignore it.
	err := p.master.Close()

	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Signal(syscall.SIGHUP)
		_ = p.cmd.Wait()
	}
	return err
}
