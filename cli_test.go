package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain lets the test binary act as the woffify command when
// WOFFIFY_TEST_MAIN=1, so the CLI tests below run the real main(): flag
// parsing, exit codes, and messages written past the muted C stderr.
func TestMain(m *testing.M) {
	if os.Getenv("WOFFIFY_TEST_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runCLI runs woffify with args and stdin, and returns its exit code, stdout
// and stderr.
func runCLI(t *testing.T, stdin []byte, args ...string) (int, []byte, string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "WOFFIFY_TEST_MAIN=1")
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run woffify: %v", err)
	}
	return code, stdout.Bytes(), stderr.String()
}

func TestCLI(t *testing.T) {
	font, err := os.ReadFile("testdata/DejaVuSerif.woff")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	junk := filepath.Join(dir, "junk.ttf")
	if err := os.WriteFile(junk, []byte("not a font at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	emptyScan := t.TempDir()
	if err := os.WriteFile(filepath.Join(emptyScan, "a.css"), []byte("p { color: red }"), 0o644); err != nil {
		t.Fatal(err)
	}
	ttc := append([]byte("ttcf"), make([]byte, 60)...)
	// A small subset keeps the Brotli step fast.
	subset := []string{"-subset-unicodes", "41-5A"}

	tests := []struct {
		name     string
		stdin    []byte
		args     []string
		code     int
		inStderr string
	}{
		{"no arguments", nil, nil, 2, "usage: woffify"},
		{"hint flag without subset", nil, []string{"-drop-hints", "x.ttf"}, 2, "require a subset"},
		{"code point beyond Unicode", nil, []string{"-subset-unicodes", "110000", "x.ttf"}, 2, "beyond U+10FFFF"},
		{"scan finds nothing", nil, []string{"-subset-scan", emptyScan, "x.ttf"}, 2, "found no code points"},
		{"missing input", nil, []string{filepath.Join(dir, "nope.ttf")}, 1, "nope.ttf"},
		{"invalid font", nil, []string{"-o", dir, junk}, 1, "1/1 conversion(s) failed"},
		{"collection on stdin", ttc, []string{"-"}, 1, errCollection.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, stderr := runCLI(t, tt.stdin, tt.args...)
			if code != tt.code {
				t.Errorf("exit code %d, want %d (stderr: %s)", code, tt.code, stderr)
			}
			if !strings.Contains(stderr, tt.inStderr) {
				t.Errorf("stderr %q does not contain %q", stderr, tt.inStderr)
			}
		})
	}

	t.Run("convert a file", func(t *testing.T) {
		src := filepath.Join(dir, "Serif.woff")
		if err := os.WriteFile(src, font, 0o644); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(dir, "out")
		code, stdout, stderr := runCLI(t, nil, append(subset, "-o", out, src)...)
		if code != 0 {
			t.Fatalf("exit code %d, stderr: %s", code, stderr)
		}
		if !strings.Contains(string(stdout), "Serif.woff2") {
			t.Errorf("stdout %q lacks the size report", stdout)
		}
		b, err := os.ReadFile(filepath.Join(out, "Serif.woff2"))
		if err != nil || !bytes.HasPrefix(b, []byte("wOF2")) {
			t.Errorf("output is not a WOFF2 file (err %v)", err)
		}
	})

	t.Run("pipe mode", func(t *testing.T) {
		code, stdout, stderr := runCLI(t, font, append(subset, "-")...)
		if code != 0 {
			t.Fatalf("exit code %d, stderr: %s", code, stderr)
		}
		if !bytes.HasPrefix(stdout, []byte("wOF2")) {
			t.Error("stdout is not a WOFF2 file")
		}
		if stderr != "" {
			t.Errorf("stderr should stay silent in pipe mode, got %q", stderr)
		}
	})
}
