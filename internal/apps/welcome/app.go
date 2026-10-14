// Package welcome is the placeholder app the desktop starts with. It exists so
// the window manager has something to lay out before any real app is written.
package welcome

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/theme"
)

// App renders a short greeting and reports its own size.
type App struct {
	Theme theme.Theme
}

// Spec is the launcher entry for this app.
func Spec(t theme.Theme) appsdk.Spec {
	return appsdk.Spec{
		ID:            "welcome",
		Name:          "Welcome",
		DefaultWidth:  46,
		DefaultHeight: 12,
		MinWidth:      24,
		MinHeight:     8,
		Launch:        func() appsdk.App { return App{Theme: t} },
	}
}

func (a App) Init() tea.Cmd { return nil }

func (a App) Update(msg tea.Msg) (appsdk.App, tea.Cmd) { return a, nil }

func (a App) View(ctx appsdk.Context) tea.View {
	heading := lipgloss.NewStyle().Foreground(a.Theme.Accent).Render("tuiDE " + a.Theme.Name)
	body := lipgloss.NewStyle().Foreground(a.Theme.TextDim).Render(
		fmt.Sprintf("%dx%d cells, focused=%v", ctx.Rect.Width, ctx.Rect.Height, ctx.Focused))
	// Quitting lives in the action palette, so the window points at the two
	// shortcuts that actually get the desktop moving instead.
	hint := lipgloss.NewStyle().Foreground(a.Theme.Text).Render("ctrl+alt+space launcher · ctrl+alt+a actions")

	return tea.NewView(lipgloss.JoinVertical(
		lipgloss.Left,
		heading,
		body,
		"",
		hint,
	))
}

func (a App) Close() tea.Cmd { return nil }
