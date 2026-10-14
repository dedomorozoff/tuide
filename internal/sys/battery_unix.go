//go:build !windows

package sys

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Battery reports the battery charge percentage (0-100) and whether on AC power.
func Battery() (percent int, charging bool, available bool) {
	// Look up Linux sysfs power supply
	supplies, err := filepath.Glob("/sys/class/power_supply/BAT*")
	if err == nil && len(supplies) > 0 {
		bat := supplies[0]
		capBytes, err := os.ReadFile(filepath.Join(bat, "capacity"))
		if err == nil {
			val, parseErr := strconv.Atoi(strings.TrimSpace(string(capBytes)))
			if parseErr == nil {
				statusBytes, _ := os.ReadFile(filepath.Join(bat, "status"))
				statusStr := strings.TrimSpace(string(statusBytes))
				isCharging := statusStr == "Charging" || statusStr == "Full"
				return val, isCharging, true
			}
		}
	}
	return 0, false, false
}
