package vt

import "unicode/utf8"

// parserState is one state of the DEC ANSI parser. The shape follows Paul
// Williams' state machine, which is the one every real terminal implements.
type parserState uint8

const (
	stateGround parserState = iota
	stateEscape
	stateEscapeIntermediate
	stateCSIEntry
	stateCSIParam
	stateCSIIntermediate
	stateCSIPassThrough
	stateCSIIgnore
	stateOSCString
	stateOSCIgnore
	stateDCSEntry
	stateDCSParam
	stateDCSIntermediate
	stateDCSPassThrough
	stateDCSIgnore
	stateSOSPMAPString
	stateUTF8
)

// maxControlSequence bounds the CSI parameter list so a runaway program cannot
// grow the emulator without limit.
const maxControlSequence = 32

// maxStringSequence bounds OSC and DCS payloads for the same reason.
const maxStringSequence = 4096

// parser is the decoding state carried between Write calls, so a sequence
// split across two reads still decodes correctly.
type parser struct {
	state parserState

	args         [maxControlSequence]int
	paramCount   int
	paramClosed  bool
	intermediate byte
	marker       byte

	utf8    [utf8.UTFMax]byte
	utf8At  int
	utf8Len int

	osc []byte
	dcs []byte
}

// reset puts the decoder back to ground without touching the screen, which is
// what a full reset of the terminal state means.
func (p *parser) reset() {
	p.state = stateGround
	p.paramCount = 0
	p.paramClosed = false
	p.intermediate = 0
	p.marker = 0
	p.utf8At, p.utf8Len = 0, 0
	p.osc = p.osc[:0]
	p.dcs = p.dcs[:0]
}

func (p *parser) startCSI() {
	p.state = stateCSIEntry
	p.paramCount = 0
	p.paramClosed = false
	p.intermediate = 0
	p.marker = 0
}

// openSlot makes sure digits have a parameter slot to accumulate into.
func (p *parser) openSlot() {
	if p.paramCount == 0 {
		p.paramCount = 1
		p.args[0] = 0
	}
	p.paramClosed = false
}

// nextSlot closes the current parameter and opens an empty one in its place.
func (p *parser) nextSlot() {
	if p.paramCount < maxControlSequence {
		p.paramCount++
	}
	p.args[p.paramCount-1] = -1
	p.paramClosed = true
}

func (p *parser) lastParam() int {
	if p.paramCount == 0 {
		return -1
	}
	return p.args[p.paramCount-1]
}

func (p *parser) setLastParam(value int) {
	if p.paramCount == 0 {
		p.openSlot()
	}
	p.args[p.paramCount-1] = value
}

// list returns the parameters collected so far.
func (p *parser) list() []int {
	if p.paramCount == 0 {
		return nil
	}
	return p.args[:p.paramCount]
}

// parseByte feeds one byte of child output through the decoder.
func (v *VT) parseByte(b byte) {
	p := &v.parser

	switch p.state {
	case stateGround:
		v.parseGround(b)
	case stateEscape:
		v.parseEscape(b)
	case stateEscapeIntermediate:
		if isFinal(b) {
			p.state = stateGround
		}
	case stateCSIEntry:
		v.parseCSI(b)
	case stateCSIParam:
		v.parseCSI(b)
	case stateCSIIntermediate:
		if isFinal(b) {
			v.executeCSI(b, p.list(), p.intermediate, p.marker)
			p.state = stateGround
		}
	case stateCSIPassThrough:
		if b >= 0x40 && b <= 0x7e {
			p.state = stateGround
		}
	case stateCSIIgnore:
		if b >= 0x40 && b <= 0x7e {
			p.state = stateGround
		}
	case stateOSCString:
		v.parseOSC(b)
	case stateOSCIgnore:
		if b == 0x07 || b == 0x9c || b == 0x18 || b == 0x1a {
			p.state = stateGround
		}
	case stateDCSEntry:
		v.parseDCS(b)
	case stateDCSParam:
		v.parseDCS(b)
	case stateDCSIntermediate:
		if isFinal(b) {
			p.state = stateDCSPassThrough
		}
	case stateDCSPassThrough:
		v.parseDCSPayload(b)
	case stateDCSIgnore:
		if b == 0x9c || b == 0x07 || b == 0x18 || b == 0x1a {
			p.state = stateGround
		}
	case stateSOSPMAPString:
		if b == 0x07 || b == 0x9c || b == 0x18 || b == 0x1a {
			p.state = stateGround
		}
	case stateUTF8:
		v.parseUTF8(b)
	}
}

