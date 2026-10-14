package vt

import (
	"strconv"
	"strings"
)

// SGR attribute numbers. Flags below sgrAttrCount are plain on/off switches;
// the rest change the way one of the flags is drawn.
const (
	sgrReset     = 0
	sgrBold      = 1
	sgrFaint     = 2
	sgrItalic    = 3
	sgrUnderline = 4
	sgrBlink     = 5
	sgrReverse   = 7
	sgrStrike    = 9

	sgrDoubleUnderline = 21
	sgrFramed          = 51
	sgrEncircled       = 52
	sgrOverline        = 53

	sgrAttrCount = 54
)

// colorKind tells how a colour operand was written.
type colorKind uint8

const (
	colorDefault colorKind = iota
	colorIndexed
	colorRGB
)

// color is a terminal colour: the default, a palette index or a direct RGB
// triple.
type color struct {
	kind    colorKind
	index   int
	r, g, b byte
}

// params renders the colour as the operands of an SGR colour attribute.
func (c color) params(base int) []string {
	switch c.kind {
	case colorIndexed:
		return []string{strconv.Itoa(base), "5", strconv.Itoa(c.index)}
	case colorRGB:
		return []string{
			strconv.Itoa(base), "2",
			strconv.Itoa(int(c.r)), strconv.Itoa(int(c.g)), strconv.Itoa(int(c.b)),
		}
	default:
		return nil
	}
}

// rendition is the set of graphic attributes currently in effect. Keeping it
// structured means turning one attribute off never has to rewrite the style
// string of every cell behind it.
type rendition struct {
	flags             [sgrAttrCount]bool
	fg, bg, underline color
}

func (r rendition) has(attr int) bool {
	return attr >= 0 && attr < len(r.flags) && r.flags[attr]
}

// style renders the rendition as an SGR prefix, or "" for the default.
func (r rendition) style() string {
	if r == (rendition{}) {
		return ""
	}

	params := make([]string, 0, 8)
	attr := func(code int) {
		if r.has(code) {
			params = append(params, strconv.Itoa(code))
		}
	}
	attr(sgrBold)
	attr(sgrFaint)
	attr(sgrItalic)

	switch {
	case r.has(sgrOverline):
		params = append(params, strconv.Itoa(sgrOverline))
	case r.has(sgrEncircled):
		params = append(params, strconv.Itoa(sgrEncircled))
	case r.has(sgrFramed):
		params = append(params, strconv.Itoa(sgrFramed))
	case r.has(sgrDoubleUnderline):
		params = append(params, strconv.Itoa(sgrDoubleUnderline))
	default:
		attr(sgrUnderline)
	}

	attr(sgrBlink)
	attr(sgrReverse)
	attr(sgrStrike)

	if r.fg.kind != colorDefault {
		params = append(params, r.fg.params(38)...)
	}
	if r.bg.kind != colorDefault {
		params = append(params, r.bg.params(48)...)
	}
	if r.underline.kind != colorDefault {
		params = append(params, r.underline.params(58)...)
	}
	if len(params) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(params, ";") + "m"
}

// basicColor maps the 30-37, 40-47, 90-97 and 100-107 ranges onto palette
// indexes.
func basicColor(code int) (index int, ok bool) {
	switch {
	case code >= 30 && code <= 37:
		return code - 30, true
	case code >= 90 && code <= 97:
		return code - 90 + 8, true
	case code >= 40 && code <= 47:
		return code - 40, true
	case code >= 100 && code <= 107:
		return code - 100 + 8, true
	}
	return 0, false
}

// styleString is the SGR prefix stamped onto the next cell.
func (v *VT) styleString() string {
	if v.sgrCache == "" && v.sgr != (rendition{}) {
		v.sgrCache = v.sgr.style()
	}
	return v.sgrCache
}

// resetSGR puts the rendition back to the terminal default.
func (v *VT) resetSGR() {
	v.sgr = rendition{}
	v.sgrCache = ""
}

// applySGR folds one CSI m sequence into the current rendition.
func (v *VT) applySGR(params []int) {
	if len(params) == 0 {
		v.resetSGR()
		return
	}
	for i := 0; i < len(params); i++ {
		code := params[i]
		switch {
		case code == 0:
			v.resetSGR()
		case code == sgrBold, code == sgrFaint, code == sgrItalic, code == sgrUnderline,
			code == sgrBlink, code == sgrReverse, code == sgrStrike:
			v.sgr.flags[code] = true
		case code == sgrDoubleUnderline:
			v.sgr.flags[sgrUnderline] = true
			v.sgr.flags[sgrDoubleUnderline] = true
		case code == 22:
			v.sgr.flags[sgrBold], v.sgr.flags[sgrFaint] = false, false
		case code == 23:
			v.sgr.flags[sgrItalic] = false
		case code == 24:
			v.sgr.flags[sgrUnderline] = false
			v.sgr.flags[sgrDoubleUnderline] = false
		case code == 25:
			v.sgr.flags[sgrBlink] = false
		case code == 27:
			v.sgr.flags[sgrReverse] = false
		case code == 29:
			v.sgr.flags[sgrStrike] = false
		case code == sgrFramed, code == sgrEncircled, code == sgrOverline:
			v.sgr.flags[code] = true
		case code == 38, code == 48, code == 58:
			c, used := parseColor(params[i+1:])
			if used == 0 {
				break
			}
			i += used
			v.setColor(code, c)
		case code == 39:
			v.setColor(38, color{})
		case code == 49:
			v.setColor(48, color{})
		case code == 59:
			v.setColor(58, color{})
		default:
			if index, ok := basicColor(code); ok {
				if code < 40 || (code >= 90 && code <= 97) {
					v.setColor(38, color{kind: colorIndexed, index: index})
				} else {
					v.setColor(48, color{kind: colorIndexed, index: index})
				}
			}
		}
		v.sgrCache = ""
	}
}

func (v *VT) setColor(base int, c color) {
	switch base {
	case 38:
		v.sgr.fg = c
	case 48:
		v.sgr.bg = c
	case 58:
		v.sgr.underline = c
	}
	v.sgrCache = ""
}

// parseColor reads the operands of an extended colour selector and reports how
// many parameters it consumed.
func parseColor(rest []int) (color, int) {
	if len(rest) == 0 {
		return color{}, 0
	}
	switch rest[0] {
	case 5:
		if len(rest) < 2 {
			return color{}, 0
		}
		return color{kind: colorIndexed, index: int(clampByte(rest[1]))}, 2
	case 2:
		if len(rest) < 4 {
			return color{}, 0
		}
		return color{
			kind: colorRGB,
			r:    byte(clampByte(rest[1])),
			g:    byte(clampByte(rest[2])),
			b:    byte(clampByte(rest[3])),
		}, 4
	default:
		return color{}, 0
	}
}

func clampByte(v int) byte { return byte(min(max(v, 0), 255)) }
