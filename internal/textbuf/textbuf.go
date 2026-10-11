// Package textbuf is a small line-based editable text buffer used by the
// desktop's editor-style apps. Columns count runes, not bytes.
package textbuf

import "strings"

// Buffer is a document plus a cursor position.
type Buffer struct {
	lines []string
	row   int
	col   int
}

// New returns a buffer with a single empty line.
func New() *Buffer {
	return &Buffer{lines: []string{""}}
}

// Rows is the number of lines.
func (b *Buffer) Rows() int { return len(b.lines) }

// Row is the cursor's line index.
func (b *Buffer) Row() int { return b.row }

// Col is the cursor's rune offset within its line.
func (b *Buffer) Col() int { return b.col }

// Line returns one line without its newline.
func (b *Buffer) Line(index int) string {
	if index < 0 || index >= len(b.lines) {
		return ""
	}
	return b.lines[index]
}

// Lines returns the document as a slice of lines.
func (b *Buffer) Lines() []string { return b.lines }

// String renders the document with newlines.
func (b *Buffer) String() string { return strings.Join(b.lines, "\n") }

// Insert writes text at the cursor.
func (b *Buffer) Insert(text string) {
	if text == "" {
		return
	}

	line := []rune(b.lines[b.row])
	col := min(b.col, len(line))
	b.lines[b.row] = string(line[:col]) + text + string(line[col:])
	b.col = col + len([]rune(text))
}

// Newline splits the current line at the cursor.
func (b *Buffer) Newline() {
	line := []rune(b.lines[b.row])
	col := min(b.col, len(line))

	head := string(line[:col])
	tail := string(line[col:])

	b.lines = append(b.lines, "")
	copy(b.lines[b.row+2:], b.lines[b.row+1:])
	b.lines[b.row] = head
	b.lines[b.row+1] = tail

	b.row++
	b.col = 0
}

// Backspace deletes the rune before the cursor, joining lines when needed.
func (b *Buffer) Backspace() {
	if b.col > 0 {
		line := []rune(b.lines[b.row])
		b.lines[b.row] = string(line[:b.col-1]) + string(line[b.col:])
		b.col--
		return
	}
	if b.row == 0 {
		return
	}

	previous := b.lines[b.row-1]
	b.col = len([]rune(previous))
	b.lines[b.row-1] = previous + b.lines[b.row]
	b.lines = append(b.lines[:b.row], b.lines[b.row+1:]...)
	b.row--
}

// MoveLeft walks the cursor back one cell, wrapping to the previous line.
func (b *Buffer) MoveLeft() {
	if b.col > 0 {
		b.col--
		return
	}
	if b.row > 0 {
		b.row--
		b.col = len([]rune(b.lines[b.row]))
	}
}

// MoveRight walks the cursor forward one cell, wrapping to the next line.
func (b *Buffer) MoveRight() {
	if b.col < len([]rune(b.lines[b.row])) {
		b.col++
		return
	}
	if b.row < len(b.lines)-1 {
		b.row++
		b.col = 0
	}
}

// MoveUp moves the cursor one line up, keeping the column where possible.
func (b *Buffer) MoveUp() {
	if b.row > 0 {
		b.row--
		b.col = min(b.col, len([]rune(b.lines[b.row])))
	}
}

// MoveDown moves the cursor one line down, keeping the column where possible.
func (b *Buffer) MoveDown() {
	if b.row < len(b.lines)-1 {
		b.row++
		b.col = min(b.col, len([]rune(b.lines[b.row])))
	}
}

// MoveHome puts the cursor at the start of the line.
func (b *Buffer) MoveHome() { b.col = 0 }

// MoveEnd puts the cursor at the end of the line.
func (b *Buffer) MoveEnd() { b.col = len([]rune(b.lines[b.row])) }
