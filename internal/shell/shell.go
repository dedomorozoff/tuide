// Package shell is the single Bubble Tea model of the desktop: it owns the
// window manager, routes input and composites the final frame.
package shell

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/compositor"
	"github.com/dedomorozoff/tuide/internal/config"
	"github.com/dedomorozoff/tuide/internal/geom"
	"github.com/dedomorozoff/tuide/internal/input"
	"github.com/dedomorozoff/tuide/internal/session"
	"github.com/dedomorozoff/tuide/internal/sys"
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
	// StartLabel is the text on the taskbar's menu button. Empty falls back to
	// the built-in mark.
	StartLabel string
	// Restore, when set, rebuilds the desktop from a saved session on the
	// first resize instead of opening the launch list.
	Restore *session.Snapshot
	// Persist makes the desktop write its session file while it runs. Servers
	// turn it on; a local run keeps its hands off the shared session.
	Persist bool
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
	// hover is the resize handle under the pointer while no gesture is
	// running; the frame lights it up to advertise that it can be resized.
	hover hoverState
	// memo remembers the last mouse click, so the next one can be told apart
	// as a double click.
	memo lastClick
	// snapLast remembers where each window was last snapped so two
	// perpendicular snaps combine into a quarter.
	snapLast map[string]wm.Zone
	// inspector shows the window manager state overlay.
	inspector bool
	// tabbing shows the alt-tab window list; it clears on the next key.
	tabbing bool
	// launcher is the app launcher / action palette overlay.
	launcher launcherState
	// calendar is the clock's popup month view.
	calendar calendarState
	// now is the time the last tick reported; the taskbar clock reads it.
	now time.Time
	// frameSizes remembers the content size each app was last told about, so
	// only real resizes produce a message.
	frameSizes map[string][2]int
	// net is the last thing the connectivity probe reported.
	net netState
	// netTick counts ticks to throttle connectivity probes.
	netTick int
	// ticks counts clock ticks to throttle session saves.
	ticks int
	// restore is the saved session waiting for the first resize, if any.
	restore *session.Snapshot
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
		cfg: config.Config{
			Theme:      palette.Name,
			Startup:    options.Launch,
			Taskbar:    options.Taskbar,
			Mouse:      options.Mouse,
			StartLabel: options.StartLabel,
		},
		width:  width,
		height: height,
		now:    time.Now(),
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
	m.restore = options.Restore
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
	if m.restore != nil && !m.restore.Empty() {
		// The saved session replaces the startup list: a restored desktop is
		// the one the user left behind.
		ids = nil
	}

	cmds := make([]tea.Cmd, 0, len(ids)+1)
	for _, id := range ids {
		if cmd := m.launch(id); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	cmds = append(cmds, tickCmd(), probeNetCmd)
	return tea.Batch(cmds...)
}

// Update handles a message and returns the next model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	model, cmd := m.update(msg)
	if m.shutdown {
		return model, cmd
	}
	return model, tea.Batch(cmd, m.syncFrames())
}

// update is the message dispatch itself. Update wraps it so the frame sizes
// stay in step with the geometry no matter which message changed it.
func (m *Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// A headless server has no terminal of its own and starts out at
		// 0x0 until the first client attaches. Restoring on that size would
		// clamp every saved window into an empty work area, so the empty size
		// is not a real resize.
		if msg.Width <= 0 || msg.Height <= 0 {
			return m, nil
		}
		cmd := m.resize(msg.Width, msg.Height)
		if m.restore != nil {
			// The work area is real only now, so the session is rebuilt here
			// and never again.
			cmd = tea.Batch(cmd, m.restoreWindows(*m.restore))
			m.restore = nil
		}
		return m, cmd

	case tickMsg:
		m.now = time.Time(msg)
		m.netTick++
		m.ticks++
		cmds := []tea.Cmd{tickCmd(), m.windows.Broadcast(appsdk.Tick{Time: m.now})}
		if m.netTick%30 == 0 {
			cmds = append(cmds, probeNetCmd)
		}
		if m.options.Persist && m.ticks%saveEveryTicks == 0 {
			cmds = append(cmds, saveSessionCmd(m.Snapshot()))
		}
		return m, tea.Batch(cmds...)

	case sessionSavedMsg:
		if msg.err != nil {
			m.lastErr = msg.err
		}
		return m, nil

	case netMsg:
		m.net = netOffline
		if msg {
			m.net = netOnline
		}
		return m, nil

	case appsdk.RequestConfig:
		return m, m.replyToFocused(appsdk.ConfigState{Config: m.cfg})

	case appsdk.SaveConfig:
		m.applyConfig(msg.Config)
		cfgToSave := msg.Config
		saveCmd := func() tea.Msg {
			return configSavedMsg{err: cfgToSave.Save()}
		}
		return m, saveCmd

	case configSavedMsg:
		if msg.err != nil {
			m.lastErr = msg.err
		}
		return m, m.replyToFocused(appsdk.ConfigSaved{Err: msg.err})

	case tea.KeyPressMsg:
		return m, m.onKeyPress(msg)

	case tea.MouseClickMsg:
		return m, m.onClick(msg)

	case tea.MouseMotionMsg:
		return m, m.onMotion(msg)

	case tea.MouseReleaseMsg:
		return m, m.onRelease(msg)
	}

	return m, m.windows.Update(msg)
}

