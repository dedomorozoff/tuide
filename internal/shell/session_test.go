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

// statefulApp keeps a blob across a session save, the way the terminal keeps
// its screen, so the shell's save and restore paths can be observed.
type statefulApp struct {
	content string
}

func (a *statefulApp) Init() tea.Cmd                        { return nil }
func (a *statefulApp) Close() tea.Cmd                       { return nil }
func (a *statefulApp) Update(tea.Msg) (appsdk.App, tea.Cmd) { return a, nil }
func (a *statefulApp) View(appsdk.Context) tea.View         { return tea.NewView("") }

func (a *statefulApp) SnapshotState() []byte { return []byte(a.content) }
func (a *statefulApp) RestoreState(data []byte) error {
	a.content = string(data)
	return nil
}

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

func TestSnapshotCarriesAppStateAndRestoreReplaysIt(t *testing.T) {
	palette := theme.Get(theme.DefaultName)

	desktop := New(palette, []appsdk.Spec{
		specFor("alpha", func() appsdk.App { return &statefulApp{content: "live screen"} }),
	}, Options{Width: 120, Height: 40, Taskbar: true})
	_, _ = desktop.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = desktop.launch("alpha")

	snap := desktop.Snapshot()
	if len(snap.Windows) != 1 {
		t.Fatalf("snapshot holds %d windows, want 1", len(snap.Windows))
	}
	if got := string(snap.Windows[0].Content); got != "live screen" {
		t.Fatalf("saved content = %q, want the app state", got)
	}

	restored := New(palette, []appsdk.Spec{
		specFor("alpha", func() appsdk.App { return &statefulApp{} }),
	}, Options{Width: 120, Height: 40, Taskbar: true, Restore: &snap})
	_ = restored.Init()
	_, _ = restored.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	w := restored.windows.Find("alpha")
	if w == nil {
		t.Fatal("alpha was not restored")
	}
	app, ok := w.App.(*statefulApp)
	if !ok {
		t.Fatalf("restored app = %T, want *statefulApp", w.App)
	}
	if app.content != "live screen" {
		t.Errorf("restored content = %q, want the saved blob", app.content)
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

func TestRestoreWaitsForRealTerminalSize(t *testing.T) {
	rect := geom.Rect{X: 5, Y: 3, Width: 40, Height: 12}
	snap := session.Snapshot{
		Windows: []session.Window{
			{ID: "alpha", App: "alpha", Rect: rect},
		},
	}
	desktop := sessionDesktop(&snap)
	_ = desktop.Init()

	_, _ = desktop.Update(tea.WindowSizeMsg{Width: 0, Height: 0})
	if desktop.windows.Find("alpha") != nil {
		t.Fatal("the session was restored on an empty terminal size")
	}
	if desktop.restore == nil {
		t.Fatal("the session was dropped instead of waiting for a real size")
	}

	_, _ = desktop.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	alpha := desktop.windows.Find("alpha")
	if alpha == nil {
		t.Fatal("alpha was not restored after a real resize")
	}
	if alpha.Rect != rect {
		t.Errorf("alpha rect = %v, want %v", alpha.Rect, rect)
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
