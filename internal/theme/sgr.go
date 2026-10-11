package theme

import (
	"image/color"
	"strconv"
)

// Background returns the SGR sequence painting a background color.
func Background(c color.Color) string {
	return sgr(48, c)
}

// Foreground returns the SGR sequence painting a foreground color.
func Foreground(c color.Color) string {
	return sgr(38, c)
}

func sgr(layer int, c color.Color) string {
	r, g, b, _ := c.RGBA()
	return "\x1b[" + strconv.Itoa(layer) + ";2;" +
		strconv.FormatUint(uint64(r>>8), 10) + ";" +
		strconv.FormatUint(uint64(g>>8), 10) + ";" +
		strconv.FormatUint(uint64(b>>8), 10) + "m"
}
