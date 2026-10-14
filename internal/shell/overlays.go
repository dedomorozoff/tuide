package shell

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/dedomorozoff/tuide/internal/geom"
	"github.com/dedomorozoff/tuide/internal/wm"
)

// drawTabHUD lists the visible windows while alt-tab cycles the focus. It
// stays up until the next key press, so holding the rhythm of alt-tab keeps it
// readable.
func (m *Model) drawTabHUD() {
	focused := m.windows.Focused()

	lines := make([]string, 0, m.windows.Count())
	index := 0
	for _, w := range m.windows.Windows() {
		if w.State == wm.Minimized {
			continue
		}
		index++

		mark := "  "
		style := m.styles.Panel
		if w == focused {
			mark = "› "
			style = m.styles.Selection
		}
		lines = append(lines, style.Render(fmt.Sprintf("%s%d  %s", mark, index, w.Spec.Name)))
	}
	if len(lines) == 0 {
		return
	}
	m.drawPanel(strings.Join(lines, "\n"))
}

// drawInspector shows the window manager state: workspace, layout and every
// window with its geometry, stacking position and state.
func (m *Model) drawInspector() {
	focused := m.windows.Focused()

	focus := "-"
	if focused != nil {
		focus = focused.Spec.Name
	}

	lines := []string{
		m.styles.Strong.Render("tuiDE inspector"),
		fmt.Sprintf("workspace  %d/%d", m.windows.Active()+1, wm.Workspaces),
		fmt.Sprintf("layout     %s", m.windows.Layout()),
		fmt.Sprintf("windows    %d", m.windows.Count()),
		fmt.Sprintf("focus      %s", focus),
		"",
		"z   window        rect           state",
	}

	for i, w := range m.windows.Windows() {
		mark := " "
		if w == focused {
			mark = "●"
		}
		lines = append(lines, fmt.Sprintf("%s%-2d %-12s %3d,%-3d %3dx%-3d %s",
			mark,
			i+1,
			truncate(w.Spec.Name, 12),
			w.Rect.X, w.Rect.Y,
			w.Rect.Width, w.Rect.Height,
			stateName(w.State),
		))
	}

	m.drawPanel(strings.Join(lines, "\n"))
}

// drawPanel centres a box on the screen. Every overlay goes through it so they
// share one look and one placement rule.
func (m *Model) drawPanel(content string) {
	box := m.panelStyle().Render(content)

	width := min(lipgloss.Width(box), m.width)
	height := min(lipgloss.Height(box), m.height)

	rect := geom.Rect{
		X:      max(0, (m.width-width)/2),
		Y:      max(0, (m.height-height)/2),
		Width:  width,
		Height: height,
	}
	m.screen.Blit(rect, box)
}

func (m *Model) panelStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(m.styles.Theme.Text).
		Background(m.styles.Theme.Panel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Theme.Accent).
		Padding(0, 1).
		MaxWidth(max(0, m.width-4))
}

func stateName(state wm.State) string {
	switch state {
	case wm.Maximized:
		return "maximized"
	case wm.Minimized:
		return "minimized"
	}
	return "normal"
}

// truncate cuts a string down to at most limit display cells, marking that
// something was left off with an ellipsis.
func truncate(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= limit {
		return s
	}
	if limit == 1 {
		return "…"
	}

	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > limit {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}
