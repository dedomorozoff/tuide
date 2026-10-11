package theme

import (
	"sort"

	"charm.land/lipgloss/v2"
)

const DefaultName = "midnight"

// Get returns the named palette, falling back to the default one.
func Get(name string) Theme {
	if t, ok := palettes[name]; ok {
		return t
	}
	return palettes[DefaultName]
}

// Names lists every registered palette in alphabetical order.
func Names() []string {
	names := make([]string, 0, len(palettes))
	for name := range palettes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Styles holds the lipgloss styles derived from a palette.
type Styles struct {
	Theme Theme

	Panel       lipgloss.Style
	Border      lipgloss.Style
	BorderFocus lipgloss.Style

	Title       lipgloss.Style
	TitleFocus  lipgloss.Style
	TitleDim    lipgloss.Style
	TitleAccent lipgloss.Style

	Taskbar            lipgloss.Style
	TaskbarText        lipgloss.Style
	TaskbarItem        lipgloss.Style
	TaskbarItemFocused lipgloss.Style

	Hint      lipgloss.Style
	Danger    lipgloss.Style
	Warning   lipgloss.Style
	Strong    lipgloss.Style
	Selection lipgloss.Style
}

// NewStyles derives every component style from a palette.
func NewStyles(t Theme) Styles {
	return Styles{
		Theme: t,

		Panel: lipgloss.NewStyle().Foreground(t.Text).Background(t.Panel),

		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border),
		BorderFocus: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderFocused),

		Title:       lipgloss.NewStyle().Foreground(t.Text).Background(t.TitleBar),
		TitleFocus:  lipgloss.NewStyle().Foreground(t.TextStrong).Background(t.TitleBarFocus),
		TitleDim:    lipgloss.NewStyle().Foreground(t.TextDim).Background(t.TitleBar),
		TitleAccent: lipgloss.NewStyle().Foreground(t.TextStrong).Background(t.TitleBarFocus),

		Taskbar:            lipgloss.NewStyle().Foreground(t.Text).Background(t.Taskbar),
		TaskbarText:        lipgloss.NewStyle().Foreground(t.TextDim).Background(t.Taskbar),
		TaskbarItem:        lipgloss.NewStyle().Foreground(t.Text).Background(t.Taskbar).Padding(0, 1),
		TaskbarItemFocused: lipgloss.NewStyle().Foreground(t.TextStrong).Background(t.AccentDim).Padding(0, 1),

		Hint:      lipgloss.NewStyle().Foreground(t.TextDim),
		Danger:    lipgloss.NewStyle().Foreground(t.Danger),
		Warning:   lipgloss.NewStyle().Foreground(t.Warning),
		Strong:    lipgloss.NewStyle().Foreground(t.TextStrong).Bold(true),
		Selection: lipgloss.NewStyle().Foreground(t.TextStrong).Background(t.Selection),
	}
}

// WindowChrome returns the lipgloss styles for a window at a given size.
// Focused and inactive windows get different border and title colors.
func (s Styles) WindowChrome(focused bool, width, height int) (frame, title lipgloss.Style) {
	frameStyle := s.Border
	titleStyle := s.TitleDim
	if focused {
		frameStyle = s.BorderFocus
		titleStyle = s.TitleFocus
	}
	frame = frameStyle.Width(max(0, width-2)).Height(max(0, height-2))
	title = titleStyle.Width(max(0, width-2))
	return frame, title
}
