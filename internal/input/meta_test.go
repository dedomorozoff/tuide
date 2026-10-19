package input

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestAltReportedAsMetaStillMatches makes sure a terminal that reports the
// alt key as the meta key does not lose the chord: on the keyboards we
// support they are the same physical key.
func TestAltReportedAsMetaStillMatches(t *testing.T) {
	keymap := New(Mapping{Action: ActionQuit, Mods: tea.ModCtrl | tea.ModAlt, Key: 'q'})

	action, ok := keymap.Lookup(press(tea.ModCtrl|tea.ModMeta, 'q'))
	if !ok || action != ActionQuit {
		t.Errorf("ctrl+meta+q = %q, %v, want %q, true", action, ok, ActionQuit)
	}
}
