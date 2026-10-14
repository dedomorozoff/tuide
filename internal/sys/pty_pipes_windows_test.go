//go:build windows

package sys

import (
	"bytes"
	"os"
	"testing"
	"time"
)

const pipesEchoMarker = "tuide-pipes-ok"

const liveEnvVar = "TUIDE_LIVE_TESTS"

// requireLiveChild skips a test that would start a real console process on the
// machine running the suite. These tests are worth having and not worth paying
// for on every `go test ./...`, so they are opt in.
func requireLiveChild(t *testing.T) {
	t.Helper()
	if os.Getenv(liveEnvVar) != "1" {
		t.Skipf("set %s=1 to run tests that spawn a real child", liveEnvVar)
	}
}

// cmdExe is the child these tests drive: it starts in milliseconds, unlike the
// default PowerShell, and understands `echo`.
func cmdExe(t *testing.T) string {
	t.Helper()
	requireLiveChild(t)

	shell := os.Getenv("COMSPEC")
	if shell == "" {
		shell = `C:\Windows\System32\cmd.exe`
	}
	if _, err := os.Stat(shell); err != nil {
		t.Skipf("command processor unavailable: %v", err)
	}
	return shell
}

func TestPipesPTYEchoesWhatWasWritten(t *testing.T) {
	terminal, err := startPipesPTY(PTYOptions{
		Command: []string{cmdExe(t)},
		Width:   80,
		Height:  24,
	})
	if err != nil {
		t.Fatalf("start pipes terminal: %v", err)
	}
	defer func() { _ = terminal.Close() }()

	if _, err := terminal.Write([]byte("echo " + pipesEchoMarker + "\r\n")); err != nil {
		t.Fatalf("write to pipes terminal: %v", err)
	}

	got := readPipesUntil(t, terminal, pipesEchoMarker)
	if !bytes.Contains(got, []byte(pipesEchoMarker)) {
		t.Fatalf("child never echoed %q, got:\n%s", pipesEchoMarker, got)
	}
}

func TestPipesPTYResizeIsAccepted(t *testing.T) {
	terminal, err := startPipesPTY(PTYOptions{
		Command: []string{cmdExe(t)},
		Width:   80,
		Height:  24,
	})
	if err != nil {
		t.Fatalf("start pipes terminal: %v", err)
	}
	defer func() { _ = terminal.Close() }()

	if err := terminal.Resize(120, 40); err != nil {
		t.Errorf("a resize with no console behind it must not fail: %v", err)
	}
}

func TestPipesPTYReportsExitedChild(t *testing.T) {
	terminal, err := startPipesPTY(PTYOptions{
		Command: []string{cmdExe(t), "/c", "exit", "0"},
		Width:   80,
		Height:  24,
	})
	if err != nil {
		t.Fatalf("start pipes terminal: %v", err)
	}
	defer func() { _ = terminal.Close() }()

	client, ok := terminal.(*pipesPTY)
	if !ok {
		t.Fatalf("transport is %T, want *pipesPTY", terminal)
	}

	deadline := time.Now().Add(10 * time.Second)
	for client.Alive() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if client.Alive() {
		t.Error("a child that already exited must not be reported as running")
	}
}

// TestTransportOverride pins the decision the terminal depends on: the
// environment variable has to win over the probe, in both directions, so a
// broken host can be worked around and a working one can be forced.
func TestTransportOverride(t *testing.T) {
	if resolveConPTY(ptyTransportPipes) {
		t.Errorf("%s=%s must never report the pseudoconsole as usable",
			ptyTransportVar, ptyTransportPipes)
	}
	if !resolveConPTY(ptyTransportConPTY) {
		t.Errorf("%s=%s must report the pseudoconsole as usable without probing",
			ptyTransportVar, ptyTransportConPTY)
	}
	if resolveConPTY("  PIPES  ") {
		t.Errorf("the override must be trimmed and matched case insensitively")
	}
}

func readPipesUntil(t *testing.T, terminal PTY, marker string) []byte {
	t.Helper()

	collected := make(chan []byte, 1)
	go func() {
		var all bytes.Buffer
		buffer := make([]byte, 4096)
		for {
			n, err := terminal.Read(buffer)
			if n > 0 {
				all.Write(buffer[:n])
				if bytes.Contains(all.Bytes(), []byte(marker)) {
					break
				}
			}
			if err != nil {
				break
			}
		}
		collected <- all.Bytes()
	}()

	select {
	case got := <-collected:
		return got
	case <-time.After(15 * time.Second):
		t.Fatalf("child produced no output within 15s")
		return nil
	}
}
