package wm

import (
	"errors"
	"fmt"

	"charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/geom"
)

// Workspaces is how many virtual desktops the manager offers.
const Workspaces = 9

// WorkspaceError reports an out-of-range workspace index.
var ErrNoWorkspace = errors.New("wm: no such workspace")

// ErrNoWindow reports an unknown window id.
var ErrNoWindow = errors.New("wm: no such window")

// Manager owns every window, its geometry, the stacking order and the
// workspaces they live on.
type Manager struct {
	windows   []*Window
	history   []string
	workArea  geom.Rect
	focusedID string
	active    int
	layout    Layout
	step      int
}

// New returns an empty manager laying out inside workArea.
func New(workArea geom.Rect) *Manager {
	return &Manager{workArea: workArea, active: 0}
}

// WorkArea is the region windows may occupy, excluding the taskbar.
func (m *Manager) WorkArea() geom.Rect { return m.workArea }

// SetWorkArea resizes the work area and re-fits every floating window.
func (m *Manager) SetWorkArea(area geom.Rect) {
	previous := m.workArea
	m.workArea = area

	dx := area.X - previous.X
	dy := area.Y - previous.Y
	for _, w := range m.windows {
		w.Floating = geom.Rect{
			X:      w.Floating.X + dx,
			Y:      w.Floating.Y + dy,
			Width:  w.Floating.Width,
			Height: w.Floating.Height,
		}
		w.Restore = w.Floating
		if m.layout == LayoutTiled {
			continue
		}
		w.Rect = m.clampToWork(w, w.Rect)
	}
	m.arrange()
}

// Layout is how the manager arranges windows.
func (m *Manager) Layout() Layout { return m.layout }

// SetLayout switches between floating and tiled arrangement.
func (m *Manager) SetLayout(layout Layout) {
	if m.layout == layout {
		return
	}
	m.layout = layout

	if layout == LayoutTiled {
		for _, w := range m.windows {
			w.Floating = w.Rect
		}
		m.arrange()
		return
	}

	for _, w := range m.windows {
		w.Rect = m.clampToWork(w, w.Floating)
		w.State = Normal
	}
}

// Active is the index of the workspace currently shown.
func (m *Manager) Active() int { return m.active }

// SetWorkspace shows another virtual desktop.
func (m *Manager) SetWorkspace(index int) error {
	if index < 0 || index >= Workspaces {
		return fmt.Errorf("wm: workspace %d: %w", index, ErrNoWorkspace)
	}
	if index == m.active {
		return nil
	}
	m.active = index
	m.focusedID = ""
	m.arrange()
	m.focusTop()
	return nil
}

// MoveToWorkspace sends a window to another desktop without following it.
func (m *Manager) MoveToWorkspace(id string, index int) error {
	if index < 0 || index >= Workspaces {
		return fmt.Errorf("wm: workspace %d: %w", index, ErrNoWorkspace)
	}
	w := m.Find(id)
	if w == nil {
		return fmt.Errorf("wm: move %s to workspace: %w", id, ErrNoWindow)
	}
	w.Workspace = index
	if m.focusedID == id {
		m.focusedID = ""
		m.focusTop()
	}
	m.arrange()
	return nil
}

// Count is how many windows live on the active workspace.
func (m *Manager) Count() int {
	total := 0
	for _, w := range m.windows {
		if w.Workspace == m.active {
			total++
		}
	}
	return total
}

// Windows returns the active workspace's windows in stacking order.
func (m *Manager) Windows() []*Window {
	windows := make([]*Window, 0, len(m.windows))
	for _, w := range m.windows {
		if w.Workspace == m.active {
			windows = append(windows, w)
		}
	}
	return windows
}

// All returns every window on every workspace, in stacking order.
func (m *Manager) All() []*Window { return m.windows }

// Find returns a window by id, wherever it lives, or nil.
func (m *Manager) Find(id string) *Window {
	for _, w := range m.windows {
		if w.ID == id {
			return w
		}
	}
	return nil
}

// Focused returns the focused window, or nil when nothing is focused.
func (m *Manager) Focused() *Window {
	return m.Find(m.focusedID)
}

// Open launches an app and places a window for it on the active workspace.
func (m *Manager) Open(spec appsdk.Spec) (*Window, error) {
	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("wm: open %s: %w", spec.ID, err)
	}
	if m.Find(spec.ID) != nil {
		return nil, fmt.Errorf("wm: open %s: %w", spec.ID, errors.New("window already open"))
	}

	rect := m.place(spec)
	m.step++

	w := &Window{
		ID:        spec.ID,
		Spec:      spec,
		App:       spec.Launch(),
		Rect:      rect,
		Restore:   rect,
		Floating:  rect,
		State:     Normal,
		Workspace: m.active,
		Opened:    m.step,
	}

	m.windows = append(m.windows, w)
	m.focusedID = w.ID
	m.arrange()
	return w, nil
}

