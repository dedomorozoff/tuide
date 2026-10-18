package shell

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dedomorozoff/tuide/internal/geom"
)

// calendarState is the clock's popup month view: whether it is up, and which
// month it shows. The month is kept as its first day, so it can be moved on in
// whole months; the day comes from the clock, which is what lets the view
// follow the clock instead of freezing the day it was opened on.
type calendarState struct {
	open  bool
	month time.Time
}

// The month grid is seven columns of two cells, Monday first — the week the
// rest of the desktop keeps.
const (
	calendarColumns = 7
	calendarCell    = 2
	calendarGutter  = 1
)

// calendarGridWidth is how wide the week rows are in cells.
func calendarGridWidth() int {
	return calendarColumns*calendarCell + (calendarColumns-1)*calendarGutter
}

// toggleCalendar opens or closes the clock's month view. It always opens on the
// month the clock is in: a popup reopened days later comes back where the clock
// is, not on a month that was browsed to once.
func (m *Model) toggleCalendar() {
	if m.calendar.open {
		m.calendar = calendarState{}
		return
	}
	m.calendar = calendarState{open: true, month: monthStart(m.now)}
}

// shiftMonth moves the month on show by whole months.
func (m *Model) shiftMonth(months int) {
	m.calendar.month = m.calendar.month.AddDate(0, months, 0)
}

// calendarKey drives the month view and reports whether the key was its own.
// Only the navigation keys are claimed: anything else takes the popup down and
// carries on to the desktop, so the calendar is never a keyboard trap.
func (m *Model) calendarKey(key tea.Key) bool {
	if hasModifier(key.Mod, tea.ModCtrl|tea.ModAlt|tea.ModShift) {
		return false
	}

	switch key.Code {
	case tea.KeyEscape, tea.KeyEnter:
		m.calendar = calendarState{}
		return true
	case tea.KeyLeft:
		m.shiftMonth(-1)
		return true
	case tea.KeyRight:
		m.shiftMonth(1)
		return true
	case tea.KeyUp:
		m.shiftMonth(-12)
		return true
	case tea.KeyDown:
		m.shiftMonth(12)
		return true
	case tea.KeyHome:
		m.calendar.month = monthStart(m.now)
		return true
	}
	return false
}

// onCalendarClick dismisses the month view. The click that opened it arrives by
// the same road once the view is up, so a click on the clock is a toggle either
// way; a click anywhere else just closes the popup, the way a click outside a
// menu does.
func (m *Model) onCalendarClick() tea.Cmd {
	m.calendar = calendarState{}
	return nil
}

// calendarLayout renders the popup and works out where it lands: above the
// taskbar, at the right end, under the clock that opened it — the same anchor
// the launcher gets at the other end of the bar.
func (m *Model) calendarLayout() (string, geom.Rect) {
	palette := m.styles.Theme
	head := lipgloss.NewStyle().Foreground(palette.TextStrong).Background(palette.Panel).Bold(true)
	dim := lipgloss.NewStyle().Foreground(palette.TextDim).Background(palette.Panel)
	day := lipgloss.NewStyle().Foreground(palette.Text).Background(palette.Panel)
	today := lipgloss.NewStyle().Foreground(palette.Background).Background(palette.Accent).Bold(true)

	weekdays := []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
	lines := []string{
		head.Render(centre(m.calendar.month.Format("January 2006"), calendarGridWidth())),
		dim.Render(strings.Join(weekdays, strings.Repeat(" ", calendarGutter))),
	}

	for _, week := range weeksOf(m.calendar.month) {
		cells := make([]string, 0, calendarColumns)
		for _, number := range week {
			switch {
			case number == 0:
				cells = append(cells, day.Render(strings.Repeat(" ", calendarCell)))
			case m.isToday(number):
				cells = append(cells, today.Render(fmt.Sprintf("%*d", calendarCell, number)))
			default:
				cells = append(cells, day.Render(fmt.Sprintf("%*d", calendarCell, number)))
			}
		}
		lines = append(lines, strings.Join(cells, strings.Repeat(" ", calendarGutter)))
	}

	lines = append(lines, "", dim.Render("left/right month | up/down year | esc close"))

	content := strings.Join(lines, "\n")
	return content, m.calendarRect(content)
}

// isToday reports whether a day number of the month on show is the clock's day.
// A day number on its own means nothing unless the month is the clock's too.
func (m *Model) isToday(day int) bool {
	month := m.calendar.month
	return month.Month() == m.now.Month() && month.Year() == m.now.Year() && day == m.now.Day()
}

// weeksOf lays a month out as weeks of day numbers, zero standing for a cell
// before the first day or after the last. Weeks start on Monday.
func weeksOf(first time.Time) [][]int {
	offset := (int(first.Weekday()) + 6) % 7
	days := daysInMonth(first)

	weeks := make([][]int, 0, 6)
	week := make([]int, 0, calendarColumns)
	for range offset {
		week = append(week, 0)
	}
	for day := 1; day <= days; day++ {
		week = append(week, day)
		if len(week) == calendarColumns {
			weeks = append(weeks, week)
			week = make([]int, 0, calendarColumns)
		}
	}
	if len(week) > 0 {
		for len(week) < calendarColumns {
			week = append(week, 0)
		}
		weeks = append(weeks, week)
	}
	return weeks
}

// monthStart is the first day of the month a time falls in.
func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// daysInMonth is how many days the month of a date has: day zero of the month
// after it is the last day of this one, leap years included.
func daysInMonth(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
}

// calendarRect places the popup above the clock it came from.
func (m *Model) calendarRect(content string) geom.Rect {
	box := m.panelStyle().Render(content)

	bottom := m.windows.WorkArea().Bottom()
	width := min(lipgloss.Width(box), m.width)
	height := min(lipgloss.Height(box), max(0, bottom))

	return geom.Rect{
		X:      max(0, m.width-width),
		Y:      max(0, bottom-height),
		Width:  width,
		Height: height,
	}
}

// drawCalendar paints the month view.
func (m *Model) drawCalendar() {
	content, rect := m.calendarLayout()
	m.screen.Blit(rect, m.panelStyle().Render(content))
}

// centre pads a plain string out to the middle of a column.
func centre(s string, width int) string {
	pad := width - lipgloss.Width(s)
	if pad <= 0 {
		return s
	}
	left := pad / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", pad-left)
}
