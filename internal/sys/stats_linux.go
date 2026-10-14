package sys

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// cpuTimes reads the first line of /proc/stat: user nice system idle iowait
// irq softirq steal. Iowait counts as idle; guest is already inside user.
func cpuTimes() (CPUCounters, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return CPUCounters{}, err
	}
	line, _, _ := strings.Cut(string(data), "\n")
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return CPUCounters{}, errNoCPU
	}

	var counters CPUCounters
	var idle uint64
	for i, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return CPUCounters{}, err
		}
		counters.Total += value
		if i == 3 || i == 4 {
			idle += value
		}
	}
	counters.Idle = idle
	return counters, nil
}

func memory() (MemoryInfo, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return MemoryInfo{}, err
	}

	var info MemoryInfo
	for line := range strings.SplitSeq(string(data), "\n") {
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		kilobytes, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		switch name {
		case "MemTotal":
			info.Total = kilobytes * 1024
		case "MemAvailable":
			info.Available = kilobytes * 1024
		}
	}
	if info.Total == 0 {
		return MemoryInfo{}, errNoMemory
	}
	return info, nil
}

func processes() ([]Process, error) {
	items, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	pageSize := uint64(os.Getpagesize())
	procs := make([]Process, 0, len(items))
	for _, item := range items {
		pid, err := strconv.Atoi(item.Name())
		if err != nil || !item.IsDir() {
			continue
		}
		procs = append(procs, Process{
			PID:      pid,
			Name:     processName(pid),
			MemBytes: processRSS(pid, pageSize),
		})
	}
	return procs, nil
}

// processName prefers the executable from cmdline and falls back to comm,
// which the kernel truncates to 15 characters.
func processName(pid int) string {
	dir := "/proc/" + strconv.Itoa(pid)
	if data, err := os.ReadFile(filepath.Join(dir, "cmdline")); err == nil {
		if arg, _, _ := strings.Cut(string(data), "\x00"); arg != "" {
			return filepath.Base(arg)
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "comm")); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

// processRSS reads the resident set size in pages from /proc/[pid]/statm.
func processRSS(pid int, pageSize uint64) uint64 {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/statm")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0
	}
	resident, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0
	}
	return resident * pageSize
}

func kill(pid int) error {
	if pid <= 0 {
		return errBadPID
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		return err
	}
	return nil
}