// OpenRestored places a window with a saved geometry and workspace, the way a
// saved session is rebuilt. Unlike Open it takes the geometry as given instead
// of cascading a fresh one.
func (m *Manager) OpenRestored(spec appsdk.Spec, rect geom.Rect, workspace int, state State) (*Window, error) {
	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("wm: restore %s: %w", spec.ID, err)
	}
	if m.Find(spec.ID) != nil {
		return nil, fmt.Errorf("wm: restore %s: %w", spec.ID, errors.New("window already open"))
	}
	if workspace < 0 || workspace >= Workspaces {
		workspace = m.active
	}

	m.step++

	w := &Window{
		ID:        spec.ID,
		Spec:      spec,
		App:       spec.Launch(),
		State:     state,
		Workspace: workspace,
		Opened:    m.step,
	}
	w.Rect = m.clampToWork(w, rect)
	w.Restore = w.Rect
	w.Floating = w.Rect
	if state == Maximized {
		w.Rect = m.workArea
	}

	m.windows = append(m.windows, w)
	m.focusedID = w.ID
	m.arrange()
	return w, nil
}

// Focus makes a window the top of the stack and the target of key input.
func (m *Manager) Focus(id string) error {
	w := m.Find(id)
	if w == nil {
		return fmt.Errorf("wm: focus %s: %w", id, ErrNoWindow)
	}
	if w.State == Minimized {
		w.State = Normal
		m.arrange()
	}
	if m.focusedID != id {
		m.pushHistory(m.focusedID)
	}
	m.raise(w)
	m.focusedID = id
	m.settleZoom()
	return nil
}

// FocusLast switches back to the previously focused window.
func (m *Manager) FocusLast() bool {
	for _, id := range m.history {
		w := m.Find(id)
		if w == nil || w.Workspace != m.active || w.State == Minimized {
			continue
		}
		m.focusedID = ""
		if err := m.Focus(id); err != nil {
			return false
		}
		return true
	}
	return false
}

// Reveal focuses a window wherever it lives, switching to its workspace first
// so the user can actually see what took the keyboard.
func (m *Manager) Reveal(id string) error {
	w := m.Find(id)
	if w == nil {
		return fmt.Errorf("wm: reveal %s: %w", id, ErrNoWindow)
	}
	if w.Workspace != m.active {
		if err := m.SetWorkspace(w.Workspace); err != nil {
			return err
		}
	}
	return m.Focus(id)
}

// Cycle moves focus through the visible windows of the active workspace.
func (m *Manager) Cycle(step int) {
	visible := m.visible()
	if len(visible) == 0 {
		return
	}
	current := m.Focused()

	index := 0
	for i, w := range visible {
		if w == current {
			index = i
		}
	}
	next := visible[((index+step)%len(visible)+len(visible))%len(visible)]

	if m.focusedID != next.ID {
		m.pushHistory(m.focusedID)
	}
	m.raise(next)
	m.focusedID = next.ID
	m.settleZoom()
}

// Close removes a window and hands focus to the one below it.
func (m *Manager) Close(id string) tea.Cmd {
	index := -1
	for i, w := range m.windows {
		if w.ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return nil
	}

	w := m.windows[index]
	m.windows = append(m.windows[:index], m.windows[index+1:]...)
	m.dropHistory(id)

	if m.focusedID == id {
		m.focusedID = ""
		m.focusTop()
	}
	m.arrange()
	return w.App.Close()
}

// Move repositions and resizes a window. Tiled and maximized windows own
// their geometry, so their moves are ignored.
func (m *Manager) Move(id string, rect geom.Rect) error {
	w := m.Find(id)
	if w == nil {
		return fmt.Errorf("wm: move %s: %w", id, ErrNoWindow)
	}
	if m.layout == LayoutTiled || w.State == Maximized {
		return nil
	}
	w.Rect = m.clampToWork(w, rect)
	w.Floating = w.Rect
	w.Restore = w.Rect
	return nil
}

// Snap moves a window to one of the standard zones.
func (m *Manager) Snap(id string, zone Zone) error {
	w := m.Find(id)
	if w == nil {
		return fmt.Errorf("wm: snap %s: %w", id, ErrNoWindow)
	}
	if m.layout == LayoutTiled {
		return nil
	}

	rect := zone.Rect(m.workArea)
	if zone == ZoneNone {
		return nil
	}
	rect = m.clampToWork(w, rect)

	w.State = Normal
	w.Rect = rect
	w.Floating = rect
	w.Restore = rect
	return nil
}

// ToggleMaximized swaps a window between its restored size and the work area.
// With tiling on it folds a window out of the tiled grid instead: the zoomed
// window takes the whole work area while the rest keep their tiling slots, and
// folding it back returns it to the slot it had.
func (m *Manager) ToggleMaximized(id string) error {
	w := m.Find(id)
	if w == nil {
		return fmt.Errorf("wm: maximize %s: %w", id, ErrNoWindow)
	}
	if m.layout == LayoutTiled {
		if w.State == Maximized {
			w.State = Normal
		} else {
			w.State = Maximized
			w.Restore = w.Rect
		}
		m.arrange()
		return nil
	}

	switch w.State {
	case Maximized:
		w.State = Normal
		w.Rect = m.clampToWork(w, w.Restore)
		w.Floating = w.Rect
	default:
		w.Restore = w.Rect
		w.State = Maximized
		w.Rect = m.workArea
	}
	return nil
}

