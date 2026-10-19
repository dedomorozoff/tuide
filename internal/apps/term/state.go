package term

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

// SnapshotState implements appsdk.Stateful. The emulator is rendered to an ANSI
// stream and gzipped; the grid size travels in a header so a restore rebuilds
// the terminal at the size the snapshot was taken at, before the window resizes
// it back to its frame.
//
// The process itself is not saved - it is gone with the server. What comes back
// is the picture: the screen, the scrollback behind it, the cursor and the
// title, under which a fresh shell starts.
func (a *App) SnapshotState() []byte {
	width, height := a.term.Size()

	var raw bytes.Buffer
	fmt.Fprintf(&raw, "%d %d\n", width, height)
	raw.Write(a.term.Snapshot())

	var packed bytes.Buffer
	zw := gzip.NewWriter(&packed)
	if _, err := zw.Write(raw.Bytes()); err != nil {
		return nil
	}
	if err := zw.Close(); err != nil {
		return nil
	}
	return packed.Bytes()
}

// RestoreState implements appsdk.Stateful. The packed stream is unpacked and
// replayed into the emulator, so the window opens showing what was there before
// and the shell that starts afterwards types under it.
func (a *App) RestoreState(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("term: restore: %w", err)
	}
	defer zr.Close()

	raw, err := io.ReadAll(zr)
	if err != nil {
		return fmt.Errorf("term: restore: %w", err)
	}

	header, screen, ok := bytes.Cut(raw, []byte("\n"))
	if !ok {
		return fmt.Errorf("term: restore: missing size header")
	}
	var width, height int
	if _, err := fmt.Sscanf(string(header), "%d %d", &width, &height); err != nil {
		return fmt.Errorf("term: restore: size header %q: %w", header, err)
	}

	if width > 0 && height > 0 {
		a.term.Resize(width, height)
		a.width, a.height = width, height
	}
	a.term.Write(screen)

	// Park the cursor on the last line: the restored screen keeps its own
	// cursor, but the shell that takes over should start a fresh line rather
	// than type over the middle of a full-screen frame.
	a.term.Write([]byte(fmt.Sprintf("\x1b[%d;1H", max(1, height))))
	return nil
}
