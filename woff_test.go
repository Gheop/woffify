package main

import (
	"encoding/binary"
	"os"
	"testing"
)

// headMagic is the magic number at offset 12 of a font's "head" table. Finding
// it after decoding proves the zlib inflate and offset reconstruction are right.
const headMagic = 0x5F0F3CF5

func TestDecodeWOFF(t *testing.T) {
	data, err := os.ReadFile("testdata/DejaVuSerif.woff")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	sfnt, isCFF, err := decodeWOFF(data)
	if err != nil {
		t.Fatalf("decodeWOFF: %v", err)
	}
	if isCFF {
		t.Errorf("DejaVuSerif is a glyf font, isCFF should be false")
	}

	tables := parseSFNTTables(t, sfnt)

	head, ok := tables["head"]
	if !ok {
		t.Fatal("head table missing from reconstructed SFNT")
	}
	if len(head) < 16 {
		t.Fatalf("head table too short: %d bytes", len(head))
	}
	if got := binary.BigEndian.Uint32(head[12:]); got != headMagic {
		t.Errorf("head magicNumber = %#x, want %#x", got, headMagic)
	}

	for _, tag := range []string{"glyf", "loca", "cmap", "hmtx"} {
		if _, ok := tables[tag]; !ok {
			t.Errorf("expected table %q missing", tag)
		}
	}
}

// parseSFNTTables reads the SFNT header and returns each table's data. It also
// checks the invariants: known SFNT version, in-bounds and 4-byte-aligned
// offsets, tags in ascending order.
func parseSFNTTables(t *testing.T, b []byte) map[string][]byte {
	t.Helper()
	if len(b) < 12 {
		t.Fatal("SFNT too short")
	}
	switch v := binary.BigEndian.Uint32(b); v {
	case 0x00010000, 0x4F54544F, 0x74727565, 0x74746366: // 1.0, OTTO, true, ttcf
	default:
		t.Fatalf("unknown SFNT version: %#x", v)
	}

	numTables := int(binary.BigEndian.Uint16(b[4:]))
	tables := make(map[string][]byte, numTables)
	var prevTag uint32
	for i := 0; i < numTables; i++ {
		rec := 12 + 16*i
		if rec+16 > len(b) {
			t.Fatalf("table record %d out of bounds", i)
		}
		tag := binary.BigEndian.Uint32(b[rec:])
		offset := binary.BigEndian.Uint32(b[rec+8:])
		length := binary.BigEndian.Uint32(b[rec+12:])

		if i > 0 && tag <= prevTag {
			t.Errorf("tags not sorted: %#x after %#x", tag, prevTag)
		}
		prevTag = tag
		if offset%4 != 0 {
			t.Errorf("offset not 4-aligned for tag %#x: %d", tag, offset)
		}
		if int(offset)+int(length) > len(b) {
			t.Fatalf("table %#x out of bounds: offset=%d length=%d size=%d", tag, offset, length, len(b))
		}

		var name [4]byte
		binary.BigEndian.PutUint32(name[:], tag)
		tables[string(name[:])] = b[offset : offset+length]
	}
	return tables
}

// BenchmarkDecodeWOFF measures the isolated cost of WOFF-to-SFNT decoding, the
// only extra work woffify does compared to a plain woff2 encode.
func BenchmarkDecodeWOFF(b *testing.B) {
	data, err := os.ReadFile("testdata/DejaVuSerif.woff")
	if err != nil {
		b.Fatalf("read fixture: %v", err)
	}
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := decodeWOFF(data); err != nil {
			b.Fatal(err)
		}
	}
}

func TestDecodeWOFFRejectsNonWOFF(t *testing.T) {
	cases := map[string][]byte{
		"empty":         {},
		"too short":     {0x77, 0x4F},
		"TTF signature": append([]byte{0x00, 0x01, 0x00, 0x00}, make([]byte, 60)...),
	}
	for name, data := range cases {
		if _, _, err := decodeWOFF(data); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
