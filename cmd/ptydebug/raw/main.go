package main

import (
	"flag"
	"fmt"
	"os"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const pseudoConsoleAttr = 0x00020016

// stillActive is the exit code a running process reports.
const stillActive = 259

type variant struct {
	name    string
	env     bool
	inherit bool
	conpty  bool
	pipes   bool
}

func main() {
	variants := flag.String("variants", "", "comma separated variant names, empty runs every variant")
	exe := flag.String("exe", "", "program to run")
	wait := flag.Duration("wait", 3*time.Second, "how long to collect output")
	flag.Parse()

	if *exe == "" {
		fmt.Fprintln(os.Stderr, "usage: raw [-variants list] [-wait d] -exe path args...")
		os.Exit(2)
	}

	all := []variant{
		{"noinherit+env", true, false, true, false},
		{"noinherit+nilenv", false, false, true, false},
		{"inherit+env", true, true, true, false},
		{"inherit+nilenv", false, true, true, false},
		{"plain+env", true, false, false, false},
		{"plain+nilenv", false, false, false, false},
		{"pipes+env", true, false, false, true},
		{"pipes+nilenv", false, false, false, true},
	}

	selected := all
	if *variants != "" {
		wanted := map[string]bool{}
		for _, name := range splitComma(*variants) {
			wanted[name] = true
		}
		selected = nil
		for _, v := range all {
			if wanted[v.name] {
				selected = append(selected, v)
			}
		}
	}

	failures := 0
	go func() {
		time.Sleep(time.Duration(len(selected))*(*wait+5*time.Second) + 10*time.Second)
		fmt.Println("\nwatchdog: a variant did not finish, aborting")
		os.Exit(2)
	}()

	for _, v := range selected {
		if err := run(v, *exe, flag.Args(), *wait); err != nil {
			fmt.Printf("  %-16s FAILED: %v\n", v.name, err)
			failures++
		}
	}
	if failures > 0 {
		fmt.Printf("\n%d of %d variants produced no output\n", failures, len(selected))
		os.Exit(1)
	}
	os.Exit(0)
}

// run starts one ConPTY session, collects its output and reports whether the
// child survived and what it exited with.
func run(v variant, exe string, args []string, wait time.Duration) error {
	if !v.conpty {
		if v.pipes {
			return runPipes(v, exe, args, wait)
		}
		return runPlain(v, exe, args, wait)
	}

	var in, inWrite, out, outWrite windows.Handle
	if err := windows.CreatePipe(&in, &inWrite, nil, 0); err != nil {
		return fmt.Errorf("input pipe: %w", err)
	}
	if err := windows.CreatePipe(&out, &outWrite, nil, 0); err != nil {
		return fmt.Errorf("output pipe: %w", err)
	}
	var console windows.Handle
	size := windows.Coord{X: 80, Y: 24}
	if err := windows.CreatePseudoConsole(size, in, outWrite, 0, &console); err != nil {
		return fmt.Errorf("CreatePseudoConsole(in=%d out=%d): %w",
			in, outWrite, err)
	}
	windows.CloseHandle(in)
	windows.CloseHandle(outWrite)

	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return fmt.Errorf("attribute list: %w", err)
	}
	defer attrs.Delete()
	if err := attrs.Update(pseudoConsoleAttr, unsafe.Pointer(&console), unsafe.Sizeof(console)); err != nil {
		return fmt.Errorf("attach pseudoconsole: %w", err)
	}

	si := &windows.StartupInfoEx{
		StartupInfo: windows.StartupInfo{
			Cb:         uint32(unsafe.Sizeof(windows.StartupInfoEx{})),
			ShowWindow: windows.SW_HIDE,
			Flags:      windows.STARTF_USESHOWWINDOW,
		},
		ProcThreadAttributeList: attrs.List(),
	}

	wideExe, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return fmt.Errorf("exe: %w", err)
	}
	line, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(args))
	if err != nil {
		return fmt.Errorf("command line: %w", err)
	}

	var env *uint16
	if v.env {
		env = envBlock(os.Environ())
	}

	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT)
	pi := new(windows.ProcessInformation)
	if err := windows.CreateProcess(wideExe, line, nil, nil, v.inherit, flags, env, nil, &si.StartupInfo, pi); err != nil {
		return fmt.Errorf("CreateProcess: %w", err)
	}
	defer func() {
		windows.TerminateProcess(pi.Process, 1)
		windows.WaitForSingleObject(pi.Process, 2000)
		windows.CloseHandle(pi.Thread)
		windows.CloseHandle(pi.Process)
		windows.ClosePseudoConsole(console)
		windows.CloseHandle(out)
		windows.CloseHandle(inWrite)
	}()

	conhostBefore := countConhost()
	fmt.Printf("  %-16s pid=%d conhost=%d\n", v.name, pi.ProcessId, conhostBefore)

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if conhost := countConhost(); conhost != conhostBefore {
					fmt.Printf("  %-16s conhost %d -> %d\n", v.name, conhostBefore, conhost)
					conhostBefore = conhost
				}
			case <-stop:
				return
			}
		}
	}()

	return collect(v.name, pi.Process, out, wait)
}

