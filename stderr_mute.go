//go:build linux

package main

import (
	"os"
	"syscall"
)

// quietCStderr mutes the C libraries' stderr only when they are noisy. The
// release build compiles woff2 without FONT_COMPRESSION_BIN, so it prints
// nothing, and keeping fd 2 open lets crash reports (Go fatal errors, C aborts)
// reach the caller. System woff2 packages built with CMake's default
// NOISY_LOGGING print a line per font, hence the mute in dynamic builds.
func quietCStderr() *os.File {
	if !noisyCLibs {
		return os.Stderr
	}
	return muteCStderr()
}

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
