//go:build !windows

package sys

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
)

// StartBackground launches argv detached from any terminal and from this
// process: the child survives the parent's exit, which is what keeps a session
// alive when the ssh connection that started it goes away.
func StartBackground(argv []string) error {
	if len(argv) == 0 {
		return errors.New("sys: no command to start")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("sys: start %s: %w", argv[0], err)
	}
	return cmd.Process.Release()
}
