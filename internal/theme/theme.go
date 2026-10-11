package theme

import "image/color"

// Theme is a flat palette plus the styles every component renders with.
// Component code never picks a color on its own, it only reads from here.
type Theme struct {
	Name string

	Background    color.Color
	Panel         color.Color
	TitleBar      color.Color
	TitleBarFocus color.Color
	Taskbar       color.Color

	Border        color.Color
	BorderFocused color.Color

	Text       color.Color
	TextDim    color.Color
	TextStrong color.Color

	Accent    color.Color
	AccentDim color.Color
	Danger    color.Color
	Warning   color.Color
	Selection color.Color
	Cursor    color.Color
}
