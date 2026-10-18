// Package wire is the frame protocol between a tuiDE server and its clients,
// plus the state file through which a client finds the running server.
package wire

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Type is the kind of a frame.
type Type byte

const (
	// TypeHello opens a connection and carries the auth token.
	TypeHello Type = 'H'
	// TypeWelcome accepts a client into the session.
	TypeWelcome Type = 'W'
	// TypeError rejects a client with a human-readable reason.
	TypeError Type = 'E'
	// TypeInput carries terminal input bytes to the desktop.
	TypeInput Type = 'I'
	// TypeOutput carries desktop output bytes to a client.
	TypeOutput Type = 'O'
	// TypeResize reports a new client terminal size.
	TypeResize Type = 'R'
	// TypeDetach asks the server to drop this client.
	TypeDetach Type = 'D'
	// TypeShutdown asks the server to end the session.
	TypeShutdown Type = 'S'
	// TypeGoodbye tells a client the session is over.
	TypeGoodbye Type = 'G'
	// TypeList answers a state query.
	TypeList Type = 'L'
)

// MaxPayload caps one frame so a broken or hostile peer cannot ask for a huge
// allocation.
const MaxPayload = 4 << 20

const headerSize = 4

// Hello opens a session: it carries the auth token and the size of the client
// terminal. List asks for the server state instead of a session, and Control
// marks a connection that only gives orders (kill-server) and never sees the
// screen.
type Hello struct {
	Token   string `json:"token"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	List    bool   `json:"list,omitempty"`
	Control bool   `json:"control,omitempty"`
}

// Resize reports a new client terminal size.
type Resize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Info describes a running server.
type Info struct {
	Pid     int `json:"pid"`
	Clients int `json:"clients"`
}

// Write sends one frame: a four byte big-endian length, the type byte and the
// payload.
func Write(w io.Writer, kind Type, payload []byte) error {
	if len(payload) > MaxPayload {
		return fmt.Errorf("wire: payload of %d bytes is over the %d limit", len(payload), MaxPayload)
	}
	header := binary.BigEndian.AppendUint32(nil, uint32(1+len(payload)))
	if _, err := w.Write(header); err != nil {
		return fmt.Errorf("wire: write header: %w", err)
	}
	if _, err := w.Write(append([]byte{byte(kind)}, payload...)); err != nil {
		return fmt.Errorf("wire: write body: %w", err)
	}
	return nil
}

// Read receives one frame.
func Read(r io.Reader) (Type, []byte, error) {
	var header [headerSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, err
	}
	size := binary.BigEndian.Uint32(header[:])
	if size < 1 {
		return 0, nil, errors.New("wire: frame without a type")
	}
	if size > MaxPayload+1 {
		return 0, nil, fmt.Errorf("wire: frame of %d bytes is over the %d limit", size, MaxPayload+1)
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(r, body); err != nil {
		return 0, nil, err
	}
	return Type(body[0]), body[1:], nil
}
