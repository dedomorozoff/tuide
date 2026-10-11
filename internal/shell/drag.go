package shell

import (
	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/geom"
	"github.com/dedomorozoff/tuide/internal/wm"
)

// dragSnapMargin is how many cells from a work-area edge a window must be
// dropped before it snaps to that edge.
const dragSnapMargin = 2

type dragKind int

const (
	dragNone dragKind = iota
	dragMove
	dragResize
)

// dragState is a mouse gesture in flight. It remembers the geometry the window
// had when the gesture started so every motion is relative to the anchor, not
// to the previous motion.
type dragState struct {
	kind   dragKind
	id     string
	origin geom.Rect
	startX int
	startY int
	edges  wm.Edges
}

// active reports whether a gesture is in progress.
func (d dragState) active() bool { return d.kind != dragNone }

// onClick focuses the window under the cursor and begins a drag when the click
// landed on the title bar or a resize edge. Taskbar clicks toggle the launcher
// and window buttons.
func (m *Model) onClick(mouse tea.Mouse) tea.Cmd {
	m.tabbing = false
	if !m.options.Mouse || mouse.Button != tea.MouseLeft {
		return nil
	}

	if m.launcher.open {
		m.launcher.open = false
		return nil
	}

	if m.options.Taskbar {
		if cmd := m.onTaskbarClick(mouse); cmd != nil || m.launcher.open {
			return cmd
		}
	}

	w, region, edges := m.windows.RegionAt(mouse.X, mouse.Y)
	if w == nil {
		return nil
	}
	m.focus(w.ID)

	// Tiled and maximized windows own their geometry, so there is nothing
	// to drag.
	if m.windows.Layout() == wm.LayoutTiled || w.State == wm.Maximized {
		return nil
	}

	switch region {
	case wm.RegionTitle:
		m.drag = dragState{
			kind:   dragMove,
			id:     w.ID,
			origin: w.Rect,
			startX: mouse.X,
			startY: mouse.Y,
		}
	case wm.RegionBorder:
		if edges.Empty() {
			return nil
		}
		m.drag = dragState{
			kind:   dragResize,
			id:     w.ID,
			origin: w.Rect,
			startX: mouse.X,
			startY: mouse.Y,
			edges:  edges,
		}
	}
	return nil
}

func (m *Model) onMotion(mouse tea.Mouse) tea.Cmd {
	if m.drag.kind == dragNone {
		return nil
	}
	if m.windows.Find(m.drag.id) == nil {
		m.drag = dragState{}
		return nil
	}

	dx := mouse.X - m.drag.startX
	dy := mouse.Y - m.drag.startY

	rect := m.drag.origin
	switch m.drag.kind {
	case dragMove:
		rect.X += dx
		rect.Y += dy
	case dragResize:
		if m.drag.edges.Has(wm.EdgeLeft) {
			rect.X += dx
			rect.Width -= dx
		}
		if m.drag.edges.Has(wm.EdgeRight) {
			rect.Width += dx
		}
		if m.drag.edges.Has(wm.EdgeTop) {
			rect.Y += dy
			rect.Height -= dy
		}
		if m.drag.edges.Has(wm.EdgeBottom) {
			rect.Height += dy
		}
	}

	m.moveWindow(m.drag.id, rect)
	return nil
}

// onTaskbarClick handles the start button and the window buttons. It reports
// whether the click landed on the taskbar at all.
func (m *Model) onTaskbarClick(mouse tea.Mouse) tea.Cmd {
	y := max(0, m.height-TaskbarHeight)
	if mouse.Y != y || mouse.X < 0 || mouse.X >= m.width {
		return nil
	}

	for _, item := range m.taskbarItems() {
		if !item.rect.Contains(mouse.X, mouse.Y) {
			continue
		}
		switch {
		case item.id == "start":
			m.openLauncher(launcherApps)
		case item.id == "clock" || item.id == "hint":
		default:
			if w := m.windows.Find(item.id); w != nil {
				if w == m.windows.Focused() && w.State != wm.Minimized {
					m.toggleMinimized(w.ID)
					return nil
				}
				m.focus(w.ID)
			}
		}
		return nil
	}
	return nil
}

func (m *Model) onRelease(mouse tea.Mouse) tea.Cmd {
	if m.drag.kind == dragNone {
		return nil
	}

	id := m.drag.id
	m.drag = dragState{}

	if m.windows.Find(id) == nil {
		return nil
	}
	if zone := wm.DropZone(mouse.X, mouse.Y, m.windows.WorkArea(), dragSnapMargin); zone != wm.ZoneNone {
		m.snap(id, zone)
	}
	return nil
}
