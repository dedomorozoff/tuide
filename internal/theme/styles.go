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
	TaskbarWarning     lipgloss.Style

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

		// The border styles carry the background on purpose. A cell with no
		// style at all is rendered as a full reset, so a window frame without
		// one hands its whole interior back to the host terminal's colours: the
		// window body came out in whatever foreground and background the
		// terminal happened to use, which on a light profile reads as white
		// text on a white block.
		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Background(t.Background),

		BorderFocus: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderFocused).
			Background(t.Background),

		Title:       lipgloss.NewStyle().Foreground(t.Text).Background(t.TitleBar),
		TitleFocus:  lipgloss.NewStyle().Foreground(t.TextStrong).Background(t.TitleBarFocus),
		TitleDim:    lipgloss.NewStyle().Foreground(t.TextDim).Background(t.TitleBar),
		TitleAccent: lipgloss.NewStyle().Foreground(t.TextStrong).Background(t.TitleBarFocus),

		Taskbar:            lipgloss.NewStyle().Foreground(t.Text).Background(t.Taskbar),
		TaskbarText:        lipgloss.NewStyle().Foreground(t.TextDim).Background(t.Taskbar),
		TaskbarItem:        lipgloss.NewStyle().Foreground(t.Text).Background(t.Taskbar).Padding(0, 1),
		TaskbarItemFocused: lipgloss.NewStyle().Foreground(t.TextStrong).Background(t.AccentDim).Padding(0, 1),
		// The status light carries its own background: it is drawn straight
		// onto the bar rather than inside a box that would supply one.
		TaskbarWarning: lipgloss.NewStyle().Foreground(t.Warning).Background(t.Taskbar),

		Hint:      lipgloss.NewStyle().Foreground(t.TextDim),
		Danger:    lipgloss.NewStyle().Foreground(t.Danger),
		Warning:   lipgloss.NewStyle().Foreground(t.Warning),
		Strong:    lipgloss.NewStyle().Foreground(t.TextStrong).Bold(true),
		Selection: lipgloss.NewStyle().Foreground(t.TextStrong).Background(t.Selection),
	}
}

// WindowChrome returns the lipgloss styles for a window at a given size.
// Focused and inactive windows get different border and title colors.
//
// The frame is a plain border box: the caller blits the title over the top
// border afterwards, so the title sits on the frame rather than consuming a
// row inside it. lipgloss counts the border into the width and the height it
// is given, so the frame takes the whole rectangle.
func (s Styles) WindowChrome(focused bool, width, height int) (frame, title lipgloss.Style) {
	frameStyle := s.Border
	titleStyle := s.TitleDim
	if focused {
		frameStyle = s.BorderFocus
		titleStyle = s.TitleFocus
	}
	frame = frameStyle.Width(max(0, width)).Height(max(0, height))
	title = titleStyle.Width(max(0, width-2))
	return frame, title
}
