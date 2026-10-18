package shell

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/geom"
	"github.com/dedomorozoff/tuide/internal/session"
	"github.com/dedomorozoff/tuide/internal/theme"
	"github.com/dedomorozoff/tuide/internal/wm"
)

func sessionDesktop(restore *session.Snapshot) *Model {
	palette := theme.Get(theme.DefaultName)
	return New(palette, []appsdk.Spec{
		specFor("alpha", func() appsdk.App { return &frameApp{} }),
		specFor("beta", func() appsdk.App { return &frameApp{} }),
	}, Options{
		Width:   120,
		Height:  40,
		Taskbar: true,
		Mouse:   true,
		Restore: restore,
	})
}

func TestSnapshotRestoreRoundTrip(t *testing.T) {
	desktop := sessionDesktop(nil)
	_, _ = desktop.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	_ = desktop.launch("alpha")
	_ = desktop.launch("beta")
	desktop.moveToWorkspace(3)
	desktop.snap("alpha", wm.ZoneLeft)

	snap := desktop.Snapshot()
	if len(snap.Windows) != 2 {
		t.Fatalf("snapshot holds %d windows, want 2", len(snap.Windows))
	}

	restored := sessionDesktop(&snap)
	_ = restored.Init()
	_, _ = restored.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	alpha := restored.windows.Find("alpha")
	if alpha == nil {
		t.Fatal("alpha was not restored")
	}
	if alpha.Rect != desktop.windows.Find("alpha").Rect {
		t.Errorf("alpha rect = %v, want %v", alpha.Rect, desktop.windows.Find("alpha").Rect)
	}
	if alpha.Workspace != 0 {
		t.Errorf("alpha workspace = %d, want 0", alpha.Workspace)
	}

	beta := restored.windows.Find("beta")
	if beta == nil {
		t.Fatal("beta was not restored")
	}
	if beta.Workspace != 3 {
		t.Errorf("beta workspace = %d, want 3", beta.Workspace)
	}

	if restored.windows.Active() != 3 {
		t.Errorf("active workspace = %d, want 3", restored.windows.Active())
	}
	if focused := restored.windows.Focused(); focused == nil || focused.ID != "beta" {
		t.Errorf("focused = %v, want beta", focused)
	}
}

func TestRestoreKeepsWindowState(t *testing.T) {
	desktop := sessionDesktop(nil)
	_, _ = desktop.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = desktop.launch("alpha")
	if err := desktop.windows.ToggleMinimized("alpha"); err != nil {
		t.Fatalf("minimize: %v", err)
	}

	snap := desktop.Snapshot()
	if len(snap.Windows) != 1 || snap.Windows[0].State != int(wm.Minimized) {
		t.Fatalf("snapshot = %+v, want one minimized window", snap.Windows)
	}

	restored := sessionDesktop(&snap)
	_ = restored.Init()
	_, _ = restored.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	alpha := restored.windows.Find("alpha")
	if alpha == nil || alpha.State != wm.Minimized {
		t.Errorf("alpha = %v, want minimized", alpha)
	}
}

func TestRestoreSkipsUnknownApps(t *testing.T) {
	snap := session.Snapshot{
		Windows: []session.Window{
			{ID: "ghost", App: "ghost", Rect: desktopRect()},
			{ID: "alpha", App: "alpha"},
		},
	}
	desktop := sessionDesktop(&snap)
	_ = desktop.Init()
	_, _ = desktop.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	if desktop.windows.Find("ghost") != nil {
		t.Error("an unknown app was restored")
	}
	if desktop.windows.Find("alpha") == nil {
		t.Error("a known app was skipped")
	}
}

func TestPersistSavesSession(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TUIDE_CONFIG_DIR", dir)

	palette := theme.Get(theme.DefaultName)
	desktop := New(palette, []appsdk.Spec{
		specFor("alpha", func() appsdk.App { return &frameApp{} }),
	}, Options{Width: 120, Height: 40, Taskbar: true, Persist: true})
	_, _ = desktop.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = desktop.launch("alpha")

	var save tea.Cmd
	for range saveEveryTicks {
		_, save = desktop.Update(tickMsg{})
	}
	if save == nil {
		t.Fatal("the tick throttle scheduled no save")
	}

	deadline := time.Now().Add(5 * time.Second)
	saved := false
	for _, cmd := range flatten(save) {
		msg := cmd()
		if result, ok := msg.(sessionSavedMsg); ok {
			if result.err != nil {
				t.Fatalf("save: %v", result.err)
			}
			saved = true
		}
		if saved || time.Now().After(deadline) {
			break
		}
	}
	if !saved {
		t.Fatal("30 ticks scheduled no session save")
	}

	snap, err := session.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(snap.Windows) != 1 || snap.Windows[0].App != "alpha" {
		t.Errorf("saved session = %+v, want the alpha window", snap.Windows)
	}
}

// flatten pulls the commands out of a batch; the clock tick inside one blocks
// for a second, so the search stops as soon as the save has been seen.
func flatten(cmd tea.Cmd) []tea.Cmd {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Cmd{func() tea.Msg { return msg }}
	}
	var out []tea.Cmd
	for _, inner := range batch {
		out = append(out, flatten(inner)...)
	}
	return out
}

func desktopRect() geom.Rect {
	return geom.Rect{Width: 40, Height: 12}
}