// runPipes is the fallback candidate: no pseudoconsole, the child gets
// inheritable pipes instead. Programs that speak ANSI over stdio work; the
// question is whether this environment cooperates at all.
func runPipes(v variant, exe string, args []string, wait time.Duration) error {
	sa := windows.SecurityAttributes{
		Length:        uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		InheritHandle: 1,
	}

	var in, inWrite, out, outWrite windows.Handle
	if err := windows.CreatePipe(&in, &inWrite, &sa, 0); err != nil {
		return fmt.Errorf("input pipe: %w", err)
	}
	if err := windows.CreatePipe(&out, &outWrite, &sa, 0); err != nil {
		return fmt.Errorf("output pipe: %w", err)
	}
	defer windows.CloseHandle(inWrite)
	defer windows.CloseHandle(out)

	wideExe, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return fmt.Errorf("exe: %w", err)
	}
	line, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(args))
	if err != nil {
		return fmt.Errorf("command line: %w", err)
	}

	var env *uint16
	if v.env {
		env = envBlock(os.Environ())
	}

	si := &windows.StartupInfo{
		Cb:         uint32(unsafe.Sizeof(windows.StartupInfo{})),
		Flags:      windows.STARTF_USESTDHANDLES | windows.STARTF_USESHOWWINDOW,
		ShowWindow: windows.SW_HIDE,
		StdInput:   in,
		StdOutput:  outWrite,
		StdErr:     outWrite,
	}

	pi := new(windows.ProcessInformation)
	flags := uint32(windows.CREATE_UNICODE_ENVIRONMENT)
	if err := windows.CreateProcess(wideExe, line, nil, nil, true, flags, env, nil, si, pi); err != nil {
		return fmt.Errorf("CreateProcess: %w", err)
	}

	// The parent keeps its ends out of the child's handle table, so the read
	// side sees EOF when the child is done instead of hanging forever.
	windows.SetHandleInformation(in, windows.HANDLE_FLAG_INHERIT, 0)
	windows.SetHandleInformation(out, windows.HANDLE_FLAG_INHERIT, 0)

	defer func() {
		windows.TerminateProcess(pi.Process, 1)
		windows.WaitForSingleObject(pi.Process, 2000)
		windows.CloseHandle(pi.Thread)
		windows.CloseHandle(pi.Process)
	}()

	fmt.Printf("  %-16s pid=%d (pipes, no pseudoconsole)\n", v.name, pi.ProcessId)

	if err := windows.WriteFile(inWrite, []byte("echo hello-from-pipes\r\n"), nil, nil); err != nil {
		fmt.Printf("  %-16s write error: %v\n", v.name, err)
	}

	done := make(chan error, 1)
	go func() { done <- collect(v.name, pi.Process, out, wait) }()

	windows.WriteFile(inWrite, []byte("exit\r\n"), nil, nil)
	return <-done
}

