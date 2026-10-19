package shell

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/apps/welcome"
	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/theme"
)

// TestProgramQuitsOnCtrlQ checks the way out of the desktop: ctrl+q ends the
// session on its own, the way ctrl+alt+q always did.
func TestProgramQuitsOnCtrlQ(t *testing.T) {
	palette := theme.Get(theme.DefaultName)
	model := New(palette, []appsdk.Spec{welcome.Spec(palette)}, Options{
		Width:   goldenWidth,
		Height:  goldenHeight,
		Taskbar: true,
		Mouse:   true,
	})

	var out bytes.Buffer
	program := tea.NewProgram(model,
		tea.WithInput(strings.NewReader("")),
		tea.WithOutput(&out),
		tea.WithWindowSize(goldenWidth, goldenHeight),
		tea.WithoutSignals(),
	)

	done := make(chan error, 1)
	go func() {
		_, err := program.Run()
		done <- err
	}()

	program.Send(tea.KeyPressMsg(tea.Key{Code: 'q', Mod: tea.ModCtrl}))

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(10 * time.Second):
		program.Kill()
		t.Fatal("program did not exit after ctrl+q")
	}
}
