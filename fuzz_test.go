package main

import (
	"os"
	"testing"
)

// FuzzDecodeWOFF checks that the WOFF decoder never panics on arbitrary input.
func FuzzDecodeWOFF(f *testing.F) {
	if data, err := os.ReadFile("testdata/DejaVuSerif.woff"); err == nil {
		f.Add(data)
	}
	f.Add([]byte("wOFF"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _, _ = decodeWOFF(b)
	})
}

// FuzzToSFNT checks the full input dispatch (WOFF, EOT, TTC, SFNT) never panics.
func FuzzToSFNT(f *testing.F) {
	if data, err := os.ReadFile("testdata/DejaVuSerif.woff"); err == nil {
		f.Add(data)
	}
	f.Add(append([]byte("ttcf"), make([]byte, 40)...))
	f.Add(append([]byte{0, 1, 0, 0}, make([]byte, 40)...))
	f.Fuzz(func(t *testing.T, b []byte) {
		if isEOT(b) {
			_, _ = decodeEOT(b)
		}
		_, _ = toSFNT(b)
	})
}
