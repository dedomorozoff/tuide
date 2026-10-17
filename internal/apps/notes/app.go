// Package notes is a small editable text window. It is the desktop's proof
// that unclaimed keys reach the focused app.
package notes

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/textbuf"
	"github.com/dedomorozoff/tuide/internal/theme"
)

// App is a document plus the cursor the shell draws inside the window.
type App struct {
	theme  theme.Theme
	buffer *textbuf.Buffer
	scroll int
}

// Spec is the launcher entry for this app.
func Spec(t theme.Theme) appsdk.Spec {
	return appsdk.Spec{
		ID:            "notes",
		Name:          "Notes",
		DefaultWidth:  44,
		DefaultHeight: 14,
		MinWidth:      24,
		MinHeight:     6,
		Launch:        func() appsdk.App { return New(t) },
	}
}

// New returns an empty document.
func New(t theme.Theme) *App {
	return &App{theme: t, buffer: textbuf.New()}
}

// Init implements appsdk.App.
func (a *App) Init() tea.Cmd { return nil }

// Close implements appsdk.App.
func (a *App) Close() tea.Cmd { return nil }

// Update edits the document.
func (a *App) Update(msg tea.Msg) (appsdk.App, tea.Cmd) {
	press, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return a, nil
	}

	switch key := press.Key(); key.Code {
	case tea.KeyEnter:
		a.buffer.Newline()
	case tea.KeyBackspace:
		a.buffer.Backspace()
	case tea.KeyLeft:
		a.buffer.MoveLeft()
	case tea.KeyRight:
		a.buffer.MoveRight()
	case tea.KeyUp:
		a.buffer.MoveUp()
	case tea.KeyDown:
		a.buffer.MoveDown()
	case tea.KeyHome:
		a.buffer.MoveHome()
	case tea.KeyEnd:
		a.buffer.MoveEnd()
	case tea.KeySpace:
		a.buffer.Insert(" ")
	default:
		if key.Text != "" {
			a.buffer.Insert(key.Text)
		}
	}
	return a, nil
}

// View draws the visible slice of the document with a block cursor.
func (a *App) View(ctx appsdk.Context) tea.View {
	width, height := ctx.Rect.Width, ctx.Rect.Height
	if width <= 0 || height <= 0 {
		return tea.NewView("")
	}

	rows := height
	footer := ""
	if height >= 3 {
		rows = height - 1
		footer = lipgloss.NewStyle().
			Foreground(a.theme.TextDim).
			Render(a.status())
	}

	a.followCursor(rows)

	lines := make([]string, 0, height)
	for i := 0; i < rows; i++ {
		row := a.scroll + i
		if row >= a.buffer.Rows() {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, a.renderLine(a.buffer.Line(row), row == a.buffer.Row()))
	}
	if footer != "" {
		lines = append(lines, footer)
	}

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// followCursor scrolls the window so the cursor line stays visible.
func (a *App) followCursor(rows int) {
	if rows <= 0 {
		a.scroll = 0
		return
	}
	if a.buffer.Row() < a.scroll {
		a.scroll = a.buffer.Row()
	}
	if a.buffer.Row() >= a.scroll+rows {
		a.scroll = a.buffer.Row() - rows + 1
	}
	if a.scroll < 0 {
		a.scroll = 0
	}
}

func (a *App) renderLine(line string, cursor bool) string {
	text := lipgloss.NewStyle().Foreground(a.theme.Text)

	if !cursor {
		return text.Render(line)
	}

	runes := []rune(line)
	col := min(a.buffer.Col(), len(runes))
	block := lipgloss.NewStyle().
		Foreground(a.theme.Background).
		Background(a.theme.Cursor)

	if col >= len(runes) {
		return text.Render(string(runes)) + block.Render(" ")
	}
	return text.Render(string(runes[:col])) +
		block.Render(string(runes[col])) +
		text.Render(string(runes[col+1:]))
}

func (a *App) status() string {
	return fmt.Sprintf("Ln %d, Col %d | %d lines",
		a.buffer.Row()+1, a.buffer.Col()+1, a.buffer.Rows())
}
