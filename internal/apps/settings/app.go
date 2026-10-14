// Package settings provides a desktop configuration application. It requests
// the current configuration on launch and pushes changes back to the shell
// immediately, saving them to disk.
package settings

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/config"
	"github.com/dedomorozoff/tuide/internal/theme"
)

// App is the settings window model.
type App struct {
	theme    theme.Theme
	cfg      config.Config
	themes   []string
	selected int
	status   string
}

// Spec is the launcher entry for the settings application.
func Spec(t theme.Theme) appsdk.Spec {
	return appsdk.Spec{
		ID:            "settings",
		Name:          "Settings",
		DefaultWidth:  42,
		DefaultHeight: 14,
		MinWidth:      30,
		MinHeight:     10,
		Launch:        func() appsdk.App { return New(t) },
	}
}

// New creates an uninitialized settings app.
func New(t theme.Theme) *App {
	return &App{
		theme:  t,
		themes: theme.Names(),
		cfg:    config.Default(),
	}
}

// Init asks the desktop shell for the current config state.
func (a *App) Init() tea.Cmd {
	return func() tea.Msg {
		return appsdk.RequestConfig{}
	}
}

// Close implements appsdk.App.
func (a *App) Close() tea.Cmd { return nil }

const (
	fieldTheme = iota
	fieldTaskbar
	fieldMouse
	fieldCount
)

// Update handles config state, saving confirmations and keyboard navigation.
func (a *App) Update(msg tea.Msg) (appsdk.App, tea.Cmd) {
	switch msg := msg.(type) {
	case appsdk.ConfigState:
		a.cfg = msg.Config
		a.theme = theme.Get(a.cfg.Theme)
		return a, nil

	case appsdk.ConfigSaved:
		if msg.Err != nil {
			a.status = fmt.Sprintf("Error: %v", msg.Err)
		} else {
			a.status = "Saved"
		}
		return a, nil

	case tea.KeyPressMsg:
		switch msg.Key().Code {
		case tea.KeyUp, 'k':
			if a.selected > 0 {
				a.selected--
			} else {
				a.selected = fieldCount - 1
			}
		case tea.KeyDown, 'j':
			if a.selected < fieldCount-1 {
				a.selected++
			} else {
				a.selected = 0
			}
		case tea.KeyLeft, tea.KeyRight, tea.KeySpace, tea.KeyEnter, 'h', 'l':
			return a.toggleCurrent()
		}
	}
	return a, nil
}

func (a *App) toggleCurrent() (appsdk.App, tea.Cmd) {
	switch a.selected {
	case fieldTheme:
		idx := 0
		for i, name := range a.themes {
			if name == a.cfg.Theme {
				idx = i
				break
			}
		}
		idx = (idx + 1) % len(a.themes)
		a.cfg.Theme = a.themes[idx]
		a.theme = theme.Get(a.cfg.Theme)

	case fieldTaskbar:
		a.cfg.Taskbar = !a.cfg.Taskbar

	case fieldMouse:
		a.cfg.Mouse = !a.cfg.Mouse
	}

	cfgToSave := a.cfg
	return a, func() tea.Msg {
		return appsdk.SaveConfig{Config: cfgToSave}
	}
}

// View renders the settings form.
func (a *App) View(ctx appsdk.Context) tea.View {
	width := ctx.Rect.Width
	if width <= 0 || ctx.Rect.Height <= 0 {
		return tea.NewView("")
	}

	accent := lipgloss.NewStyle().Foreground(a.theme.Accent).Bold(true)
	dim := lipgloss.NewStyle().Foreground(a.theme.TextDim)
	text := lipgloss.NewStyle().Foreground(a.theme.Text)
	highlight := lipgloss.NewStyle().Foreground(a.theme.TextStrong).Background(a.theme.Selection)

	lines := []string{
		accent.Render("Desktop Settings"),
		"",
	}

	items := []struct {
		label string
		value string
	}{
		{"Theme:  ", fmt.Sprintf("< %s >", a.cfg.Theme)},
		{"Taskbar:", fmt.Sprintf("[%s]", boolLabel(a.cfg.Taskbar))},
		{"Mouse:  ", fmt.Sprintf("[%s]", boolLabel(a.cfg.Mouse))},
	}

	for i, item := range items {
		cursor := "  "
		if i == a.selected && ctx.Focused {
			cursor = "> "
		}

		line := fmt.Sprintf("%s%-10s %s", cursor, item.label, item.value)
		if i == a.selected && ctx.Focused {
			lines = append(lines, highlight.Render(line))
		} else {
			lines = append(lines, text.Render(line))
		}
	}

	lines = append(lines, "")
	if a.status != "" {
		lines = append(lines, dim.Render("• "+a.status))
	} else {
		lines = append(lines, dim.Render("↑/↓ select • space/enter toggle"))
	}

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func boolLabel(b bool) string {
	if b {
		return "Enabled"
	}
	return "Disabled"
}
