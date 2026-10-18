package shell

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/theme"
)

// clickClock sends a click to the middle of the taskbar clock.
func clickClock(t *testing.T, m *Model, button tea.MouseButton) {
	t.Helper()

	rect := m.clockRect()
	if rect.Width == 0 {
		t.Fatal("the taskbar is not showing a clock")
	}
	m.Update(tea.MouseClickMsg(tea.Mouse{
		X:      rect.X + rect.Width/2,
		Y:      rect.Y,
		Button: button,
	}))
}

// monthOf is the month on show, the way a user would name it.
func monthOf(m *Model) string { return m.calendar.month.Format("2006-01") }

func TestClockClickOpensTheMonthView(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()

	clickClock(t, m, tea.MouseLeft)
	if !m.calendar.open {
		t.Fatal("a click on the clock should open the month view")
	}
	if got := monthOf(m); got != "2026-10" {
		t.Errorf("month = %s, want the clock's own month", got)
	}

	clickClock(t, m, tea.MouseLeft)
	if m.calendar.open {
		t.Error("a second click on the clock should put the month view away")
	}
}

func TestClickAwayDismissesTheMonthView(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()
	m.toggleCalendar()

	m.Update(tea.MouseClickMsg(tea.Mouse{X: 2, Y: 2, Button: tea.MouseLeft}))

	if m.calendar.open {
		t.Error("a click anywhere else should dismiss the month view")
	}
}

func TestMonthViewStartsOnTheClockMonthEvenAfterBrowsing(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()

	m.toggleCalendar()
	m.shiftMonth(3)
	m.toggleCalendar()

	m.toggleCalendar()
	if got := monthOf(m); got != "2026-10" {
		t.Errorf("month = %s, want the clock's month again", got)
	}
}

func TestCalendarKeysMoveTheMonth(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()
	m.toggleCalendar()

	press(t, m, tea.Key{Code: tea.KeyRight})
	if got := monthOf(m); got != "2026-11" {
		t.Errorf("right = %s, want the month after the clock's", got)
	}

	press(t, m, tea.Key{Code: tea.KeyLeft})
	press(t, m, tea.Key{Code: tea.KeyLeft})
	if got := monthOf(m); got != "2026-09" {
		t.Errorf("left twice = %s, want the month before the clock's", got)
	}

	press(t, m, tea.Key{Code: tea.KeyDown})
	if got := monthOf(m); got != "2027-09" {
		t.Errorf("down = %s, want a year on", got)
	}

	press(t, m, tea.Key{Code: tea.KeyUp})
	press(t, m, tea.Key{Code: tea.KeyHome})
	if got := monthOf(m); got != "2026-10" {
		t.Errorf("home = %s, want back at the clock's month", got)
	}

	press(t, m, tea.Key{Code: tea.KeyEscape})
	if m.calendar.open {
		t.Error("esc should close the month view")
	}
}

func TestCalendarStepsAsideForTheRestOfTheKeyboard(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()
	m.toggleCalendar()

	press(t, m, tea.Key{Code: 'd', Mod: tea.ModCtrl | tea.ModAlt})

	if m.calendar.open {
		t.Error("a desktop chord should take the month view down")
	}
	if !m.inspector {
		t.Error("the chord the calendar stepped aside for should still run")
	}
}

func TestCalendarKeyIgnoresDesktopChords(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()
	m.toggleCalendar()

	month := monthOf(m)
	if m.calendarKey(tea.Key{Code: tea.KeyLeft, Mod: tea.ModCtrl | tea.ModAlt}) {
		t.Error("ctrl+alt+left is the desktop's window move, not the calendar's")
	}
	if got := monthOf(m); got != month {
		t.Errorf("month = %s, want it left where it was", got)
	}
}

func TestCalendarMarksTodayOnlyInTheClockMonth(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()
	m.toggleCalendar()
	_ = m.View()

	palette := theme.NewStyles(theme.Get(theme.DefaultName)).Theme
	if got := accentCells(t, m, palette); got != calendarCell {
		t.Errorf("today takes %d accent cells, want %d", got, calendarCell)
	}

	m.shiftMonth(1)
	_ = m.View()
	if got := accentCells(t, m, palette); got != 0 {
		t.Errorf("a month that is not the clock's has %d accent cells, want none", got)
	}
}

// accentCells counts the cells of the month view painted in the accent colour,
// which is how the grid marks the clock's day.
func accentCells(t *testing.T, m *Model, palette theme.Theme) int {
	t.Helper()

	_, rect := m.calendarLayout()
	accent := theme.Background(palette.Accent)

	count := 0
	for y := rect.Y; y < rect.Bottom(); y++ {
		for x := rect.X; x < rect.Right(); x++ {
			if m.screen.At(x, y).Background == accent {
				count++
			}
		}
	}
	return count
}

// TestCalendarPaintsEveryCellOfItsOwn pins the trap the popup is built around:
// a cell that names no background inherits the one it covers, so any cell the
// calendar leaves unstyled shows the window underneath instead of the panel.
func TestCalendarPaintsEveryCellOfItsOwn(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()
	m.toggleCalendar()
	_ = m.View()

	_, rect := m.calendarLayout()
	for y := rect.Y; y < rect.Bottom(); y++ {
		for x := rect.X; x < rect.Right(); x++ {
			if got := m.screen.At(x, y).Background; got == "" {
				t.Errorf("cell %d,%d carries no background: the window under the popup would show through", x, y)
			}
		}
	}
}

func TestWeeksOfLaysOutTheMonth(t *testing.T) {
	// October 2026 starts on a Thursday and ends on a Saturday, so it fills
	// five weeks with three empty cells in front.
	weeks := weeksOf(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC))
	if len(weeks) != 5 {
		t.Fatalf("weeks = %d, want 5", len(weeks))
	}

	want := []int{0, 0, 0, 1, 2, 3, 4}
	for i, day := range want {
		if weeks[0][i] != day {
			t.Errorf("first week = %v, want %v", weeks[0], want)
			break
		}
	}

	last := weeks[len(weeks)-1]
	if len(last) != calendarColumns {
		t.Fatalf("last week has %d cells, want a full %d", len(last), calendarColumns)
	}
	if last[5] != 31 {
		t.Errorf("last week = %v, want the 31st on the Saturday", last)
	}
}

func TestDaysInMonth(t *testing.T) {
	cases := []struct {
		date time.Time
		days int
	}{
		{time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC), 31},
		{time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC), 28},
		{time.Date(2024, time.February, 1, 0, 0, 0, 0, time.UTC), 29},
		{time.Date(2026, time.December, 1, 0, 0, 0, 0, time.UTC), 31},
	}
	for _, tc := range cases {
		if got := daysInMonth(tc.date); got != tc.days {
			t.Errorf("%s = %d days, want %d", tc.date.Format("January 2006"), got, tc.days)
		}
	}
}

func TestCalendarOverlayFrame(t *testing.T) {
	m := newTestDesktop()
	_ = m.Init()
	m.toggleCalendar()
	assertGolden(t, "desktop-calendar", render(t, m))
}
