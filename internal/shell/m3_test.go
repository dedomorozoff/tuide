package shell

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/compositor"
	"github.com/dedomorozoff/tuide/internal/geom"
	"github.com/dedomorozoff/tuide/internal/theme"
)

// frameApp is an app that only cares about the size of its window.
type frameApp struct {
	frames []appsdk.Frame
}

func (a *frameApp) Init() tea.Cmd { return nil }
func (a *frameApp) Close() tea.Cmd {
	return nil
}

func (a *frameApp) Update(msg tea.Msg) (appsdk.App, tea.Cmd) {
	if frame, ok := msg.(appsdk.Frame); ok {
		a.frames = append(a.frames, frame)
	}
	return a, nil
}

func (a *frameApp) View(appsdk.Context) tea.View { return tea.NewView("") }

// paintApp draws one cell per column so the compositor path is observable.
type paintApp struct {
	painted geom.Rect
}

func (a *paintApp) Init() tea.Cmd                        { return nil }
func (a *paintApp) Close() tea.Cmd                       { return nil }
func (a *paintApp) Update(tea.Msg) (appsdk.App, tea.Cmd) { return a, nil }
func (a *paintApp) View(appsdk.Context) tea.View         { return tea.NewView("") }

func (a *paintApp) Paint(screen *compositor.Screen, rect geom.Rect) {
	a.painted = rect
	for x := range rect.Width {
		screen.Set(rect.X+x, rect.Y, compositor.Cell{Text: "#", Width: 1})
	}
}

// mouseApp is an app that takes pointer input in its own window, the way a
// terminal does, so the shell stops making window gestures there.
type mouseApp struct {
	owns   bool
	clicks int
}

func (a *mouseApp) Init() tea.Cmd                { return nil }
func (a *mouseApp) Close() tea.Cmd               { return nil }
func (a *mouseApp) View(appsdk.Context) tea.View { return tea.NewView("") }
func (a *mouseApp) OwnsPointer() bool            { return a.owns }

func (a *mouseApp) Update(msg tea.Msg) (appsdk.App, tea.Cmd) {
	if _, ok := msg.(tea.MouseClickMsg); ok {
		a.clicks++
	}
	return a, nil
}

func specFor(id string, launch func() appsdk.App) appsdk.Spec {
	return appsdk.Spec{
		ID:            id,
		Name:          id,
		DefaultWidth:  20,
		DefaultHeight: 8,
		MinWidth:      10,
		MinHeight:     4,
		Launch:        launch,
	}
}

func desktopWith(t *testing.T, launch string, specs ...appsdk.Spec) *Model {
	t.Helper()
	palette := theme.Get(theme.DefaultName)
	return New(palette, specs, Options{
		Width:   goldenWidth,
		Height:  goldenHeight,
		Taskbar: true,
		Mouse:   true,
		Launch:  []string{launch},
	})
}

func TestFrameIsSentOncePerSize(t *testing.T) {
	app := &frameApp{}
	m := desktopWith(t, "probe", specFor("probe", func() appsdk.App { return app }))
	_ = m.Init()

	first := m.windows.Windows()[0].ContentRect()
	if len(app.frames) != 0 {
		t.Fatalf("opening a window sent %d frames before any update", len(app.frames))
	}

	_, _ = m.Update(tea.WindowSizeMsg{Width: goldenWidth, Height: goldenHeight})
	if len(app.frames) != 1 {
		t.Fatalf("frames = %d, want one on the first update", len(app.frames))
	}
	if app.frames[0].Width != first.Width || app.frames[0].Height != first.Height {
		t.Fatalf("frame = %dx%d, want %dx%d",
			app.frames[0].Width, app.frames[0].Height, first.Width, first.Height)
	}

	// The same geometry must not produce another message.
	_, _ = m.Update(tea.WindowSizeMsg{Width: goldenWidth, Height: goldenHeight})
	if len(app.frames) != 1 {
		t.Fatalf("frames = %d, want the unchanged geometry to stay quiet", len(app.frames))
	}

	// A window that grows has to tell its app again.
	m.toggleMaximized("probe")
	_, _ = m.Update(tea.WindowSizeMsg{Width: goldenWidth, Height: goldenHeight})
	if len(app.frames) != 2 {
		t.Fatalf("frames = %d, want a second one after the window grew", len(app.frames))
	}
	if app.frames[1].Width <= app.frames[0].Width {
		t.Fatalf("frame widths = %d then %d, want a wider one",
			app.frames[0].Width, app.frames[1].Width)
	}
}

func TestPaintedAppDrawsStraightIntoTheScreen(t *testing.T) {
	app := &paintApp{}
	m := desktopWith(t, "canvas", specFor("canvas", func() appsdk.App { return app }))
	_ = m.Init()

	w := m.windows.Windows()[0]
	_ = m.View()

	if app.painted != w.ContentRect() {
		t.Fatalf("painted %+v, want the content rectangle %+v", app.painted, w.ContentRect())
	}
	if got := m.screen.At(w.ContentRect().X, w.ContentRect().Y).Text; got != "#" {
		t.Fatalf("cell under the window = %q, want the painted marker", got)
	}
}

func TestMouseGoesToTheAppThatOwnsIt(t *testing.T) {
	app := &mouseApp{owns: true}
	m := desktopWith(t, "pointer", specFor("pointer", func() appsdk.App { return app }))
	_ = m.Init()

	w := m.windows.Windows()[0]
	body := w.ContentRect()
	_, _ = m.Update(tea.MouseClickMsg(tea.Mouse{
		X:      body.X + 1,
		Y:      body.Y,
		Button: tea.MouseLeft,
	}))

	if app.clicks != 1 {
		t.Fatalf("clicks = %d, want the app to have received one", app.clicks)
	}
	if m.drag.active() {
		t.Error("a click the app claimed must not start a window drag")
	}
}

