package appsdk

import "errors"

var (
	ErrNoID     = errors.New("appsdk: spec has no id")
	ErrNoName   = errors.New("appsdk: spec has no name")
	ErrNoLaunch = errors.New("appsdk: spec has no launch function")
	ErrClosed   = errors.New("appsdk: app is closed")
)
