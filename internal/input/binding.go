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
	// ActionToggleCalendar has no key of its own: the month view belongs to the
	// taskbar clock, and the action palette is the way in without a pointer.
	ActionToggleCalendar Action = "toggle-calendar"

	ActionOpenLauncher Action = "open-launcher"
	ActionOpenPalette  Action = "open-palette"
)

// WorkspaceKeys is how many numbered workspaces the keymap can address.
const WorkspaceKeys = 9

// WindowKeys is how many window slots the keymap can address, one per digit.
const WindowKeys = 9

const (
	workspacePrefix       = "workspace-"
	moveToWorkspacePrefix = "move-to-workspace-"
	focusWindowPrefix     = "focus-window-"
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

// FocusWindowAction is the action that activates window slot n, counting from
// 1 in the order the windows were opened.
func FocusWindowAction(n int) Action {
	return Action(focusWindowPrefix + strconv.Itoa(n))
}

// ParseWorkspace returns the 1-based workspace a switch action names.
func ParseWorkspace(action Action) (int, bool) {
	return indexed(action, workspacePrefix, WorkspaceKeys)
}

// ParseMoveToWorkspace returns the 1-based workspace a send action names.
func ParseMoveToWorkspace(action Action) (int, bool) {
	return indexed(action, moveToWorkspacePrefix, WorkspaceKeys)
}

// ParseFocusWindow returns the 1-based window slot an activate action names.
func ParseFocusWindow(action Action) (int, bool) {
	return indexed(action, focusWindowPrefix, WindowKeys)
}

func indexed(action Action, prefix string, limit int) (int, bool) {
	rest, ok := strings.CutPrefix(string(action), prefix)
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(rest)
	if err != nil || n < 1 || n > limit {
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

// normalizeMods folds the modifiers that mean the same thing into one. A
// terminal that reports the alt key as the meta key sends the same chord in
// a different guise, and on the keyboards we support they are the same
// physical key.
func normalizeMods(mods tea.KeyMod) tea.KeyMod {
	mods &^= tea.ModCapsLock | tea.ModNumLock | tea.ModScrollLock
	if mods&tea.ModMeta != 0 {
		mods &^= tea.ModMeta
		mods |= tea.ModAlt
	}
	return mods
}
