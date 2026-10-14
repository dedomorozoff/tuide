//go:build windows

package sys

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// procThreadAttributePseudoConsole is ProcThreadAttributePseudoConsole. The
// Windows headers define it, but the Go bindings do not.
const procThreadAttributePseudoConsole = 0x00020016

// ptyTransport names the two ways a child process can be given a terminal on
// Windows. TUIDE_PTY forces one; otherwise the pseudoconsole is probed once.
const (
	ptyTransportConPTY = "conpty"
	ptyTransportPipes  = "pipes"
)

// ptyTransportVar overrides the transport, which is the only way to test the
// pseudoconsole on a host where it is known to be broken.
const ptyTransportVar = "TUIDE_PTY"

// conptyProbeTimeout is how long the transport probe waits for its throwaway
// child.
const conptyProbeTimeout = 3 * time.Second

// conptyUsable is decided once per process: a machine where the pseudoconsole
// works must not pay for the probe on every terminal window, and a machine
// where it does not must not keep paying for it either.
var conptyUsable = sync.OnceValue(func() bool {
	return resolveConPTY(os.Getenv(ptyTransportVar))
})

// resolveConPTY is the transport decision behind the cache: the environment
// variable wins, otherwise a throwaway child says whether the pseudoconsole
// really works.
func resolveConPTY(override string) bool {
	switch strings.ToLower(strings.TrimSpace(override)) {
	case ptyTransportPipes:
		return false
	case ptyTransportConPTY:
		return true
	}
	return probeConPTY()
}

// probeConPTY starts a child on a pseudoconsole and watches it finish. Hosts
// where the pseudoconsole cannot host a session kill the child at DLL init and
// the terminal would stay empty forever, so the child exiting cleanly is the
// signal that the real transport will work.
func probeConPTY() bool {
	shell := os.Getenv("COMSPEC")
	if shell == "" {
		shell = "cmd.exe"
	}

	terminal, err := startConPTY(PTYOptions{
		Command: []string{shell, "/c", "exit", "0"},
		Width:   80,
		Height:  24,
	})
	if err != nil {
		return false
	}
	defer func() { _ = terminal.Close() }()

	client, ok := terminal.(*windowsPTY)
	if !ok {
		return true
	}
	return client.exitCode(conptyProbeTimeout) == 0
}

// windowsPTY is a ConPTY session: a pseudoconsole handle with the pipes that
// carry bytes to and from the client process.
type windowsPTY struct {
	console windows.Handle
	input   *os.File
	output  *os.File
	process windows.Handle
	thread  windows.Handle

	mu     sync.Mutex
	alive  bool
	closed bool
}

// startPTY creates the terminal. A working pseudoconsole is the better
// transport, so it is tried first; a host that cannot host one would leave the
// window permanently blank, so the probe falls back to plain pipes instead.
func startPTY(options PTYOptions) (PTY, error) {
	if !conptyUsable() {
		return startPipesPTY(options)
	}
	return startConPTY(options)
}

// startConPTY creates a ConPTY session: a pseudoconsole handle with the pipes
// that carry bytes to and from the client process.
func startConPTY(options PTYOptions) (PTY, error) {
	input, output, console, err := createConsole(options.Width, options.Height)
	if err != nil {
		return nil, err
	}

	process, thread, err := spawnWithConsole(options.Command, options.Dir, childEnv(options.Env), console)
	if err != nil {
		windows.ClosePseudoConsole(console)
		_ = windows.CloseHandle(input)
		_ = windows.CloseHandle(output)
		return nil, err
	}

	terminal := &windowsPTY{
		console: console,
		input:   os.NewFile(uintptr(input), "tuide-pty-stdin"),
		output:  os.NewFile(uintptr(output), "tuide-pty-stdout"),
		process: process,
		thread:  thread,
		alive:   true,
	}
	go terminal.wait()
	return terminal, nil
}

// createConsole wires two anonymous pipes into a pseudoconsole of the given
// size. The returned handles are the parent ends: write to input, read from
// output.
func createConsole(width, height int) (input, output, console windows.Handle, err error) {
	var in, inWrite, out, outWrite windows.Handle
	if err := windows.CreatePipe(&in, &inWrite, nil, 0); err != nil {
		return 0, 0, 0, fmt.Errorf("sys: pty: input pipe: %w", err)
	}
	if err := windows.CreatePipe(&out, &outWrite, nil, 0); err != nil {
		_ = windows.CloseHandle(in)
		_ = windows.CloseHandle(inWrite)
		return 0, 0, 0, fmt.Errorf("sys: pty: output pipe: %w", err)
	}

	size := windows.Coord{X: int16(max(1, width)), Y: int16(max(1, height))}
	if err := windows.CreatePseudoConsole(size, in, outWrite, 0, &console); err != nil {
		_ = windows.CloseHandle(in)
		_ = windows.CloseHandle(inWrite)
		_ = windows.CloseHandle(out)
		_ = windows.CloseHandle(outWrite)
		return 0, 0, 0, fmt.Errorf("sys: pty: create pseudoconsole: %w", err)
	}

	// The pseudoconsole duplicated what it needs during creation.
	_ = windows.CloseHandle(in)
	_ = windows.CloseHandle(outWrite)
	return inWrite, out, console, nil
}

