// Package appsdk is the contract between the desktop environment and the
// applications running inside its windows.
package appsdk

import (
	"charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/compositor"
	"github.com/dedomorozoff/tuide/internal/geom"
	"github.com/dedomorozoff/tuide/internal/theme"
)

// Context is what the window manager tells an app about its current frame.
type Context struct {
	Rect    geom.Rect
	Focused bool
	// Theme is the desktop palette the frame must be drawn with. It comes from
	// the shell on every frame, so a theme change reaches open windows at once.
	Theme theme.Theme
}

// Title is the text the window shows in its title bar.
type Title string

// App is a program running inside a window. It is a plain model: the window
// manager filters messages and hands over only what the app should see.
type App interface {
	// Init returns the command to start the app, if any.
	Init() tea.Cmd

	// Update handles a message. It must not block and must not do I/O.
	Update(tea.Msg) (App, tea.Cmd)

	// View renders into the rectangle the window manager allocated.
	View(Context) tea.View

	// Close releases resources. It returns a command to run after teardown.
	Close() tea.Cmd
}

// Named is implemented by apps that choose their own window title once they
// are running, which the terminal does when the program inside it sets one.
type Named interface {
	// Title is the name to show in the title bar.
	Title() string
}

// Painter is implemented by apps that draw a prepared grid of cells rather
// than a string. The terminal does this: a frame of cells is copied straight
// into the compositor and no styled string is ever built for it.
type Painter interface {
	// Paint draws into the rectangle the window manager allocated. It must not
	// block and must not do I/O.
	Paint(screen *compositor.Screen, rect geom.Rect)
}

// MouseConsumer is implemented by apps that take pointer input inside their
// own window. While one owns the pointer the shell forwards every press,
// motion and release that lands in the window body and stops turning them
// into window drags. A terminal does this because a drag across its text is
// a selection, not a move.
type MouseConsumer interface {
	// OwnsPointer reports whether the app is using the pointer right now.
	OwnsPointer() bool
}

// Spec describes an application that the launcher can start.
type Spec struct {
	ID   string
	Name string

	DefaultWidth  int
	DefaultHeight int
	MinWidth      int
	MinHeight     int

	Launch func() App
}

// Validate reports whether the spec can be launched.
func (s Spec) Validate() error {
	if s.ID == "" {
		return ErrNoID
	}
	if s.Name == "" {
		return ErrNoName
	}
	if s.Launch == nil {
		return ErrNoLaunch
	}
	return nil
}
