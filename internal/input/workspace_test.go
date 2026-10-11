package input

import "testing"

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
