package main

import (
	"fmt"
	"os"
	"time"

	"github.com/dedomorozoff/tuide/internal/sys"
)

func main() {
	command := os.Args[1:]
	if len(command) == 0 {
		fmt.Println("default shell:", sys.DefaultShell())
		return
	}
	fmt.Println("command:", command)

	terminal, err := sys.StartPTY(sys.PTYOptions{Command: command, Width: 80, Height: 24})
	if err != nil {
		fmt.Println("start error:", err)
		return
	}

	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := terminal.Read(buffer)
			if n > 0 {
				fmt.Printf("READ %d: % x\n", n, buffer[:n])
			}
			if err != nil {
				fmt.Println("read error:", err)
				return
			}
		}
	}()

	time.Sleep(500 * time.Millisecond)
	if _, err := terminal.Write([]byte("echo tuide-pty-ok\r\n")); err != nil {
		fmt.Println("write error:", err)
	}

	time.Sleep(4 * time.Second)
	fmt.Println("closing")
	terminal.Close()
	fmt.Println("closed")
	os.Exit(0)
}
