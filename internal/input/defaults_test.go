package input

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestCtrlQEndsTheSession keeps the way out of the desktop one chord away:
// ctrl+q quits, the way ctrl+alt+q always did.
func TestCtrlQEndsTheSession(t *testing.T) {
	keymap := DefaultKeymap()

	action, ok := keymap.Lookup(press(tea.ModCtrl, 'q'))
	if !ok || action != ActionQuit {
		t.Errorf("ctrl+q = %q, %v, want %q, true", action, ok, ActionQuit)
	}

	action, ok = keymap.Lookup(press(tea.ModCtrl|tea.ModAlt, 'q'))
	if !ok || action != ActionQuit {
		t.Errorf("ctrl+alt+q = %q, %v, want %q, true", action, ok, ActionQuit)
	}
}

// TestCtrlCAndCtrlWStayWithTheProgram makes sure the desktop did not grow a
// taste for the bare ctrl chords: the program inside the window owns them.
func TestCtrlCAndCtrlWStayWithTheProgram(t *testing.T) {
	keymap := DefaultKeymap()

	for _, code := range []rune{'c', 'w'} {
		if _, ok := keymap.Lookup(press(tea.ModCtrl, code)); ok {
			t.Errorf("ctrl+%c should reach the program inside the window", code)
		}
	}
}
