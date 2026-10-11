// Package clock is a window that shows the current local time and date. It
// refreshes from the tick the desktop broadcasts every second.
package clock

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/theme"
)

// App renders the time it was last told about.
type App struct {
	theme theme.Theme
	now   time.Time
}

// Spec is the launcher entry for this app.
func Spec(t theme.Theme) appsdk.Spec {
	return appsdk.Spec{
		ID:            "clock",
		Name:          "Clock",
		DefaultWidth:  28,
		DefaultHeight: 9,
		MinWidth:      16,
		MinHeight:     5,
		Launch:        func() appsdk.App { return New(t) },
	}
}

// New returns a clock reading the wall clock until the first tick arrives.
func New(t theme.Theme) *App { return &App{theme: t, now: time.Now()} }

// Init implements appsdk.App.
func (a *App) Init() tea.Cmd { return nil }

// Close implements appsdk.App.
func (a *App) Close() tea.Cmd { return nil }

// Update stores the time carried by a tick.
func (a *App) Update(msg tea.Msg) (appsdk.App, tea.Cmd) {
	if tick, ok := msg.(appsdk.Tick); ok {
		a.now = tick.Time
	}
	return a, nil
}

// View centres the time and the date inside the window.
func (a *App) View(ctx appsdk.Context) tea.View {
	width := ctx.Rect.Width
	if width <= 0 || ctx.Rect.Height <= 0 {
		return tea.NewView("")
	}

	timeText := lipgloss.NewStyle().
		Foreground(a.theme.TextStrong).
		Bold(true).
		Render(a.now.Format("15:04:05"))
	dateText := lipgloss.NewStyle().
		Foreground(a.theme.TextDim).
		Render(a.now.Format("Mon 02 Jan 2006"))

	lines := []string{center(timeText, width), center(dateText, width)}

	// Put the pair in the middle of the window when there is room.
	if pad := (ctx.Rect.Height - len(lines)) / 2; pad > 0 {
		lines = append(make([]string, pad, pad+len(lines)), lines...)
	}

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func center(text string, width int) string {
	visible := lipgloss.Width(text)
	if visible >= width {
		return text
	}
	return strings.Repeat(" ", (width-visible)/2) + text
}
