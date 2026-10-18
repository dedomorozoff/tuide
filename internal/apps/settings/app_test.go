package settings

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/config"
	"github.com/dedomorozoff/tuide/internal/theme"
)

// update sends one message and narrows the app Update hands back.
func update(t *testing.T, app *App, msg tea.Msg) (*App, tea.Cmd) {
	t.Helper()

	next, cmd := app.Update(msg)
	typed, ok := next.(*App)
	if !ok {
		t.Fatalf("Update returned %T, want *App", next)
	}
	return typed, cmd
}

func press(key tea.Key) tea.Msg { return tea.KeyPressMsg(key) }

// settingsWithLabel is a settings window holding a known configuration, sitting
// on the menu label row.
func settingsWithLabel(t *testing.T, label string) *App {
	t.Helper()

	app := New(theme.Get(theme.DefaultName))
	app, _ = update(t, app, appsdk.ConfigState{Config: config.Config{
		Theme:      theme.DefaultName,
		Taskbar:    true,
		Mouse:      true,
		StartLabel: label,
	}})

	for app.selected != fieldStartLabel {
		app, _ = update(t, app, press(tea.Key{Code: tea.KeyDown}))
	}
	return app
}

// typeText sends text one key at a time, the way a terminal delivers it.
func typeText(t *testing.T, app *App, text string) *App {
	t.Helper()

	for _, r := range text {
		app, _ = update(t, app, press(tea.Key{Code: r, Text: string(r)}))
	}
	return app
}

// savedConfig runs the command Update handed back and returns the configuration
// it wants written.
func savedConfig(t *testing.T, cmd tea.Cmd) config.Config {
	t.Helper()

	if cmd == nil {
		t.Fatal("no command returned, want a config save")
	}
	msg := cmd()
	saved, ok := msg.(appsdk.SaveConfig)
	if !ok {
		t.Fatalf("command returned %T, want appsdk.SaveConfig", msg)
	}
	return saved.Config
}

func TestMenuLabelOpensForTypingWithTheCurrentValue(t *testing.T) {
	app := settingsWithLabel(t, "menu")

	app, cmd := update(t, app, press(tea.Key{Code: tea.KeyEnter}))
	if !app.editing {
		t.Fatal("enter on the label row should open the label for typing")
	}
	if got := app.draft.String(); got != "menu" {
		t.Errorf("draft = %q, want the configured label", got)
	}
	if got := app.draft.Col(); got != len("menu") {
		t.Errorf("cursor = %d, want it past the label so typing extends it", got)
	}
	if cmd != nil {
		t.Error("opening the label should not save anything yet")
	}
}

func TestMenuLabelIsCommittedOnEnter(t *testing.T) {
	app := settingsWithLabel(t, "")
	app, _ = update(t, app, press(tea.Key{Code: tea.KeyEnter}))
	app = typeText(t, app, "go")

	app, cmd := update(t, app, press(tea.Key{Code: tea.KeyEnter}))

	if app.editing {
		t.Error("enter should close the label")
	}
	if app.cfg.StartLabel != "go" {
		t.Errorf("label = %q, want %q", app.cfg.StartLabel, "go")
	}
	if got := savedConfig(t, cmd).StartLabel; got != "go" {
		t.Errorf("saved label = %q, want %q", got, "go")
	}
}

func TestMenuLabelDraftIsThrownAwayOnEscape(t *testing.T) {
	app := settingsWithLabel(t, "menu")
	app, _ = update(t, app, press(tea.Key{Code: tea.KeyEnter}))
	app = typeText(t, app, "!")

	app, cmd := update(t, app, press(tea.Key{Code: tea.KeyEscape}))

	if app.editing {
		t.Error("esc should close the label")
	}
	if app.cfg.StartLabel != "menu" {
		t.Errorf("label = %q, want it untouched", app.cfg.StartLabel)
	}
	if cmd != nil {
		t.Error("esc should not save anything")
	}
}

func TestMenuLabelCanBeClearedBackToTheMark(t *testing.T) {
	app := settingsWithLabel(t, "menu")
	app, _ = update(t, app, press(tea.Key{Code: tea.KeyEnter}))

	for range len("menu") {
		app, _ = update(t, app, press(tea.Key{Code: tea.KeyBackspace}))
	}
	app, cmd := update(t, app, press(tea.Key{Code: tea.KeyEnter}))

	if app.cfg.StartLabel != "" {
		t.Errorf("label = %q, want it cleared back to the built-in mark", app.cfg.StartLabel)
	}
	if got := savedConfig(t, cmd).StartLabel; got != "" {
		t.Errorf("saved label = %q, want empty", got)
	}
}

func TestMenuLabelStaysShortAndOnOneLine(t *testing.T) {
	app := settingsWithLabel(t, "")
	app, _ = update(t, app, press(tea.Key{Code: tea.KeyEnter}))

	app, _ = update(t, app, tea.PasteMsg{Content: "start\nmenu" + strings.Repeat("x", 40)})

	draft := app.draft.String()
	if strings.ContainsAny(draft, "\r\n") {
		t.Errorf("draft = %q, want it on one line", draft)
	}
	if got := len([]rune(draft)); got != config.StartLabelMax {
		t.Errorf("draft = %q (%d runes), want the %d rune cap filled",
			draft, got, config.StartLabelMax)
	}
}

func TestMenuLabelIgnoresTypingWhileAnotherRowIsSelected(t *testing.T) {
	app := settingsWithLabel(t, "menu")
	app.selected = fieldTaskbar

	app = typeText(t, app, "z")

	if app.cfg.StartLabel != "menu" {
		t.Errorf("label = %q, want it untouched by a key meant for another row",
			app.cfg.StartLabel)
	}
}

func TestSwitchRowTogglesAndSaves(t *testing.T) {
	app := New(theme.Get(theme.DefaultName))
	app, _ = update(t, app, appsdk.ConfigState{Config: config.Config{
		Theme:   theme.DefaultName,
		Taskbar: true,
		Mouse:   true,
	}})
	app.selected = fieldTaskbar

	app, cmd := update(t, app, press(tea.Key{Code: tea.KeySpace}))

	if app.cfg.Taskbar {
		t.Error("space should turn the taskbar off")
	}
	if got := savedConfig(t, cmd).Taskbar; got {
		t.Error("the save should carry the new taskbar value")
	}
}
