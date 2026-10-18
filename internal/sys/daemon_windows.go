//go:build windows

package sys

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// StartBackground launches argv detached from any console and from this
// process: the child survives the parent's exit, which is what keeps a session
// alive when the terminal that started it goes away.
func StartBackground(argv []string) error {
	if len(argv) == 0 {
		return errors.New("sys: no command to start")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("sys: start %s: %w", argv[0], err)
	}
	return cmd.Process.Release()
}
