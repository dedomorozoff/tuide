package server

import (
	"net"
	"strings"
	"testing"

	"github.com/dedomorozoff/tuide/internal/wire"
)

func TestReplayRestoresTextStylesAndModes(t *testing.T) {
	m := newMirror()
	m.Write([]byte("\x1b[2J\x1b[H\x1b[1;31mred\x1b[0m plain"))
	m.Write([]byte("\x1b[?2004h\x1b[?1002h\x1b[?1006h"))

	replay := string(m.replayLocked())
	for _, want := range []string{"red", "plain", "38;5;1", "\x1b[?2004h", "\x1b[?1006h"} {
		if !strings.Contains(replay, want) {
			t.Errorf("replay is missing %q:\n%q", want, replay)
		}
	}
}

func TestReplaySkipsTrailingBlanks(t *testing.T) {
	m := newMirror()
	m.Write([]byte("\x1b[2J\x1b[Hhi"))

	replay := string(m.replayLocked())
	lines := strings.Split(replay, "\x1b[2;1H")
	if len(lines) < 2 {
		t.Fatalf("replay has no second row: %q", replay)
	}
	if strings.Contains(lines[1], "hi") {
		t.Errorf("second row repeated the first: %q", lines[1])
	}
}

func TestReplayTracksCursorVisibility(t *testing.T) {
	m := newMirror()
	m.Write([]byte("\x1b[?25h"))
	if !strings.Contains(string(m.replayLocked()), "\x1b[?25h") {
		t.Error("replay dropped a visible cursor")
	}
	m.Write([]byte("\x1b[?25l"))
	if !strings.Contains(string(m.replayLocked()), "\x1b[?25l") {
		t.Error("replay dropped a hidden cursor")
	}
}

func TestWideCellsSurviveReplay(t *testing.T) {
	m := newMirror()
	m.Write([]byte("\x1b[2J\x1b[Hпривет"))

	replay := string(m.replayLocked())
	if !strings.Contains(replay, "привет") {
		t.Errorf("replay lost a wide run: %q", replay)
	}
}

func TestAttachSendsReplayFrame(t *testing.T) {
	m := newMirror()
	m.Write([]byte("\x1b[2J\x1b[Hhello frame"))

	server, peer := net.Pipe()
	defer server.Close()
	defer peer.Close()

	c := &clientConn{conn: server}
	attached := make(chan struct{})
	go func() {
		m.attach(c, defaultWidth, defaultHeight)
		close(attached)
	}()

	kind, payload, err := wire.Read(peer)
	if err != nil {
		t.Fatalf("read replay: %v", err)
	}
	if kind != wire.TypeOutput {
		t.Fatalf("kind = %q, want %q", kind, wire.TypeOutput)
	}
	if !strings.Contains(string(payload), "hello frame") {
		t.Errorf("replay = %q, want the current screen", payload)
	}
	<-attached

	if got := m.count(); got != 1 {
		t.Errorf("clients = %d, want 1", got)
	}
	m.remove(c)
	if got := m.count(); got != 0 {
		t.Errorf("clients after remove = %d, want 0", got)
	}
}

func TestMirrorResizesWithClients(t *testing.T) {
	m := newMirror()
	m.resize(120, 40)
	width, height := m.screen.Size()
	if width != 120 || height != 40 {
		t.Errorf("size = %dx%d, want 120x40", width, height)
	}
}
