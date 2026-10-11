package appsdk

import (
	"time"

	"tuide/internal/config"
)

// Tick is broadcast by the desktop once a second so apps that display the time
// can refresh without owning a timer. Every open app receives it, focused or
// not, so clocks keep running in the background.
type Tick struct {
	Time time.Time
}

// RequestConfig asks the desktop for the current configuration. The reply is
// ConfigState, delivered back to the requesting window.
type RequestConfig struct{}

// ConfigState carries the desktop's current configuration.
type ConfigState struct {
	Config config.Config
}

// SaveConfig asks the desktop to apply a new configuration and persist it. The
// reply is ConfigSaved, delivered back to the requesting window.
type SaveConfig struct {
	Config config.Config
}

// ConfigSaved reports the outcome of SaveConfig; Err is nil on success.
type ConfigSaved struct {
	Err error
}