// ToggleMinimized hides a window without closing it.
func (m *Manager) ToggleMinimized(id string) error {
	w := m.Find(id)
	if w == nil {
		return fmt.Errorf("wm: minimize %s: %w", id, ErrNoWindow)
	}
	if w.State == Minimized {
		return m.Focus(id)
	}

	if w.State == Maximized {
		w.State = Normal
	}
	w.State = Minimized

	if m.focusedID == id {
		m.focusedID = ""
		m.focusTop()
	}
	m.arrange()
	return nil
}

// RegionAt returns the topmost window under a cell together with which part
// of it the cell hits and which edges it would resize.
func (m *Manager) RegionAt(x, y int) (*Window, Region, Edges) {
	w := m.HitTest(x, y)
	if w == nil {
		return nil, RegionNone, 0
	}
	return w, w.RegionAt(x, y), w.EdgesAt(x, y)
}

// HitTest returns the topmost visible window under the given cell.
func (m *Manager) HitTest(x, y int) *Window {
	for i := len(m.windows) - 1; i >= 0; i-- {
		w := m.windows[i]
		if w.Workspace != m.active || w.State == Minimized {
			continue
		}
		if w.Rect.Contains(x, y) {
			return w
		}
	}
	return nil
}

// Update forwards a message to the focused window's app.
func (m *Manager) Update(msg tea.Msg) tea.Cmd {
	w := m.Focused()
	if w == nil || w.App == nil {
		return nil
	}
	app, cmd := w.App.Update(msg)
	w.App = app
	return cmd
}

// Broadcast forwards a message to every app, focused or not, and collects
// their commands. It is how the desktop pushes a clock tick into every window.
func (m *Manager) Broadcast(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.windows))
	for _, w := range m.windows {
		if w.App == nil {
			continue
		}
		app, cmd := w.App.Update(msg)
		w.App = app
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// visible lists the active workspace's windows in stacking order, skipping
// minimized ones.
func (m *Manager) visible() []*Window {
	visible := make([]*Window, 0, len(m.windows))
	for _, w := range m.windows {
		if w.Workspace == m.active && w.State != Minimized {
			visible = append(visible, w)
		}
	}
	return visible
}

func (m *Manager) raise(w *Window) {
	for i, other := range m.windows {
		if other == w {
			m.windows = append(m.windows[:i], m.windows[i+1:]...)
			break
		}
	}
	m.windows = append(m.windows, w)
}

// settleZoom keeps the one tiled zoom on the window that has the focus. With
// the zoom in place alt-tab walks full-screen windows, and the zoomed one can
// never hide behind another. Nothing to migrate when no zoom or no focus move
// is in play.
func (m *Manager) settleZoom() {
	if m.layout != LayoutTiled {
		return
	}
	var kept, stray *Window
	for _, w := range m.visible() {
		if w.State != Maximized {
			continue
		}
		if w.ID == m.focusedID {
			kept = w
		} else if stray == nil {
			stray = w
		} else {
			w.State = Normal
		}
	}
	if stray != nil {
		stray.State = Normal
		if kept == nil {
			if w := m.Focused(); w != nil && w.State == Normal {
				w.State = Maximized
			}
		}
	}
	m.arrange()
}

func (m *Manager) focusTop() {
	for i := len(m.windows) - 1; i >= 0; i-- {
		w := m.windows[i]
		if w.Workspace == m.active && w.State != Minimized {
			m.focusedID = w.ID
			m.settleZoom()
			return
		}
	}
}

func (m *Manager) pushHistory(id string) {
	if id == "" {
		return
	}
	m.dropHistory(id)
	m.history = append([]string{id}, m.history...)
	if len(m.history) > 16 {
		m.history = m.history[:16]
	}
}

func (m *Manager) dropHistory(id string) {
	kept := m.history[:0]
	for _, candidate := range m.history {
		if candidate != id {
			kept = append(kept, candidate)
		}
	}
	m.history = kept
}

func (m *Manager) clampToWork(w *Window, rect geom.Rect) geom.Rect {
	return rect.ClampTo(m.workArea, w.Spec.MinWidth, w.Spec.MinHeight)
}

// place puts a new window in the middle of the work area, cascading repeats
// so stacked windows stay individually clickable.
func (m *Manager) place(spec appsdk.Spec) geom.Rect {
	width := spec.DefaultWidth
	height := spec.DefaultHeight
	if width <= 0 {
		width = m.workArea.Width * 2 / 3
	}
	if height <= 0 {
		height = m.workArea.Height * 2 / 3
	}
	width = min(width, m.workArea.Width)
	height = min(height, m.workArea.Height)

	cascade := min(m.step, 8) * 2
	x := (m.workArea.Width-width)/2 + cascade
	y := (m.workArea.Height-height)/2 + cascade

	return geom.Rect{X: x, Y: y, Width: width, Height: height}.
		ClampTo(m.workArea, spec.MinWidth, spec.MinHeight)
}
