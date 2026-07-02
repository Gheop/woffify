package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func sortedRunes(r []rune) []rune {
	sort.Slice(r, func(i, j int) bool { return r[i] < r[j] })
	return r
}

func TestExtractCSSCodepoints(t *testing.T) {
	css := `
.fa-house::before { content: "\f015"; }
.fa-user::before  { content: '\f007'; }
.emoji::after     { content: "\1F600"; }
.plain            { color: #fff; }
.text::before     { content: "Chapter "; }
`
	got := sortedRunes(extractCSSCodepoints(css))
	want := sortedRunes([]rune{0xF015, 0xF007, 0x1F600})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestScanCodepoints(t *testing.T) {
	dir := t.TempDir()
	css := `.i::before { content: "\f015"; } .j::before { content: "\f015"; }`
	if err := os.WriteFile(filepath.Join(dir, "icons.css"), []byte(css), 0o644); err != nil {
		t.Fatal(err)
	}
	// A non-CSS file must be ignored.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte(`\f099`), 0o644); err != nil {
		t.Fatal(err)
	}

	cps, origin, err := scanCodepoints([]string{dir}, "css")
	if err != nil {
		t.Fatalf("scanCodepoints: %v", err)
	}
	if len(cps) != 1 || cps[0] != 0xF015 { // deduplicated, .txt ignored
		t.Errorf("got %v, want [U+F015]", cps)
	}
	if !filepath.IsAbs(origin[0xF015]) && origin[0xF015] == "" {
		t.Error("origin should point at the source file")
	}

	if _, _, err := scanCodepoints([]string{dir}, "text"); err == nil {
		t.Error("unsupported mode should error")
	}
}
