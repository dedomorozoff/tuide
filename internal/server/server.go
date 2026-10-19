// Package server runs the desktop headless and serves attached clients over a
// local socket: the tmux half of tuiDE. The desktop keeps running while no
// client is attached, and every attach is given the current screen back.
package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/dedomorozoff/tuide/internal/shell"
	"github.com/dedomorozoff/tuide/internal/wire"
)

// helloDeadline is how long a connecting client has to say hello.
const helloDeadline = 5 * time.Second

// Server owns the desktop program and the clients attached to it.
type Server struct {
	model *shell.Model
	log   *log.Logger
	token string

	mu       sync.Mutex
	ln       net.Listener
	addr     string
	program  *tea.Program
	inputEnd *os.File
	mirror   *mirror
}

// clientConn is one attached client. The write mutex keeps frames whole when
// the mirror and control replies write at the same time.
type clientConn struct {
	conn net.Conn
	wmu  sync.Mutex
}

func (c *clientConn) send(kind wire.Type, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(clientWriteDeadline))
	return wire.Write(c.conn, kind, payload)
}

// New builds a server around a desktop model. A nil log discards diagnostics.
func New(model *shell.Model, logw io.Writer) *Server {
	if logw == nil {
		logw = io.Discard
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		panic(fmt.Sprintf("server: token: %v", err))
	}
	return &Server{
		model: model,
		log:   log.New(logw, "server: ", log.LstdFlags),
		token: hex.EncodeToString(token[:]),
	}
}

// Addr is the listening address, set once Run starts listening.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

// Token is the token clients must present in their hello.
func (s *Server) Token() string { return s.token }

// Model returns the desktop as it stands. Valid after Run has started; the
// shell mutates itself in place, so the pointer stays current.
func (s *Server) Model() *shell.Model { return s.model }

// Run serves the session until the desktop quits.
func (s *Server) Run() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("server: listen: %w", err)
	}

	s.mu.Lock()
	s.ln = listener
	s.addr = listener.Addr().String()
	s.mu.Unlock()

	if err := wire.WriteState(wire.State{Pid: os.Getpid(), Addr: s.addr, Token: s.token}); err != nil {
		listener.Close()
		return err
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		listener.Close()
		wire.RemoveState()
		return fmt.Errorf("server: input pipe: %w", err)
	}

	s.mirror = newMirror()
	s.inputEnd = writer
	s.program = tea.NewProgram(s.model,
		tea.WithInput(reader),
		tea.WithOutput(s.mirror),
		tea.WithColorProfile(colorProfile()),
	)

	go s.acceptLoop()
	s.log.Printf("listening on %s", s.addr)

	_, runErr := s.program.Run()

	listener.Close()
	wire.RemoveState()
	s.goodbye()
	writer.Close()
	reader.Close()

	if runErr != nil {
		return fmt.Errorf("server: run desktop: %w", runErr)
	}
	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.serveClient(conn)
	}
}

func (s *Server) serveClient(conn net.Conn) {
	_ = conn.SetReadDeadline(time.Now().Add(helloDeadline))
	kind, payload, err := wire.Read(conn)
	if err != nil || kind != wire.TypeHello {
		reject(conn, "hello expected")
		return
	}
	var hello wire.Hello
	if err := json.Unmarshal(payload, &hello); err != nil {
		reject(conn, "malformed hello")
		return
	}
	if hello.Token != s.token {
		reject(conn, "bad token")
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	if hello.List {
		info := wire.Info{Pid: os.Getpid(), Clients: s.mirror.count()}
		data, _ := json.Marshal(info)
		_ = wire.Write(conn, wire.TypeList, data)
		conn.Close()
		return
	}

	c := &clientConn{conn: conn}
	if err := c.send(wire.TypeWelcome, nil); err != nil {
		conn.Close()
		return
	}

	if hello.Control {
		s.controlLoop(c)
		conn.Close()
		return
	}

	s.mirror.attach(c, hello.Width, hello.Height)
	s.program.Send(tea.WindowSizeMsg{Width: max(1, hello.Width), Height: max(1, hello.Height)})

	s.readLoop(c)

	s.mirror.remove(c)
	conn.Close()
}

// controlLoop serves a connection that only gives orders: it never touches
// the screen, so a dropped control frame cannot lose the session state.
func (s *Server) controlLoop(c *clientConn) {
	for {
		kind, _, err := wire.Read(c.conn)
		if err != nil {
			return
		}
		if kind == wire.TypeShutdown {
			s.program.Send(tea.QuitMsg{})
			return
		}
	}
}

func (s *Server) readLoop(c *clientConn) {
	for {
		kind, payload, err := wire.Read(c.conn)
		if err != nil {
			return
		}
		switch kind {
		case wire.TypeInput:
			if _, err := s.inputEnd.Write(payload); err != nil {
				s.log.Printf("input: %v", err)
				return
			}
		case wire.TypeResize:
			var size wire.Resize
			if json.Unmarshal(payload, &size) != nil || size.Width <= 0 || size.Height <= 0 {
				continue
			}
			s.mirror.resize(size.Width, size.Height)
			s.program.Send(tea.WindowSizeMsg{Width: size.Width, Height: size.Height})
		case wire.TypeDetach:
			return
		case wire.TypeShutdown:
			s.program.Send(tea.QuitMsg{})
			return
		}
	}
}

// goodbye lets every attached client restore its terminal before the socket
// disappears under them.
func (s *Server) goodbye() {
	for _, c := range s.mirror.all() {
		_ = c.send(wire.TypeGoodbye, nil)
		c.conn.Close()
	}
}

func reject(conn net.Conn, reason string) {
	_ = wire.Write(conn, wire.TypeError, []byte(reason))
	conn.Close()
}

// colorProfile is the profile the desktop draws its frames in. The mirror is
// not a terminal, so Bubble Tea would fall back to NoTTY on its own and strip
// every color out of the stream; the decision comes from the environment the
// session was started with instead, the way a client terminal looks to the
// shell that launched it.
func colorProfile() colorprofile.Profile {
	return colorprofile.Env(os.Environ())
}
