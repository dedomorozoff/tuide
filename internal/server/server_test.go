package server

import (
	"net"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/appsdk"
	"github.com/dedomorozoff/tuide/internal/client"
	"github.com/dedomorozoff/tuide/internal/shell"
	"github.com/dedomorozoff/tuide/internal/theme"
	"github.com/dedomorozoff/tuide/internal/wire"
)

// noopApp is a window that renders nothing, enough to keep the desktop busy.
type noopApp struct{}

func (noopApp) Init() tea.Cmd                        { return nil }
func (noopApp) Close() tea.Cmd                       { return nil }
func (noopApp) Update(tea.Msg) (appsdk.App, tea.Cmd) { return noopApp{}, nil }
func (noopApp) View(appsdk.Context) tea.View         { return tea.NewView("") }

func testDesktop() *shell.Model {
	spec := appsdk.Spec{
		ID:     "noop",
		Name:   "noop",
		Launch: func() appsdk.App { return noopApp{} },
	}
	return shell.New(theme.Get(theme.DefaultName), []appsdk.Spec{spec}, shell.Options{
		Width:   80,
		Height:  24,
		Taskbar: true,
		Mouse:   true,
		Persist: true,
	})
}

// TestServerLifeCycle walks a client through the whole tmux story: list an
// empty session, attach and get the screen, detach and leave the session
// running, then shut the server down.
func TestServerLifeCycle(t *testing.T) {
	t.Setenv("TUIDE_CONFIG_DIR", t.TempDir())

	srv := New(testDesktop(), nil)
	done := make(chan error, 1)
	go func() { done <- srv.Run() }()

	addr := waitAddr(t, srv)

	info, err := client.List(addr, srv.Token())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if info.Clients != 0 {
		t.Fatalf("clients on an empty session = %d, want 0", info.Clients)
	}

	conn := rawAttach(t, addr, srv.Token())
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	kind, payload, err := wire.Read(conn)
	if err != nil {
		t.Fatalf("read after attach: %v", err)
	}
	if kind != wire.TypeOutput {
		t.Fatalf("first frame = %q, want %q", kind, wire.TypeOutput)
	}
	if len(payload) == 0 {
		t.Fatal("attach replayed an empty frame")
	}

	info, err = client.List(addr, srv.Token())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if info.Clients != 1 {
		t.Fatalf("clients after attach = %d, want 1", info.Clients)
	}

	if err := wire.Write(conn, wire.TypeDetach, nil); err != nil {
		t.Fatalf("detach: %v", err)
	}
	conn.Close()
	waitClients(t, srv, 0)

	if err := client.Shutdown(addr, srv.Token()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("server did not stop after shutdown")
	}

	if state, err := wire.ReadState(); err != nil || state != nil {
		t.Fatalf("state file after shutdown = %v, %v; want nil, nil", state, err)
	}
}

// TestServerSurvivesClientDisconnect is the point of the whole exercise: a
// dropped client must not take the session with it.
func TestServerSurvivesClientDisconnect(t *testing.T) {
	t.Setenv("TUIDE_CONFIG_DIR", t.TempDir())

	srv := New(testDesktop(), nil)
	done := make(chan error, 1)
	go func() { done <- srv.Run() }()

	addr := waitAddr(t, srv)

	conn := rawAttach(t, addr, srv.Token())
	conn.Close()
	waitClients(t, srv, 0)

	select {
	case err := <-done:
		t.Fatalf("server exited after a client dropped: %v", err)
	default:
	}

	if err := client.Shutdown(addr, srv.Token()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("server did not stop after shutdown")
	}
}

func TestServerRejectsBadToken(t *testing.T) {
	t.Setenv("TUIDE_CONFIG_DIR", t.TempDir())

	srv := New(testDesktop(), nil)
	done := make(chan error, 1)
	go func() { done <- srv.Run() }()

	addr := waitAddr(t, srv)
	if _, err := client.List(addr, "wrong"); err == nil {
		t.Fatal("List accepted a bad token")
	}

	if err := client.Shutdown(addr, srv.Token()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("server did not stop after shutdown")
	}
}

func waitAddr(t *testing.T, srv *Server) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if addr := srv.Addr(); addr != "" {
			return addr
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("server never started listening")
	return ""
}

func waitClients(t *testing.T, srv *Server, want int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if srv.mirror.count() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("clients = %d, want %d", srv.mirror.count(), want)
}

// rawAttach is a hand-rolled client: the packaged one needs a real terminal.
func rawAttach(t *testing.T, addr, token string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	hello := []byte(`{"token":"` + token + `","width":40,"height":12}`)
	if err := wire.Write(conn, wire.TypeHello, hello); err != nil {
		t.Fatalf("hello: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	kind, _, err := wire.Read(conn)
	if err != nil {
		t.Fatalf("handshake: %v", err)
	}
	if kind != wire.TypeWelcome {
		t.Fatalf("handshake reply = %q, want %q", kind, wire.TypeWelcome)
	}
	return conn
}
