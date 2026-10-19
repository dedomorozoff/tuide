package shell

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/session"
	"github.com/dedomorozoff/tuide/internal/wm"
)

// saveEveryTicks is how often a persisting desktop writes its session file.
const saveEveryTicks = 30

// sessionSavedMsg reports the outcome of a background session save.
type sessionSavedMsg struct{ err error }

// WithRestore makes the desktop rebuild the saved session on the first resize.
func (m *Model) WithRestore(snap session.Snapshot) *Model {
	m.restore = &snap
	return m
}

// Snapshot captures the desktop for the session file. Window geometry comes
// from the floating rect: it is the user's own placement in every layout,
// while tiled slots are recomputed on restore anyway.
func (m *Model) Snapshot() session.Snapshot {
	focused := ""
	if w := m.windows.Focused(); w != nil {
		focused = w.ID
	}

	windows := make([]session.Window, 0, len(m.windows.All()))
	for _, w := range m.windows.All() {
		var content []byte
		if stateful, ok := w.App.(appsdk.Stateful); ok {
			content = stateful.SnapshotState()
		}
		windows = append(windows, session.Window{
			ID:        w.ID,
			App:       m.baseApp(w.ID),
			Rect:      w.Floating,
			Workspace: w.Workspace,
			State:     int(w.State),
			Content:   content,
		})
	}
	return session.Snapshot{
		Layout:  int(m.windows.Layout()),
		Active:  m.windows.Active(),
		Focused: focused,
		Windows: windows,
	}
}

// baseApp maps a window id like "term-2" back to the spec that opened it.
func (m *Model) baseApp(id string) string {
	for _, base := range m.order {
		if id == base || strings.HasPrefix(id, base+"-") {
			return base
		}
	}
	return id
}

// saveSessionCmd writes a snapshot taken earlier, off the update loop.
func saveSessionCmd(snap session.Snapshot) tea.Cmd {
	return func() tea.Msg {
		return sessionSavedMsg{err: session.Save(snap)}
	}
}

// restoreWindows rebuilds a saved desktop: windows come back with their
// geometry and workspace, then layout, workspace and focus fall into place.
// It runs on the first resize so the work area is real before anything is
// clamped into it.
func (m *Model) restoreWindows(snap session.Snapshot) tea.Cmd {
	m.windows.SetLayout(wm.Layout(snap.Layout))

	cmds := make([]tea.Cmd, 0, len(snap.Windows))
	for _, saved := range snap.Windows {
		spec, ok := m.specs[saved.App]
		if !ok || saved.ID == "" {
			continue
		}
		spec.ID = saved.ID
		w, err := m.windows.OpenRestored(spec, saved.Rect, saved.Workspace, wm.State(saved.State))
		if err != nil {
			m.lastErr = err
			continue
		}
		if w.App != nil {
			// A stateful app replays its saved blob before it starts, so a
			// restored terminal shows its last screen under the new shell.
			if stateful, ok := w.App.(appsdk.Stateful); ok && len(saved.Content) > 0 {
				if err := stateful.RestoreState(saved.Content); err != nil {
					m.lastErr = err
				}
			}
			cmds = append(cmds, w.App.Init())
		}
	}

	if snap.Active >= 0 && snap.Active < wm.Workspaces {
		if err := m.windows.SetWorkspace(snap.Active); err != nil {
			m.lastErr = err
		}
	}
	if snap.Focused != "" {
		if err := m.windows.Reveal(snap.Focused); err != nil {
			m.lastErr = err
		}
	}
	return tea.Batch(cmds...)
}
