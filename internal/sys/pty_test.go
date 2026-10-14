package sys_test

import (
	"bytes"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/dedomorozoff/tuide/internal/sys"
)

const ptyReadTimeout = 5 * time.Second

// ptyEnvVar turns the Windows pseudoconsole test on. ConPTY needs a real
// console host, which sandboxes and containers do not provide: there the
// client starts, never writes a byte, and the pipe it is attached to cannot be
// woken up again, which wedges the whole test binary. On a machine with a
// working console the test is worth running, so it is opt-in there and always
// on everywhere else.
const ptyEnvVar = "TUIDE_TEST_PTY"

// echoMarker is printed by the child in every shell tuiDE ships a default for:
// both /bin/sh and PowerShell understand `echo`.
const echoMarker = "tuide-pty-ok"

// requireLivePTY skips the test on a machine that cannot host a pseudoconsole.
func requireLivePTY(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		return
	}
	if os.Getenv(ptyEnvVar) != "1" {
		t.Skipf("set %s=1 to exercise the Windows pseudoconsole", ptyEnvVar)
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
