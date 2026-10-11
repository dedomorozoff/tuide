package shell

import (
	"sort"

	"charm.land/lipgloss/v2"

	"github.com/dedomorozoff/tuide/internal/geom"
	"github.com/dedomorozoff/tuide/internal/theme"
	"github.com/dedomorozoff/tuide/internal/wm"
)

const hint = " ctrl+q quit "

// startLabel is the taskbar's menu button; clicking it opens the launcher.
const startLabel = "≡ start"

func (m *Model) drawWindow(w *wm.Window, focused bool) {
	m.screen.Fill(w.Rect, theme.Background(m.styles.Theme.Background))

	frame, title := m.styles.WindowChrome(focused, w.Rect.Width, w.Rect.Height)
	m.screen.Blit(w.Rect, frame.Render(title.Render(windowLabel(w, focused))))

	if w.App == nil {
		return
	}
	m.screen.BlitView(w.ContentRect(), w.App.View(w.Context(focused)))
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
	rightWidth := lipgloss.Width(m.clockText()) + hintWidth
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

func windowLabel(w *wm.Window, focused bool) string {
	mark := "○"
	switch {
	case w.State == wm.Minimized:
		mark = "○"
	case focused:
		mark = "●"
	}
	return mark + " " + w.Spec.Name
}
