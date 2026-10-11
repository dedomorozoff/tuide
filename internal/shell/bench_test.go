package shell

import (
	"fmt"
	"testing"

	"tuide/internal/appsdk"
	"tuide/internal/theme"
	"tuide/internal/wm"
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
