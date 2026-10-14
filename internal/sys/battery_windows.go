package sys

import (
	"syscall"
	"unsafe"
)

type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemPowerStatus = kernel32.NewProc("GetSystemPowerStatus")
)

// Battery reports the battery charge percentage (0-100) and whether on AC power.
// If battery info is unavailable or unsupported, available is false.
func Battery() (percent int, charging bool, available bool) {
	var status systemPowerStatus
	r, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&status)))
	if r == 0 {
		return 0, false, false
	}
	// BatteryFlag 128 means no system battery
	if status.BatteryFlag == 128 || status.BatteryLifePercent == 255 {
		return 0, status.ACLineStatus == 1, false
	}
	return int(status.BatteryLifePercent), status.ACLineStatus == 1, true
}
