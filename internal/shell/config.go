package shell

import (
	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/config"
	"github.com/dedomorozoff/tuide/internal/theme"
)

// configSavedMsg carries the result of the background config write.
type configSavedMsg struct {
	err error
}

// applyConfig pushes a new configuration into the running desktop: palette,
// taskbar and mouse take effect immediately, the startup list applies on the
// next launch. It never touches the disk; persistence happens in a command.
func (m *Model) applyConfig(cfg config.Config) {
	m.cfg = cfg
	m.styles = theme.NewStyles(theme.Get(cfg.Theme))
	m.options.Taskbar = cfg.Taskbar
	m.options.Mouse = cfg.Mouse
	m.windows.SetWorkArea(workArea(m.width, m.height, cfg.Taskbar))
}

// replyToFocused delivers a message to the focused window's app. Configuration
// requests are answered this way: the settings app must be focused to ask.
func (m *Model) replyToFocused(msg tea.Msg) tea.Cmd {
	w := m.windows.Focused()
	if w == nil || w.App == nil {
		return nil
	}
	app, cmd := w.App.Update(msg)
	w.App = app
	return cmd
}
