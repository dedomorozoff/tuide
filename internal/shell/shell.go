// Package shell is the single Bubble Tea model of the desktop: it owns the
// window manager, routes input and composites the final frame.
package shell

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/compositor"
	"github.com/dedomorozoff/tuide/internal/config"
	"github.com/dedomorozoff/tuide/internal/geom"
	"github.com/dedomorozoff/tuide/internal/input"
	"github.com/dedomorozoff/tuide/internal/theme"
	"github.com/dedomorozoff/tuide/internal/wm"
)

// TaskbarHeight is how many rows the taskbar occupies at the bottom.
const TaskbarHeight = 1

const (
	moveStep     = 4
	resizeStep   = 2
	defaultWidth = 80
	defaultHeigh = 24
)

// Options are the startup parameters of the desktop.
type Options struct {
	Width   int
	Height  int
	Taskbar bool
	Mouse   bool
	Launch  []string
}

// Model is the root model. Applications live inside windows and only ever see
// the messages the window manager decided to forward.
type Model struct {
	cfg      config.Config
	styles   theme.Styles
	windows  *wm.Manager
	screen   *compositor.Screen
	keymap   *input.Keymap
	specs    map[string]appsdk.Spec
	order    []string
	options  Options
	width    int
	height   int
	lastErr  error
	shutdown bool

	// drag is the mouse gesture in flight, if any.
	drag dragState
	// snapLast remembers where each window was last snapped so two
	// perpendicular snaps combine into a quarter.
	snapLast map[string]wm.Zone
	// inspector shows the window manager state overlay.
	inspector bool
	// tabbing shows the alt-tab window list; it clears on the next key.
	tabbing bool
	// launcher is the app launcher / action palette overlay.
	launcher launcherState
	// now is the time the last tick reported; the taskbar clock reads it.
	now time.Time
}

// New builds the desktop from a palette, a set of launchable apps and options.
func New(palette theme.Theme, specs []appsdk.Spec, options Options) *Model {
	width, height := options.Width, options.Height
	if width <= 0 || height <= 0 {
		width, height = defaultWidth, defaultHeigh
	}

	m := &Model{
		styles:   theme.NewStyles(palette),
		specs:    make(map[string]appsdk.Spec, len(specs)),
		snapLast: make(map[string]wm.Zone),
		options:  options,
		width:    width,
		height:   height,
		now:      time.Now(),
	}
	for _, spec := range specs {
		if spec.ID == "" {
			continue
		}
		if _, exists := m.specs[spec.ID]; !exists {
			m.order = append(m.order, spec.ID)
		}
		m.specs[spec.ID] = spec
	}

	m.keymap = input.DefaultKeymap()
	m.windows = wm.New(workArea(width, height, options.Taskbar))
	m.screen = compositor.New(width, height)
	return m
}

// WithKeymap replaces the default key bindings.
func (m *Model) WithKeymap(keymap *input.Keymap) *Model {
	m.keymap = keymap
	return m
}

// Init opens the apps named in the launch options. An empty list means an
// empty desktop: nothing is opened behind the user's back, the launcher
// (ctrl+alt+space) starts apps on demand.
func (m *Model) Init() tea.Cmd {
	ids := m.options.Launch

	cmds := make([]tea.Cmd, 0, len(ids)+1)
	for _, id := range ids {
		if cmd := m.launch(id); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	cmds = append(cmds, tickCmd())
	return tea.Batch(cmds...)
}

// Update handles a message and returns the next model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, m.resize(msg.Width, msg.Height)

	case tickMsg:
		m.now = time.Time(msg)
		return m, tea.Batch(tickCmd(), m.windows.Broadcast(appsdk.Tick{Time: m.now}))

	case tea.KeyPressMsg:
		return m, m.onKeyPress(msg)

	case tea.MouseClickMsg:
		return m, m.onClick(msg.Mouse())

	case tea.MouseMotionMsg:
		return m, m.onMotion(msg.Mouse())

	case tea.MouseReleaseMsg:
		return m, m.onRelease(msg.Mouse())
	}

	return m, m.windows.Update(msg)
}

