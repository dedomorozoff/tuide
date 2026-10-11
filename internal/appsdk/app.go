// Package appsdk is the contract between the desktop environment and the
// applications running inside its windows.
package appsdk

import (
	"charm.land/bubbletea/v2"

	"tuide/internal/geom"
	"tuide/internal/theme"
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
