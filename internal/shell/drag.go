package shell

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
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
func (m *Model) onClick(msg tea.MouseClickMsg) tea.Cmd {
	mouse := msg.Mouse()
	m.tabbing = false
	if !m.options.Mouse || mouse.Button != tea.MouseLeft {
		return m.consumeMouse(msg, mouse)
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

	// Check if click landed on title buttons [_][^][x]
	if region == wm.RegionTitle {
		innerWidth := max(0, w.Rect.Width-2)
		left := windowLabel(w, true)
		if innerWidth >= 9 && strings.HasSuffix(left, "[_][^][x]") {
			// Window inner title starts at w.Rect.X + 1.
			// The 9 characters "[_][^][x]" end at w.Rect.Right() - 2.
			ctlStart := w.Rect.Right() - 1 - 9
			if mouse.X >= ctlStart && mouse.X < w.Rect.Right()-1 {
				offset := mouse.X - ctlStart
				switch {
				case offset >= 0 && offset <= 2: // [_]
					m.toggleMinimized(w.ID)
					return nil
				case offset >= 3 && offset <= 5: // [^]
					m.toggleMaximized(w.ID)
					return nil
				case offset >= 6 && offset <= 8: // [x]
					return m.windows.Close(w.ID)
				}
			}
		}
	}

	// A program that tracks the pointer inside the window owns it: the shell
	// must not turn its clicks into a drag.
	if region == wm.RegionContent && appOwnsPointer(w) {
		return m.windows.Update(msg)
	}

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

// consumeMouse hands a non left button press to the focused app when it asked
// for the pointer, so a program can see right clicks and middle clicks too.
func (m *Model) consumeMouse(msg tea.MouseMsg, mouse tea.Mouse) tea.Cmd {
	w := m.windows.Focused()
	if w == nil || !appOwnsPointer(w) {
		return nil
	}
	if !w.ContentRect().Contains(mouse.X, mouse.Y) {
		return nil
	}
	return m.windows.Update(msg)
}

// appOwnsPointer reports whether a window's app is using the pointer.
func appOwnsPointer(w *wm.Window) bool {
	consumer, ok := w.App.(appsdk.MouseConsumer)
	return ok && consumer.OwnsPointer()
}

func (m *Model) onMotion(msg tea.MouseMotionMsg) tea.Cmd {
	mouse := msg.Mouse()
	if m.drag.kind == dragNone {
		return m.consumeMouse(msg, mouse)
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

func (m *Model) onRelease(msg tea.MouseReleaseMsg) tea.Cmd {
	mouse := msg.Mouse()
	if m.drag.kind == dragNone {
		return m.consumeMouse(msg, mouse)
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
