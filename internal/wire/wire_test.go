package wire

import (
	"bytes"
	"net"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	payload := []byte("hello\x1b[31m")
	var buf bytes.Buffer
	if err := Write(&buf, TypeOutput, payload); err != nil {
		t.Fatalf("Write: %v", err)
	}

	kind, got, err := Read(&buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if kind != TypeOutput {
		t.Errorf("kind = %q, want %q", kind, TypeOutput)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload = %q, want %q", got, payload)
	}
}

func TestEmptyPayloadRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, TypeDetach, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	kind, got, err := Read(&buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if kind != TypeDetach {
		t.Errorf("kind = %q, want %q", kind, TypeDetach)
	}
	if len(got) != 0 {
		t.Errorf("payload = %q, want empty", got)
	}
}

func TestWriteRejectsHugePayload(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, TypeOutput, make([]byte, MaxPayload+1)); err == nil {
		t.Error("Write accepted a payload over the limit")
	}
}

func TestReadOverPipeKeepsFrameIntact(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	payload := bytes.Repeat([]byte("frame"), 1000)
	go func() {
		_ = Write(server, TypeOutput, payload)
	}()

	kind, got, err := Read(client)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if kind != TypeOutput {
		t.Errorf("kind = %q, want %q", kind, TypeOutput)
	}
	if !bytes.Equal(got, payload) {
		t.Error("payload did not survive a piped read")
	}
}

func TestStateFileRoundTrip(t *testing.T) {
	t.Setenv("TUIDE_CONFIG_DIR", t.TempDir())

	if state, err := ReadState(); err != nil || state != nil {
		t.Fatalf("ReadState on empty dir = %v, %v; want nil, nil", state, err)
	}

	want := State{Pid: 42, Addr: "127.0.0.1:9999", Token: "secret"}
	if err := WriteState(want); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	got, err := ReadState()
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if got == nil || *got != want {
		t.Errorf("ReadState = %v, want %v", got, want)
	}

	if err := RemoveState(); err != nil {
		t.Fatalf("RemoveState: %v", err)
	}
	if state, err := ReadState(); err != nil || state != nil {
		t.Fatalf("ReadState after RemoveState = %v, %v; want nil, nil", state, err)
	}
	if err := RemoveState(); err != nil {
		t.Errorf("RemoveState on a missing file: %v", err)
	}
}
