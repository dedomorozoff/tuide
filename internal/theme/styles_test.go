package theme

import (
	"testing"

	"charm.land/lipgloss/v2"
)

// TestWindowFrameCarriesTheBackground pins the fix for window bodies painted in
// the host terminal's own colours. The compositor renders a cell with no style
// as a full reset, so a border style without a background left the whole window
// interior unstyled: on a terminal with a light profile the body came out as
// white text on a white block.
func TestWindowFrameCarriesTheBackground(t *testing.T) {
	palette := Get(DefaultName)
	styles := NewStyles(palette)

	frames := map[string]lipgloss.Style{
		"Border":      styles.Border,
		"BorderFocus": styles.BorderFocus,
	}
	for name, style := range frames {
		if style.GetBackground() == nil {
			t.Errorf("%s has no background, the window interior would fall back to the terminal", name)
		}
	}
}

// TestTheDefaultPaletteIsDark keeps the desktop dark out of the box. A light
// palette is still available by name, but nothing about a fresh install should
// paint the screen white.
func TestTheDefaultPaletteIsDark(t *testing.T) {
	if DefaultName != "midnight" {
		t.Errorf("default palette = %q, want midnight", DefaultName)
	}

	palette := Get(DefaultName)
	r, g, b, _ := palette.Background.RGBA()
	luma := float64(r>>8) + float64(g>>8) + float64(b>>8)
	if luma > 3*128*257 {
		t.Errorf("default background is light: %v", palette.Background)
	}
}
