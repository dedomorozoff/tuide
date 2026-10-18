//go:build !windows

package sys

import (
	"os"
	"os/signal"
	"syscall"
)

// NotifyResize reports terminal size changes: the channel fires once per
// change. The returned function stops the notifications.
func NotifyResize(f *os.File) (<-chan struct{}, func()) {
	sizes := make(chan struct{}, 1)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGWINCH)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-signals:
				select {
				case sizes <- struct{}{}:
				default:
				}
			case <-done:
				return
			}
		}
	}()

	return sizes, func() {
		signal.Stop(signals)
		close(done)
	}
}
