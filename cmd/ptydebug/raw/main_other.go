//go:build !windows

package main

import (
	"fmt"
	"os"
)

// The sweep it runs is a Windows pseudoconsole experiment, so on any other
// platform the command only says so instead of failing to build.
func main() {
	fmt.Fprintln(os.Stderr, "ptydebug/raw: the pseudoconsole sweep is Windows only")
	os.Exit(2)
}
