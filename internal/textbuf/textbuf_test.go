package textbuf

import "testing"

func TestInsertAndNewline(t *testing.T) {
	b := New()
	b.Insert("hello")
	b.Newline()
	b.Insert("world")

	if got := b.String(); got != "hello\nworld" {
		t.Errorf("document = %q, want %q", got, "hello\nworld")
	}
	if b.Row() != 1 || b.Col() != 5 {
		t.Errorf("cursor = %d,%d, want 1,5", b.Row(), b.Col())
	}
}

func TestNewlineSplitsInTheMiddle(t *testing.T) {
	b := New()
	b.Insert("helloworld")
	b.MoveHome()
	for i := 0; i < 5; i++ {
		b.MoveRight()
	}
	b.Newline()

	if got := b.String(); got != "hello\nworld" {
		t.Errorf("document = %q, want %q", got, "hello\nworld")
	}
	if b.Row() != 1 || b.Col() != 0 {
		t.Errorf("cursor = %d,%d, want 1,0", b.Row(), b.Col())
	}
}

func TestBackspaceJoinsLines(t *testing.T) {
	b := New()
	b.Insert("one")
	b.Newline()
	b.Insert("two")

	b.Backspace()
	b.Backspace()
	b.Backspace()
	if got := b.String(); got != "one\n" {
		t.Fatalf("after clearing the second line document = %q, want %q", got, "one\n")
	}

	b.Backspace()
	if got := b.String(); got != "one" {
		t.Errorf("document = %q, want %q", got, "one")
	}
	if b.Row() != 0 || b.Col() != 3 {
		t.Errorf("cursor = %d,%d, want 0,3", b.Row(), b.Col())
	}
}

func TestBackspaceAtStartOfDocumentDoesNothing(t *testing.T) {
	b := New()
	b.Insert("x")
	b.MoveHome()
	b.Backspace()

	if got := b.String(); got != "x" {
		t.Errorf("document = %q, want %q", got, "x")
	}
}

func TestVerticalMovementClampsColumn(t *testing.T) {
	b := New()
	b.Insert("long line")
	b.Newline()
	b.Insert("hi")

	b.MoveUp()
	if b.Row() != 0 || b.Col() != 2 {
		t.Errorf("cursor = %d,%d, want 0,2", b.Row(), b.Col())
	}

	b.MoveDown()
	if b.Row() != 1 || b.Col() != 2 {
		t.Errorf("cursor = %d,%d, want 1,2", b.Row(), b.Col())
	}
}

func TestHorizontalMovementWrapsLines(t *testing.T) {
	b := New()
	b.Insert("ab")
	b.Newline()
	b.Insert("cd")

	b.MoveUp()
	b.MoveEnd()
	b.MoveRight()
	if b.Row() != 1 || b.Col() != 0 {
		t.Errorf("moving right at end of line = %d,%d, want 1,0", b.Row(), b.Col())
	}

	b.MoveLeft()
	if b.Row() != 0 || b.Col() != 2 {
		t.Errorf("moving left at start of line = %d,%d, want 0,2", b.Row(), b.Col())
	}
}

func TestInsertIsRuneAware(t *testing.T) {
	b := New()
	b.Insert("да")
	b.MoveHome()
	b.Insert("!")

	if got := b.String(); got != "!да" {
		t.Errorf("document = %q, want %q", got, "!да")
	}
	if b.Col() != 1 {
		t.Errorf("column = %d, want 1", b.Col())
	}
}

func TestHomeAndEnd(t *testing.T) {
	b := New()
	b.Insert("abc")
	b.MoveHome()
	if b.Col() != 0 {
		t.Errorf("home column = %d, want 0", b.Col())
	}
	b.MoveEnd()
	if b.Col() != 3 {
		t.Errorf("end column = %d, want 3", b.Col())
	}
}
