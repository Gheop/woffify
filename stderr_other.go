//go:build !linux

package main

import "os"

// quietCStderr is a no-op on non-Linux platforms.
func quietCStderr() *os.File { return os.Stderr }
