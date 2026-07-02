package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"
	"sort"
)

// woffSignature is the magic number of a WOFF 1.0 file ("wOFF").
const woffSignature = 0x774F4646

// decodeWOFF rebuilds the original SFNT (TTF/OTF) bytes from a WOFF 1.0 file.
// WOFF 1.0 is just a container: each SFNT table is stored either raw or
// zlib-compressed, so the reconstruction is lossless. The second return value
// is true for OpenType/CFF fonts (flavor "OTTO").
//
// Reference: https://www.w3.org/TR/WOFF/ (sections 3 and 4).
func decodeWOFF(b []byte) (sfnt []byte, isCFF bool, err error) {
	if len(b) < 44 {
		return nil, false, fmt.Errorf("file too short for a WOFF header")
	}
	if binary.BigEndian.Uint32(b[0:]) != woffSignature {
		return nil, false, fmt.Errorf("missing WOFF signature (not a .woff file)")
	}
	flavor := binary.BigEndian.Uint32(b[4:])
	numTables := int(binary.BigEndian.Uint16(b[12:]))
	isCFF = flavor == 0x4F54544F // "OTTO"

	// Read the table directory (20 bytes per entry, right after the header).
	type entry struct {
		tag                             uint32
		offset, compLen, origLen, cksum uint32
	}
	dir := make([]entry, numTables)
	pos := 44
	for i := range dir {
		if pos+20 > len(b) {
			return nil, false, fmt.Errorf("truncated table directory (table %d/%d)", i, numTables)
		}
		dir[i] = entry{
			tag:     binary.BigEndian.Uint32(b[pos:]),
			offset:  binary.BigEndian.Uint32(b[pos+4:]),
			compLen: binary.BigEndian.Uint32(b[pos+8:]),
			origLen: binary.BigEndian.Uint32(b[pos+12:]),
			cksum:   binary.BigEndian.Uint32(b[pos+16:]),
		}
		pos += 20
	}

	// A valid SFNT requires the table records sorted by tag.
	sort.Slice(dir, func(i, j int) bool { return dir[i].tag < dir[j].tag })

	// Decompress tables and compute 4-byte-aligned offsets.
	tables := make([][]byte, numTables)
	sfntOffset := 12 + 16*numTables // SFNT header + table records
	offsets := make([]uint32, numTables)
	for i, e := range dir {
		if int(e.offset)+int(e.compLen) > len(b) {
			return nil, false, fmt.Errorf("table data out of bounds (tag %s)", tagString(e.tag))
		}
		raw := b[e.offset : e.offset+e.compLen]
		var data []byte
		if e.compLen < e.origLen {
			// Table is zlib-compressed.
			data, err = zlibInflate(raw, e.origLen)
			if err != nil {
				return nil, false, fmt.Errorf("decompressing table %s: %w", tagString(e.tag), err)
			}
		} else {
			// Stored uncompressed (compLen == origLen).
			data = raw
		}
		if uint32(len(data)) != e.origLen {
			return nil, false, fmt.Errorf("table %s: got %d bytes, want %d",
				tagString(e.tag), len(data), e.origLen)
		}
		tables[i] = data
		offsets[i] = uint32(sfntOffset)
		sfntOffset += align4(len(data))
	}

	// Write the SFNT header (offset table).
	out := &bytes.Buffer{}
	out.Grow(sfntOffset)
	searchRange, entrySelector, rangeShift := sfntSearchParams(numTables)
	writeU32(out, flavor)
	writeU16(out, uint16(numTables))
	writeU16(out, searchRange)
	writeU16(out, entrySelector)
	writeU16(out, rangeShift)

	// Table records.
	for i, e := range dir {
		writeU32(out, e.tag)
		writeU32(out, e.cksum)
		writeU32(out, offsets[i])
		writeU32(out, e.origLen)
	}

	// Table data, each padded with zeros to a 4-byte boundary.
	for _, data := range tables {
		out.Write(data)
		for pad := align4(len(data)) - len(data); pad > 0; pad-- {
			out.WriteByte(0)
		}
	}

	return out.Bytes(), isCFF, nil
}

// zlibInflate decompresses a zlib stream of known decompressed size.
func zlibInflate(src []byte, origLen uint32) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	buf := bytes.NewBuffer(make([]byte, 0, origLen))
	if _, err := io.Copy(buf, r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// sfntSearchParams computes searchRange/entrySelector/rangeShift for the SFNT header.
func sfntSearchParams(numTables int) (searchRange, entrySelector, rangeShift uint16) {
	if numTables == 0 {
		return 0, 0, 0
	}
	exp := bits.Len(uint(numTables)) - 1 // floor(log2(numTables))
	pow2 := 1 << exp
	searchRange = uint16(pow2 * 16)
	entrySelector = uint16(exp)
	rangeShift = uint16(numTables*16) - searchRange
	return
}

func align4(n int) int { return (n + 3) &^ 3 }

func writeU16(w io.Writer, v uint16) {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], v)
	w.Write(b[:])
}

func writeU32(w io.Writer, v uint32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	w.Write(b[:])
}

func tagString(tag uint32) string {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], tag)
	for i, c := range b {
		if c < 0x20 || c > 0x7e {
			b[i] = '.'
		}
	}
	return string(b[:])
}
