package vt

import (
	"strings"
	"testing"
)

func TestPlainTextLandsOnScreen(t *testing.T) {
	v := New(20, 4)
	v.Write([]byte("hello"))

	if got := v.Line(0); got != "hello" {
		t.Fatalf("Line(0) = %q, want %q", got, "hello")
	}
	x, y, _, _ := v.CursorAt()
	if x != 5 || y != 0 {
		t.Fatalf("cursor at %d,%d, want 5,0", x, y)
	}
}

func TestCarriageReturnOverwritesInPlace(t *testing.T) {
	v := New(20, 3)
	v.Write([]byte("abc\rXY"))

	if got := v.Line(0); got != "XYc" {
		t.Fatalf("Line(0) = %q, want %q", got, "XYc")
	}
	if x, _, _, _ := v.CursorAt(); x != 2 {
		t.Fatalf("cursor column = %d, want 2", x)
	}
}

func TestAutoWrapMovesToNextRow(t *testing.T) {
	v := New(4, 3)
	v.Write([]byte("abcd"))

	if got := v.Line(0); got != "abcd" {
		t.Fatalf("Line(0) = %q, want abcd", got)
	}
	v.Write([]byte("e"))
	if got := v.Line(1); got != "e" {
		t.Fatalf("Line(1) = %q, want e", got)
	}
	if x, y, _, _ := v.CursorAt(); x != 1 || y != 1 {
		t.Fatalf("cursor at %d,%d, want 1,1", x, y)
	}
}

func TestWrapOnlyWrapsWhenANewCharacterArrives(t *testing.T) {
	v := New(3, 3)
	v.Write([]byte("abc"))

	x, y, _, _ := v.CursorAt()
	if x != 2 || y != 0 {
		t.Fatalf("cursor parked at %d,%d, want 2,0", x, y)
	}
}

func TestAutowrapOffPinsTheCursorToTheRightMargin(t *testing.T) {
	v := New(3, 2)
	v.Write([]byte("\x1b[?7labcdef"))

	if got := v.Line(0); got != "abf" {
		t.Fatalf("Line(0) = %q, want the cursor pinned over the margin", got)
	}
}

func TestLineFeedScrollsAndFeedsHistory(t *testing.T) {
	v := New(10, 2)
	v.Write([]byte("one\r\ntwo\r\nthree"))

	if got := v.Line(0); got != "two" {
		t.Fatalf("Line(0) = %q, want two", got)
	}
	if got := v.Line(1); got != "three" {
		t.Fatalf("Line(1) = %q, want three", got)
	}
	if v.ScrollbackLen() != 1 {
		t.Fatalf("scrollback = %d lines, want 1", v.ScrollbackLen())
	}
	if !v.AtBottom() {
		t.Error("the viewport should still show the live screen")
	}

	v.ScrollUp(1)
	if !strings.Contains(v.Line(0), "one") {
		t.Fatalf("Line(0) after scroll = %q, want the history row", v.Line(0))
	}
	v.ScrollToBottom()
	if got := v.Line(0); got != "two" {
		t.Fatalf("Line(0) after scroll to bottom = %q, want two", got)
	}
}

func TestScrollbackStopsAtTheTop(t *testing.T) {
	v := New(10, 2)
	v.Write([]byte("a\r\nb\r\nc\r\nd\r\ne"))

	v.ScrollUp(100)
	if v.Offset() != v.ScrollbackLen() {
		t.Fatalf("offset = %d, want the whole history of %d", v.Offset(), v.ScrollbackLen())
	}
}

