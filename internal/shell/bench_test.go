package shell

import (
	"fmt"
	"testing"

	"github.com/dedomorozoff/tuide/internal/apps/term"
	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/theme"
	"github.com/dedomorozoff/tuide/internal/wm"
)

// tenWindowDesktop builds a desktop with ten stub windows, matching M1's
// "ten empty windows, steady frame rate" acceptance test.
func tenWindowDesktop(b *testing.B) *Model {
	b.Helper()

	const count = 10
	ids := make([]string, count)
	specs := make([]appsdk.Spec, count)
	for i := range ids {
		ids[i] = fmt.Sprintf("app-%d", i)
		specs[i] = stubSpec(ids[i])
	}

	m := New(theme.Get(theme.DefaultName), specs, Options{
		Width:   200,
		Height:  60,
		Taskbar: true,
		Mouse:   true,
		Launch:  ids,
	})
	_ = m.Init()
	return m
}

func BenchmarkViewTenWindows(b *testing.B) {
	m := tenWindowDesktop(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func BenchmarkViewTenWindowsTiled(b *testing.B) {
	m := tenWindowDesktop(b)
	m.windows.SetLayout(wm.LayoutTiled)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func BenchmarkViewTenWindowsInspector(b *testing.B) {
	m := tenWindowDesktop(b)
	m.inspector = true

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

// terminalDesktop builds a desktop with one terminal window and a few stub
// windows behind it, which is the shape a real session has. Painting a
// terminal costs one cell visit per grid position, content or not.
func terminalDesktop(b *testing.B, count int) *Model {
	b.Helper()

	palette := theme.Get(theme.DefaultName)
	specs := []appsdk.Spec{term.Spec(palette)}
	for i := range count {
		specs = append(specs, stubSpec(fmt.Sprintf("app-%d", i)))
	}

	m := New(palette, specs, Options{
		Width:   200,
		Height:  60,
		Taskbar: true,
		Mouse:   true,
		Launch:  []string{"term"},
	})
	_ = m.Init()
	return m
}

func BenchmarkViewTerminal(b *testing.B) {
	m := terminalDesktop(b, 0)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func BenchmarkViewTerminalWithNineWindows(b *testing.B) {
	m := terminalDesktop(b, 9)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}
