package sys

import (
	"net"
	"time"
)

// probeTargets are well-known addresses the connectivity probe tries in order.
// A successful TCP handshake means the network is usable; nothing is exchanged
// beyond it.
var probeTargets = []string{"1.1.1.1:443", "8.8.8.8:53"}

// probeTimeout bounds a single probe attempt.
const probeTimeout = time.Second

// Online reports whether any probe target accepts a TCP connection. It blocks,
// so it must run in a tea.Cmd, never in Update.
func Online() bool {
	for _, target := range probeTargets {
		conn, err := net.DialTimeout("tcp", target, probeTimeout)
		if err == nil {
			_ = conn.Close()
			return true
		}
	}
	return false
}
