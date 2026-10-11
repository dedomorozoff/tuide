package input

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func press(mods tea.KeyMod, code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Mod: mods})
}

func TestLookupPlainKey(t *testing.T) {
	keymap := New(Mapping{Action: ActionQuit, Key: 'q'})

	action, ok := keymap.Lookup(press(0, 'q'))
	if !ok || action != ActionQuit {
		t.Errorf("Lookup = %q, %v, want %q, true", action, ok, ActionQuit)
	}
}

func TestLookupRequiresExactModifiers(t *testing.T) {
	keymap := New(Mapping{Action: ActionQuit, Mods: tea.ModCtrl, Key: 'q'})

	if _, ok := keymap.Lookup(press(0, 'q')); ok {
		t.Error("unmodified q should not match a ctrl binding")
	}
	if _, ok := keymap.Lookup(press(tea.ModCtrl|tea.ModShift, 'q')); ok {
		t.Error("ctrl+shift+q should not match a ctrl binding")
	}
}

func TestLookupIgnoresLockModifiers(t *testing.T) {
	keymap := New(Mapping{Action: ActionQuit, Mods: tea.ModCtrl, Key: 'q'})

	_, ok := keymap.Lookup(press(tea.ModCtrl|tea.ModCapsLock, 'q'))
	if !ok {
		t.Error("CapsLock should not break the binding")
	}
}

func TestLookupUnknownKey(t *testing.T) {
	keymap := New(Mapping{Action: ActionQuit, Mods: tea.ModCtrl, Key: 'q'})

	if _, ok := keymap.Lookup(press(0, 'z')); ok {
		t.Error("unbound key should not resolve")
	}
}

func TestFirstMappingWins(t *testing.T) {
	keymap := New(
		Mapping{Action: ActionQuit, Key: 'q'},
		Mapping{Action: ActionClose, Key: 'q'},
	)

	action, _ := keymap.Lookup(press(0, 'q'))
	if action != ActionQuit {
		t.Errorf("Lookup = %q, want the first mapping", action)
	}
}

func TestBindOverridesDefaults(t *testing.T) {
	keymap := DefaultKeymap()
	keymap.Bind(Mapping{Action: ActionClose, Key: 'q'})

	action, ok := keymap.Lookup(press(0, 'q'))
	if !ok || action != ActionClose {
		t.Errorf("Lookup = %q, want the user override", action)
	}
}

func TestDefaultsAreUnique(t *testing.T) {
	seen := make(map[Binding]Action)
	for _, mapping := range Defaults() {
		binding := Binding{Mods: mapping.Mods, Key: mapping.Key}
		if previous, ok := seen[binding]; ok {
			t.Errorf("binding %v maps to both %q and %q", binding, previous, mapping.Action)
		}
		seen[binding] = mapping.Action
	}
}
