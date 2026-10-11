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
		Name:          "daylight",
		Background:    hex("#ffffff"),
		Panel:         hex("#f6f8fa"),
		TitleBar:      hex("#eaeef2"),
		TitleBarFocus: hex("#0969da"),
		Taskbar:       hex("#f6f8fa"),
		Border:        hex("#d0d7de"),
		BorderFocused: hex("#0969da"),
		Text:          hex("#1f2328"),
		TextDim:       hex("#656d76"),
		TextStrong:    hex("#010409"),
		Accent:        hex("#0969da"),
		AccentDim:     hex("#0550ae"),
		Danger:        hex("#cf222e"),
		Warning:       hex("#9a6700"),
		Selection:     hex("#b6d7ff"),
		Cursor:        hex("#0969da"),
	})
}
