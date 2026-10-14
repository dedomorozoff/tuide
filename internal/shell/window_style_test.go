package shell

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestDesktopDoesNotAdvertiseAQuitShortcut checks the one piece of text the
// desktop shows before anything is opened. Quitting is a command in the action
// palette now, and neither the taskbar nor the welcome window repeats it.
func TestDesktopDoesNotAdvertiseAQuitShortcut(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()

	frame := ansi.Strip(m.View().Content)
	if strings.Contains(frame, "ctrl+alt+q") {
		t.Errorf("the desktop must not advertise a quit shortcut:\n%s", frame)
	}
}
