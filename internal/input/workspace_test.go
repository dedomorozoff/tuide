package input

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestWorkspaceActionsRoundTrip(t *testing.T) {
	for n := 1; n <= WorkspaceKeys; n++ {
		if got, ok := ParseWorkspace(WorkspaceAction(n)); !ok || got != n {
			t.Errorf("ParseWorkspace(WorkspaceAction(%d)) = %d, %v", n, got, ok)
		}
		if got, ok := ParseMoveToWorkspace(MoveToWorkspaceAction(n)); !ok || got != n {
			t.Errorf("ParseMoveToWorkspace(MoveToWorkspaceAction(%d)) = %d, %v", n, got, ok)
		}
	}
}

func TestWorkspaceActionsDoNotCross(t *testing.T) {
	if _, ok := ParseWorkspace(MoveToWorkspaceAction(3)); ok {
		t.Error("a send action must not parse as a switch action")
	}
	if _, ok := ParseMoveToWorkspace(WorkspaceAction(3)); ok {
		t.Error("a switch action must not parse as a send action")
	}
}

func TestWorkspaceActionsRejectGarbage(t *testing.T) {
	cases := []Action{
		ActionQuit,
		ActionSnapLeft,
		"",
		"workspace-",
		"workspace-0",
		"workspace-10",
		"workspace-x",
	}
	for _, action := range cases {
		if n, ok := ParseWorkspace(action); ok {
			t.Errorf("ParseWorkspace(%q) = %d, true; want false", action, n)
		}
	}
}

func TestFocusWindowActionsRoundTrip(t *testing.T) {
	for n := 1; n <= WindowKeys; n++ {
		if got, ok := ParseFocusWindow(FocusWindowAction(n)); !ok || got != n {
			t.Errorf("ParseFocusWindow(FocusWindowAction(%d)) = %d, %v", n, got, ok)
		}
	}
}

func TestFocusWindowActionsRejectGarbage(t *testing.T) {
	cases := []Action{
		ActionQuit,
		ActionFocusNext,
		"",
		"focus-window-",
		"focus-window-0",
		"focus-window-10",
		"focus-window-b",
	}
	for _, action := range cases {
		if n, ok := ParseFocusWindow(action); ok {
			t.Errorf("ParseFocusWindow(%q) = %d, true; want false", action, n)
		}
	}
}

func TestNumberedActionsDoNotCross(t *testing.T) {
	if _, ok := ParseFocusWindow(WorkspaceAction(3)); ok {
		t.Error("a workspace action must not parse as a window slot")
	}
	if _, ok := ParseFocusWindow(MoveToWorkspaceAction(3)); ok {
		t.Error("a send action must not parse as a window slot")
	}
	if _, ok := ParseWorkspace(FocusWindowAction(3)); ok {
		t.Error("a window slot must not parse as a workspace")
	}
	if _, ok := ParseMoveToWorkspace(FocusWindowAction(3)); ok {
		t.Error("a window slot must not parse as a send action")
	}
}

func TestWindowSlotKeysAndWorkspaceKeysStayApart(t *testing.T) {
	keymap := DefaultKeymap()

	action, ok := keymap.Lookup(press(tea.ModAlt, '3'))
	if !ok || action != FocusWindowAction(3) {
		t.Errorf("alt+3 = %q, %v, want %q", action, ok, FocusWindowAction(3))
	}
	action, ok = keymap.Lookup(press(tea.ModCtrl|tea.ModAlt, '3'))
	if !ok || action != WorkspaceAction(3) {
		t.Errorf("ctrl+alt+3 = %q, %v, want %q", action, ok, WorkspaceAction(3))
	}
	action, ok = keymap.Lookup(press(tea.ModCtrl|tea.ModAlt|tea.ModShift, '3'))
	if !ok || action != MoveToWorkspaceAction(3) {
		t.Errorf("ctrl+alt+shift+3 = %q, %v, want %q", action, ok, MoveToWorkspaceAction(3))
	}
}

func TestDefaultKeymapHasNoDuplicates(t *testing.T) {
	seen := make(map[Binding]Action)
	for _, mapping := range Defaults() {
		binding := Binding{Mods: mapping.Mods, Key: mapping.Key}
		if other, dup := seen[binding]; dup {
			t.Errorf("key %+v is bound to both %q and %q", binding, other, mapping.Action)
		}
		seen[binding] = mapping.Action
	}
}
