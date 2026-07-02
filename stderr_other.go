//go:build !linux

package main

import "os"

// muteCStderr is a no-op on non-Linux platforms.
func muteCStderr() *os.File { return os.Stderr }
