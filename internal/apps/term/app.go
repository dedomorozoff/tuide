package term

import (
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/compositor"
	"github.com/dedomorozoff/tuide/internal/geom"
	"github.com/dedomorozoff/tuide/internal/sys"
	"github.com/dedomorozoff/tuide/internal/theme"
	"github.com/dedomorozoff/tuide/internal/vt"
)

// App is one terminal window: a pseudo terminal running a child process, a
// virtual terminal decoding what it writes, and the commands that move bytes in
// both directions.
type App struct {
	theme theme.Theme

	term vt.VT

	pty  sys.PTY
	out  chan []byte
	done chan struct{}

	// mu guards the write queue below. Keystrokes arrive one Update at a time,
	// but the writer that drains them is a command, so it runs beside them.
	mu      sync.Mutex
	pending [][]byte
	writing bool

	width  int
	height int
	rect   geom.Rect

	// exited and failure describe why the screen has stopped moving: either
	// the child is gone or it never started.
	exited  bool
	failure error

	title string
	// lastClipboard is the previous OSC 52 payload, so a new one is only
	// pushed onto the host clipboard once.
	lastClipboard string
}

// how the two pumps are wired to the child process.
const (
	defaultWidth  = 80
	defaultHeight = 24

	// readChunk is how much output is pulled from the child at a time.
	readChunk = 8192
	// readQueue absorbs bursts of child output without stalling the reader.
	readQueue = 64

	// wheelStep is how many history lines one wheel click moves.
	wheelStep = 3
)

// Spec is the launcher entry for this app.
func Spec(t theme.Theme) appsdk.Spec {
	return appsdk.Spec{
		ID:            "term",
		Name:          "Terminal",
		DefaultWidth:  80,
		DefaultHeight: 24,
		MinWidth:      20,
		MinHeight:     6,
		Launch:        func() appsdk.App { return New(t) },
	}
}

// New returns a terminal window with no child process yet.
func New(t theme.Theme) *App {
	a := &App{theme: t, term: *vt.New(defaultWidth, defaultHeight), width: defaultWidth, height: defaultHeight}
	a.term.OnTitle = func(title string) { a.title = title }
	return a
}

// Init implements appsdk.App. Starting the child process is I/O, so it happens
// in a command and reports back.
func (a *App) Init() tea.Cmd {
	return func() tea.Msg {
		terminal, err := sys.StartPTY(sys.PTYOptions{
			Width:  a.width,
			Height: a.height,
		})
		return startedMsg{terminal: terminal, err: err}
	}
}

// Close implements appsdk.App. The read and write pumps stop and the child
// process is torn down.
func (a *App) Close() tea.Cmd {
	if a.pty == nil {
		return nil
	}
	a.stop()
	terminal := a.pty
	a.pty = nil
	return func() tea.Msg {
		_ = terminal.Close()
		return closedMsg{}
	}
}

func (a *App) stop() {
	if a.done != nil {
		close(a.done)
		a.done = nil
	}
}

// Update implements appsdk.App.
func (a *App) Update(msg tea.Msg) (appsdk.App, tea.Cmd) {
	switch msg := msg.(type) {
	case startedMsg:
		return a, a.onStarted(msg)

	case outputMsg:
		return a, a.onOutput(msg)

	case writtenMsg:
		return a, nil

	case exitedMsg:
		a.exited = true
		return a, nil

	case failureMsg:
		// A failed resize or a failed reply still leaves the screen alive, so
		// this only records the reason; the banner says what went wrong
		// without pretending the terminal is dead.
		if msg.err != nil {
			a.failure = msg.err
		}
		return a, nil

	case appsdk.Frame:
		return a, a.onFrame(msg)

	case tea.PasteMsg:
		return a, a.send(paste(msg.Content, a.term.Modes()))

	case tea.KeyPressMsg:
		return a, a.onKey(msg)

	case tea.MouseClickMsg:
		return a, a.onMouse(msg.Mouse(), true)

	case tea.MouseReleaseMsg:
		return a, a.onMouse(msg.Mouse(), false)

	case tea.MouseMotionMsg:
		return a, a.onMotion(msg.Mouse())

	case tea.MouseWheelMsg:
		return a, a.onWheel(msg.Mouse())

	case blinkMsg:
		a.term.AdvanceBlink(bool(msg))
		return a, a.scheduleBlink()
	}
	return a, nil
}

// View implements appsdk.App. The terminal paints itself through
// appsdk.Painter, so the string view is only a placeholder for the moments
// before the window has a size.
func (a *App) View(ctx appsdk.Context) tea.View {
	a.rect = ctx.Rect
	view := tea.NewView("")
	view.WindowTitle = a.title
	return view
}

