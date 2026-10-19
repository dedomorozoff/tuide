package session

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dedomorozoff/tuide/internal/geom"
)

func TestSaveAndLoadRoundTrip(t *testing.T) {
	t.Setenv("TUIDE_CONFIG_DIR", t.TempDir())

	if snap, err := Load(); err != nil || !snap.Empty() {
		t.Fatalf("Load on empty dir = %v, %v; want empty, nil", snap, err)
	}

	want := Snapshot{
		Layout:  1,
		Active:  2,
		Focused: "term-2",
		Windows: []Window{
			{ID: "notes", App: "notes", Rect: geom.Rect{X: 4, Y: 2, Width: 30, Height: 10}, Workspace: 2, State: 0},
			{ID: "term-2", App: "term", Rect: geom.Rect{Width: 80, Height: 24}, Workspace: 2, State: 1, Content: []byte{1, 2, 3}},
		},
	}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load = %+v, want %+v", got, want)
	}
}

func TestLoadReportsCorruptFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TUIDE_CONFIG_DIR", dir)

	if err := Save(Snapshot{Active: 1}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Error("Load accepted a corrupt session file")
	}
}
