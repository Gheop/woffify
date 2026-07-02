//go:build linux

package main

import (
	"os"
	"syscall"
)

// muteCStderr redirects the process-level stderr file descriptor to /dev/null,
// so the C libraries stay silent (the woff2 encoder prints a line per call to
// stderr), and returns an *os.File wrapping the original stderr for woffify's
// own diagnostics. Done once before starting workers, so it is race-free.
func muteCStderr() *os.File {
	saved, err := syscall.Dup(int(os.Stderr.Fd()))
	if err != nil {
		return os.Stderr
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		syscall.Close(saved)
		return os.Stderr
	}
	if err := syscall.Dup3(int(devNull.Fd()), int(os.Stderr.Fd()), 0); err != nil {
		devNull.Close()
		syscall.Close(saved)
		return os.Stderr
	}
	devNull.Close()
	return os.NewFile(uintptr(saved), "stderr")
}