func isFinal(b byte) bool { return b >= 0x40 && b <= 0x7e }

func (v *VT) parseGround(b byte) {
	p := &v.parser
	switch {
	case b == 0x1b:
		p.state = stateEscape
		p.intermediate = 0
	case b == 0x9b:
		p.startCSI()
	case b == 0x9d:
		p.state = stateOSCString
		p.osc = p.osc[:0]
	case b == 0x90:
		p.state = stateDCSEntry
		p.dcs = p.dcs[:0]
		p.paramCount = 0
		p.paramClosed = false
		p.intermediate = 0
		p.marker = 0
	case b == 0x98, b == 0x9e, b == 0x9f:
		p.state = stateSOSPMAPString
	case b == 0x7f:
		// DEL is discarded.
	case b < 0x20:
		v.executeControl(b)
	case b < 0x80:
		v.putRune(rune(b))
	case b < 0xa0:
		v.executeControl(b)
	case b < 0xc0:
		// A lone continuation byte is invalid; show a replacement.
		v.putRune(utf8.RuneError)
	case b < 0xf5:
		p.utf8[0] = b
		p.utf8At = 1
		p.utf8Len = utf8Expect(b)
		p.state = stateUTF8
	default:
		v.putRune(utf8.RuneError)
	}
}

func utf8Expect(first byte) int {
	switch {
	case first >= 0xf0:
		return 4
	case first >= 0xe0:
		return 3
	default:
		return 2
	}
}

// parseUTF8 assembles a multi-byte rune and prints it once complete.
func (v *VT) parseUTF8(b byte) {
	p := &v.parser
	if b < 0x80 || b >= 0xc0 {
		p.state = stateGround
		v.putRune(utf8.RuneError)
		v.parseByte(b)
		return
	}

	p.utf8[p.utf8At] = b
	p.utf8At++
	if p.utf8At < p.utf8Len {
		return
	}

	p.state = stateGround
	r, size := utf8.DecodeRune(p.utf8[:p.utf8At])
	if r == utf8.RuneError && size <= 1 {
		v.putRune(utf8.RuneError)
		return
	}
	v.putRune(r)
}

// parseEscape handles the byte right after ESC.
func (v *VT) parseEscape(b byte) {
	p := &v.parser
	switch {
	case b == '[':
		p.startCSI()
	case b == ']':
		p.state = stateOSCString
		p.osc = p.osc[:0]
	case b == 'P':
		p.state = stateDCSEntry
		p.dcs = p.dcs[:0]
		p.paramCount = 0
		p.paramClosed = false
		p.intermediate = 0
		p.marker = 0
	case b == 'X', b == '^', b == '_':
		p.state = stateSOSPMAPString
	case b >= 0x20 && b <= 0x2f:
		p.intermediate = b
		p.state = stateEscapeIntermediate
	case b >= 0x30 && b <= 0x3f:
		// ESC 7 and ESC 8 sit in the parameter byte range but are two
		// character sequences, so they run here rather than opening a CSI.
		v.executeEscape(b, p.intermediate)
		p.state = stateGround
	case isFinal(b):
		v.executeEscape(b, p.intermediate)
		p.state = stateGround
	default:
		p.state = stateGround
	}
}

// executeEscape dispatches the two character sequences that start with ESC.
func (v *VT) executeEscape(final byte, intermediate byte) {
	switch {
	case final == 'c' && intermediate == 0:
		v.fullReset()
	case final == 'D':
		v.index()
	case final == 'E':
		v.nextLine()
	case final == 'H':
		// A tab stop is set; the emulator only uses the default stops.
	case final == 'M':
		v.reverseIndex()
	case final == '7':
		v.saveCursor()
	case final == '8':
		v.restoreCursor()
	case final == '=':
		v.modes.AppKeypad = true
	case final == '>':
		v.modes.AppKeypad = false
	case final == 'n' && intermediate == 'o':
		// Locking shift, accepted and ignored.
	case final == '\\':
		// A stray string terminator.
	case intermediate == '(' || intermediate == ')' || intermediate == '*' || intermediate == '+':
		// Character set designation: only ASCII and UTF-8 are drawn, so the
		// sets a program may select make no difference.
	case intermediate == '#':
		// Double width and height lines are not implemented.
	}
}