func TestMouseStaysWithTheShellWhenTheAppLeavesIt(t *testing.T) {
	app := &mouseApp{owns: false}
	m := desktopWith(t, "pointer", specFor("pointer", func() appsdk.App { return app }))
	_ = m.Init()

	w := m.windows.Windows()[0]
	title := w.TitleRect()
	_, _ = m.Update(tea.MouseClickMsg(tea.Mouse{
		X:      title.X + 1,
		Y:      title.Y,
		Button: tea.MouseLeft,
	}))

	if app.clicks != 0 {
		t.Errorf("clicks = %d, want the app to be left alone", app.clicks)
	}
	if !m.drag.active() {
		t.Error("a click on the title bar should still start a drag")
	}
}

func TestADragInsideAnOwnedBodyMovesNothing(t *testing.T) {
	app := &mouseApp{owns: true}
	m := desktopWith(t, "pointer", specFor("pointer", func() appsdk.App { return app }))
	_ = m.Init()

	w := m.windows.Windows()[0]
	body := w.ContentRect()
	start := geom.Rect{X: body.X + 2, Y: body.Y + 1}

	_, _ = m.Update(tea.MouseClickMsg(tea.Mouse{X: start.X, Y: start.Y, Button: tea.MouseLeft}))
	_, _ = m.Update(tea.MouseMotionMsg(tea.Mouse{X: start.X + 6, Y: start.Y + 2}))
	_, _ = m.Update(tea.MouseReleaseMsg(tea.Mouse{X: start.X + 6, Y: start.Y + 2}))

	if m.drag.active() {
		t.Error("a drag inside a window the app owns must not move the window")
	}
	if m.windows.Find("pointer").Rect != w.Rect {
		t.Error("the window should not have moved")
	}
	if app.clicks != 1 {
		t.Errorf("clicks = %d, want the press to have reached the app", app.clicks)
	}
}

func TestControlChordsReachTheFocusedApp(t *testing.T) {
	m := desktopWith(t, "probe", specFor("probe", func() appsdk.App { return &frameApp{} }))
	_ = m.Init()

	// Ctrl alone is never a desktop shortcut any more: the shell has to let
	// these through so a terminal inside can receive ctrl+c and ctrl+w.
	for _, code := range []rune{'c', 'w', 'q'} {
		if _, ok := m.keymap.Lookup(tea.KeyPressMsg(tea.Key{Code: code, Mod: tea.ModCtrl})); ok {
			t.Errorf("ctrl+%c should not be a desktop shortcut", code)
		}
	}
}

// namedApp is an app that renames its own window.
type namedApp struct {
	title string
}

func (a *namedApp) Init() tea.Cmd                        { return nil }
func (a *namedApp) Close() tea.Cmd                       { return nil }
func (a *namedApp) View(appsdk.Context) tea.View         { return tea.NewView("") }
func (a *namedApp) Update(tea.Msg) (appsdk.App, tea.Cmd) { return a, nil }
func (a *namedApp) Title() string                        { return a.title }

func TestRuntimeTitleReplacesTheSpecName(t *testing.T) {
	app := &namedApp{title: "vim README.md"}
	m := desktopWith(t, "probe", specFor("probe", func() appsdk.App { return app }))
	_ = m.Init()

	w := m.windows.Windows()[0]
	w.Rect.Width = 40

	got := ansi.Strip(m.windowLabel(w, true))
	if !strings.Contains(got, "vim README.md") {
		t.Fatalf("label = %q, want the runtime name", got)
	}
	if strings.Contains(got, w.Spec.Name) {
		t.Fatalf("label = %q, want the spec name gone", got)
	}
	if !strings.HasSuffix(got, "[_][■][x]") {
		t.Fatalf("label = %q, want the controls kept", got)
	}
}

func TestRuntimeTitleIsCutToTheSpaceLeft(t *testing.T) {
	app := &namedApp{title: strings.Repeat("x", 200)}
	m := desktopWith(t, "probe", specFor("probe", func() appsdk.App { return app }))
	_ = m.Init()

	w := m.windows.Windows()[0]
	got := ansi.Strip(m.windowLabel(w, true))

	if lipgloss.Width(got) > w.Rect.Width-2 {
		t.Fatalf("label is %d wide, want no more than the %d it had",
			lipgloss.Width(got), w.Rect.Width-2)
	}
	if !strings.HasSuffix(got, "[_][■][x]") {
		t.Fatalf("label = %q, want the window controls to survive", got)
	}
	if !strings.Contains(got, "...") {
		t.Fatalf("label = %q, want an ellipsis where the name was cut", got)
	}
}

func TestANarrowWindowStillShowsItsName(t *testing.T) {
	app := &namedApp{}
	m := desktopWith(t, "probe", specFor("probe", func() appsdk.App { return app }))
	_ = m.Init()

	w := m.windows.Windows()[0]
	w.Rect.Width = 12

	got := ansi.Strip(m.windowLabel(w, true))
	if lipgloss.Width(got) > 10 {
		t.Fatalf("label = %q (%d wide), want it inside the 10 cell title bar", got, lipgloss.Width(got))
	}
	if !strings.Contains(got, w.Spec.Name) {
		t.Fatalf("label = %q, want the name even without room for the controls", got)
	}
}
