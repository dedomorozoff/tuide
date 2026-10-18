//go:build windows

package sys

import (
	"os"
	"time"
)

// NotifyResize reports terminal size changes: the channel fires once per
// change. Windows has no resize signal, so the size is polled. The returned
// function stops the notifications.
func NotifyResize(f *os.File) (<-chan struct{}, func()) {
	sizes := make(chan struct{}, 1)
	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()
		width, height, _ := TerminalSize(f)
		for {
			select {
			case <-ticker.C:
				nextWidth, nextHeight, err := TerminalSize(f)
				if err != nil || (nextWidth == width && nextHeight == height) {
					continue
				}
				width, height = nextWidth, nextHeight
				select {
				case sizes <- struct{}{}:
				default:
				}
			case <-done:
				return
			}
		}
	}()

	return sizes, func() { close(done) }
}