// View composites the whole desktop into a single frame.
func (m *Model) View() tea.View {
	m.screen.Clear()
	m.screen.Fill(m.screen.Bounds(), theme.Background(m.styles.Theme.Background))

	focused := m.windows.Focused()
	for _, w := range m.windows.Windows() {
		if w.State == wm.Minimized {
			continue
		}
		m.drawWindow(w, w == focused)
	}

	if m.options.Taskbar {
		m.drawTaskbar()
	}

	if m.tabbing {
		m.drawTabHUD()
	}
	if m.inspector {
		m.drawInspector()
	}
	if m.launcher.open {
		m.drawLauncher()
	}

	view := tea.NewView(m.screen.String())
	if m.options.Mouse {
		view.MouseMode = tea.MouseModeCellMotion
	} else {
		view.MouseMode = tea.MouseModeNone
	}
	return view
}

// Err returns the last failure the desktop ran into, if any.
func (m *Model) Err() error { return m.lastErr }

// tickMsg carries the wall clock into the model once a second.
type tickMsg time.Time

// tickCmd schedules the next clock tick.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(now time.Time) tea.Msg { return tickMsg(now) })
}

// resize applies a new terminal size.
func (m *Model) resize(width, height int) tea.Cmd {
	m.width, m.height = width, height
	m.screen.Resize(width, height)
	m.windows.SetWorkArea(workArea(width, height, m.options.Taskbar))
	return nil
}

func (m *Model) onKeyPress(msg tea.KeyPressMsg) tea.Cmd {
	if m.launcher.open {
		return m.launcherKey(msg)
	}
	action, ok := m.keymap.Lookup(msg)
	if !ok || (action != input.ActionFocusNext && action != input.ActionFocusPrev) {
		m.tabbing = false
	}
	if ok {
		return m.perform(action)
	}
	return m.windows.Update(msg)
}

func (m *Model) launch(id string) tea.Cmd {
	spec, ok := m.specs[id]
	if !ok {
		m.lastErr = ErrUnknownApp
		return nil
	}
	if existing := m.windows.Find(id); existing != nil {
		m.focus(existing.ID)
		return nil
	}
	w, err := m.windows.Open(spec)
	if err != nil {
		m.lastErr = err
		return nil
	}
	return w.App.Init()
}

func (m *Model) perform(action input.Action) tea.Cmd {
	switch action {
	case input.ActionQuit:
		m.shutdown = true
		return tea.Quit
	case input.ActionFocusNext:
		m.tabbing = true
		m.windows.Cycle(1)
		return nil
	case input.ActionFocusPrev:
		m.tabbing = true
		m.windows.Cycle(-1)
		return nil
	case input.ActionFocusLast:
		m.windows.FocusLast()
		return nil
	case input.ActionToggleLayout:
		m.windows.SetLayout(m.windows.Layout().Next())
		return nil
	case input.ActionToggleInspector:
		m.inspector = !m.inspector
		return nil
	case input.ActionOpenLauncher:
		m.tabbing = false
		m.openLauncher(launcherApps)
		return nil
	case input.ActionOpenPalette:
		m.tabbing = false
		m.openLauncher(launcherActions)
		return nil
	}

	if index, ok := input.ParseWorkspace(action); ok {
		if err := m.windows.SetWorkspace(index - 1); err != nil {
			m.lastErr = err
		}
		return nil
	}
	if index, ok := input.ParseMoveToWorkspace(action); ok {
		m.moveToWorkspace(index - 1)
		return nil
	}

	focused := m.windows.Focused()
	if focused == nil {
		return nil
	}

	switch action {
	case input.ActionClose:
		delete(m.snapLast, focused.ID)
		return m.windows.Close(focused.ID)
	case input.ActionMaximize:
		m.toggleMaximized(focused.ID)
		return nil
	case input.ActionMinimize:
		m.toggleMinimized(focused.ID)
		return nil
	case input.ActionMoveLeft:
		return m.move(focused, -moveStep, 0, 0, 0)
	case input.ActionMoveRight:
		return m.move(focused, moveStep, 0, 0, 0)
	case input.ActionMoveUp:
		return m.move(focused, 0, -moveStep, 0, 0)
	case input.ActionMoveDown:
		return m.move(focused, 0, moveStep, 0, 0)
	case input.ActionResizeLeft:
		return m.move(focused, -resizeStep, 0, resizeStep, 0)
	case input.ActionResizeRight:
		return m.move(focused, resizeStep, 0, resizeStep, 0)
	case input.ActionResizeUp:
		return m.move(focused, 0, -resizeStep, 0, resizeStep)
	case input.ActionResizeDown:
		return m.move(focused, 0, resizeStep, 0, resizeStep)
	case input.ActionSnapLeft:
		return m.snapFocused(wm.ZoneLeft)
	case input.ActionSnapRight:
		return m.snapFocused(wm.ZoneRight)
	case input.ActionSnapTop:
		return m.snapFocused(wm.ZoneTop)
	case input.ActionSnapBottom:
		return m.snapFocused(wm.ZoneBottom)
	case input.ActionSnapCenter:
		return m.snapFocused(wm.ZoneCenter)
	}
	return nil
}

