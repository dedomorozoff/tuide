package sys

import (
	"encoding/binary"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// cpuTimes reads kern.cp_time: user nice system idle interrupt.
func cpuTimes() (CPUCounters, error) {
	raw, err := unix.SysctlRaw("kern.cp_time")
	if err != nil {
		return CPUCounters{}, err
	}
	if len(raw) < 20 {
		return CPUCounters{}, errNoCPU
	}

	var counters CPUCounters
	for i := range 5 {
		value := uint64(int32(binary.LittleEndian.Uint32(raw[i*4:])))
		counters.Total += value
		if i == 3 {
			counters.Idle = value
		}
	}
	return counters, nil
}

func memory() (MemoryInfo, error) {
	total, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return MemoryInfo{}, err
	}

	pageSize, available, err := vmStat()
	if err != nil {
		return MemoryInfo{}, err
	}
	return MemoryInfo{Total: total, Available: available * pageSize}, nil
}

// vmStat parses the output of vm_stat(1): free, inactive and speculative
// pages count as available for a new allocation.
func vmStat() (pageSize, available uint64, err error) {
	out, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0, 0, err
	}

	for line := range strings.SplitSeq(string(out), "\n") {
		switch {
		case strings.Contains(line, "page size of"):
			size, _, _ := strings.Cut(line, "page size of ")
			size, _, _ = strings.Cut(size, " bytes")
			pageSize, _ = strconv.ParseUint(strings.TrimSpace(size), 10, 64)
		case strings.HasPrefix(line, "Pages free:"),
			strings.HasPrefix(line, "Pages inactive:"),
			strings.HasPrefix(line, "Pages speculative:"):
			available += vmPages(line)
		}
	}
	if pageSize == 0 {
		return 0, 0, errNoMemory
	}
	return pageSize, available, nil
}

func vmPages(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return 0
	}
	value := strings.TrimSuffix(fields[2], ".")
	pages, _ := strconv.ParseUint(value, 10, 64)
	return pages
}

// processes asks the kernel for the whole process table. Darwin's kinfo_proc
// carries no resident set size, so MemBytes stays 0 there.
func processes() ([]Process, error) {
	table, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, err
	}

	procs := make([]Process, 0, len(table))
	for _, entry := range table {
		pid := int(entry.Proc.P_pid)
		if pid <= 0 {
			continue
		}
		procs = append(procs, Process{PID: pid, Name: commName(entry.Proc.P_comm[:])})
	}
	return procs, nil
}

func commName(comm []byte) string {
	if end := strings.IndexByte(string(comm), 0); end >= 0 {
		comm = comm[:end]
	}
	return string(comm)
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
