package shell

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/geom"
	"github.com/dedomorozoff/tuide/internal/wm"
)

// dragSnapMargin is how many cells from a work-area edge a window must be
// dropped before it snaps to that edge.
const dragSnapMargin = 2

// doubleClickGap is how long after a click a second one on the same cell is
// still the same double click gesture.
const doubleClickGap = 400 * time.Millisecond

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
	// unfold marks a move gesture that started on a maximized or zoomed window:
	// the first real motion folds the window back and the drag goes on from
	// its restored size. moved remembers whether the pointer really moved, so a
	// held press never snaps and a released click stays a click.
	unfold bool
	moved  bool
}

// lastClick remembers one mouse click, so the next one can recognize the pair.
type lastClick struct {
	at time.Time
	x  int
	y  int
}

// doubleClick reports whether this click and the remembered one form a double
// click on the same cell.
func (m *Model) doubleClick(mouse tea.Mouse) bool {
	pair := !m.memo.at.IsZero() && time.Since(m.memo.at) <= doubleClickGap &&
		m.memo.x == mouse.X && m.memo.y == mouse.Y
	m.memo = lastClick{at: time.Now(), x: mouse.X, y: mouse.Y}
	return pair
}

// active reports whether a gesture is in progress.
func (d dragState) active() bool { return d.kind != dragNone }

