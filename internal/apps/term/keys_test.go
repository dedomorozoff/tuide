package term

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/vt"
)

func TestCursorKeysFollowTheModeTheProgramAskedFor(t *testing.T) {
	cases := []struct {
		name  string
		modes vt.Modes
		want  string
	}{
		{name: "normal", modes: vt.Modes{}, want: "\x1b[A"},
		{name: "application cursor keys", modes: vt.Modes{AppCursor: true}, want: "\x1bOA"},
		{name: "application keypad", modes: vt.Modes{AppKeypad: true}, want: "\x1bOA"},
	}
	for _, c := range cases {
		got := string(encodeKey(tea.Key{Code: tea.KeyUp}, c.modes))
		if got != c.want {
			t.Errorf("%s: up = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestHomeAndEndFollowTheKeypadMode(t *testing.T) {
	keypad := string(encodeKey(tea.Key{Code: tea.KeyHome}, vt.Modes{AppKeypad: true}))
	if keypad != "\x1bOH" {
		t.Errorf("home in application keypad = %q, want %q", keypad, "\x1bOH")
	}

	cursor := string(encodeKey(tea.Key{Code: tea.KeyHome}, vt.Modes{AppCursor: true}))
	if cursor != "\x1bOH" {
		t.Errorf("home with application cursor keys = %q, want %q", cursor, "\x1bOH")
	}
}

func TestFunctionKeysUseTheXtermLayout(t *testing.T) {
	if got := string(encodeKey(tea.Key{Code: tea.KeyF1}, vt.Modes{})); got != "\x1bOP" {
		t.Errorf("F1 = %q, want %q", got, "\x1bOP")
	}
	if got := string(encodeKey(tea.Key{Code: tea.KeyF10}, vt.Modes{})); got != "\x1b[21~" {
		t.Errorf("F10 = %q, want %q", got, "\x1b[21~")
	}
}
