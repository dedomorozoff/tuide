// Package client attaches a local terminal to a running tuiDE server: input,
// output and resize flow over the wire, and the client can leave the session
// behind at any moment without touching it.
package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/dedomorozoff/tuide/internal/sys"
	"github.com/dedomorozoff/tuide/internal/wire"
)

// DetachKey is the byte that pulls the client out of the session: ctrl+\, the
// chord screen and dtach use for the same purpose. The client eats it before
// it reaches the desktop, so the desktop never has to know about attaching.
const DetachKey = byte(0x1c)

// ErrDetached reports that the client left the session on purpose.
var ErrDetached = errors.New("client: detached")

// List asks the server for its state.
func List(addr, token string) (wire.Info, error) {
	var info wire.Info
	conn, kind, payload, err := dial(addr, wire.Hello{Token: token, List: true})
	if err != nil {
		return info, err
	}
	defer conn.Close()
	if kind != wire.TypeList {
		return info, fmt.Errorf("client: list: unexpected reply %q", kind)
	}
	if err := json.Unmarshal(payload, &info); err != nil {
		return info, fmt.Errorf("client: parse server info: %w", err)
	}
	return info, nil
}

// Shutdown asks the server to end the session.
func Shutdown(addr, token string) error {
	conn, _, _, err := dial(addr, wire.Hello{Token: token, Control: true})
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := wire.Write(conn, wire.TypeShutdown, nil); err != nil {
		return fmt.Errorf("client: shutdown: %w", err)
	}
	return nil
}

// Attach connects the local terminal to the session and blocks until the user
// detaches, the terminal closes or the session ends.
func Attach(addr, token string) error {
	restoreInput, err := sys.MakeRaw(os.Stdin)
	if err != nil {
		return fmt.Errorf("client: attach needs a terminal: %w", err)
	}
	restoreOutput, err := sys.EnableVTProcessing(os.Stdout)
	if err != nil {
		_ = restoreInput()
		return err
	}
	defer func() {
		_ = restoreOutput()
		_ = restoreInput()
		_ = sys.ResetTerminal(os.Stdout)
	}()

	width, height, err := sys.TerminalSize(os.Stdout)
	if err != nil {
		return err
	}

	conn, kind, _, err := dial(addr, wire.Hello{Token: token, Width: width, Height: height})
	if err != nil {
		return err
	}
	defer conn.Close()
	if kind != wire.TypeWelcome {
		return fmt.Errorf("client: attach: unexpected reply %q", kind)
	}

	var wmu sync.Mutex
	errc := make(chan error, 2)
	go forwardInput(conn, &wmu, errc)
	go watchResize(conn, &wmu, os.Stdout, errc)

	for {
		kind, payload, err := wire.Read(conn)
		if err != nil {
			select {
			case cause := <-errc:
				return cause
			default:
				return fmt.Errorf("client: connection lost: %w", err)
			}
		}
		switch kind {
		case wire.TypeOutput:
			if _, err := os.Stdout.Write(payload); err != nil {
				return fmt.Errorf("client: write terminal: %w", err)
			}
		case wire.TypeGoodbye:
			return nil
		case wire.TypeError:
			return errors.New("client: " + string(payload))
		}
	}
}

// forwardInput pumps terminal bytes to the server and watches for the detach
// key. Everything before the key still reaches the desktop; the key itself is
// eaten here and never becomes part of the session.
func forwardInput(conn net.Conn, wmu *sync.Mutex, errc chan<- error) {
	buf := make([]byte, 4096)
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			forward, detached := splitDetach(buf[:n])
			if len(forward) > 0 {
				if writeErr := writeFrame(wmu, conn, wire.TypeInput, forward); writeErr != nil {
					fail(conn, errc, writeErr)
					return
				}
			}
			if detached {
				if writeErr := writeFrame(wmu, conn, wire.TypeDetach, nil); writeErr != nil {
					fail(conn, errc, writeErr)
					return
				}
				fail(conn, errc, ErrDetached)
				return
			}
		}
		if err != nil {
			fail(conn, errc, err)
			return
		}
	}
}

// splitDetach cuts the first detach key out of a chunk.
func splitDetach(chunk []byte) (forward []byte, detached bool) {
	for i, b := range chunk {
		if b == DetachKey {
			return chunk[:i], true
		}
	}
	return chunk, false
}

// watchResize tells the server when the local terminal changes size.
func watchResize(conn net.Conn, wmu *sync.Mutex, term *os.File, errc chan<- error) {
	sizes, cancel := sys.NotifyResize(term)
	defer cancel()
	for range sizes {
		width, height, err := sys.TerminalSize(term)
		if err != nil {
			continue
		}
		payload, _ := json.Marshal(wire.Resize{Width: width, Height: height})
		if err := writeFrame(wmu, conn, wire.TypeResize, payload); err != nil {
			fail(conn, errc, err)
			return
		}
	}
}

func writeFrame(wmu *sync.Mutex, conn net.Conn, kind wire.Type, payload []byte) error {
	wmu.Lock()
	defer wmu.Unlock()
	return wire.Write(conn, kind, payload)
}

// fail reports the reason the pumps stopped and hangs the connection up, which
// is what unblocks the main read loop.
func fail(conn net.Conn, errc chan<- error, err error) {
	select {
	case errc <- err:
	default:
	}
	conn.Close()
}

// dial opens a connection and exchanges the hello.
func dial(addr string, hello wire.Hello) (net.Conn, wire.Type, []byte, error) {
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("client: connect: %w", err)
	}
	payload, err := json.Marshal(hello)
	if err != nil {
		conn.Close()
		return nil, 0, nil, fmt.Errorf("client: encode hello: %w", err)
	}
	if err := wire.Write(conn, wire.TypeHello, payload); err != nil {
		conn.Close()
		return nil, 0, nil, fmt.Errorf("client: hello: %w", err)
	}
	kind, reply, err := wire.Read(conn)
	if err != nil {
		conn.Close()
		return nil, 0, nil, fmt.Errorf("client: handshake: %w", err)
	}
	if kind == wire.TypeError {
		conn.Close()
		return nil, 0, nil, errors.New("client: " + string(reply))
	}
	return conn, kind, reply, nil
}
