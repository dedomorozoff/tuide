package sys_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dedomorozoff/tuide/internal/sys"
)

// TestResetTerminalClearsTheScreen pins the exit behaviour: the desktop leaves
// its final frame on the main screen, so the reset has to wipe it and put the
// cursor back in the corner instead of leaving the frame behind the prompt.
func TestResetTerminalClearsTheScreen(t *testing.T) {
	var out bytes.Buffer
	if err := sys.ResetTerminal(&out); err != nil {
		t.Fatalf("ResetTerminal: %v", err)
	}

	got := out.String()
	for _, want := range []string{"\x1b[?25h", "\x1b[?1049l"} {
		if !strings.Contains(got, want) {
			t.Errorf("reset sequence %q is missing %q", got, want)
		}
	}
	if !strings.HasSuffix(got, "\x1b[2J\x1b[H") {
		t.Errorf("reset sequence %q should end with clear screen and home cursor", got)
	}
}
