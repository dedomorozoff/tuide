// Package settings provides a desktop configuration application. It requests
// the current configuration on launch and pushes changes back to the shell
// immediately, saving them to disk.
package settings

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/config"
	"github.com/dedomorozoff/tuide/internal/textbuf"
	"github.com/dedomorozoff/tuide/internal/theme"
)

// App is the settings window model.
type App struct {
	theme    theme.Theme
	cfg      config.Config
	themes   []string
	selected int
	status   string
	// editing is true while the menu label is being typed into; draft holds
	// what has been typed so far, and only reaches the config on enter.
	editing bool
	draft   *textbuf.Buffer
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
		draft:  textbuf.New(),
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
	fieldStartLabel
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

	case tea.PasteMsg:
		if a.editing {
			a.insert(msg.Content)
		}
		return a, nil

	case tea.KeyPressMsg:
		if a.editing {
			return a.editKey(msg)
		}

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
			return a.activateField()
		}
	}
	return a, nil
}

// activateField acts on the selected row: the theme cycles, the switches flip
// and the menu label opens for typing.
func (a *App) activateField() (appsdk.App, tea.Cmd) {
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

	case fieldStartLabel:
		a.editing = true
		a.draft.Load(a.cfg.StartLabel)
		// The cursor goes to the end of the label: typing extends what is
		// already there, the way it does in every other text field.
		a.draft.MoveEnd()
		return a, nil
	}

	return a, a.saveCmd()
}

// editKey types into the menu label. Enter commits the draft and saves, esc
// throws it away, everything else is the draft's own business.
func (a *App) editKey(msg tea.KeyPressMsg) (appsdk.App, tea.Cmd) {
	key := msg.Key()

	switch key.Code {
	case tea.KeyEscape:
		a.editing = false
		return a, nil
	case tea.KeyEnter:
		a.editing = false
		a.cfg.StartLabel = a.draft.String()
		return a, a.saveCmd()
	case tea.KeyBackspace:
		a.draft.Backspace()
	case tea.KeyDelete:
		a.draft.Delete()
	case tea.KeyLeft:
		a.draft.MoveLeft()
	case tea.KeyRight:
		a.draft.MoveRight()
	case tea.KeyHome:
		a.draft.MoveHome()
	case tea.KeyEnd:
		a.draft.MoveEnd()
	default:
		if key.Text != "" {
			a.insert(key.Text)
		} else if unicode.IsPrint(key.Code) {
			a.insert(string(key.Code))
		}
	}
	return a, nil
}

// insert adds text to the label draft, keeping it on one line and inside the
// length the config allows: a menu button cannot be two rows tall, and the
// taskbar is not the place to find that out.
func (a *App) insert(text string) {
	text = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(text)

	room := config.StartLabelMax - len([]rune(a.draft.String()))
	if room <= 0 {
		return
	}
	runes := []rune(text)
	if len(runes) > room {
		runes = runes[:room]
	}
	a.draft.Insert(string(runes))
}

// saveCmd hands the whole configuration back to the shell, which applies it and
// writes it out.
func (a *App) saveCmd() tea.Cmd {
	cfg := a.cfg
	return func() tea.Msg {
		return appsdk.SaveConfig{Config: cfg}
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
		{"Start:  ", startLabelValue(a.cfg.StartLabel)},
	}

	for i, item := range items {
		if i == a.selected && ctx.Focused && a.editing {
			lines = append(lines, a.editingLine(item.label, highlight))
			continue
		}

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
	lines = append(lines, dim.Render(a.hint()))

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// editingLine draws the row being typed into. It is assembled from styled
// pieces instead of wrapping one string in the selection style, because a
// nested style drops its background where the inner reset lands: the cursor
// would punch a hole through the rest of the row.
func (a *App) editingLine(label string, selection lipgloss.Style) string {
	cursor := lipgloss.NewStyle().
		Foreground(a.theme.Background).
		Background(a.theme.Cursor)

	draft := []rune(a.draft.String())
	col := min(a.draft.Col(), len(draft))

	under := " "
	after := ""
	if col < len(draft) {
		under = string(draft[col])
		after = string(draft[col+1:])
	}

	return selection.Render(fmt.Sprintf("> %-10s [", label)) +
		selection.Render(string(draft[:col])) +
		cursor.Render(under) +
		selection.Render(after) +
		selection.Render("]")
}

// hint is the line under the form: what the keys do right now.
func (a *App) hint() string {
	switch {
	case a.status != "":
		return "* " + a.status
	case a.editing:
		return "enter save | esc cancel | left/right move"
	default:
		return "up/down select | space/enter toggle or edit"
	}
}

// startLabelValue is the value shown for the menu button's text.
func startLabelValue(label string) string {
	if label == "" {
		return "[built-in mark]"
	}
	return "[" + label + "]"
}

func boolLabel(b bool) string {
	if b {
		return "Enabled"
	}
	return "Disabled"
}
