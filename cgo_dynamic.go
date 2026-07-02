//go:build !static

package main

// Dynamic build (default): resolve libwoff2enc, harfbuzz-subset and brotli via
// pkg-config, linking against the system shared libraries. Used for local
// development and testing. The static build (Docker) uses cgo_static.go.

// #cgo pkg-config: libwoff2enc harfbuzz-subset
import "C"
