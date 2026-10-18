//go:build windows

package sys

import (
	"strings"
	"testing"
)

func feed(decoder *legacyDecoder, chunks ...string) string {
	var out strings.Builder
	for _, chunk := range chunks {
		out.Write(decoder.adapt([]byte(chunk)))
	}
	out.Write(decoder.flushForClose())
	return out.String()
}

func TestLegacyDecoderPassesUTF8Through(t *testing.T) {
	got := feed(newLegacyDecoder(), "Превед мир", "\r\n")
	if got != "Превед мир\r\n" {
		t.Fatalf("adapt = %q, want the same text back", got)
	}
}

func TestLegacyDecoderDecodesOEMStream(t *testing.T) {
	// "Превед мир" in cp866, the console code page of a Russian locale.
	got := feed(newLegacyDecoder(), "\x8f\xe0\xa5\xa2\xa5\xa4 \xac\xa8\xe0\r\n")
	if got != "Превед мир\r\n" {
		t.Fatalf("adapt = %q, want decoded cp866 text", got)
	}
}

func TestLegacyDecoderHoldsASplitRune(t *testing.T) {
	got := feed(newLegacyDecoder(), "Преве", "д", " мир")
	if got != "Превед мир" {
		t.Fatalf("adapt = %q, want the rune glued back", got)
	}
}

func TestLegacyDecoderCarriesTheDecisionToLaterChunks(t *testing.T) {
	got := feed(newLegacyDecoder(), "seconds: ", "\x8f\xe0\xa5\xa2\xa5\xa4 ", "\xac\xa8\xe0\r\n")
	if got != "seconds: Превед мир\r\n" {
		t.Fatalf("adapt = %q, want the whole stream decoded once it turned legacy", got)
	}
}
