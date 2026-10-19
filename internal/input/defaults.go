package input

import (
	tea "charm.land/bubbletea/v2"
)

// Defaults are the bindings the desktop starts with. They are deliberately
// conservative: the focused app gets every key that is not claimed here.
//
// The desktop modifier is Ctrl+Alt. Bare Ctrl chords belong to the program
// inside the window: ctrl+c interrupts and ctrl+w deletes a word. Ctrl+q is
// the exception: it ends the session, the way ctrl+alt+q always did, so the
// way out is one chord away. Three bindings break that rule on purpose, and
// each one buys something the desktop cannot get otherwise:
//
//   - alt+tab cycles the focus, and no terminal program binds it;
//   - ctrl+p opens the launcher, which is the way in on hosts that cannot
//     report Ctrl+Alt at all — a Linux virtual console, a serial link — at
//     the price of one readline key, the previous history entry;
//   - alt+1..9 activate a window by its taskbar slot, the price being tmux's
//     own alt+digit window keys and the tab keys of terminals like
//     gnome-terminal.
func Defaults() []Mapping {
	mappings := []Mapping{
		{Action: ActionQuit, Mods: tea.ModCtrl | tea.ModAlt, Key: 'q'},
		{Action: ActionQuit, Mods: tea.ModCtrl, Key: 'q'},

		{Action: ActionFocusNext, Mods: tea.ModAlt, Key: tea.KeyTab},
		{Action: ActionFocusPrev, Mods: tea.ModAlt | tea.ModShift, Key: tea.KeyTab},
		{Action: ActionFocusLast, Mods: tea.ModCtrl | tea.ModAlt, Key: 'p'},

		{Action: ActionClose, Mods: tea.ModCtrl | tea.ModAlt, Key: 'w'},
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
		// ctrl+p is the console-friendly way into the launcher: a host that
		// cannot report Ctrl+Alt chords still delivers a bare Ctrl.
		{Action: ActionOpenLauncher, Mods: tea.ModCtrl, Key: 'p'},
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

	// alt+1..9 pick a window by its slot on the taskbar, the way win+1 picks a
	// taskbar button. The slot is fixed when the window opens, so a number
	// keeps meaning the same window until the workspace changes.
	for n := 1; n <= WindowKeys; n++ {
		mappings = append(mappings, Mapping{
			Action: FocusWindowAction(n),
			Mods:   tea.ModAlt,
			Key:    rune('0' + n),
		})
	}

	return mappings
}

// DefaultKeymap is the lookup table the shell starts with.
func DefaultKeymap() *Keymap {
	return New(Defaults()...)
}