// runPlain is the control: the same child, the same environment block, no
// pseudoconsole and no redirection at all. It answers one question only: does
// this environment start the program in the first place? Pair it with a
// command that exits without touching stdio, so the exit code is the verdict.
func runPlain(v variant, exe string, args []string, wait time.Duration) error {
	wideExe, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return fmt.Errorf("exe: %w", err)
	}
	line, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(args))
	if err != nil {
		return fmt.Errorf("command line: %w", err)
	}

	var env *uint16
	if v.env {
		env = envBlock(os.Environ())
	}

	si := &windows.StartupInfo{
		Cb:         uint32(unsafe.Sizeof(windows.StartupInfo{})),
		Flags:      windows.STARTF_USESHOWWINDOW,
		ShowWindow: windows.SW_HIDE,
	}

	pi := new(windows.ProcessInformation)
	flags := uint32(windows.CREATE_UNICODE_ENVIRONMENT)
	if err := windows.CreateProcess(wideExe, line, nil, nil, false, flags, env, nil, si, pi); err != nil {
		return fmt.Errorf("CreateProcess: %w", err)
	}
	defer func() {
		windows.TerminateProcess(pi.Process, 1)
		windows.WaitForSingleObject(pi.Process, 2000)
		windows.CloseHandle(pi.Thread)
		windows.CloseHandle(pi.Process)
	}()

	fmt.Printf("  %-16s pid=%d (no pseudoconsole)\n", v.name, pi.ProcessId)

	var code uint32
	event, _ := windows.WaitForSingleObject(pi.Process, uint32(wait.Milliseconds()))
	if err := windows.GetExitCodeProcess(pi.Process, &code); err != nil {
		return fmt.Errorf("exit code: %w", err)
	}

	alive := event != uint32(windows.WAIT_OBJECT_0)
	fmt.Printf("  %-16s alive=%v exit=0x%08X\n", v.name, alive, code)
	if code == 0xC0000142 {
		return fmt.Errorf("child died at DLL init (STATUS_DLL_INIT_FAILED)")
	}
	return nil
}

// collect reads a child's output until it goes quiet, then reports how much
// arrived and what the child exited with.
func collect(name string, process windows.Handle, out windows.Handle, wait time.Duration) error {
	file := os.NewFile(uintptr(out), "child-out")
	reads := make(chan string, 256)
	readErr := make(chan error, 1)
	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := file.Read(buffer)
			if n > 0 {
				reads <- string(buffer[:n])
			}
			if err != nil {
				readErr <- err
				return
			}
		}
	}()

	timer := time.NewTimer(wait)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer timer.Stop()
	defer ticker.Stop()

	total := 0
	exited := false
	for {
		select {
		case data := <-reads:
			total += len(data)
			fmt.Printf("  %-16s READ %d: %q\n", name, len(data), data)
		case err := <-readErr:
			fmt.Printf("  %-16s read error: %v\n", name, err)
		case <-ticker.C:
			if event, _ := windows.WaitForSingleObject(process, 0); event == windows.WAIT_OBJECT_0 {
				exited = true
			}
		case <-timer.C:
			var code uint32
			if err := windows.GetExitCodeProcess(process, &code); err != nil {
				fmt.Printf("  %-16s exit code unavailable: %v\n", name, err)
				code = stillActive
			}
			alive := !exited && code == stillActive
			fmt.Printf("  %-16s total=%d bytes alive=%v exit=0x%08X\n",
				name, total, alive, code)
			if total == 0 {
				return fmt.Errorf("no output")
			}
			return nil
		}
	}
}

// countConhost counts running conhost.exe instances. ConPTY spawns one per
// pseudoconsole, so a count that does not move means the console host never
// started and no output will ever arrive.
func countConhost() int {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return -1
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return 0
	}

	count := 0
	for {
		if windows.UTF16ToString(entry.ExeFile[:]) == "conhost.exe" {
			count++
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			return count
		}
	}
}

func envBlock(entries []string) *uint16 {
	block := make([]uint16, 0, len(entries)*32)
	for _, entry := range entries {
		block = append(block, utf16.Encode([]rune(entry))...)
		block = append(block, 0)
	}
	block = append(block, 0)
	return &block[0]
}

func splitComma(s string) []string {
	var out []string
	current := ""
	for _, r := range s {
		if r == ',' {
			out = append(out, current)
			current = ""
			continue
		}
		current += string(r)
	}
	return append(out, current)
}
