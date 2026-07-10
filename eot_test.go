package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

// makeEOT wraps an SFNT in a minimal uncompressed EOT container.
func makeEOT(sfnt []byte, flags uint32) []byte {
	header := make([]byte, 82)
	binary.LittleEndian.PutUint32(header[4:], uint32(len(sfnt))) // FontDataSize
	binary.LittleEndian.PutUint32(header[8:], 0x00020001)        // Version
	binary.LittleEndian.PutUint32(header[12:], flags)            // Flags
	binary.LittleEndian.PutUint16(header[34:], eotMagic)         // MagicNumber
	eot := append(header, sfnt...)
	binary.LittleEndian.PutUint32(eot, uint32(len(eot))) // EOTSize
	return eot
}

func TestDecodeEOT(t *testing.T) {
	woff, err := os.ReadFile("testdata/DejaVuSerif.woff")
	if err != nil {
		t.Fatal(err)
	}
	sfnt, _, err := decodeWOFF(woff)
	if err != nil {
		t.Fatal(err)
	}

	eot := makeEOT(sfnt, 0)
	if !isEOT(eot) {
		t.Fatal("isEOT should be true for a synthetic EOT")
	}
	got, err := decodeEOT(eot)
	if err != nil {
		t.Fatalf("decodeEOT: %v", err)
	}
	if !bytes.Equal(got, sfnt) {
		t.Error("extracted SFNT does not match the original")
	}

	// Compressed EOT (MicroType Express) must be rejected with an error.
	if _, err := decodeEOT(makeEOT(sfnt, eotCompressed)); err == nil {
		t.Error("compressed EOT should be rejected")
	}
	// A plain TTF must not be mistaken for an EOT.
	if isEOT(sfnt) {
		t.Error("a TTF should not look like EOT")
	}
}
