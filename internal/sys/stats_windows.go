//go:build windows

package sys

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32GetSystemTimes       = kernel32.NewProc("GetSystemTimes")
	kernel32GlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	psapiGetProcessMemoryInfo    = syscall.NewLazyDLL("psapi.dll").NewProc("GetProcessMemoryInfo")
)

// systemTimes matches the Win32 SYSTEMTIME layout GetSystemTimes fills.
type systemTimes struct {
	idle   syscall.Filetime
	kernel syscall.Filetime
	user   syscall.Filetime
}

// memoryStatusEx matches MEMORYSTATUSEX.
type memoryStatusEx struct {
	length               uint32
	memoryLoad           uint32
	totalPhys            uint64
	availPhys            uint64
	totalPageFile        uint64
	availPageFile        uint64
	totalVirtual         uint64
	availVirtual         uint64
	availExtendedVirtual uint64
}

// processCounters matches PROCESS_MEMORY_COUNTERS.
type processCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uint64
	workingSetSize             uint64
	quotaPeakPagedPoolUsage    uint64
	quotaPagedPoolUsage        uint64
	quotaPeakNonPagedPoolUsage uint64
	quotaNonPagedPoolUsage     uint64
	pagefileUsage              uint64
	peakPagefileUsage          uint64
}

func filetimeTicks(t syscall.Filetime) uint64 {
	return uint64(t.HighDateTime)<<32 | uint64(t.LowDateTime)
}

// cpuTimes asks the kernel for the cumulative idle, kernel and user times.
// Kernel time already includes idle, so the total is kernel + user.
func cpuTimes() (CPUCounters, error) {
	var times systemTimes
	result, _, callErr := kernel32GetSystemTimes.Call(
		uintptr(unsafe.Pointer(&times.idle)),
		uintptr(unsafe.Pointer(&times.kernel)),
		uintptr(unsafe.Pointer(&times.user)),
	)
	if result == 0 {
		return CPUCounters{}, fmt.Errorf("sys: get system times: %w", callErr)
	}
	return CPUCounters{
		Idle:  filetimeTicks(times.idle),
		Total: filetimeTicks(times.kernel) + filetimeTicks(times.user),
	}, nil
}

func memory() (MemoryInfo, error) {
	var status memoryStatusEx
	status.length = uint32(unsafe.Sizeof(status))
	result, _, callErr := kernel32GlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if result == 0 {
		return MemoryInfo{}, fmt.Errorf("sys: memory status: %w", callErr)
	}
	return MemoryInfo{Total: status.totalPhys, Available: status.availPhys}, nil
}

func processes() ([]Process, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("sys: process snapshot: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil, fmt.Errorf("sys: process list: %w", err)
	}

	procs := make([]Process, 0, 256)
	for {
		procs = append(procs, Process{
			PID:      int(entry.ProcessID),
			Name:     windows.UTF16ToString(entry.ExeFile[:]),
			MemBytes: workingSet(int(entry.ProcessID)),
		})
		entry.Size = uint32(unsafe.Sizeof(entry))
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}
	return procs, nil
}

// workingSet reads the resident size of a process; 0 when the handle is
// denied, which happens for system processes.
func workingSet(pid int) uint64 {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, uint32(pid))
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(handle)

	var counters processCounters
	counters.cb = uint32(unsafe.Sizeof(counters))
	result, _, _ := psapiGetProcessMemoryInfo.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&counters)),
		uintptr(counters.cb),
	)
	if result == 0 {
		return 0
	}
	return counters.workingSetSize
}

func kill(pid int) error {
	if pid <= 0 {
		return errBadPID
	}
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("sys: kill %d: %w", pid, err)
	}
	defer windows.CloseHandle(handle)

	if err := windows.TerminateProcess(handle, 1); err != nil {
		return fmt.Errorf("sys: kill %d: %w", pid, err)
	}
	return nil
}
