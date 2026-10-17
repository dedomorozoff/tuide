package compositor

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const resetStyle = "\x1b[0m"

// Cell is one terminal cell: a grapheme plus the SGR prefixes that were active
// when it was written. Keeping them per cell means overwriting a cell cannot
// leak color into its neighbours.
//
// The background is kept apart from the rest of the style. Text that sets only
// a foreground, or none at all, must not punch the terminal's default
// background through whatever is under it, so a blit lets such a cell inherit
// the background it covers; a cell that names its own keeps it.
type Cell struct {
	Style      string
	Background string
	Text       string
	Width      int
}

// Blank returns a cell showing a space in the default style.
func Blank() Cell {
	return Cell{Text: " ", Width: 1}
}

// splitCells turns an ANSI-styled string into cells, carrying SGR state along.
// Escape sequences never become cells: SGR is folded into the style of the
// cells that follow, everything else is dropped.
func splitCells(s string) []Cell {
	if s == "" {
		return nil
	}

	var (
		cells  []Cell
		active sgrState
		state  byte
	)

	for len(s) > 0 {
		seq, width, n, next := ansi.DecodeSequence(s, state, nil)
		if n <= 0 {
			break
		}
		state = next
		s = s[n:]

		if isStyleSequence(seq) {
			active.apply(seq)
			continue
		}
		if isControlSequence(seq) {
			continue
		}
		if width < 1 {
			width = 1
		}
		cells = append(cells, Cell{
			Style:      active.String(),
			Background: active.background(),
			Text:       seq,
			Width:      width,
		})
	}

	return cells
}

func isStyleSequence(seq string) bool {
	return ansi.HasCsiPrefix(seq) && strings.HasSuffix(seq, "m")
}

func isControlSequence(seq string) bool {
	return ansi.HasCsiPrefix(seq) || ansi.HasOscPrefix(seq) ||
		ansi.HasApcPrefix(seq) || ansi.HasDcsPrefix(seq) ||
		ansi.HasSosPrefix(seq) || ansi.HasPmPrefix(seq) || ansi.HasEscPrefix(seq)
}

// cellRows splits styled text into rows of cells, dropping trailing newlines
// and returning at most maxRows rows.
func cellRows(s string, maxRows int) [][]Cell {
	if maxRows <= 0 {
		return nil
	}

	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "")

	lines := strings.Split(s, "\n")
	if len(lines) > maxRows {
		lines = lines[:maxRows]
	}

	rows := make([][]Cell, len(lines))
	for i, line := range lines {
		rows[i] = splitCells(line)
	}
	return rows
}

// joinRow renders cells back into a single line, restoring the default style
// at the end so nothing bleeds into the next row.
func joinRow(cells []Cell) string {
	var b strings.Builder
	style, background := "", ""
	for _, cell := range cells {
		if cell.Style != style || cell.Background != background {
			if cell.Style == "" && cell.Background == "" {
				b.WriteString(resetStyle)
			} else {
				b.WriteString(cell.Style)
				b.WriteString(cell.Background)
			}
			style, background = cell.Style, cell.Background
		}
		b.WriteString(cell.Text)
	}
	if style != "" || background != "" {
		b.WriteString(resetStyle)
	}
	return b.String()
}

// sgrState is the set of graphic renditions currently in effect. It keeps one
// sequence per attribute class, so restyling a cell never accumulates garbage.
type sgrState struct {
	active map[int]string
}

func (s *sgrState) reset() {
	s.active = nil
}

func (s *sgrState) set(key int, seq string) {
	if s.active == nil {
		s.active = make(map[int]string, 4)
	}
	s.active[key] = seq
}

func (s *sgrState) clear(keys ...int) {
	for _, key := range keys {
		delete(s.active, key)
	}
}

func (s *sgrState) String() string {
	if len(s.active) == 0 {
		return ""
	}
	var b strings.Builder
	for _, key := range slices.Sorted(maps.Keys(s.active)) {
		if key == keyBg {
			continue
		}
		b.WriteString(s.active[key])
	}
	return b.String()
}

// background is the SGR sequence that sets the current background, or empty
// when the style names none. It is rendered on its own so a cell can keep the
// background it draws over.
func (s *sgrState) background() string {
	if s.active == nil {
		return ""
	}
	return s.active[keyBg]
}

const (
	keyBold           = 1
	keyFaint          = 2
	keyItalic         = 3
	keyUnderline      = 4
	keyBlink          = 5
	keyReverse        = 7
	keyStrike         = 9
	keyFg             = 38
	keyBg             = 48
	keyUnderlineColor = 58
)

// apply folds one SGR sequence into the state.
func (s *sgrState) apply(seq string) {
	params := sgrParams(seq)
	if len(params) == 0 {
		s.reset()
		return
	}

	for i := 0; i < len(params); i++ {
		if params[i] == "" || params[i] == "0" {
			s.reset()
			continue
		}
		code, err := strconv.Atoi(params[i])
		if err != nil {
			continue
		}

		switch code {
		case 39:
			s.clear(keyFg)
			continue
		case 49:
			s.clear(keyBg)
			continue
		case 22:
			s.clear(keyBold, keyFaint)
			continue
		case 23:
			s.clear(keyItalic)
			continue
		case 24:
			s.clear(keyUnderline)
			continue
		case 25:
			s.clear(keyBlink)
			continue
		case 27:
			s.clear(keyReverse)
			continue
		case 28:
			continue
		case 29:
			s.clear(keyStrike)
			continue
		case keyFg, keyBg, keyUnderlineColor:
			consumed := extendedLen(params[i+1:])
			s.set(code, renderSGR(params[i:min(i+1+consumed, len(params))]))
			i += consumed
		case keyUnderline:
			consumed := underlineLen(params[i+1:])
			s.set(keyUnderline, renderSGR(params[i:min(i+1+consumed, len(params))]))
			i += consumed
		default:
			if code >= 30 && code <= 37 || code >= 90 && code <= 97 {
				s.set(keyFg, renderSGR(params[i:i+1]))
				continue
			}
			if code >= 40 && code <= 47 || code >= 100 && code <= 107 {
				s.set(keyBg, renderSGR(params[i:i+1]))
				continue
			}
			s.set(code, renderSGR(params[i:i+1]))
		}
	}
}

// extendedLen reports how many parameters belong to a 38/48/58 color.
func extendedLen(rest []string) int {
	if len(rest) == 0 {
		return 0
	}
	if strings.Contains(rest[0], ":") {
		return 1
	}
	switch rest[0] {
	case "2":
		return min(4, len(rest))
	case "5":
		return min(2, len(rest))
	default:
		return 0
	}
}

func underlineLen(rest []string) int {
	if len(rest) == 0 {
		return 0
	}
	if rest[0] == "" || rest[0] == "0" || rest[0] == "1" || rest[0] == "2" || rest[0] == "3" {
		return 1
	}
	return 0
}

func sgrParams(seq string) []string {
	body := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	if body == "" {
		return nil
	}
	return strings.Split(body, ";")
}

func renderSGR(params []string) string {
	return "\x1b[" + strings.Join(params, ";") + "m"
}
