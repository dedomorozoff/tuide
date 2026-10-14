//go:build windows

package sys

import (
	"os"
	"path/filepath"
)

// DefaultShell is the command line that opens when a terminal window is
// started without arguments: PowerShell when it is on PATH, otherwise the
// system command processor.
func DefaultShell() []string {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, "powershell.exe")
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return []string{path, "-NoLogo"}
		}
	}
	return []string{os.Getenv("COMSPEC")}
}
