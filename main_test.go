package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOutputPath(t *testing.T) {
	tests := []struct{ in, outDir, want string }{
		{"a/Font.ttf", "", filepath.Join("a", "Font.woff2")},
		{"a/Font.ttf", "dist", filepath.Join("dist", "Font.woff2")},
		{"Font.woff", "", "Font.woff2"},
		{"x/y/Cool-Regular.otf", "out", filepath.Join("out", "Cool-Regular.woff2")},
	}
	for _, tt := range tests {
		if got := outputPath(tt.in, tt.outDir); got != tt.want {
			t.Errorf("outputPath(%q,%q) = %q, want %q", tt.in, tt.outDir, got, tt.want)
		}
	}
}

func TestCollect(t *testing.T) {
	dir := t.TempDir()
	write := func(p string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "a.ttf"))
	write(filepath.Join(dir, "b.woff"))
	write(filepath.Join(dir, "note.txt")) // ignored (not a font extension)
	write(filepath.Join(dir, "sub", "c.otf"))

	got, err := collect([]string{dir}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("non-recursive: got %d files, want 2 (%v)", len(got), got)
	}

	got, err = collect([]string{dir}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Errorf("recursive: got %d files, want 3 (%v)", len(got), got)
	}

	if _, err := collect([]string{filepath.Join(dir, "nope.ttf")}, false); err == nil {
		t.Error("missing path should error")
	}
}

// TestRunOutputCollision verifies that two inputs mapping to the same output are
// skipped with a failure, the unique input is still converted, and no temp file
// is left behind (atomic write).
func TestRunOutputCollision(t *testing.T) {
	woff, err := os.ReadFile("testdata/DejaVuSerif.woff")
	if err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	for _, d := range []string{"a", "b"} {
		p := filepath.Join(src, d, "Font.woff")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, woff, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(src, "Unique.woff"), woff, 0o644); err != nil {
		t.Fatal(err)
	}

	inputs, err := collect([]string{src}, true)
	if err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()
	errFile, err := os.CreateTemp(t.TempDir(), "err")
	if err != nil {
		t.Fatal(err)
	}
	defer errFile.Close()

	failed := run(inputs, out, 2, true, subsetOptions{}, errFile)

	if failed != 2 {
		t.Errorf("expected 2 failures for the colliding group, got %d", failed)
	}
	if _, err := os.Stat(filepath.Join(out, "Unique.woff2")); err != nil {
		t.Error("Unique.woff2 should be produced")
	}
	if _, err := os.Stat(filepath.Join(out, "Font.woff2")); err == nil {
		t.Error("Font.woff2 should be skipped (collision), not written")
	}
	// No leftover temp files from the atomic write.
	entries, _ := os.ReadDir(out)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}
