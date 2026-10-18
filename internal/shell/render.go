package shell

import (
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/geom"
	"github.com/dedomorozoff/tuide/internal/theme"
	"github.com/dedomorozoff/tuide/internal/wm"
)

// startPaneLit and startPaneShadow are the two halves of the mark the menu
// button falls back to when no label is configured: half blocks, so the mark
// reads as two panes with a gutter between them — the desktop this window
// manager builds, in two cells. Half blocks belong to the East Asian ambiguous
// width class, so a terminal that counts ambiguous glyphs as two cells may draw
// the mark wider than we measure it; box drawing would be safer, but nothing in
// that block reads as a pane.
const (
	startPaneLit    = "▌"
	startPaneShadow = "▐"
)

// startLabelMax is how many cells a configured label may take on the bar. The
// config already holds a label to StartLabelMax runes, and this is the second
// guard: a label of wide graphemes is wider than it is long, and the bar has to
// keep its window buttons.
const startLabelMax = 12

// startButton renders the taskbar's menu button. It is a chip in the accent
// colour rather than more text on the bar, so it reads as the one button that
// is not a window. The chip carries the configured label, or the built-in mark
// when the config has none, and lights up while the launcher is open.
//
// Every cell is styled on its own, the padding included: a styled string nested
// inside another one loses its background where the inner reset lands, and on a
// taskbar that shows up as a hole in the bar.
func (m *Model) startButton() string {
	background := m.styles.Theme.Accent
	if m.launcher.open {
		background = m.styles.Theme.TextStrong
	}
	chip := lipgloss.NewStyle().Background(background)
	ink := lipgloss.NewStyle().Foreground(m.styles.Theme.Background).Background(background)
	shadow := lipgloss.NewStyle().Foreground(m.styles.Theme.AccentDim).Background(background)

	face := ink.Render(startPaneLit) + shadow.Render(startPaneShadow)
	if label := truncate(m.cfg.StartLabel, startLabelMax); label != "" {
		face = ink.Render(label)
	}

	return chip.Render(" ") + face + chip.Render(" ")
}

func (m *Model) drawWindow(w *wm.Window, focused bool) {
	m.screen.Fill(w.Rect, theme.Background(m.styles.Theme.Background))

	frame, title := m.styles.WindowChrome(focused, w.Rect.Width, w.Rect.Height)
	frameStyle := frame
	m.screen.Blit(w.Rect, frameStyle.Render(""))
	m.screen.Blit(w.TitleRect(), title.Render(m.windowLabel(w, focused)))

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
// button, the clock or the status indicators. Drawing and hit-testing share the
// layout so they can never disagree.
type taskbarItem struct {
	id    string
	label string
	rect  geom.Rect
}

// taskbarItems places every element on the bar. Window buttons that would not
// fit next to the clock and the indicators are dropped.
func (m *Model) taskbarItems() []taskbarItem {
	y := max(0, m.height-TaskbarHeight)

	start := m.startButton()

	items := []taskbarItem{{
		id:    "start",
		label: start,
		rect:  geom.Rect{Y: y, Width: lipgloss.Width(start), Height: TaskbarHeight},
	}}
	used := items[0].rect.Width

	focused := m.windows.Focused()
	indicators := m.statusIndicators()
	indWidth := lipgloss.Width(indicators)
	clockWidth := lipgloss.Width(m.clockText())
	rightWidth := indWidth + clockWidth
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
			rect:  geom.Rect{X: max(0, m.width-clockWidth-indWidth), Y: y, Width: indWidth, Height: TaskbarHeight},
		})
	}
	items = append(items, taskbarItem{
		id:    "clock",
		label: clock,
		rect:  geom.Rect{X: max(0, m.width-clockWidth), Y: y, Width: clockWidth, Height: TaskbarHeight},
	})
	return items
}

// statusIndicators renders the bar's status light. The network light is silent
// while the network is fine: a light that is always on says nothing, and the
// desktop only lights it up once a probe has actually failed.
func (m *Model) statusIndicators() string {
	if m.net != netOffline {
		return ""
	}
	return m.styles.TaskbarWarning.Render(" offline ")
}

// taskbarWindows lists windows in the order they were opened, so buttons keep
// their place when the focus moves.
func (m *Model) taskbarWindows() []*wm.Window {
	windows := m.windows.Windows()
	sort.SliceStable(windows, func(i, j int) bool { return windows[i].Opened < windows[j].Opened })
	return windows
}

// windowSlots numbers the windows the way alt+1..9 and the taskbar buttons see
// them: one slot per window of the active workspace, counted in the order the
// windows were opened — the minimized ones still hold their number, they are
// listed on the bar all the same. The number must not depend on the stacking
// order or on how many buttons happen to fit on a narrow bar, or the same key
// would mean a different window from one frame to the next.
func (m *Model) windowSlots() map[string]int {
	windows := m.taskbarWindows()
	slots := make(map[string]int, len(windows))
	for i, w := range windows {
		slots[w.ID] = i + 1
	}
	return slots
}

// clockText renders the taskbar clock from the last tick.
func (m *Model) clockText() string {
	return " " + m.now.Format("15:04") + " "
}

// startRect is where the start button sits.
func (m *Model) startRect() geom.Rect { return m.taskbarRect("start") }

// clockRect is where the taskbar clock sits.
func (m *Model) clockRect() geom.Rect { return m.taskbarRect("clock") }

// taskbarRect is where the element with this id sits, empty when the bar does
// not show it. Hit-testing and hit-testing's callers ask here rather than
// walking the item list themselves.
func (m *Model) taskbarRect(id string) geom.Rect {
	for _, item := range m.taskbarItems() {
		if item.id == id {
			return item.rect
		}
	}
	return geom.Rect{}
}

// windowLabel is the title bar text: the focus mark, the name and the window
// controls. An app may rename its window, in which case its name is used.
func (m *Model) windowLabel(w *wm.Window, focused bool) string {
	return m.titledWindowLabel(w, focused, windowTitle(w))
}

// titledWindowLabel lays the title bar out around an arbitrary name. The name
// gets whatever room the controls leave, so a long one is cut rather than
// pushed under the buttons.
func (m *Model) titledWindowLabel(w *wm.Window, focused bool, name string) string {
	if name == "" {
		name = w.Spec.Name
	}
	mark := "o"
	if focused {
		mark = "*"
	}
	left := mark + " "
	controls := m.windowControls()
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

// windowControls is the title bar's three buttons, one cell group each and
// painted in their own colors so they read as buttons even without a cursor.
// A reset leads the strip: lipgloss closes every styled segment with one, so
// without it the first button would keep the title bar's own background while
// the two behind it would not.
func (m *Model) windowControls() string {
	const reset = "\x1b[m"
	return reset + m.styles.Hint.Render("[_]") +
		m.styles.Warning.Render("[■]") +
		m.styles.Danger.Render("[x]")
}
