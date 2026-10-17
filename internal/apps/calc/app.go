// Package calc is an arithmetic calculator window: type an expression, press
// enter, read the result.
package calc

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/expr"
	"github.com/dedomorozoff/tuide/internal/theme"
)

// App holds the expression being typed and the last result.
type App struct {
	theme  theme.Theme
	input  []rune
	result string
	failed bool
}

// Spec is the launcher entry for this app.
func Spec(t theme.Theme) appsdk.Spec {
	return appsdk.Spec{
		ID:            "calc",
		Name:          "Calculator",
		DefaultWidth:  30,
		DefaultHeight: 8,
		MinWidth:      16,
		MinHeight:     4,
		Launch:        func() appsdk.App { return New(t) },
	}
}

// New returns an empty calculator.
func New(t theme.Theme) *App { return &App{theme: t} }

// Init implements appsdk.App.
func (a *App) Init() tea.Cmd { return nil }

// Close implements appsdk.App.
func (a *App) Close() tea.Cmd { return nil }

// Update edits the expression and evaluates it on enter.
func (a *App) Update(msg tea.Msg) (appsdk.App, tea.Cmd) {
	press, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return a, nil
	}

	switch key := press.Key(); key.Code {
	case tea.KeyEnter:
		a.evaluate()
	case tea.KeyBackspace:
		if len(a.input) > 0 {
			a.input = a.input[:len(a.input)-1]
			a.result = ""
			a.failed = false
		}
	case tea.KeyEscape:
		a.input = nil
		a.result = ""
		a.failed = false
	case tea.KeySpace:
		a.input = append(a.input, ' ')
	default:
		if key.Text != "" {
			a.input = append(a.input, []rune(key.Text)...)
			a.result = ""
			a.failed = false
		}
	}
	return a, nil
}

func (a *App) evaluate() {
	value, err := expr.Eval(a.expression())
	if err != nil {
		a.result = err.Error()
		a.failed = true
		return
	}
	a.result = strconv.FormatFloat(value, 'g', -1, 64)
	a.failed = false
}

func (a *App) expression() string {
	// Leading and trailing spaces are just noise for the evaluator.
	return strings.TrimSpace(string(a.input))
}

// View draws the expression, the result and a short hint.
func (a *App) View(ctx appsdk.Context) tea.View {
	if ctx.Rect.Width <= 0 || ctx.Rect.Height <= 0 {
		return tea.NewView("")
	}

	text := lipgloss.NewStyle().Foreground(a.theme.Text)
	prompt := lipgloss.NewStyle().Foreground(a.theme.Accent).Render("> ")
	cursor := lipgloss.NewStyle().
		Foreground(a.theme.Background).
		Background(a.theme.Cursor).
		Render(" ")

	lines := []string{
		prompt + text.Render(string(a.input)) + cursor,
	}

	if a.result != "" {
		style := lipgloss.NewStyle().Foreground(a.theme.TextStrong).Bold(true)
		prefix := lipgloss.NewStyle().Foreground(a.theme.TextDim).Render("= ")
		if a.failed {
			style = lipgloss.NewStyle().Foreground(a.theme.Danger)
			prefix = lipgloss.NewStyle().Foreground(a.theme.Danger).Render("! ")
		}
		lines = append(lines, "", prefix+style.Render(a.result))
	}

	if ctx.Rect.Height >= 5 {
		hint := lipgloss.NewStyle().Foreground(a.theme.TextDim).
			Render("+ - * / ( )  |  enter = result  |  esc = clear")
		lines = append(lines, "", hint)
	}

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