// spawnWithConsole starts the client process with the pseudoconsole attached
// as its standard streams.
func spawnWithConsole(command []string, dir string, env []string, console windows.Handle) (process, thread windows.Handle, err error) {
	attributes, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return 0, 0, fmt.Errorf("sys: pty: attribute list: %w", err)
	}
	defer attributes.Delete()

	if err := attributes.Update(
		procThreadAttributePseudoConsole,
		unsafe.Pointer(&console),
		unsafe.Sizeof(console),
	); err != nil {
		return 0, 0, fmt.Errorf("sys: pty: attach pseudoconsole: %w", err)
	}

	startup := &windows.StartupInfoEx{
		StartupInfo: windows.StartupInfo{
			Cb:         uint32(unsafe.Sizeof(windows.StartupInfoEx{})),
			ShowWindow: windows.SW_HIDE,
			Flags:      windows.STARTF_USESHOWWINDOW,
		},
		ProcThreadAttributeList: attributes.List(),
	}

	executable, line, environment, workingDir, err := childParams(command, dir, env)
	if err != nil {
		return 0, 0, err
	}

	info := new(windows.ProcessInformation)
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT) |
		windows.CREATE_UNICODE_ENVIRONMENT |
		windows.CREATE_DEFAULT_ERROR_MODE
	if err := windows.CreateProcess(
		executable, line, nil, nil, false, flags,
		environment, workingDir, &startup.StartupInfo, info,
	); err != nil {
		return 0, 0, fmt.Errorf("sys: pty: start %s: %w", command[0], err)
	}
	return info.Process, info.Thread, nil
}

// childParams prepares what CreateProcess needs to start the client: the
// executable, the composed command line, the UTF-16 environment block and the
// working directory. Both Windows transports start the same child, only the
// handles around it differ.
func childParams(command []string, dir string, env []string) (exe, line, environment, workingDir *uint16, err error) {
	exe, err = windows.UTF16PtrFromString(command[0])
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("sys: pty: command name: %w", err)
	}
	line, err = windows.UTF16PtrFromString(windows.ComposeCommandLine(command[1:]))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("sys: pty: command line: %w", err)
	}
	environment = utf16Block(childEnv(env))
	workingDir, err = wideDir(dir)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return exe, line, environment, workingDir, nil
}

// childEnv is the environment block for the client process: the desktop's
// environment with TERM forced, sorted because Windows requires a sorted block.
func childEnv(override []string) []string {
	env := os.Environ()
	if override != nil {
		env = override
	}

	filtered := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(strings.ToUpper(entry), "TERM=") {
			continue
		}
		filtered = append(filtered, entry)
	}
	filtered = append(filtered, "TERM=xterm-256color")
	sort.Strings(filtered)
	return filtered
}

func wideDir(dir string) (*uint16, error) {
	if dir == "" {
		return nil, nil
	}
	wide, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return nil, fmt.Errorf("sys: pty: working directory: %w", err)
	}
	return wide, nil
}

// utf16Block turns a list of KEY=VALUE entries into the double-null terminated
// UTF-16 environment block CreateProcess expects.
func utf16Block(env []string) *uint16 {
	if len(env) == 0 {
		return nil
	}
	runes := make([]uint16, 0, len(env)*32)
	for _, entry := range env {
		runes = append(runes, utf16.Encode([]rune(entry))...)
		runes = append(runes, 0)
	}
	runes = append(runes, 0)
	return &runes[0]
}

// wait blocks until the client exits and remembers the fact, so the terminal
// window can show a dead shell instead of a frozen one.
func (p *windowsPTY) wait() {
	_, _ = windows.WaitForSingleObject(p.process, windows.INFINITE)
	p.mu.Lock()
	p.alive = false
	p.mu.Unlock()
}

// Read implements PTY.
func (p *windowsPTY) Read(b []byte) (int, error) { return p.output.Read(b) }

// Write implements PTY.
func (p *windowsPTY) Write(b []byte) (int, error) { return p.input.Write(b) }

// Resize implements PTY.
func (p *windowsPTY) Resize(width, height int) error {
	size := windows.Coord{X: int16(max(1, width)), Y: int16(max(1, height))}
	if err := windows.ResizePseudoConsole(p.console, size); err != nil {
		return fmt.Errorf("sys: pty: resize %dx%d: %w", width, height, err)
	}
	return nil
}

// Alive reports whether the client process is still running.
func (p *windowsPTY) Alive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.alive
}

// exitCode waits up to timeout for the client and reports how it ended. A child
// still running when the wait is over, or one that could not be queried, is
// reported as 1 so a broken session never reads as a successful probe.
func (p *windowsPTY) exitCode(timeout time.Duration) uint32 {
	event, err := windows.WaitForSingleObject(p.process, uint32(timeout.Milliseconds()))
	if err != nil || event != uint32(windows.WAIT_OBJECT_0) {
		return 1
	}

	var code uint32
	if err := windows.GetExitCodeProcess(p.process, &code); err != nil {
		return 1
	}
	return code
}

// Close implements PTY.
func (p *windowsPTY) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	// Closing the pseudoconsole tears the session down and makes the client
	// receive a hangup, which is what lets interactive shells exit cleanly.
	windows.ClosePseudoConsole(p.console)
	_, _ = windows.WaitForSingleObject(p.process, 200)
	_ = windows.TerminateProcess(p.process, 1)
	_, _ = windows.WaitForSingleObject(p.process, windows.INFINITE)
	_ = windows.CloseHandle(p.thread)
	_ = windows.CloseHandle(p.process)

	// Closing a handle does not wake a read that is already parked on it, so
	// the read pump has to be cancelled by hand or it would never return.
	_ = windows.CancelIoEx(windows.Handle(p.output.Fd()), nil)
	_ = windows.CancelIoEx(windows.Handle(p.input.Fd()), nil)

	errIn := p.input.Close()
	errOut := p.output.Close()
	if errIn != nil {
		return fmt.Errorf("sys: pty: close input: %w", errIn)
	}
	return errOut
}