// Paint implements appsdk.Painter: the terminal grid is copied straight into
// the window rectangle. A dead or failed child gets a status line, because a
// frozen screen with no explanation is the worst thing a terminal can show.
func (a *App) Paint(screen *compositor.Screen, rect geom.Rect) {
	a.rect = rect
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	a.term.Blit(screen, rect)
	if status := a.status(); status != "" && rect.Height >= 1 {
		screen.Blit(geom.Rect{
			X:      rect.X,
			Y:      rect.Y + rect.Height - 1,
			Width:  rect.Width,
			Height: 1,
		}, status)
	}
}

// status is the one line shown when there is nothing left to type into.
func (a *App) status() string {
	var text string
	switch {
	case a.failure != nil:
		text = " " + a.failure.Error() + " "
	default:
		if !a.exited {
			return ""
		}
		text = " [process exited] "
	}
	return lipgloss.NewStyle().
		Foreground(a.theme.Background).
		Background(a.theme.Danger).
		Render(text)
}

// OwnsPointer implements appsdk.MouseConsumer. A terminal always takes the
// pointer inside its own body: a drag across the text is a selection, and when
// the program inside asked for mouse reporting the reports go to it too. The
// window is moved from its title bar instead.
func (a *App) OwnsPointer() bool { return true }

// programWantsMouse reports whether the running program asked for mouse
// reports, in which case the pointer belongs to it rather than to a selection.
func (a *App) programWantsMouse() bool { return a.term.Modes().Mouse != vt.MouseOff }

// Title is the name the window should carry once the program set one.
func (a *App) Title() string {
	if a.title != "" {
		return a.title
	}
	return "Terminal"
}

// onStarted wires the child process to the virtual terminal and starts both
// pumps.
func (a *App) onStarted(msg startedMsg) tea.Cmd {
	if msg.err != nil {
		a.failure = msg.err
		a.exited = true
		return nil
	}

	a.pty = msg.terminal
	a.out = make(chan []byte, readQueue)
	a.done = make(chan struct{})

	go a.readPump()

	if err := msg.terminal.Resize(a.width, a.height); err != nil {
		a.failure = err
	}
	return tea.Batch(a.readCmd(), a.scheduleBlink())
}