// moveToWorkspace sends the focused window to another desktop and follows it.
func (m *Model) moveToWorkspace(index int) {
	if w := m.windows.Focused(); w != nil {
		delete(m.snapLast, w.ID)
		if err := m.windows.MoveToWorkspace(w.ID, index); err != nil {
			m.lastErr = err
			return
		}
	}
	if err := m.windows.SetWorkspace(index); err != nil {
		m.lastErr = err
	}
}

// snapFocused snaps the focused window, folding the new zone into the previous
// one so two perpendicular halves meet in a quarter.
func (m *Model) snapFocused(zone wm.Zone) tea.Cmd {
	w := m.windows.Focused()
	if w == nil || m.windows.Layout() == wm.LayoutTiled {
		return nil
	}
	if previous, ok := m.snapLast[w.ID]; ok {
		zone = previous.Combine(zone)
	}
	m.snap(w.ID, zone)
	return nil
}

func (m *Model) move(w *wm.Window, dx, dy, dw, dh int) tea.Cmd {
	rect := w.Rect
	rect.X += dx
	rect.Y += dy
	rect.Width += dw
	rect.Height += dh
	m.moveWindow(w.ID, rect)
	return nil
}

// The window manager reports geometry errors; the shell keeps running and
// leaves the window where it was, so these helpers deliberately drop them.
func (m *Model) focus(id string) {
	if err := m.windows.Focus(id); err != nil {
		m.lastErr = err
	}
}

func (m *Model) moveWindow(id string, rect geom.Rect) {
	if err := m.windows.Move(id, rect); err != nil {
		m.lastErr = err
		return
	}
	// A hand-placed window is no longer part of a snap chain.
	delete(m.snapLast, id)
}

// snap sends a window to a zone and remembers where it went.
func (m *Model) snap(id string, zone wm.Zone) {
	if err := m.windows.Snap(id, zone); err != nil {
		m.lastErr = err
		return
	}
	m.snapLast[id] = zone
}

func (m *Model) toggleMaximized(id string) {
	if err := m.windows.ToggleMaximized(id); err != nil {
		m.lastErr = err
	}
}

func (m *Model) toggleMinimized(id string) {
	if err := m.windows.ToggleMinimized(id); err != nil {
		m.lastErr = err
	}
}

// workArea is the region windows may use, excluding the taskbar.
func workArea(width, height int, taskbar bool) geom.Rect {
	bottom := height
	if taskbar {
		bottom -= TaskbarHeight
	}
	return geom.Rect{Width: max(0, width), Height: max(0, bottom)}
}