// syncFrames tells an app when the rectangle it may draw into changed size.
// Geometry can move on any message at all, so this runs after every dispatch.
func (m *Model) syncFrames() tea.Cmd {
	if m.frameSizes == nil {
		m.frameSizes = make(map[string][2]int)
	}

	live := make(map[string]bool, len(m.windows.All()))
	cmds := make([]tea.Cmd, 0, len(live))

	for _, w := range m.windows.All() {
		live[w.ID] = true

		rect := w.ContentRect()
		size := [2]int{rect.Width, rect.Height}
		if previous, ok := m.frameSizes[w.ID]; ok && previous == size {
			continue
		}
		m.frameSizes[w.ID] = size

		if w.App == nil {
			continue
		}
		app, cmd := w.App.Update(appsdk.Frame{Width: size[0], Height: size[1]})
		w.App = app
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	for id := range m.frameSizes {
		if !live[id] {
			delete(m.frameSizes, id)
		}
	}
	return tea.Batch(cmds...)
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
		// The hover goes on right after its own window, so a window stacked
		// higher still covers the part of the handle it overlaps.
		if !m.drag.active() && m.hover.id == w.ID {
			m.drawResizeHover(w)
		}
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
	if m.calendar.open {
		m.drawCalendar()
	}
	if m.launcher.open {
		m.drawLauncher()
	}

	view := tea.NewView(m.screen.String())
	if m.options.Mouse {
		// All motion, not just motion while a button is held: hovering a frame
		// has to reach the shell before the button goes down for the resize
		// hint to mean anything.
		view.MouseMode = tea.MouseModeAllMotion
	} else {
		view.MouseMode = tea.MouseModeNone
	}
	return view
}

// Err returns the last failure the desktop ran into, if any.
func (m *Model) Err() error { return m.lastErr }

// tickMsg carries the wall clock into the model once a second.
type tickMsg time.Time

// netState is what the connectivity probe knows. It starts out unknown, so the
// bar stays quiet until a probe has actually failed: a warning light that is on
// before the first probe came back would be telling a story it does not know.
type netState int

const (
	netUnknown netState = iota
	netOnline
	netOffline
)

// netMsg carries connectivity status.
type netMsg bool

// tickCmd schedules the next clock tick.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(now time.Time) tea.Msg { return tickMsg(now) })
}

func probeNetCmd() tea.Msg {
	return netMsg(sys.Online())
}

// resize applies a new terminal size.
func (m *Model) resize(width, height int) tea.Cmd {
	m.width, m.height = width, height
	m.screen.Resize(width, height)
	m.windows.SetWorkArea(workArea(width, height, m.options.Taskbar))
	return nil
}

func (m *Model) onKeyPress(msg tea.KeyPressMsg) tea.Cmd {
	// The pointer did not move, but the desktop under it may have: keys move,
	// resize and close windows, so any hover from before is stale.
	m.hover = hoverState{}
	if m.launcher.open {
		return m.launcherKey(msg)
	}
	if m.calendar.open {
		if m.calendarKey(msg.Key()) {
			return nil
		}
		// The key was not the calendar's: the popup steps aside and the key
		// goes on to the desktop, so a stray press never leaves it hanging.
		m.calendar = calendarState{}
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
	if m.windows.Find(id) != nil {
		if !spec.Multi {
			if err := m.windows.Reveal(spec.ID); err != nil {
				m.lastErr = err
			}
			return nil
		}
		// A Multi app just gets one more window, numbered past the ones open.
		unique := spec
		n := 2
		for {
			unique.ID = fmt.Sprintf("%s-%d", spec.ID, n)
			if m.windows.Find(unique.ID) == nil {
				break
			}
			n++
		}
		w, err := m.windows.Open(unique)
		if err != nil {
			m.lastErr = err
			return nil
		}
		return w.App.Init()
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
	case input.ActionToggleCalendar:
		m.toggleCalendar()
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
	if slot, ok := input.ParseFocusWindow(action); ok {
		m.focusWindowSlot(slot)
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

// focusWindowSlot activates the window holding a taskbar slot, counting from 1
// in the order the windows were opened.
func (m *Model) focusWindowSlot(slot int) {
	windows := m.taskbarWindows()
	if slot < 1 || slot > len(windows) {
		return
	}
	m.activateWindow(windows[slot-1].ID)
}

// activateWindow brings a window forward the way a click on its taskbar button
// does: the focused window folds away instead, anything else comes back up and
// takes the focus. The pointer and alt+1..9 both go through here, so a button
// and a number can never disagree.
func (m *Model) activateWindow(id string) {
	w := m.windows.Find(id)
	if w == nil {
		return
	}
	if w == m.windows.Focused() && w.State != wm.Minimized {
		m.toggleMinimized(id)
		return
	}
	if err := m.windows.Reveal(id); err != nil {
		m.lastErr = err
	}
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
