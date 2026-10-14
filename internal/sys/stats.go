package sys

import "errors"

var (
	// ErrUnsupported reports that the platform does not provide the metric.
	ErrUnsupported = errors.New("sys: unsupported on this platform")

	errNoCPU    = errors.New("sys: no cpu statistics")
	errNoMemory = errors.New("sys: no memory statistics")
	errBadPID   = errors.New("sys: invalid pid")
)

// CPUCounters are the kernel's cumulative CPU counters. Only the difference
// between two samples carries meaning.
type CPUCounters struct {
	// Idle is the time at least one CPU spent idle.
	Idle uint64
	// Total is the summed time of every CPU, idle included.
	Total uint64
}

// BusyFraction reports how much of the time between prev and c was spent
// working, as a value in [0..1]. It returns 0 when no time passed.
func (c CPUCounters) BusyFraction(prev CPUCounters) float64 {
	total := c.Total - prev.Total
	if total == 0 {
		return 0
	}
	busy := (c.Total - c.Idle) - (prev.Total - prev.Idle)
	return min(max(float64(busy)/float64(total), 0), 1)
}

// MemoryInfo is the physical memory of the machine in bytes.
type MemoryInfo struct {
	Total     uint64
	Available uint64
}

// Process is one row of the process list.
type Process struct {
	PID  int
	Name string
	// MemBytes is the resident set size, or 0 when the platform does not
	// report it or the process could not be inspected.
	MemBytes uint64
}

// CPUTimes returns the system-wide cumulative CPU counters.
func CPUTimes() (CPUCounters, error) { return cpuTimes() }

// Memory returns the physical memory totals.
func Memory() (MemoryInfo, error) { return memory() }

// Processes lists the running processes.
func Processes() ([]Process, error) { return processes() }

// Kill force-terminates a process.
func Kill(pid int) error { return kill(pid) }
