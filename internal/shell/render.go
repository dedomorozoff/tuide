package shell

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/geom"
	"github.com/dedomorozoff/tuide/internal/theme"
	"github.com/dedomorozoff/tuide/internal/wm"
)

const hint = " ctrl+alt+q quit "

// startLabel is the taskbar's menu button; clicking it opens the launcher.
const startLabel = "≡ start"

func (m *Model) drawWindow(w *wm.Window, focused bool) {
	m.screen.Fill(w.Rect, theme.Background(m.styles.Theme.Background))

	frame, title := m.styles.WindowChrome(focused, w.Rect.Width, w.Rect.Height)
	m.screen.Blit(w.Rect, frame.Render(title.Render(windowLabel(w, focused))))

	if w.App == nil {
		return
	}
	// Apps that paint cells hand them over directly, which is what keeps a
	// terminal window off the string rendering path entirely.
	if painter, ok := w.App.(appsdk.Painter); ok {
		painter.Paint(m.screen, w.ContentRect())
		return
	}
	m.screen.BlitView(w.ContentRect(), w.App.View(w.Context(focused)))
}

// windowTitle is the name an app asked for, or empty when it has none.
func windowTitle(w *wm.Window) string {
	named, ok := w.App.(appsdk.Named)
	if !ok {
		return ""
	}
	return named.Title()
}

func (m *Model) drawTaskbar() {
	bar := geom.Rect{
		Y:      max(0, m.height-TaskbarHeight),
		Width:  m.width,
		Height: TaskbarHeight,
	}
	m.screen.Fill(bar, theme.Background(m.styles.Theme.Taskbar))
	for _, item := range m.taskbarItems() {
		m.screen.Blit(item.rect, item.label)
	}
}

// taskbarItem is one element of the taskbar: the start button, a window
// button, the clock or the hint. Drawing and hit-testing share the layout so
// they can never disagree.
type taskbarItem struct {
	id    string
	label string
	rect  geom.Rect
}

// taskbarItems places every element on the bar. Window buttons that would not
// fit next to the clock and hint are dropped.
func (m *Model) taskbarItems() []taskbarItem {
	y := max(0, m.height-TaskbarHeight)
	hintWidth := lipgloss.Width(hint)

	startStyle := m.styles.TaskbarItem
	if m.launcher.open {
		startStyle = m.styles.TaskbarItemFocused
	}
	start := startStyle.Render(startLabel)

	items := []taskbarItem{{
		id:    "start",
		label: start,
		rect:  geom.Rect{Y: y, Width: lipgloss.Width(start), Height: TaskbarHeight},
	}}
	used := items[0].rect.Width

	focused := m.windows.Focused()
	indicators := m.statusIndicators()
	indWidth := lipgloss.Width(indicators)
	rightWidth := indWidth + lipgloss.Width(m.clockText()) + hintWidth
	for _, w := range m.taskbarWindows() {
		style := m.styles.TaskbarItem
		if w == focused {
			style = m.styles.TaskbarItemFocused
		}
		label := style.Render(w.Spec.Name)
		width := lipgloss.Width(label)
		if m.width > 0 && used+width+rightWidth > m.width {
			continue
		}
		items = append(items, taskbarItem{
			id:    w.ID,
			label: label,
			rect:  geom.Rect{X: used, Y: y, Width: width, Height: TaskbarHeight},
		})
		used += width
	}

	clock := m.styles.TaskbarText.Render(m.clockText())
	if indWidth > 0 {
		items = append(items, taskbarItem{
			id:    "status",
			label: indicators,
			rect:  geom.Rect{X: max(0, m.width-hintWidth-lipgloss.Width(clock)-indWidth), Y: y, Width: indWidth, Height: TaskbarHeight},
		})
	}
	items = append(items,
		taskbarItem{
			id:    "clock",
			label: clock,
			rect:  geom.Rect{X: max(0, m.width-hintWidth-lipgloss.Width(clock)), Y: y, Width: lipgloss.Width(clock), Height: TaskbarHeight},
		},
		taskbarItem{
			id:    "hint",
			label: m.styles.TaskbarText.Render(hint),
			rect:  geom.Rect{X: max(0, m.width-hintWidth), Y: y, Width: hintWidth, Height: TaskbarHeight},
		},
	)
	return items
}

func (m *Model) statusIndicators() string {
	var parts []string
	if m.netOnline {
		parts = append(parts, "online")
	}
	if m.batteryKnown {
		icon := "bat"
		if m.batteryCharging {
			icon = "chg"
		}
		parts = append(parts, fmt.Sprintf("%s:%d%%", icon, m.batteryPercent))
	}
	if len(parts) == 0 {
		return ""
	}
	text := " " + strings.Join(parts, " ") + " "
	return m.styles.TaskbarText.Render(text)
}

// taskbarWindows lists windows in the order they were opened, so buttons keep
// their place when the focus moves.
func (m *Model) taskbarWindows() []*wm.Window {
	windows := m.windows.Windows()
	sort.SliceStable(windows, func(i, j int) bool { return windows[i].Opened < windows[j].Opened })
	return windows
}

// clockText renders the taskbar clock from the last tick.
func (m *Model) clockText() string {
	return " " + m.now.Format("15:04") + " "
}

// startRect is where the start button sits.
func (m *Model) startRect() geom.Rect {
	for _, item := range m.taskbarItems() {
		if item.id == "start" {
			return item.rect
		}
	}
	return geom.Rect{}
}

// windowLabel is the title bar text: the focus mark, the name and the window
// controls. An app may rename its window, in which case its name is used.
func windowLabel(w *wm.Window, focused bool) string {
	return titledWindowLabel(w, focused, windowTitle(w))
}

// titledWindowLabel lays the title bar out around an arbitrary name. The name
// gets whatever room the controls leave, so a long one is cut rather than
// pushed under the buttons.
func titledWindowLabel(w *wm.Window, focused bool, name string) string {
	if name == "" {
		name = w.Spec.Name
	}
	mark := "○"
	if focused {
		mark = "●"
	}
	left := mark + " "
	controls := "[_][^][x]"
	innerWidth := max(0, w.Rect.Width-2)
	leftWidth := lipgloss.Width(left)
	ctlWidth := lipgloss.Width(controls)

	room := innerWidth - leftWidth - ctlWidth
	if room <= 0 {
		return left + truncate(name, max(0, innerWidth-leftWidth))
	}
	name = truncate(name, room)
	return left + name + strings.Repeat(" ", room-lipgloss.Width(name)) + controls
}
