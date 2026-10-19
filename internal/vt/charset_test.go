package vt

import (
	"strings"
	"testing"
)

func TestOSCTerminatorEndsTheString(t *testing.T) {
	v := New(20, 3)
	v.Write([]byte("\x1b]0;a title\x1b\\"))
	v.Write([]byte("after"))

	if got := v.Line(0); got != "after" {
		t.Fatalf("Line(0) = %q, want %q", got, "after")
	}
	if v.Title() != "a title" {
		t.Fatalf("title = %q, want %q", v.Title(), "a title")
	}
}

func TestOSCWithBelTerminatorEndsTheString(t *testing.T) {
	v := New(20, 3)
	v.Write([]byte("\x1b]0;a title\x07"))
	v.Write([]byte("after"))

	if got := v.Line(0); got != "after" {
		t.Fatalf("Line(0) = %q, want %q", got, "after")
	}
}

func TestUnsupportedStringDoesNotSwallowTheStream(t *testing.T) {
	v := New(20, 3)
	v.Write([]byte("\x1b]7;file://host/path\x1b\\"))
	v.Write([]byte("after"))

	if got := v.Line(0); got != "after" {
		t.Fatalf("Line(0) = %q, want %q", got, "after")
	}
}

func TestDeviceControlStringDoesNotSwallowTheStream(t *testing.T) {
	v := New(20, 3)
	v.Write([]byte("\x1bP+q544e\x1b\\"))
	v.Write([]byte("after"))

	if got := v.Line(0); got != "after" {
		t.Fatalf("Line(0) = %q, want %q", got, "after")
	}
}

func TestCharacterSetDesignationDoesNotEatTheNextSequence(t *testing.T) {
	v := New(20, 3)
	v.Write([]byte("\x1b(0\x1b[37m\x1b[44mA"))

	if got := v.Line(0); got != "A" {
		t.Fatalf("Line(0) = %q, want %q", got, "A")
	}
	if style := v.Row(0)[0].Style; !strings.Contains(style, "38;5;7") {
		t.Fatalf("style = %q, want the foreground colour", style)
	}
}

func TestDecSpecialGraphicsDrawsFrames(t *testing.T) {
	v := New(20, 3)
	v.Write([]byte("\x1b(0lqk\x1b(Bx"))

	if got := v.Line(0); got != "┌─┐x" {
		t.Fatalf("Line(0) = %q, want %q", got, "┌─┐x")
	}
}

func TestShiftOutSelectsTheSecondCharacterSet(t *testing.T) {
	v := New(20, 3)
	v.Write([]byte("\x1b)0\x0ek\x0fk"))

	if got := v.Line(0); got != "┐k" {
		t.Fatalf("Line(0) = %q, want %q", got, "┐k")
	}
}
