package shell

import "errors"

var (
	// ErrUnknownApp is returned when the launch options name an app that
	// does not exist.
	ErrUnknownApp = errors.New("shell: unknown app")
)
