package vt

// decSpecialGraphics is the DEC line drawing set. The characters from 0x5f
// to 0x7e stand for the box drawing pieces, and full screen programs draw
// their frames with it: Midnight Commander, for one, switches the set before
// every border.
var decSpecialGraphics = [32]rune{
	' ', '◆', '▒', '␉', '␌', '␍', '␊', '°',
	'±', '␤', '␋', '┘', '┐', '┌', '└', '┼',
	'⎺', '⎻', '─', '⎼', '⎽', '├', '┤', '┴',
	'┬', '│', '≤', '≥', 'π', '≠', '£', '·',
}

// charsetRune maps a character through the character set in use. Only the
// DEC special graphics set draws something other than the character itself.
func (v *VT) charsetRune(r rune) rune {
	set := v.charsets[0]
	if v.g1 {
		set = v.charsets[1]
	}
	if set != '0' || r < 0x5f || r > 0x7e {
		return r
	}
	return decSpecialGraphics[r-0x5f]
}
