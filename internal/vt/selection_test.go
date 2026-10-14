package vt

import (
	"strings"
	"testing"

	"github.com/dedomorozoff/tuide/internal/compositor"
	"github.com/dedomorozoff/tuide/internal/geom"
)

func TestAPlainClickSelectsNothing(t *testing.T) {
	v := New(10, 2)
	v.Write([]byte("hello"))

	v.BeginSelection(2, 0)
	v.ExtendSelection(2, 0)

	if v.HasSelection() {
		t.Error("a click without a drag should not select anything")
	}
	if !v.Selecting() {
		t.Error("the drag should still be marked as in flight")
	}
	if got := v.SelectedText(); got != "" {
		t.Errorf("SelectedText = %q, want nothing", got)
	}
}

func TestSelectingOneLine(t *testing.T) {
	v := New(20, 3)
	v.Write([]byte("first line\r\nsecond line"))

	v.BeginSelection(6, 0)
	v.ExtendSelection(11, 0)

	if !v.HasSelection() {
		t.Fatal("the drag should have selected something")
	}
	if got := v.SelectedText(); got != "line" {
		t.Fatalf("SelectedText = %q, want line", got)
	}
}

func TestSelectingBackwardsGivesTheSameText(t *testing.T) {
	v := New(20, 2)
	v.Write([]byte("alpha beta"))

	v.BeginSelection(11, 0)
	v.ExtendSelection(6, 0)

	if got := v.SelectedText(); got != "beta" {
		t.Fatalf("SelectedText = %q, want beta", got)
	}
}

func TestSelectingAcrossLines(t *testing.T) {
	v := New(20, 4)
	v.Write([]byte("one\r\ntwo\r\nthree\r\nfour"))

	v.BeginSelection(1, 0)
	v.ExtendSelection(3, 2)

	got := v.SelectedText()
	want := "ne\ntwo\nthre"
	if got != want {
		t.Fatalf("SelectedText = %q, want %q", got, want)
	}
}

func TestSelectionToTheEndOfALineDropsTheBlanks(t *testing.T) {
	v := New(20, 2)
	v.Write([]byte("git status\r\n"))

	v.BeginSelection(4, 0)
	v.ExtendSelection(10, 0)

	if got := v.SelectedText(); got != "status" {
		t.Fatalf("SelectedText = %q, want status", got)
	}
}

func TestSelectionAcrossTheWholeLineKeepsTheBreak(t *testing.T) {
	v := New(20, 3)
	v.Write([]byte("abc\r\ndef\r\nghi"))

	v.BeginSelection(0, 0)
	v.ExtendSelection(19, 1)

	got := v.SelectedText()
	want := "abc\ndef"
	if got != want {
		t.Fatalf("SelectedText = %q, want %q", got, want)
	}
}

func TestSelectionReachesIntoTheScrollback(t *testing.T) {
	v := New(10, 2)
	v.Write([]byte("one\r\ntwo\r\nthree\r\nfour"))

	// One notch back puts the newest history line on top of the live screen.
	v.ScrollUp(1)
	if got := v.Line(0); got != "two" {
		t.Fatalf("Line(0) = %q, want the history line", got)
	}

	v.BeginSelection(0, 0)
	v.ExtendSelection(2, 1)

	got := v.SelectedText()
	want := "two\nthr"
	if got != want {
		t.Fatalf("SelectedText = %q, want %q", got, want)
	}
}

func TestSelectionSkipsWideCharacterContinuations(t *testing.T) {
	v := New(10, 1)
	v.Write([]byte("a界b"))

	v.BeginSelection(0, 0)
	v.ExtendSelection(3, 0)

	if got := v.SelectedText(); got != "a界b" {
		t.Fatalf("SelectedText = %q, want a界b without a hole", got)
	}
}

func TestNewOutputDropsTheSelection(t *testing.T) {
	v := New(10, 2)
	v.Write([]byte("hello"))

	v.BeginSelection(0, 0)
	v.ExtendSelection(4, 0)
	if !v.HasSelection() {
		t.Fatal("the selection should be there before the output arrives")
	}

	v.Write([]byte("x"))
	if v.HasSelection() {
		t.Error("output from the program must invalidate the selection")
	}
}

func TestClearSelection(t *testing.T) {
	v := New(10, 2)
	v.Write([]byte("hello"))

	v.BeginSelection(0, 0)
	v.ExtendSelection(4, 0)
	v.ClearSelection()

	if v.HasSelection() || v.Selecting() {
		t.Error("ClearSelection should leave nothing behind")
	}
}

func TestExtendingWithoutStartingDoesNothing(t *testing.T) {
	v := New(10, 2)
	v.Write([]byte("hello"))

	v.ExtendSelection(4, 0)

	if v.HasSelection() || v.Selecting() {
		t.Error("a stray motion must not start a selection")
	}
}

func TestSelectedCellsAreDrawnInReverse(t *testing.T) {
	v := New(10, 2)
	v.Write([]byte("hello"))
	v.BeginSelection(1, 0)
	v.ExtendSelection(3, 0)

	screen := compositor.New(10, 2)
	v.Blit(screen, geom.Rect{Width: 10, Height: 2})

	if style := screen.At(0, 0).Style; style != "" {
		t.Errorf("cell before the selection = %q, want the default", style)
	}
	if style := screen.At(2, 0).Style; !strings.Contains(style, "7") {
		t.Errorf("selected cell = %q, want reverse video", style)
	}
	if style := screen.At(4, 0).Style; style != "" {
		t.Errorf("cell after the selection = %q, want the default", style)
	}
}

func TestSelectionKeepsTheExistingStyle(t *testing.T) {
	v := New(10, 1)
	v.Write([]byte("\x1b[31mabc\x1b[0m"))
	v.BeginSelection(0, 0)
	v.ExtendSelection(1, 0)

	screen := compositor.New(10, 1)
	v.Blit(screen, geom.Rect{Width: 10, Height: 1})

	style := screen.At(0, 0).Style
	if !strings.Contains(style, "38;5;1") || !strings.Contains(style, "7") {
		t.Fatalf("selected cell style = %q, want red plus reverse", style)
	}
}

func TestSelectionOnTheHistoryRowIsPainted(t *testing.T) {
	v := New(10, 2)
	v.Write([]byte("one\r\ntwo\r\nthree\r\nfour"))

	v.ScrollUp(1)
	v.BeginSelection(0, 0)
	v.ExtendSelection(2, 0)

	screen := compositor.New(10, 2)
	v.Blit(screen, geom.Rect{Width: 10, Height: 2})

	if style := screen.At(1, 0).Style; !strings.Contains(style, "7") {
		t.Fatalf("history row style = %q, want the selection marked there too", style)
	}
}