// parseCSI collects parameters and runs the sequence on its final byte.
func (v *VT) parseCSI(b byte) {
	p := &v.parser
	switch {
	case b >= '0' && b <= '9':
		p.openSlot()
		p.setLastParam(max(p.lastParam(), 0)*10 + int(b-'0'))
	case b == ';':
		p.nextSlot()
	case b == ':':
		// Sub-parameters are not used by anything we run.
		p.nextSlot()
	case b >= '<' && b <= '?':
		p.marker = b
		p.state = stateCSIParam
	case b >= ' ' && b <= '/':
		p.intermediate = b
		p.state = stateCSIIntermediate
	case isFinal(b):
		v.executeCSI(b, p.list(), p.intermediate, p.marker)
		p.state = stateGround
	default:
		p.state = stateCSIIgnore
	}
}

func (v *VT) parseOSC(b byte) {
	p := &v.parser
	switch {
	case b == 0x07 || b == 0x9c:
		v.executeOSC(p.osc)
		p.state = stateGround
	case b == 0x1b:
		// ESC \ is the two byte string terminator; the payload is complete
		// before the terminator, so run it and ignore what follows.
		v.executeOSC(p.osc)
		p.osc = p.osc[:0]
		p.state = stateOSCIgnore
	case b == 0x18 || b == 0x1a:
		p.osc = p.osc[:0]
		p.state = stateGround
	default:
		if len(p.osc) < maxStringSequence {
			p.osc = append(p.osc, b)
		}
	}
}

func (v *VT) parseDCS(b byte) {
	p := &v.parser
	switch p.state {
	case stateDCSEntry:
		switch {
		case b >= '0' && b <= '9':
			p.openSlot()
			p.setLastParam(max(p.lastParam(), 0)*10 + int(b-'0'))
		case b == ';':
			p.nextSlot()
		case b >= '<' && b <= '?':
			p.marker = b
		case b >= ' ' && b <= '/':
			p.intermediate = b
			p.state = stateDCSIntermediate
		case isFinal(b):
			p.state = stateDCSPassThrough
		default:
			p.state = stateDCSIgnore
		}
	case stateDCSParam:
		switch {
		case b >= '0' && b <= '9':
			p.openSlot()
			p.setLastParam(max(p.lastParam(), 0)*10 + int(b-'0'))
		case b == ';':
			p.nextSlot()
		case b >= '<' && b <= '?':
			p.marker = b
		case b >= ' ' && b <= '/':
			p.intermediate = b
			p.state = stateDCSIntermediate
		case isFinal(b):
			p.state = stateDCSPassThrough
		default:
			p.state = stateDCSIgnore
		}
	}
}

// parseDCSPayload buffers a device control string, of which only the Sixel
// and iTerm2 graphics extensions exist and neither is supported here.
func (v *VT) parseDCSPayload(b byte) {
	p := &v.parser
	switch {
	case b == 0x1b:
		p.state = stateDCSIgnore
	case b == 0x9c || b == 0x07:
		p.state = stateGround
	case b == 0x18 || b == 0x1a:
		p.dcs = p.dcs[:0]
		p.state = stateGround
	default:
		if len(p.dcs) < maxStringSequence {
			p.dcs = append(p.dcs, b)
		}
	}
}

// fullReset is RIS: everything goes back to the power-on state.
func (v *VT) fullReset() {
	v.normal.clear()
	v.alt.clear()
	v.active = v.normal
	v.history.clear()
	v.offset = 0
	v.title = ""
	v.clipboard = ""
	v.replies = nil
	v.modes = Modes{AutoWrap: true}
	v.resetSGR()
	v.parser.reset()
	v.setTitle("")
}
