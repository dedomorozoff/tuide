package shell

import (
	"strconv"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dedomorozoff/tuide/internal/fuzzy"
	"github.com/dedomorozoff/tuide/internal/geom"
	"github.com/dedomorozoff/tuide/internal/input"
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

// launcherAccept runs the row the keyboard selection is on and closes the
// overlay. A click on a row goes through the same path.
func (m *Model) launcherAccept() tea.Cmd {
	layout := m.launcherLayout()
	m.launcher.open = false

	for _, row := range layout.rows {
		if row.index == layout.selected {
			return m.runLauncherRow(row)
		}
	}
	return nil
}

// runLauncherRow runs one entry by its position in the entry list.
func (m *Model) runLauncherRow(row launcherRow) tea.Cmd {
	entries := m.launcherEntries()
	if row.entry < 0 || row.entry >= len(entries) {
		return nil
	}
	return entries[row.entry].run()
}

// onLauncherClick runs the row the pointer is on. A click that lands on the
// title, the prompt or the frame around the overlay dismisses it without
// running anything, the same as esc.
func (m *Model) onLauncherClick(mouse tea.Mouse) tea.Cmd {
	layout := m.launcherLayout()
	m.launcher.open = false

	for _, row := range layout.rows {
		if row.rect.Contains(mouse.X, mouse.Y) {
			return m.runLauncherRow(row)
		}
	}
	return nil
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

// launcherRow is one visible entry of the overlay together with the rectangle
// of cells it occupies.
type launcherRow struct {
	label string
	// entry indexes the entry list, index positions the row in the ranked
	// list. The keyboard selection is an index, a click is a rectangle.
	entry    int
	index    int
	selected bool
	rect     geom.Rect
}

// launcherLayout is the overlay in one value: the content to render, where the
// panel lands and which cells each row owns. Drawing and click handling share
// it, so a click always agrees with what is on screen.
type launcherLayout struct {
	content string
	rect    geom.Rect
	rows    []launcherRow
	// selected is the keyboard selection as a position in the ranked list, or
	// -1 when the query matches nothing.
	selected int
}

// launcherFirstRow is the content line the rows start on: title, blank, prompt,
// blank.
const launcherFirstRow = 4

// launcherRect anchors the overlay in the bottom-left corner, above the taskbar
// and the start button that opened it, the way a start menu opens. Without a
// taskbar it sits in the corner of the work area all the same.
func (m *Model) launcherRect(content string) geom.Rect {
	box := m.panelStyle().Render(content)

	bottom := m.windows.WorkArea().Bottom()
	height := min(lipgloss.Height(box), max(0, bottom))
	width := min(lipgloss.Width(box), m.width)

	return geom.Rect{
		X:      0,
		Y:      max(0, bottom-height),
		Width:  width,
		Height: height,
	}
}

// launcherLayout builds the overlay: the filtered rows, the string to render
// and the cells those rows occupy.
func (m *Model) launcherLayout() launcherLayout {
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

	layout := launcherLayout{selected: selected}

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
			layout.rows = append(layout.rows, launcherRow{
				label:    label,
				entry:    ranked[i],
				index:    i,
				selected: i == selected,
			})
			if i == selected {
				lines = append(lines, m.styles.Selection.Render("> "+label))
				continue
			}
			lines = append(lines, "  "+label)
		}
	}

	lines = append(lines, "", m.styles.Hint.Render("enter run | esc cancel | up/down move"))
	layout.content = strings.Join(lines, "\n")
	if m.launcher.mode == launcherApps {
		layout.rect = m.launcherRect(layout.content)
	} else {
		layout.rect = m.panelRect(layout.content)
	}

	for i := range layout.rows {
		layout.rows[i].rect = geom.Rect{
			X:      layout.rect.X + panelInset,
			Y:      layout.rect.Y + panelTopInset + launcherFirstRow + i,
			Width:  max(0, layout.rect.Width-2*panelInset),
			Height: 1,
		}
	}
	return layout
}

// drawLauncher paints the overlay: a prompt with the query and the filtered
// rows, best match first.
func (m *Model) drawLauncher() {
	layout := m.launcherLayout()
	m.screen.Blit(layout.rect, m.panelStyle().Render(layout.content))
}

// launcherPrompt is the query line with a block cursor behind it.
func (m *Model) launcherPrompt() string {
	prompt := lipgloss.NewStyle().Foreground(m.styles.Theme.Accent).Render("> ")
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
