//go:build !windows

package sys

import "os"

// DefaultShell is the command line that opens when a terminal window is
// started without arguments: the user's login shell, or /bin/sh.
func DefaultShell() []string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return []string{shell}
}
