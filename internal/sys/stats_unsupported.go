//go:build !windows && !linux && !darwin

package sys

// The remaining platforms have no implementation yet; the monitor shows
// "unsupported" instead of numbers.

func cpuTimes() (CPUCounters, error) { return CPUCounters{}, ErrUnsupported }

func memory() (MemoryInfo, error) { return MemoryInfo{}, ErrUnsupported }

func processes() ([]Process, error) { return nil, ErrUnsupported }

func kill(pid int) error {
	if pid <= 0 {
		return errBadPID
	}
	return ErrUnsupported
}
