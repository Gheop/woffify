package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"reflect"
	"testing"
)

func TestParseUnicodeRanges(t *testing.T) {
	tests := []struct {
		in   string
		want []unicodeRange
	}{
		{"0-FF", []unicodeRange{{0x00, 0xFF}}},
		{"20AC", []unicodeRange{{0x20AC, 0x20AC}}},
		{"U+0000-00FF,U+20AC", []unicodeRange{{0x00, 0xFF}, {0x20AC, 0x20AC}}},
		{"0-FF, 2000-206F ", []unicodeRange{{0x00, 0xFF}, {0x2000, 0x206F}}},
	}
	for _, tt := range tests {
		got, err := parseUnicodeRanges(tt.in)
		if err != nil {
			t.Errorf("parseUnicodeRanges(%q): %v", tt.in, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parseUnicodeRanges(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}

	for _, bad := range []string{"", "xyz", "FF-0", "0-", "-FF"} {
		if _, err := parseUnicodeRanges(bad); err == nil {
			t.Errorf("parseUnicodeRanges(%q): expected an error", bad)
		}
	}
}

func TestBuildSubsetOptions(t *testing.T) {
	if o, _ := buildSubsetOptions("", "", false, false); o.active() {
		t.Error("no flags should give an inactive subset")
	}
	if o, _ := buildSubsetOptions("0-FF", "", false, false); !o.active() {
		t.Error("unicodes should give an active subset")
	}
	o, err := buildSubsetOptions("", "AZ", false, false)
	if err != nil {
		t.Fatalf("text subset: %v", err)
	}
	if len(o.unicodes) != 2 {
		t.Errorf("text %q should yield 2 ranges, got %d", "AZ", len(o.unicodes))
	}
	if _, err := buildSubsetOptions("", "", true, false); err == nil {
		t.Error("-drop-hints without a subset set should error")
	}
}

// TestConvertStream checks the `woffify -` pipe path: font in, WOFF2 out.
func TestConvertStream(t *testing.T) {
	data, err := os.ReadFile("testdata/DejaVuSerif.woff")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var out bytes.Buffer
	if err := convertStream(bytes.NewReader(data), &out, subsetOptions{}); err != nil {
		t.Fatalf("convertStream: %v", err)
	}
	b := out.Bytes()
	if len(b) < 4 || binary.BigEndian.Uint32(b) != 0x774F4632 { // "wOF2"
		t.Fatal("stream output is not a WOFF2 file")
	}
}

// TestPipeline exercises the cgo path end to end: WOFF -> SFNT -> subset -> WOFF2.
func TestPipeline(t *testing.T) {
	data, err := os.ReadFile("testdata/DejaVuSerif.woff")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	sfnt, _, err := decodeWOFF(data)
	if err != nil {
		t.Fatalf("decodeWOFF: %v", err)
	}

	full, err := encodeWOFF2(sfnt)
	if err != nil {
		t.Fatalf("encodeWOFF2: %v", err)
	}
	if len(full) < 4 || binary.BigEndian.Uint32(full) != 0x774F4632 { // "wOF2"
		t.Fatal("output is not a WOFF2 file")
	}

	sub, err := subsetSFNT(sfnt, subsetOptions{unicodes: []unicodeRange{{0x41, 0x5A}}}) // A-Z
	if err != nil {
		t.Fatalf("subsetSFNT: %v", err)
	}
	if len(sub) >= len(sfnt) {
		t.Errorf("subset (%d) should be smaller than source SFNT (%d)", len(sub), len(sfnt))
	}
	subW, err := encodeWOFF2(sub)
	if err != nil {
		t.Fatalf("encodeWOFF2(subset): %v", err)
	}
	if len(subW) >= len(full) {
		t.Errorf("subset WOFF2 (%d) should be smaller than full WOFF2 (%d)", len(subW), len(full))
	}
}
