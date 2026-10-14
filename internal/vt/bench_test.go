package vt

import (
	"testing"

	"github.com/dedomorozoff/tuide/internal/compositor"
	"github.com/dedomorozoff/tuide/internal/geom"
)

// fill paints the whole screen the way a busy shell session does: every cell
// carries a style and every line ends with a reset and a line feed.
func fill(v *VT, width, height int) {
	for row := range height {
		line := "\x1b[3" + string(rune('0'+row%8)) + "m\x1b[1muser@host\x1b[0m " +
			"\x1b[36m~/src/project\x1b[0m$ \x1b[32mgo\x1b[0m test ./... \x1b[33mok\x1b[0m\r\n"
		v.Write([]byte(line))
	}
}

func BenchmarkWriteFullScreen(b *testing.B) {
	line := []byte("\x1b[3" + string(rune('3')) + "m\x1b[1muser@host\x1b[0m " +
		"\x1b[36m~/src/project\x1b[0m$ \x1b[32mgo\x1b[0m test ./... \x1b[33mok\x1b[0m\r\n")

	b.ReportAllocs()
	b.SetBytes(int64(len(line)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := New(200, 50)
		for range 50 {
			v.Write(line)
		}
	}
}

func BenchmarkBlitFullScreen(b *testing.B) {
	const width, height = 200, 50

	v := New(width, height)
	fill(v, width, height)

	screen := compositor.New(width, height)
	rect := geom.Rect{Width: width, Height: height}

	b.ReportAllocs()
	b.SetBytes(int64(width * height))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		screen.Clear()
		v.Blit(screen, rect)
	}
}

func BenchmarkBlitFullScreenTiled(b *testing.B) {
	const width, height = 100, 25

	v := New(width, height)
	fill(v, width, height)

	screen := compositor.New(width, height)
	rect := geom.Rect{Width: width, Height: height}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		screen.Clear()
		v.Blit(screen, rect)
	}
}
