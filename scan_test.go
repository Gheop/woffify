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

func runeSet(r []rune) map[rune]bool {
	m := make(map[rune]bool, len(r))
	for _, x := range r {
		m[x] = true
	}
	return m
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

func TestExtractTextCodepoints(t *testing.T) {
	doc := `<div class="ZZZ">Café &#8594;</div><!-- Ω --><span>5&nbsp;&euro;</span>`
	set := runeSet(extractTextCodepoints(doc))

	// Visible text and decoded entities are kept.
	for _, r := range []rune{'C', 'a', 'f', 'é', '→', '5', '€'} {
		if !set[r] {
			t.Errorf("want %q (U+%04X) in text code points", r, r)
		}
	}
	// Attribute value ('Z') and comment content ('Ω') are stripped with the markup.
	for _, r := range []rune{'Z', 0x03A9} {
		if set[r] {
			t.Errorf("did not expect %q (U+%04X)", r, r)
		}
	}
}

func TestScanCodepoints(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("icons.css", `.i::before { content: "\f015"; }`)
	write("page.html", `<p>Zoé</p>`)
	write("data.bin", `\f099 ignored`) // unknown extension, skipped

	// auto: CSS escapes + HTML text, unknown extension ignored.
	cps, origin, err := scanCodepoints([]string{dir}, "auto")
	if err != nil {
		t.Fatalf("scanCodepoints auto: %v", err)
	}
	set := runeSet(cps)
	for _, r := range []rune{0xF015, 'Z', 'o', 'é'} {
		if !set[r] {
			t.Errorf("auto: want U+%04X", r)
		}
	}
	if origin[0xF015] == "" {
		t.Error("origin should point at the source file")
	}

	// css mode scans only .css (no HTML text).
	cssSet := runeSet(mustScan(t, dir, "css"))
	if !cssSet[0xF015] || cssSet['Z'] {
		t.Error("css mode should scan only .css files")
	}

	// text mode scans only markup (no CSS escapes).
	textSet := runeSet(mustScan(t, dir, "text"))
	if !textSet['Z'] || textSet[0xF015] {
		t.Error("text mode should scan only markup files")
	}

	if _, _, err := scanCodepoints([]string{dir}, "xyz"); err == nil {
		t.Error("invalid mode should error")
	}
}

func mustScan(t *testing.T, dir, mode string) []rune {
	t.Helper()
	cps, _, err := scanCodepoints([]string{dir}, mode)
	if err != nil {
		t.Fatalf("scanCodepoints %s: %v", mode, err)
	}
	return cps
}