// onClick focuses the window under the cursor and begins a drag when the click
// landed on the title bar or a resize edge. Taskbar clicks open the launcher,
// focus or minimize a window and, on the middle button, close it. A click on
// the open launcher runs the row it hit.
func (m *Model) onClick(msg tea.MouseClickMsg) tea.Cmd {
	mouse := msg.Mouse()
	m.tabbing = false
	if !m.options.Mouse {
		return m.consumeMouse(msg, mouse)
	}

	if m.launcher.open {
		return m.onLauncherClick(mouse)
	}

	if m.calendar.open {
		return m.onCalendarClick()
	}

	// The taskbar answers the middle button as well, so it is consulted before
	// the check that sends every other button to the focused app.
	if m.options.Taskbar {
		if cmd, handled := m.onTaskbarClick(mouse); handled {
			return cmd
		}
	}

	if mouse.Button != tea.MouseLeft {
		return m.consumeMouse(msg, mouse)
	}

	w, region, edges := m.windows.RegionAt(mouse.X, mouse.Y)
	if w == nil {
		return nil
	}
	m.focus(w.ID)

	// Check if click landed on title buttons [_][■][x]
	if region == wm.RegionTitle {
		innerWidth := max(0, w.Rect.Width-2)
		if innerWidth >= 9 {
			// The title bar starts at w.Rect.X + 1.
			// The 9-character control strip ends at w.Rect.Right() - 2.
			ctlStart := w.Rect.Right() - 1 - 9
			if mouse.X >= ctlStart && mouse.X < w.Rect.Right()-1 {
				offset := mouse.X - ctlStart
				switch {
				case offset >= 0 && offset <= 2: // [_]
					m.toggleMinimized(w.ID)
					return nil
				case offset >= 3 && offset <= 5: // [■]
					m.toggleMaximized(w.ID)
					return nil
				case offset >= 6 && offset <= 8: // [x]
					return m.windows.Close(w.ID)
				}
			}
		}
	}

	// Like in a window manager on the desktop, a second click on the title
	// bar is the maximize/restore gesture, doubling as zoom while tiling.
	if region == wm.RegionTitle && m.doubleClick(mouse) {
		m.toggleMaximized(w.ID)
		return nil
	}

	// A program that tracks the pointer inside the window owns it: the shell
	// must not turn its clicks into a drag.
	if region == wm.RegionContent && appOwnsPointer(w) {
		return m.windows.Update(msg)
	}

	// Tiled and maximized windows own their geometry, so there is nothing to
	// drag — except the title of a maximized one, where grabbing starts the
	// unfold gesture.
	if (m.windows.Layout() == wm.LayoutTiled || w.State == wm.Maximized) &&
		!(region == wm.RegionTitle && w.State == wm.Maximized) {
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
			unfold: w.State == wm.Maximized,
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

// unfoldWindow hands a maximized or zoomed window its restored geometry back,
// waiting for a drag away from the title. A zoom that came from tiling ends the
// arrangement altogether: tiling owns geometry, so a manual pull switches the
// desktop to floating windows.
func (m *Model) unfoldWindow(w *wm.Window) {
	if m.windows.Layout() == wm.LayoutTiled {
		m.windows.SetLayout(wm.LayoutFloating)
	}
	if w.State == wm.Maximized {
		m.toggleMaximized(w.ID)
	}
	m.moveWindow(w.ID, w.Restore)
}

func (m *Model) onMotion(msg tea.MouseMotionMsg) tea.Cmd {
	mouse := msg.Mouse()
	if m.drag.kind == dragNone {
		return m.consumeMouse(msg, mouse)
	}
	w := m.windows.Find(m.drag.id)
	if w == nil {
		m.drag = dragState{}
		return nil
	}

	// The first real motion under a grab of a maximized window folds it back.
	// The anchor moves with the restore, so the cursor keeps holding the title
	// at the place it grabbed and the drag continues from there.
	if m.drag.unfold && (mouse.X != m.drag.startX || mouse.Y != m.drag.startY) {
		maximized := m.drag.origin
		grab := m.drag.startX - maximized.X
		if grab < 0 {
			grab = 0
		}

		m.unfoldWindow(w)

		if maximized.Width > 0 {
			grab = grab * w.Rect.Width / maximized.Width
		}
		m.drag.origin = w.Rect
		m.drag.startX = w.Rect.X + grab
		m.drag.startY = mouse.Y
		m.drag.unfold = false
	}

	m.drag.moved = true

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

// onTaskbarClick handles the start button and the window buttons. The left
// button opens the launcher, focuses a window or minimizes the focused one; the
// middle button closes the window behind it. It reports whether the click
// landed on the taskbar at all.
func (m *Model) onTaskbarClick(mouse tea.Mouse) (tea.Cmd, bool) {
	y := max(0, m.height-TaskbarHeight)
	if mouse.Y != y || mouse.X < 0 || mouse.X >= m.width {
		return nil, false
	}

	for _, item := range m.taskbarItems() {
		if !item.rect.Contains(mouse.X, mouse.Y) {
			continue
		}
		return m.taskbarItemClick(item.id, mouse.Button), true
	}
	return nil, false
}

// taskbarItemClick runs the command one taskbar element stands for. Only the
// window buttons answer both buttons: everywhere else the middle one does
// nothing at all rather than closing something the user did not point at.
func (m *Model) taskbarItemClick(id string, button tea.MouseButton) tea.Cmd {
	w := m.windows.Find(id)

	if button == tea.MouseMiddle {
		if w == nil {
			return nil
		}
		delete(m.snapLast, w.ID)
		return m.windows.Close(w.ID)
	}
	if button != tea.MouseLeft {
		return nil
	}

	switch {
	case id == "start":
		m.openLauncher(launcherApps)
	case id == "clock":
		m.toggleCalendar()
	case id == "status":
	default:
		m.activateWindow(id)
	}
	return nil
}

func (m *Model) onRelease(msg tea.MouseReleaseMsg) tea.Cmd {
	mouse := msg.Mouse()
	if m.drag.kind == dragNone {
		return m.consumeMouse(msg, mouse)
	}

	id := m.drag.id
	moved := m.drag.moved
	m.drag = dragState{}

	if m.windows.Find(id) == nil {
		return nil
	}
	if moved {
		if zone := wm.DropZone(mouse.X, mouse.Y, m.windows.WorkArea(), dragSnapMargin); zone != wm.ZoneNone {
			m.snap(id, zone)
		}
	}
	return nil
}
