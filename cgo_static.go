//go:build static

package main

// Static build (Docker): link against static archives of a minimal harfbuzz
// (subset only, no freetype/glib/graphite), woff2 and brotli, all built from
// source in the image. --start-group resolves the cross-references between the
// archives. Produces a fully static binary with no dynamic dependencies.

/*
#cgo CFLAGS: -I/usr/include/harfbuzz -I/usr/include
#cgo CXXFLAGS: -I/usr/include/harfbuzz -I/usr/include -std=c++11
#cgo LDFLAGS: -Wl,--start-group -lwoff2enc -lharfbuzz-subset -lharfbuzz -lbrotlienc -lbrotlicommon -Wl,--end-group -lstdc++ -lm
*/
import "C"
