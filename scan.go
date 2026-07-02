package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// stringSlice is a repeatable string flag (e.g. -subset-scan a -subset-scan b).
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// cssContentDecl matches the value of a CSS `content` declaration.
var cssContentDecl = regexp.MustCompile(`(?i)content\s*:\s*([^;{}]*)`)

// cssEscape matches a CSS unicode escape: a backslash then 1-6 hex digits.
var cssEscape = regexp.MustCompile(`\\([0-9a-fA-F]{1,6})`)

// extractCSSCodepoints pulls the code points of `\fXXX`-style escapes found in
// `content` declarations. This is the icon-font case (Font Awesome, icomoon):
// each `content: "\f015"` maps to one glyph the page actually uses.
func extractCSSCodepoints(css string) []rune {
	var cps []rune
	for _, decl := range cssContentDecl.FindAllStringSubmatch(css, -1) {
		for _, esc := range cssEscape.FindAllStringSubmatch(decl[1], -1) {
			if v, err := strconv.ParseInt(esc[1], 16, 32); err == nil && v > 0 {
				cps = append(cps, rune(v))
			}
		}
	}
	return cps
}

// scanCodepoints walks the given files and directories, extracts code points
// from matching source files, and returns them in first-seen order along with
// the file each was first seen in (for -subset-scan-report).
func scanCodepoints(paths []string, mode string) ([]rune, map[rune]string, error) {
	if mode != "css" {
		return nil, nil, fmt.Errorf("unsupported -subset-scan-mode %q (only \"css\" for now)", mode)
	}

	origin := map[rune]string{}
	var order []rune
	scanFile := func(path string) error {
		if strings.ToLower(filepath.Ext(path)) != ".css" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, r := range extractCSSCodepoints(string(data)) {
			if _, ok := origin[r]; !ok {
				origin[r] = path
				order = append(order, r)
			}
		}
		return nil
	}

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, nil, err
		}
		if !info.IsDir() {
			if err := scanFile(p); err != nil {
				return nil, nil, err
			}
			continue
		}
		err = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			return scanFile(path)
		})
		if err != nil {
			return nil, nil, err
		}
	}
	return order, origin, nil
}

// reportScan prints the retained code points and their origin file to stderr.
func reportScan(cps []rune, origin map[rune]string) {
	fmt.Fprintf(os.Stderr, "woffify: -subset-scan kept %d code point(s):\n", len(cps))
	for _, r := range cps {
		fmt.Fprintf(os.Stderr, "  U+%04X  %s\n", r, origin[r])
	}
}
