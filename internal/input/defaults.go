package input

import (
	tea "charm.land/bubbletea/v2"
)

// Defaults are the bindings the desktop starts with. They are deliberately
// conservative: the focused app gets every key that is not claimed here.
//
// The desktop modifier is Ctrl+Alt. Window move and resize live on the arrow
// keys; snapping, layouts and workspaces live on the letter and number rows.
func Defaults() []Mapping {
	mappings := []Mapping{
		{Action: ActionQuit, Mods: tea.ModCtrl, Key: 'q'},

		{Action: ActionFocusNext, Mods: tea.ModAlt, Key: tea.KeyTab},
		{Action: ActionFocusPrev, Mods: tea.ModAlt | tea.ModShift, Key: tea.KeyTab},
		{Action: ActionFocusLast, Mods: tea.ModCtrl | tea.ModAlt, Key: 'p'},

		{Action: ActionClose, Mods: tea.ModCtrl, Key: 'w'},
		{Action: ActionMaximize, Mods: tea.ModCtrl | tea.ModAlt, Key: 'm'},
		{Action: ActionMinimize, Mods: tea.ModCtrl | tea.ModAlt, Key: 'n'},

		{Action: ActionMoveLeft, Mods: tea.ModCtrl | tea.ModAlt, Key: tea.KeyLeft},
		{Action: ActionMoveRight, Mods: tea.ModCtrl | tea.ModAlt, Key: tea.KeyRight},
		{Action: ActionMoveUp, Mods: tea.ModCtrl | tea.ModAlt, Key: tea.KeyUp},
		{Action: ActionMoveDown, Mods: tea.ModCtrl | tea.ModAlt, Key: tea.KeyDown},

		{Action: ActionResizeLeft, Mods: tea.ModCtrl | tea.ModAlt | tea.ModShift, Key: tea.KeyLeft},
		{Action: ActionResizeRight, Mods: tea.ModCtrl | tea.ModAlt | tea.ModShift, Key: tea.KeyRight},
		{Action: ActionResizeUp, Mods: tea.ModCtrl | tea.ModAlt | tea.ModShift, Key: tea.KeyUp},
		{Action: ActionResizeDown, Mods: tea.ModCtrl | tea.ModAlt | tea.ModShift, Key: tea.KeyDown},

		// Snapping uses vim directions on the home row. Snapping to a half
		// twice in a row is a no-op; snapping to a perpendicular half turns
		// the window into the quarter where the halves meet.
		{Action: ActionSnapLeft, Mods: tea.ModCtrl | tea.ModAlt, Key: 'h'},
		{Action: ActionSnapBottom, Mods: tea.ModCtrl | tea.ModAlt, Key: 'j'},
		{Action: ActionSnapTop, Mods: tea.ModCtrl | tea.ModAlt, Key: 'k'},
		{Action: ActionSnapRight, Mods: tea.ModCtrl | tea.ModAlt, Key: 'l'},
		{Action: ActionSnapCenter, Mods: tea.ModCtrl | tea.ModAlt, Key: 'c'},

		{Action: ActionToggleLayout, Mods: tea.ModCtrl | tea.ModAlt, Key: 't'},
		{Action: ActionToggleInspector, Mods: tea.ModCtrl | tea.ModAlt, Key: 'd'},

		{Action: ActionOpenLauncher, Mods: tea.ModCtrl | tea.ModAlt, Key: tea.KeySpace},
		{Action: ActionOpenPalette, Mods: tea.ModCtrl | tea.ModAlt, Key: 'a'},
	}

	for n := 1; n <= WorkspaceKeys; n++ {
		key := rune('0' + n)
		mappings = append(mappings,
			Mapping{
				Action: WorkspaceAction(n),
				Mods:   tea.ModCtrl | tea.ModAlt,
				Key:    key,
			},
			Mapping{
				Action: MoveToWorkspaceAction(n),
				Mods:   tea.ModCtrl | tea.ModAlt | tea.ModShift,
				Key:    key,
			},
		)
	}

	return mappings
}

// DefaultKeymap is the lookup table the shell starts with.
func DefaultKeymap() *Keymap {
	return New(Defaults()...)
}
