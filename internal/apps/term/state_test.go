package term

import (
	"testing"

	"github.com/dedomorozoff/tuide/internal/theme"
)

func TestStateRoundTripKeepsTheScreen(t *testing.T) {
	a := New(theme.Get(theme.DefaultName))
	a.term.Write([]byte("hello\r\nworld"))

	data := a.SnapshotState()
	if len(data) == 0 {
		t.Fatal("snapshot is empty")
	}

	b := New(theme.Get(theme.DefaultName))
	if err := b.RestoreState(data); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if got := b.term.Line(0); got != "hello" {
		t.Errorf("Line(0) = %q, want hello", got)
	}
	if got := b.term.Line(1); got != "world" {
		t.Errorf("Line(1) = %q, want world", got)
	}
}

func TestStateRoundTripKeepsTheGridSize(t *testing.T) {
	a := New(theme.Get(theme.DefaultName))
	a.term.Resize(30, 8)
	a.term.Write([]byte("sized"))

	b := New(theme.Get(theme.DefaultName))
	if err := b.RestoreState(a.SnapshotState()); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if width, height := b.term.Size(); width != 30 || height != 8 {
		t.Errorf("restored size = %dx%d, want 30x8", width, height)
	}
}

func TestRestoreEmptyStateIsANoOp(t *testing.T) {
	b := New(theme.Get(theme.DefaultName))
	if err := b.RestoreState(nil); err != nil {
		t.Fatalf("restore of an empty blob: %v", err)
	}
}

func TestRestoreRejectsGarbage(t *testing.T) {
	b := New(theme.Get(theme.DefaultName))
	if err := b.RestoreState([]byte("not gzip")); err == nil {
		t.Error("restore accepted a blob that is not a snapshot")
	}
}
