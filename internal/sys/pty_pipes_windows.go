//go:build windows

package sys

import (
	"fmt"
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// pipesPTY is the fallback terminal for hosts that cannot host a pseudoconsole.
// The child gets anonymous pipes instead of a console, which is enough for a
// shell that speaks the protocol over stdio. It is not a full terminal: a
// program that asks the console how wide it is gets nothing back and falls back
// to line mode, and there is no console to resize.
type pipesPTY struct {
	input   *os.File
	output  *os.File
	process windows.Handle
	thread  windows.Handle

	mu     sync.Mutex
	alive  bool
	closed bool
}

// startPipesPTY starts the client with pipes wired to its standard streams.
func startPipesPTY(options PTYOptions) (PTY, error) {
	executable, line, environment, workingDir, err := childParams(
		options.Command, options.Dir, options.Env)
	if err != nil {
		return nil, err
	}

	inherit := windows.SecurityAttributes{
		Length:        uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		InheritHandle: 1,
	}

	var input, inputWrite, output, outputWrite windows.Handle
	if err := windows.CreatePipe(&input, &inputWrite, &inherit, 0); err != nil {
		return nil, fmt.Errorf("sys: pty: input pipe: %w", err)
	}
	if err := windows.CreatePipe(&output, &outputWrite, &inherit, 0); err != nil {
		_ = windows.CloseHandle(input)
		_ = windows.CloseHandle(inputWrite)
		return nil, fmt.Errorf("sys: pty: output pipe: %w", err)
	}

	startup := &windows.StartupInfo{
		Cb:         uint32(unsafe.Sizeof(windows.StartupInfo{})),
		ShowWindow: windows.SW_HIDE,
		Flags:      windows.STARTF_USESTDHANDLES | windows.STARTF_USESHOWWINDOW,
		StdInput:   input,
		StdOutput:  outputWrite,
		StdErr:     outputWrite,
	}

	info := new(windows.ProcessInformation)
	if err := windows.CreateProcess(
		executable, line, nil, nil, true,
		windows.CREATE_UNICODE_ENVIRONMENT|windows.CREATE_DEFAULT_ERROR_MODE,
		environment, workingDir, startup, info,
	); err != nil {
		_ = windows.CloseHandle(input)
		_ = windows.CloseHandle(inputWrite)
		_ = windows.CloseHandle(output)
		_ = windows.CloseHandle(outputWrite)
		return nil, fmt.Errorf("sys: pty: start %s: %w", options.Command[0], err)
	}

	// The parent keeps its ends out of the child handle table. Without this the
	// child holds a copy of the read end too and the read never reaches EOF.
	windows.SetHandleInformation(input, windows.HANDLE_FLAG_INHERIT, 0)
	windows.SetHandleInformation(output, windows.HANDLE_FLAG_INHERIT, 0)

	terminal := &pipesPTY{
		input:   os.NewFile(uintptr(inputWrite), "tuide-pty-stdin"),
		output:  os.NewFile(uintptr(output), "tuide-pty-stdout"),
		process: info.Process,
		thread:  info.Thread,
		alive:   true,
	}
	go terminal.wait()
	return terminal, nil
}

func (p *pipesPTY) wait() {
	_, _ = windows.WaitForSingleObject(p.process, windows.INFINITE)
	p.mu.Lock()
	p.alive = false
	p.mu.Unlock()
}

func (p *pipesPTY) Read(b []byte) (int, error) { return p.output.Read(b) }

func (p *pipesPTY) Write(b []byte) (int, error) { return p.input.Write(b) }

// Resize does nothing: with pipes there is no console to resize and no way to
// tell the child how big the window is. The virtual terminal still redraws
// itself into whatever rectangle the window manager gives it.
func (p *pipesPTY) Resize(width, height int) error { return nil }

func (p *pipesPTY) Alive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.alive
}

func (p *pipesPTY) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	_ = windows.TerminateProcess(p.process, 1)
	_, _ = windows.WaitForSingleObject(p.process, 2000)
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