// onOutput decodes what the child wrote and answers whatever it asked for.
func (a *App) onOutput(msg outputMsg) tea.Cmd {
	a.term.Write(msg)

	cmds := []tea.Cmd{a.readCmd()}
	if replies := a.term.TakeReplies(); len(replies) > 0 {
		cmds = append(cmds, a.send(replies))
	}
	if cmd := a.takeClipboard(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

// takeClipboard turns an OSC 52 payload into a clipboard write. Programs use it
// to copy across a session boundary, so the text belongs on the host.
func (a *App) takeClipboard() tea.Cmd {
	payload := a.term.Clipboard()
	if payload == a.lastClipboard {
		return nil
	}
	a.lastClipboard = payload

	text, err := decodeClipboard(payload)
	if err != nil || text == "" {
		return nil
	}
	return tea.SetClipboard(text)
}

// onFrame resizes the emulator and tells the child about it. The window manager
// is the only thing that can move a window, so this is the only place a resize
// can come from.
func (a *App) onFrame(msg appsdk.Frame) tea.Cmd {
	width, height := max(msg.Width, 1), max(msg.Height, 1)
	if width == a.width && height == a.height {
		return nil
	}
	a.width, a.height = width, height
	a.term.Resize(width, height)
	if a.pty == nil {
		return nil
	}
	terminal := a.pty
	return func() tea.Msg {
		return failureMsg{err: terminal.Resize(width, height)}
	}
}

// onKey forwards a key press to the child process. Anything the window manager
// claimed never reaches here, so whatever arrives is meant for the shell, with
// one exception: ctrl+shift+c copies the selection. Pasting needs no shortcut
// of our own — the host terminal turns ctrl+shift+v into a bracketed paste,
// which arrives as tea.PasteMsg.
func (a *App) onKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.Key()
	if key.Mod&(tea.ModCtrl|tea.ModShift) == (tea.ModCtrl|tea.ModShift) &&
		(key.Code == 'c' || key.Code == 'C') {
		return a.copySelection()
	}
	return a.send(encodeKey(key, a.term.Modes()))
}

// onMouse turns a press or a release into either a report for the running
// program or the start of a text selection.
func (a *App) onMouse(mouse tea.Mouse, press bool) tea.Cmd {
	x, y := a.cellOf(mouse)
	if a.programWantsMouse() {
		if press && mouse.Button != tea.MouseLeft {
			return nil
		}
		button := mouseButtonCode(mouse.Button)
		if button < 0 {
			return nil
		}
		if press && mouse.Mod&tea.ModShift != 0 {
			// Shift is the usual way to ask for the host to do the copy.
			return nil
		}
		return a.send(encodeMouse(mouseEvent{button: button, x: x, y: y, press: press}, a.term.Modes().Mouse))
	}

	if mouse.Button != tea.MouseLeft {
		return nil
	}
	if press {
		a.term.BeginSelection(x, y)
		return nil
	}

	a.term.ClearSelection()
	return a.copySelection()
}

// onMotion extends a selection while a drag is in flight, and reports the
// pointer to the program when it asked for motion events.
func (a *App) onMotion(mouse tea.Mouse) tea.Cmd {
	mode := a.term.Modes().Mouse
	if !a.term.Selecting() && mode != vt.MouseAny && mode != vt.MouseSGR {
		return nil
	}
	if a.term.Selecting() {
		x, y := a.cellOf(mouse)
		a.term.ExtendSelection(x, y)
		return nil
	}
	if mode == vt.MouseOff {
		return nil
	}

	x, y := a.cellOf(mouse)
	return a.send(encodeMouse(mouseEvent{button: 32, x: x, y: y, motion: true}, mode))
}

// cellOf converts a screen position into a cell of the terminal grid.
func (a *App) cellOf(mouse tea.Mouse) (x, y int) {
	return mouse.X - a.rect.X, mouse.Y - a.rect.Y
}

// copySelection puts whatever the selection covers on the host clipboard.
func (a *App) copySelection() tea.Cmd {
	text := a.term.SelectedText()
	if text == "" {
		return nil
	}
	return tea.SetClipboard(text)
}

// onWheel scrolls the viewport. Programs that track the wheel get a report
// instead, everything else gets history, which is what a user expects.
func (a *App) onWheel(mouse tea.Mouse) tea.Cmd {
	x, y := a.cellOf(mouse)
	mode := a.term.Modes().Mouse

	switch mouse.Button {
	case tea.MouseWheelUp:
		if mode != vt.MouseOff {
			return a.send(encodeMouse(mouseEvent{button: 64, x: x, y: y, press: true}, mode))
		}
		a.term.ScrollUp(wheelStep)
	case tea.MouseWheelDown:
		if mode != vt.MouseOff {
			return a.send(encodeMouse(mouseEvent{button: 65, x: x, y: y, press: true}, mode))
		}
		a.term.ScrollDown(wheelStep)
	}
	return nil
}

// send queues bytes for the child process.
//
// The order the user typed in is the order the shell has to see, and Bubble
// Tea runs every command in its own goroutine, so the bytes cannot be pushed
// onto a channel from the command: two keystrokes would race and arrive
// swapped. They are appended here instead, where Update is single threaded, and
// one writer at a time drains them in that order.
func (a *App) send(data []byte) tea.Cmd {
	if len(data) == 0 || a.pty == nil {
		return nil
	}

	a.mu.Lock()
	a.pending = append(a.pending, data)
	idle := !a.writing
	a.writing = true
	a.mu.Unlock()

	if !idle {
		return nil
	}
	return a.writeCmd()
}

// writeCmd drains the queue in one go. It keeps the writer busy while keys keep
// arriving and only hands the role back once nothing is left.
func (a *App) writeCmd() tea.Cmd {
	terminal, done := a.pty, a.done

	return func() tea.Msg {
		for {
			a.mu.Lock()
			batch := a.pending
			a.pending = nil
			if len(batch) == 0 {
				a.writing = false
				a.mu.Unlock()
				return writtenMsg{}
			}
			a.mu.Unlock()

			for _, chunk := range batch {
				if _, err := terminal.Write(chunk); err != nil {
					a.mu.Lock()
					a.writing = false
					a.mu.Unlock()
					return failureMsg{err: err}
				}
			}

			select {
			case <-done:
				a.mu.Lock()
				a.writing = false
				a.mu.Unlock()
				return writtenMsg{}
			default:
			}
		}
	}
}

// readCmd waits for the next chunk of child output.
func (a *App) readCmd() tea.Cmd {
	out := a.out
	return func() tea.Msg {
		chunk, ok := <-out
		if !ok {
			return exitedMsg{}
		}
		return outputMsg(chunk)
	}
}

func (a *App) readPump() {
	terminal, out, done := a.pty, a.out, a.done
	buffer := make([]byte, readChunk)
	for {
		n, err := terminal.Read(buffer)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buffer[:n])
			select {
			case out <- chunk:
			case <-done:
				return
			}
		}
		if err != nil {
			close(out)
			return
		}
	}
}

// scheduleBlink keeps the cursor blinking at the rate the DECSCUSR modes use.
func (a *App) scheduleBlink() tea.Cmd {
	on := !a.term.NormalCursorOn()
	return tea.Tick(vt.BlinkPeriod, func(time.Time) tea.Msg { return blinkMsg(on) })
}

// startedMsg reports the outcome of spawning the child process.
type startedMsg struct {
	terminal sys.PTY
	err      error
}

// outputMsg is a chunk of bytes the child process wrote.
type outputMsg []byte

// writtenMsg confirms bytes reached the input queue.
type writtenMsg struct{}

// exitedMsg reports that the child process is gone and the output is drained.
type exitedMsg struct{}

// failureMsg reports something that went wrong in the background.
type failureMsg struct{ err error }

// blinkMsg toggles the cursor blink.
type blinkMsg bool

// closedMsg confirms the child process was torn down.
type closedMsg struct{}
