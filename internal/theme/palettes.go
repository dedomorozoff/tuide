package theme

import (
	"fmt"
	"image/color"
	"strings"
)

func hex(s string) color.Color {
	value := strings.TrimPrefix(s, "#")
	if len(value) != 6 {
		panic("theme: bad hex color " + s)
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(value, "%02x%02x%02x", &r, &g, &b); err != nil {
		panic("theme: bad hex color " + s)
	}
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}
}

var palettes = map[string]Theme{}

func register(t Theme) {
	palettes[t.Name] = t
}

func init() {
	register(Theme{
		Name:          "midnight",
		Background:    hex("#0d1117"),
		Panel:         hex("#161b22"),
		TitleBar:      hex("#21262d"),
		TitleBarFocus: hex("#1f6feb"),
		Taskbar:       hex("#161b22"),
		Border:        hex("#30363d"),
		BorderFocused: hex("#58a6ff"),
		Text:          hex("#c9d1d9"),
		TextDim:       hex("#8b949e"),
		TextStrong:    hex("#f0f6fc"),
		Accent:        hex("#58a6ff"),
		AccentDim:     hex("#1f6feb"),
		Danger:        hex("#f85149"),
		Warning:       hex("#d29922"),
		Selection:     hex("#264f78"),
		Cursor:        hex("#58a6ff"),
	})

	register(Theme{
		Name:          "nord",
		Background:    hex("#2e3440"),
		Panel:         hex("#3b4252"),
		TitleBar:      hex("#434c5e"),
		TitleBarFocus: hex("#5e81ac"),
		Taskbar:       hex("#3b4252"),
		Border:        hex("#4c566a"),
		BorderFocused: hex("#88c0d0"),
		Text:          hex("#d8dee9"),
		TextDim:       hex("#81a1c1"),
		TextStrong:    hex("#eceff4"),
		Accent:        hex("#88c0d0"),
		AccentDim:     hex("#5e81ac"),
		Danger:        hex("#bf616a"),
		Warning:       hex("#ebcb8b"),
		Selection:     hex("#434c5e"),
		Cursor:        hex("#88c0d0"),
	})
}
