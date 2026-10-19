package server

import (
	"sync"
	"time"

	"github.com/dedomorozoff/tuide/internal/vt"
	"github.com/dedomorozoff/tuide/internal/wire"
)

// clientWriteDeadline is how long one frame may take to reach a client. A
// client that stops reading is dropped instead of freezing the desktop.
const clientWriteDeadline = 5 * time.Second

// defaultWidth and defaultHeight are the screen the mirror tracks before the
// first client attaches.
const (
	defaultWidth  = 80
	defaultHeight = 24
)

// mirror is the desktop's output sink. It parses the program's byte stream
// into a screen, so a client attaching an hour later can be given the frame
// that is up right now, and it fans every chunk out to the attached clients.
type mirror struct {
	mu      sync.Mutex
	screen  *vt.VT
	clients []*clientConn
}

func newMirror() *mirror {
	return &mirror{screen: vt.New(defaultWidth, defaultHeight)}
}

// Write is the program's output. It never fails and never wedges on a client:
// a stuck connection is dropped, not obeyed.
func (m *mirror) Write(p []byte) (int, error) {
	frames := expandNewlines(p)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.screen.Write(frames)
	dead := make([]*clientConn, 0, len(m.clients))
	for _, c := range m.clients {
		if c.send(wire.TypeOutput, frames) != nil {
			c.conn.Close()
			dead = append(dead, c)
		}
	}
	for _, c := range dead {
		m.removeLocked(c)
	}
	return len(p), nil
}

// expandNewlines turns the program's bare newlines into carriage return and
// line feed. The renderer talks by the rules of a terminal whose line
// discipline maps a newline on its way out; no such driver sits between the
// server and a client terminal, so the bytes must carry the carriage
// themselves or every frame drawn with a newline lands at the wrong column.
func expandNewlines(p []byte) []byte {
	for i, b := range p {
		if b == '\n' && (i == 0 || p[i-1] != '\r') {
			out := make([]byte, 0, len(p)+len(p)/16+16)
			for i, b := range p {
				if b == '\n' && (i == 0 || p[i-1] != '\r') {
					out = append(out, '\r')
				}
				out = append(out, b)
			}
			return out
		}
	}
	return p
}

// attach adds a client and hands it the current screen.
func (m *mirror) attach(c *clientConn, width, height int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients = append(m.clients, c)
	m.screen.Resize(max(1, width), max(1, height))
	if err := c.send(wire.TypeOutput, m.replayLocked()); err != nil {
		c.conn.Close()
		m.removeLocked(c)
	}
}

// remove drops a client.
func (m *mirror) remove(c *clientConn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeLocked(c)
}

func (m *mirror) removeLocked(c *clientConn) {
	kept := m.clients[:0]
	for _, other := range m.clients {
		if other != c {
			kept = append(kept, other)
		}
	}
	m.clients = kept
}

// resize follows the attached client's terminal.
func (m *mirror) resize(width, height int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.screen.Resize(width, height)
}

// count is how many clients are attached.
func (m *mirror) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.clients)
}

// all is a snapshot of the client list.
func (m *mirror) all() []*clientConn {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*clientConn, len(m.clients))
	copy(out, m.clients)
	return out
}
