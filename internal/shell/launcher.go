package shell

import (
	"strconv"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tuide/internal/fuzzy"
	"tuide/internal/input"
)

// launcherMode picks what the overlay lists: launchable apps or shell actions.
type launcherMode int

const (
	launcherApps launcherMode = iota
	launcherActions
)

// launcherState is the app launcher / action palette overlay.
type launcherState struct {
	open     bool
	mode     launcherMode
	query    []rune
	selected int
}

// launcherEntry is one row of the overlay: a label to filter and a command to
// run when the row is accepted.
type launcherEntry struct {
	label string
	run   func() tea.Cmd
}

// actionEntry is a shell command the action palette offers.
type actionEntry struct {
	label  string
	action input.Action
}

// shellActions are the commands the action palette lists. Workspaces are
// appended so the palette can address every virtual desktop.
func shellActions() []actionEntry {
	entries := []actionEntry{
		{"Close window", input.ActionClose},
		{"Maximize window", input.ActionMaximize},
		{"Minimize window", input.ActionMinimize},
		{"Snap left", input.ActionSnapLeft},
		{"Snap right", input.ActionSnapRight},
		{"Snap top", input.ActionSnapTop},
		{"Snap bottom", input.ActionSnapBottom},
		{"Snap center", input.ActionSnapCenter},
		{"Toggle layout", input.ActionToggleLayout},
		{"Toggle inspector", input.ActionToggleInspector},
	}
	for n := 1; n <= input.WorkspaceKeys; n++ {
		entries = append(entries, actionEntry{
			label:  "Workspace " + strconv.Itoa(n),
			action: input.WorkspaceAction(n),
		})
	}
	return append(entries, actionEntry{"Quit tuiDE", input.ActionQuit})
}

// openLauncher shows the overlay in the given mode with a fresh query.
func (m *Model) openLauncher(mode launcherMode) {
	m.launcher = launcherState{open: true, mode: mode}
}

// launcherEntries are the rows the overlay filters in its current mode.
func (m *Model) launcherEntries() []launcherEntry {
	if m.launcher.mode == launcherActions {
		actions := shellActions()
		entries := make([]launcherEntry, 0, len(actions))
		for _, entry := range actions {
			action := entry.action
			entries = append(entries, launcherEntry{
				label: entry.label,
				run:   func() tea.Cmd { return m.perform(action) },
			})
		}
		return entries
	}

	entries := make([]launcherEntry, 0, len(m.order))
	for _, id := range m.order {
		id := id
		entries = append(entries, launcherEntry{
			label: m.specs[id].Name,
			run:   func() tea.Cmd { return m.launch(id) },
		})
	}
	return entries
}

// rankLauncher filters the entries by the typed query, best match first.
func (m *Model) rankLauncher(entries []launcherEntry) []int {
	labels := make([]string, len(entries))
	for i, entry := range entries {
		labels[i] = entry.label
	}
	return fuzzy.Rank(string(m.launcher.query), labels)
}

// launcherKey handles every key while the overlay is open, including the ones
// the desktop keymap would otherwise claim.
func (m *Model) launcherKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.Key()
	switch key.Code {
	case tea.KeyEscape:
		m.launcher.open = false
		return nil
	case tea.KeyEnter:
		return m.launcherAccept()
	case tea.KeyUp, tea.KeyDown:
		m.launcherMove(key.Code == tea.KeyUp)
		return nil
	case tea.KeyBackspace:
		if len(m.launcher.query) > 0 {
			m.launcher.query = m.launcher.query[:len(m.launcher.query)-1]
			m.launcher.selected = 0
		}
		return nil
	}

	if action, ok := m.keymap.Lookup(msg); ok {
		switch action {
		case input.ActionQuit:
			m.shutdown = true
			return tea.Quit
		case input.ActionOpenLauncher, input.ActionOpenPalette:
			m.launcher.open = false
			return nil
		}
	}

	if hasModifier(key.Mod, tea.ModCtrl|tea.ModAlt) {
		return nil
	}
	if key.Text != "" {
		m.launcher.query = append(m.launcher.query, []rune(key.Text)...)
		m.launcher.selected = 0
	} else if unicode.IsPrint(key.Code) {
		m.launcher.query = append(m.launcher.query, key.Code)
		m.launcher.selected = 0
	}
	return nil
}

// launcherAccept runs the selected entry and closes the overlay.
func (m *Model) launcherAccept() tea.Cmd {
	entries := m.launcherEntries()
	ranked := m.rankLauncher(entries)
	selected := clampSelection(m.launcher.selected, len(ranked))

	m.launcher.open = false
	if selected < 0 {
		return nil
	}
	return entries[ranked[selected]].run()
}

// launcherMove steps the selection through the filtered rows, wrapping around.
func (m *Model) launcherMove(up bool) {
	ranked := m.rankLauncher(m.launcherEntries())
	selected := clampSelection(m.launcher.selected, len(ranked))
	if selected < 0 {
		return
	}
	delta := 1
	if up {
		delta = -1
	}
	m.launcher.selected = (selected + delta + len(ranked)) % len(ranked)
}

// drawLauncher paints the overlay: a prompt with the query and the filtered
// rows, best match first.
func (m *Model) drawLauncher() {
	entries := m.launcherEntries()
	ranked := m.rankLauncher(entries)
	selected := clampSelection(m.launcher.selected, len(ranked))

	title := "launcher"
	if m.launcher.mode == launcherActions {
		title = "actions"
	}

	lines := []string{
		m.styles.Strong.Render(title),
		"",
		m.launcherPrompt(),
		"",
	}

	if selected < 0 {
		lines = append(lines, m.styles.Hint.Render("no matches"))
	} else {
		rows := max(1, m.height-9)
		start := 0
		if selected >= rows {
			start = selected - rows + 1
		}
		for i := start; i < min(len(ranked), start+rows); i++ {
			label := entries[ranked[i]].label
			if i == selected {
				lines = append(lines, m.styles.Selection.Render("› "+label))
				continue
			}
			lines = append(lines, "  "+label)
		}
	}

	lines = append(lines, "", m.styles.Hint.Render("enter run · esc cancel · ↑↓ move"))
	m.drawPanel(strings.Join(lines, "\n"))
}

// launcherPrompt is the query line with a block cursor behind it.
func (m *Model) launcherPrompt() string {
	prompt := lipgloss.NewStyle().Foreground(m.styles.Theme.Accent).Render("❯ ")
	query := lipgloss.NewStyle().Foreground(m.styles.Theme.Text).Render(string(m.launcher.query))
	cursor := lipgloss.NewStyle().
		Foreground(m.styles.Theme.Background).
		Background(m.styles.Theme.Cursor).
		Render(" ")
	return prompt + query + cursor
}

// clampSelection keeps the selection inside the filtered rows; -1 means the
// list is empty.
func clampSelection(selected, count int) int {
	if count == 0 {
		return -1
	}
	return min(max(selected, 0), count-1)
}

// hasModifier reports whether any of mask is set, ignoring lock modifiers.
func hasModifier(mods, mask tea.KeyMod) bool {
	return mods&^(tea.ModCapsLock|tea.ModNumLock|tea.ModScrollLock)&mask != 0
}
