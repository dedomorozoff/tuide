package term

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dedomorozoff/tuide/internal/vt"
)

// TestAltReportedAsMetaStillReachesTheChild covers a terminal that reports
// the alt key as the meta key: the child expects the alt form of the key.
func TestAltReportedAsMetaStillReachesTheChild(t *testing.T) {
	if got := string(encodeKey(tea.Key{Code: 'b', Text: "b", Mod: tea.ModMeta}, vt.Modes{})); got != "\x1bb" {
		t.Errorf("meta+b = %q, want %q", got, "\x1bb")
	}
}
