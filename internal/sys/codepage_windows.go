//go:build windows

package sys

import (
	"unicode/utf8"

	"golang.org/x/sys/windows"
	"golang.org/x/text/encoding/charmap"
)

// legacyDecoder turns the child output into UTF-8. With pipes between the
// processes the child encodes its text with the console code page it was given
// at CreateProcess — on a Russian locale that is cp866 — while everything
// downstream only understands UTF-8. Valid UTF-8 passes through untouched; the
// first sequence that can never grow into a valid rune switches the stream to
// the legacy code page, and from then on every chunk is re-encoded.
type legacyDecoder struct {
	pending []byte
	decode  func([]byte) []byte
}

var oemEncodings = map[uint32]*charmap.Charmap{
	437:  charmap.CodePage437,
	850:  charmap.CodePage850,
	852:  charmap.CodePage852,
	855:  charmap.CodePage855,
	866:  charmap.CodePage866,
	1251: charmap.Windows1251,
}

// oemEncoding names the charmap for the active OEM code page of the system.
func oemEncoding() *charmap.Charmap {
	getOEMCP := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetOEMCP")
	page, _, _ := getOEMCP.Call()
	return oemEncodings[uint32(page)]
}

func newLegacyDecoder() *legacyDecoder {
	return &legacyDecoder{}
}

// adapt decodes a chunk. A rune split between reads is held back so it can
// complete on the next chunk before anything is given away.
func (d *legacyDecoder) adapt(chunk []byte) []byte {
	data := make([]byte, 0, len(d.pending)+len(chunk))
	data = append(data, d.pending...)
	data = append(data, chunk...)
	d.pending = nil

	if d.decode != nil {
		return d.decode(data)
	}

	for i := 0; i < len(data); {
		if data[i] < utf8.RuneSelf {
			i++
			continue
		}
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size <= 1 {
			if enc := oemEncoding(); enc != nil {
				d.decode = decodeWith(enc)
			} else {
				d.decode = passthrough
			}
			return d.decode(data)
		}
		if i+size > len(data) {
			d.pending = append(d.pending, data[i:]...)
			return data[:i]
		}
		i += size
	}
	return data
}

// flushForClose returns whatever the detector still holds after EOF, so a cut
// off rune does not stay locked in the hold-back buffer forever.
func (d *legacyDecoder) flushForClose() []byte {
	if len(d.pending) == 0 {
		return nil
	}
	data := d.pending
	d.pending = nil
	if d.decode == nil {
		return nil
	}
	return d.decode(data)
}

func decodeWith(enc *charmap.Charmap) func([]byte) []byte {
	return func(chunk []byte) []byte {
		out, err := enc.NewDecoder().Bytes(chunk)
		if err != nil {
			return chunk
		}
		return out
	}
}

func passthrough(chunk []byte) []byte { return chunk }