func TestEraseInDisplayVariants(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"to end", "abcdef\x1b[1;3H\x1b[J", "ab"},
		{"to start", "abcdef\x1b[1;3H\x1b[1J", "   def"},
		{"all", "abcdef\x1b[2J", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := New(10, 1)
			v.Write([]byte(tc.input))
			if got := v.Line(0); got != tc.want {
				t.Errorf("Line(0) = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEraseInLine(t *testing.T) {
	v := New(10, 1)
	v.Write([]byte("abcdef\x1b[1;3H\x1b[K"))
	if got := v.Line(0); got != "ab" {
		t.Fatalf("Line(0) = %q, want ab", got)
	}
}

func TestScrollingRegionKeepsTextOutsideIt(t *testing.T) {
	v := New(6, 4)
	v.Write([]byte("\x1b[2;3r")) // rows 2..3 in one based terms
	v.Write([]byte("\x1b[2;1Hmiddle"))
	v.Write([]byte("\x1b[3J"))

	if got := v.Line(0); got != "" {
		t.Errorf("the row above the region changed: %q", got)
	}
	if got := v.Line(1); got != "middle" {
		t.Errorf("Line(1) = %q, want middle", got)
	}
}

func TestOriginModeConfinesTheCursorToTheRegion(t *testing.T) {
	v := New(10, 6)
	v.Write([]byte("\x1b[3;5r\x1b[?6h\x1b[1;1HX"))

	if _, y, _, _ := v.CursorAt(); y != 2 {
		t.Fatalf("cursor row = %d, want the top of the region at 2", y)
	}
}

func TestInsertAndDeleteCharacters(t *testing.T) {
	v := New(10, 1)
	v.Write([]byte("abcdef\x1b[1;3H\x1b[2@"))
	if got := v.Line(0); got != "ab  cdef" {
		t.Fatalf("Line(0) = %q, want %q", got, "ab  cdef")
	}

	v = New(10, 1)
	v.Write([]byte("abcdef\x1b[1;3H\x1b[2P"))
	if got := v.Line(0); got != "abef" {
		t.Fatalf("Line(0) = %q, want abef", got)
	}
}

func TestEraseCharacters(t *testing.T) {
	v := New(10, 1)
	v.Write([]byte("abcdef\x1b[1;3H\x1b[2X"))
	if got := v.Line(0); got != "ab  ef" {
		t.Fatalf("Line(0) = %q, want %q", got, "ab  ef")
	}
}

func TestSGRIsStampedOnCellsAndResetsCleanly(t *testing.T) {
	v := New(10, 1)
	v.Write([]byte("a\x1b[1;31mb\x1b[0mc"))

	if style := v.cell(0, 0).Style; style != "" {
		t.Errorf("plain cell style = %q, want empty", style)
	}
	red := v.cell(1, 0).Style
	if !strings.Contains(red, "1") || !strings.Contains(red, "38;5;1") {
		t.Errorf("styled cell = %q, want bold red", red)
	}
	if style := v.cell(2, 0).Style; style != "" {
		t.Errorf("cell after reset = %q, want empty", style)
	}
}

func TestTrueColorSGR(t *testing.T) {
	v := New(4, 1)
	v.Write([]byte("\x1b[38;2;12;34;56mX"))

	if style := v.cell(0, 0).Style; style != "\x1b[38;2;12;34;56m" {
		t.Fatalf("style = %q, want the direct RGB form", style)
	}
}

func TestSGRResetClearsEveryAttribute(t *testing.T) {
	v := New(4, 1)
	v.Write([]byte("\x1b[1;3;4;7;38;5;9m\x1b[0mX"))

	if style := v.cell(0, 0).Style; style != "" {
		t.Fatalf("style after reset = %q, want empty", style)
	}
}

func TestUnderlineOffUsesTheWidelyReadCode(t *testing.T) {
	v := New(4, 1)
	v.Write([]byte("\x1b[4m\x1b[24mX"))

	if style := v.cell(0, 0).Style; style != "" {
		t.Fatalf("style = %q, want no underline", style)
	}
}

func TestAlternateScreenIsSeparateAndRestoresThePrimary(t *testing.T) {
	v := New(10, 3)
	v.Write([]byte("primary\r\n\x1b[?1049h"))
	v.Write([]byte("alt"))

	if !v.Modes().AltScreen {
		t.Error("alt screen mode should be set")
	}
	if got := v.Line(0); got != "alt" {
		t.Errorf("alt screen Line(0) = %q, want alt", got)
	}

	v.Write([]byte("\x1b[?1049l"))
	if v.Modes().AltScreen {
		t.Error("alt screen mode should be cleared")
	}
	if got := v.Line(0); got != "primary" {
		t.Errorf("primary Line(0) = %q, want primary", got)
	}
}

func TestAlternateScreenScrollsWithoutTouchingHistory(t *testing.T) {
	v := New(6, 2)
	v.Write([]byte("aa\r\nbb\x1b[?1049hcc\r\ndd\r\nee"))
	v.Write([]byte("\x1b[?1049l"))

	if v.ScrollbackLen() != 0 {
		t.Fatalf("scrollback = %d, want 0: alt screen lines are discarded", v.ScrollbackLen())
	}
}

func TestCursorVisibilityAndShape(t *testing.T) {
	v := New(6, 2)
	v.Write([]byte("\x1b[?25l"))
	if _, _, _, visible := v.CursorAt(); visible {
		t.Error("cursor should be hidden")
	}

	v.Write([]byte("\x1b[?25h\x1b[5 q"))
	_, _, shape, visible := v.CursorAt()
	if !visible {
		t.Error("cursor should be visible again")
	}
	if shape != CursorBar {
		t.Errorf("cursor shape = %d, want bar", shape)
	}
}

func TestSaveAndRestoreCursor(t *testing.T) {
	v := New(10, 5)
	v.Write([]byte("\x1b[3;5H\x1b7\x1b[1;1H\x1b8"))

	if x, y, _, _ := v.CursorAt(); x != 4 || y != 2 {
		t.Fatalf("cursor at %d,%d, want 4,2", x, y)
	}
}

func TestOSCSetsTheTitle(t *testing.T) {
	v := New(10, 2)
	var seen string
	v.OnTitle = func(title string) { seen = title }

	v.Write([]byte("\x1b]0;my shell\x07"))
	if v.Title() != "my shell" || seen != "my shell" {
		t.Fatalf("title = %q, callback = %q, want my shell", v.Title(), seen)
	}

	v.Write([]byte("\x1b]2;other\x1b\\"))
	if v.Title() != "other" {
		t.Fatalf("title after ST terminated OSC = %q, want other", v.Title())
	}
}

func TestBellCallback(t *testing.T) {
	v := New(4, 1)
	rang := false
	v.OnBell = func() { rang = true }

	v.Write([]byte("a\x07"))
	if !rang {
		t.Error("BEL should reach the callback")
	}
}

func TestDeviceStatusReportsTheCursor(t *testing.T) {
	v := New(10, 5)
	v.Write([]byte("\x1b[3;7H\x1b[6n"))

	if got := string(v.TakeReplies()); got != "\x1b[3;7R" {
		t.Fatalf("reply = %q, want \\x1b[3;7R", got)
	}
	if v.TakeReplies() != nil {
		t.Error("replies should be drained once taken")
	}
}

func TestPrimaryDeviceAttributes(t *testing.T) {
	v := New(10, 5)
	v.Write([]byte("\x1b[c"))

	if got := string(v.TakeReplies()); !strings.HasPrefix(got, "\x1b[?62;") || !strings.HasSuffix(got, "c") {
		t.Fatalf("reply = %q, want a device attributes report", got)
	}
}

func TestWindowSizeQuery(t *testing.T) {
	v := New(80, 24)
	v.Write([]byte("\x1b[18t"))

	if got := string(v.TakeReplies()); got != "\x1b[8;24;80t" {
		t.Fatalf("reply = %q, want \\x1b[8;24;80t", got)
	}
}

func TestModesFollowSetAndReset(t *testing.T) {
	v := New(10, 3)
	v.Write([]byte("\x1b[?1h\x1b[?2004h\x1b[?1000h\x1b[?1006h\x1b[4h"))

	modes := v.Modes()
	if !modes.AppCursor || !modes.BracketedPaste || !modes.Insert {
		t.Fatalf("modes = %+v, want app cursor, bracketed paste and insert", modes)
	}
	if modes.Mouse != MouseSGR {
		t.Fatalf("mouse mode = %d, want SGR", modes.Mouse)
	}

	v.Write([]byte("\x1b[?2004l\x1b[?1000l\x1b[4l"))
	modes = v.Modes()
	if modes.BracketedPaste || modes.Insert {
		t.Fatalf("modes = %+v, want bracketed paste and insert off", modes)
	}
	if modes.Mouse != MouseOff {
		t.Fatalf("mouse mode = %d, want off", modes.Mouse)
	}
}

func TestWideRunesTakeTwoCells(t *testing.T) {
	v := New(6, 1)
	v.Write([]byte("界a"))

	if cell := v.cell(0, 0); cell.Width != 2 || cell.Text != "界" {
		t.Fatalf("first cell = %+v, want a two cell 界", cell)
	}
	if cell := v.cell(1, 0); cell.Width != 0 || cell.Text != "" {
		t.Fatalf("continuation cell = %+v, want an empty cell of width 0", cell)
	}
	if cell := v.cell(2, 0); cell.Width != 1 || cell.Text != "a" {
		t.Fatalf("third cell = %+v, want a", cell)
	}
	if got := v.Line(0); got != "界a" {
		t.Fatalf("Line(0) = %q, want 界a", got)
	}
}

func TestNarrowRunesStayOneCell(t *testing.T) {
	v := New(6, 1)
	v.Write([]byte("Жa"))

	if cell := v.cell(0, 0); cell.Width != 1 || cell.Text != "Ж" {
		t.Fatalf("first cell = %+v, want a one cell Ж", cell)
	}
	if cell := v.cell(1, 0); cell.Text != "a" {
		t.Fatalf("second cell = %+v, want a", cell)
	}
}

func TestUTF8SplitAcrossWrites(t *testing.T) {
	v := New(6, 1)
	full := []byte("界")
	v.Write(full[:1])
	v.Write(full[1:])

	if got := v.Line(0); got != "界" {
		t.Fatalf("Line(0) = %q, want 界", got)
	}
}

func TestCSISplitAcrossWrites(t *testing.T) {
	v := New(10, 4)
	v.Write([]byte("\x1b["))
	v.Write([]byte("3"))
	v.Write([]byte(";4H"))

	if x, y, _, _ := v.CursorAt(); x != 3 || y != 2 {
		t.Fatalf("cursor at %d,%d, want 3,2", x, y)
	}
}

func TestInvalidUTF8BecomesAReplacement(t *testing.T) {
	v := New(6, 1)
	v.Write([]byte{0xff, 0xfe})

	if got := v.Line(0); got != "��" {
		t.Fatalf("Line(0) = %q, want two replacement runes", got)
	}
}

func TestReverseIndexScrollsTheRegionDown(t *testing.T) {
	v := New(6, 2)
	v.Write([]byte("one\r\ntwo\x1b[1;1H\x1bM"))

	if got := v.Line(0); got != "" {
		t.Fatalf("Line(0) = %q, want the blank line pushed to the top", got)
	}
	if got := v.Line(1); got != "one" {
		t.Fatalf("Line(1) = %q, want one", got)
	}
}

func TestTabStops(t *testing.T) {
	v := New(20, 1)
	v.Write([]byte("a\tb"))

	if got := v.Line(0); got != "a       b" {
		t.Fatalf("Line(0) = %q, want the second column at 8", got)
	}
}

func TestResizeKeepsContentAndPushesTheTopIntoHistory(t *testing.T) {
	v := New(20, 4)
	v.Write([]byte("l1\r\nl2\r\nl3\r\nl4"))
	v.Resize(20, 2)

	if got := v.Line(0); got != "l3" {
		t.Fatalf("Line(0) = %q, want l3", got)
	}
	if got := v.Line(1); got != "l4" {
		t.Fatalf("Line(1) = %q, want l4", got)
	}
	v.ScrollUp(2)
	if !strings.Contains(v.Line(0), "l1") || !strings.Contains(v.Line(1), "l2") {
		t.Fatalf("history rows = %q, %q; want l1 and l2", v.Line(0), v.Line(1))
	}
}

func TestResizeWiderKeepsRowsIntact(t *testing.T) {
	v := New(6, 2)
	v.Write([]byte("ab\r\ncd"))
	v.Resize(10, 2)

	if got := v.Line(1); got != "cd" {
		t.Fatalf("Line(1) = %q, want cd", got)
	}
}

func TestEraseDisplayThreeClearsTheScrollback(t *testing.T) {
	v := New(10, 2)
	v.Write([]byte("a\r\nb\r\nc\x1b[3J"))

	if v.ScrollbackLen() != 0 {
		t.Fatalf("scrollback = %d, want 0", v.ScrollbackLen())
	}
}

func TestResetClearsEverything(t *testing.T) {
	v := New(10, 3)
	v.Write([]byte("stuff\x1b]0;title\x07\x1b[?1049h"))
	v.Write([]byte("\x1bc"))

	if got := v.Line(0); got != "" {
		t.Errorf("Line(0) = %q, want an empty screen", got)
	}
	if v.Title() != "" {
		t.Errorf("title = %q, want empty", v.Title())
	}
	if v.Modes().AltScreen {
		t.Error("alt screen should be off after a reset")
	}
	if _, _, _, visible := v.CursorAt(); !visible {
		t.Error("cursor should be visible after a reset")
	}
}

func TestHistoryRingDropsTheOldestLine(t *testing.T) {
	h := newHistory(3)
	h.push([]Cell{{Text: "1", Width: 1}})
	h.push([]Cell{{Text: "2", Width: 1}})
	h.push([]Cell{{Text: "3", Width: 1}})
	h.push([]Cell{{Text: "4", Width: 1}})

	if h.len() != 3 {
		t.Fatalf("history length = %d, want 3", h.len())
	}
	if got := h.row(0)[0].Text; got != "2" {
		t.Fatalf("oldest row = %q, want 2", got)
	}
	if got := h.row(2)[0].Text; got != "4" {
		t.Fatalf("newest row = %q, want 4", got)
	}
}
