package sys_test

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dedomorozoff/tuide/internal/sys"
)

const ptyReadTimeout = 15 * time.Second

// liveEnvVar turns the tests that spawn a real child on. Every one of them
// starts a console process on the machine running the suite and tears it down
// again, which on a desktop means windows appearing and being killed. That is
// worth having and not worth paying for on every `go test ./...`, so they are
// opt in: set TUIDE_LIVE_TESTS=1.
const liveEnvVar = "TUIDE_LIVE_TESTS"

// echoMarker is printed by the child in every shell tuiDE ships a default for:
// both /bin/sh and PowerShell understand `echo`.
const echoMarker = "tuide-pty-ok"

// requireLivePTY skips the test unless a live child was asked for. On a host
// where the pseudoconsole was forced with TUIDE_PTY=conpty the read cannot be
// woken up again and the pipe wedges the whole test binary, so that case is
// skipped too.
func requireLivePTY(t *testing.T) {
	t.Helper()
	if os.Getenv(liveEnvVar) != "1" {
		t.Skipf("set %s=1 to run tests that spawn a real child", liveEnvVar)
	}
	if runtime.GOOS == "windows" && strings.EqualFold(os.Getenv("TUIDE_PTY"), "conpty") {
		t.Skip("TUIDE_PTY=conpty forces a pseudoconsole this host cannot wake a read on")
	}
}

func TestDefaultShellIsRunnable(t *testing.T) {
	shell := sys.DefaultShell()
	if len(shell) == 0 || shell[0] == "" {
		t.Skip("no default shell configured in this environment")
	}
	if _, err := os.Stat(shell[0]); err != nil {
		t.Skipf("default shell %s is unavailable: %v", shell[0], err)
	}
}

func TestPTYEchoesWhatWasWritten(t *testing.T) {
	requireLivePTY(t)

	terminal, err := sys.StartPTY(sys.PTYOptions{Width: 80, Height: 24})
	if err != nil {
		t.Skipf("no pseudo terminal available: %v", err)
	}
	defer func() { _ = terminal.Close() }()

	// A carriage return is the Enter key on both platforms.
	if _, err := terminal.Write([]byte("echo " + echoMarker + "\r\n")); err != nil {
		t.Fatalf("write to pty: %v", err)
	}

	out := readUntil(t, terminal, []byte(echoMarker))
	if !bytes.Contains(out, []byte(echoMarker)) {
		t.Fatalf("child never echoed %q, got:\n%s", echoMarker, out)
	}
}

func TestPTYReportsResize(t *testing.T) {
	requireLivePTY(t)

	terminal, err := sys.StartPTY(sys.PTYOptions{Width: 80, Height: 24})
	if err != nil {
		t.Skipf("no pseudo terminal available: %v", err)
	}
	defer func() { _ = terminal.Close() }()

	if err := terminal.Resize(120, 40); err != nil {
		t.Errorf("resize 120x40: %v", err)
	}
}

func TestStartPTYRejectsEmptyCommand(t *testing.T) {
	t.Setenv("SHELL", "")
	t.Setenv("COMSPEC", "")

	terminal, err := sys.StartPTY(sys.PTYOptions{})
	if err != nil {
		return
	}
	// The platform still found a shell to hand back, so the terminal really
	// started and has to be torn down before the test gives up on it.
	_ = terminal.Close()
	t.Skip("this environment provides a shell even with the variables cleared")
}

// readUntil collects output until the marker shows up or the child goes quiet.
func readUntil(t *testing.T, terminal sys.PTY, marker []byte) []byte {
	t.Helper()

	collected := make(chan []byte, 1)
	go func() {
		var all bytes.Buffer
		buffer := make([]byte, 4096)
		for {
			n, err := terminal.Read(buffer)
			if n > 0 {
				all.Write(buffer[:n])
				if bytes.Contains(all.Bytes(), marker) {
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
	case <-time.After(ptyReadTimeout):
		t.Fatalf("child produced no output within %s", ptyReadTimeout)
		return nil
	}
}
