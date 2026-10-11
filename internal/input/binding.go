package input

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Action is a named command the shell performs when a key is pressed.
type Action string

const (
	ActionQuit        Action = "quit"
	ActionClose       Action = "close-window"
	ActionFocusNext   Action = "focus-next"
	ActionFocusPrev   Action = "focus-prev"
	ActionFocusLast   Action = "focus-last"
	ActionMaximize    Action = "toggle-maximize"
	ActionMinimize    Action = "toggle-minimize"
	ActionMoveLeft    Action = "move-left"
	ActionMoveRight   Action = "move-right"
	ActionMoveUp      Action = "move-up"
	ActionMoveDown    Action = "move-down"
	ActionResizeLeft  Action = "resize-left"
	ActionResizeRight Action = "resize-right"
	ActionResizeUp    Action = "resize-up"
	ActionResizeDown  Action = "resize-down"

	ActionSnapLeft   Action = "snap-left"
	ActionSnapRight  Action = "snap-right"
	ActionSnapTop    Action = "snap-top"
	ActionSnapBottom Action = "snap-bottom"
	ActionSnapCenter Action = "snap-center"

	ActionToggleLayout    Action = "toggle-layout"
	ActionToggleInspector Action = "toggle-inspector"

	ActionOpenLauncher Action = "open-launcher"
	ActionOpenPalette  Action = "open-palette"
)

// WorkspaceKeys is how many numbered workspaces the keymap can address.
const WorkspaceKeys = 9

const (
	workspacePrefix       = "workspace-"
	moveToWorkspacePrefix = "move-to-workspace-"
)

// WorkspaceAction is the action that switches to workspace n, counting from 1.
func WorkspaceAction(n int) Action {
	return Action(workspacePrefix + strconv.Itoa(n))
}

// MoveToWorkspaceAction is the action that sends the focused window to
// workspace n, counting from 1, and follows it there.
func MoveToWorkspaceAction(n int) Action {
	return Action(moveToWorkspacePrefix + strconv.Itoa(n))
}

// ParseWorkspace returns the 1-based workspace a switch action names.
func ParseWorkspace(action Action) (int, bool) {
	return indexed(action, workspacePrefix)
}

// ParseMoveToWorkspace returns the 1-based workspace a send action names.
func ParseMoveToWorkspace(action Action) (int, bool) {
	return indexed(action, moveToWorkspacePrefix)
}

func indexed(action Action, prefix string) (int, bool) {
	rest, ok := strings.CutPrefix(string(action), prefix)
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(rest)
	if err != nil || n < 1 || n > WorkspaceKeys {
		return 0, false
	}
	return n, true
}

// Binding maps a single key combination to an action.
type Binding struct {
	Mods tea.KeyMod
	Key  rune
}

// Matches reports whether a key press triggers this binding. Lock modifiers
// are ignored so CapsLock never changes what a shortcut means.
func (b Binding) Matches(key tea.Key) bool {
	return normalizeMods(key.Mod) == normalizeMods(b.Mods) && key.Code == b.Key
}

func normalizeMods(mods tea.KeyMod) tea.KeyMod {
	return mods &^ (tea.ModCapsLock | tea.ModNumLock | tea.ModScrollLock)
}
