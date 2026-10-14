package main

import (
	"fmt"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const pseudoConsoleAttr = 0x00020016

func main() {
	fmt.Println("variant:", os.Args[1])

	var in, inW, out, outW windows.Handle
	if err := windows.CreatePipe(&in, &inW, nil, 0); err != nil {
		panic(err)
	}
	if err := windows.CreatePipe(&out, &outW, nil, 0); err != nil {
		panic(err)
	}

	var console windows.Handle
	size := windows.Coord{X: 80, Y: 24}
	if err := windows.CreatePseudoConsole(size, in, outW, 0, &console); err != nil {
		panic(err)
	}
	windows.CloseHandle(in)
	windows.CloseHandle(outW)

	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		panic(err)
	}
	defer attrs.Delete()
	if err := attrs.Update(pseudoConsoleAttr, unsafe.Pointer(&console), unsafe.Sizeof(console)); err != nil {
		panic(err)
	}

	si := &windows.StartupInfoEx{
		StartupInfo: windows.StartupInfo{
			Cb:         uint32(unsafe.Sizeof(windows.StartupInfoEx{})),
			ShowWindow: windows.SW_HIDE,
			Flags:      windows.STARTF_USESHOWWINDOW,
		},
		ProcThreadAttributeList: attrs.List(),
	}

	exe, err := windows.UTF16PtrFromString(os.Args[2])
	if err != nil {
		panic(err)
	}
	line, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(os.Args[3:]))
	if err != nil {
		panic(err)
	}

	var env *uint16
	inherit := true
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_DEFAULT_ERROR_MODE)
	if os.Args[1] == "nilenv" {
		env = nil
	} else {
		env = makeEnvBlock(os.Environ())
		inherit = false
		flags |= 0
	}

	pi := new(windows.ProcessInformation)
	if err := windows.CreateProcess(exe, line, nil, nil, inherit, flags, env, nil, &si.StartupInfo, pi); err != nil {
		panic(err)
	}
	fmt.Println("pid:", pi.ProcessId)

	go func() {
		buffer := make([]byte, 4096)
		file := os.NewFile(uintptr(out), "out")
		for {
			n, err := file.Read(buffer)
			if n > 0 {
				fmt.Printf("READ %d: %q\n", n, string(buffer[:n]))
			}
			if err != nil {
				fmt.Println("read error:", err)
				return
			}
		}
	}()

	time.Sleep(3 * time.Second)
	windows.ClosePseudoConsole(console)
	fmt.Println("done")
}

func makeEnvBlock(envv []string) *uint16 {
	block := make([]uint16, 0, len(envv)*32)
	for _, entry := range envv {
		for _, r := range entry {
			block = append(block, uint16(r))
		}
		block = append(block, 0)
	}
	block = append(block, 0)
	return &block[0]
}
