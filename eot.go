package main

import (
	"encoding/binary"
	"fmt"
)

// eotMagic is the EOT magic number, stored little-endian at offset 34.
const eotMagic = 0x504C

// EOT header flags (a subset of the TTEMBED_* constants).
const (
	eotCompressed = 0x00000004 // TTEMBED_TTCOMPRESSED: MicroType Express
	eotXOR        = 0x10000000 // TTEMBED_XORENCRYPTDATA
)

// isEOT reports whether data looks like an EOT (Embedded OpenType) file. EOT has
// no leading magic, so we key off the magic number at offset 34 plus a known
// header version.
func isEOT(data []byte) bool {
	if len(data) < 36 {
		return false
	}
	if binary.LittleEndian.Uint16(data[34:]) != eotMagic {
		return false
	}
	switch binary.LittleEndian.Uint32(data[8:]) { // Version
	case 0x00010000, 0x00020001, 0x00020002:
		return true
	}
	return false
}

// decodeEOT extracts the SFNT (TTF/OTF) bytes embedded in an EOT file. EOT is a
// container: a header followed by the font data, which is the trailing
// FontDataSize bytes. Only uncompressed EOT is supported — the case every modern
// generator emits; MicroType Express compression is a dead Microsoft format and
// is rejected with a clear message.
func decodeEOT(data []byte) ([]byte, error) {
	if len(data) < 36 {
		return nil, fmt.Errorf("file too short for an EOT header")
	}
	flags := binary.LittleEndian.Uint32(data[12:])
	if flags&eotCompressed != 0 {
		return nil, fmt.Errorf("EOT MicroType Express compression not supported; regenerate from the original TTF/OTF")
	}
	if flags&eotXOR != 0 {
		return nil, fmt.Errorf("XOR-encrypted EOT not supported")
	}
	fontDataSize := binary.LittleEndian.Uint32(data[4:])
	if fontDataSize == 0 || int(fontDataSize) > len(data) {
		return nil, fmt.Errorf("EOT font data size out of range")
	}
	sfnt := data[len(data)-int(fontDataSize):]
	if !looksLikeSFNT(sfnt) {
		return nil, fmt.Errorf("EOT payload is not a valid SFNT font")
	}
	return sfnt, nil
}

// looksLikeSFNT checks the 4-byte SFNT version tag at the start of b.
func looksLikeSFNT(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	switch binary.BigEndian.Uint32(b) {
	case 0x00010000, 0x4F54544F, 0x74727565, 0x74746366: // 1.0, OTTO, "true", "ttcf"
		return true
	}
	return false
}
