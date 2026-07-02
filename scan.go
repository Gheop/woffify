package main

import (
	"fmt"
	"html"
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

// textExts are the markup/template extensions scanned in text mode.
var textExts = map[string]bool{
	".html": true, ".htm": true, ".xhtml": true, ".xml": true, ".svg": true,
	".txt": true, ".md": true, ".vue": true, ".hbs": true, ".ejs": true,
	".njk": true, ".twig": true, ".liquid": true, ".mustache": true,
}

// cssContentDecl matches the value of a CSS `content` declaration.
var cssContentDecl = regexp.MustCompile(`(?i)content\s*:\s*([^;{}]*)`)

// cssEscape matches a CSS unicode escape: a backslash then 1-6 hex digits.
var cssEscape = regexp.MustCompile(`\\([0-9a-fA-F]{1,6})`)

// htmlComment and htmlTag strip markup so text mode keeps only visible text.
var htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)
var htmlTag = regexp.MustCompile(`<[^>]*>`)

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

// extractTextCodepoints strips tags and comments, decodes HTML entities, and
// returns the code points of the remaining visible characters. It covers text
// fonts (literal characters in templates); runtime-injected text is not seen.
func extractTextCodepoints(s string) []rune {
	s = htmlComment.ReplaceAllString(s, "")
	s = htmlTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	var cps []rune
	for _, r := range s {
		if r >= 0x20 && r != 0x7F { // skip control characters
			cps = append(cps, r)
		}
	}
	return cps
}

// fileMode decides how a file is scanned given the requested mode and its
// extension. It returns "css", "text" or "" (skip).
func fileMode(mode, ext string) string {
	switch mode {
	case "css":
		if ext == ".css" {
			return "css"
		}
	case "text":
		if textExts[ext] {
			return "text"
		}
	case "auto":
		if ext == ".css" {
			return "css"
		}
		if textExts[ext] {
			return "text"
		}
	}
	return ""
}

// scanCodepoints walks the given files and directories, extracts code points
// from matching source files, and returns them in first-seen order along with
// the file each was first seen in (for -subset-scan-report).
func scanCodepoints(paths []string, mode string) ([]rune, map[rune]string, error) {
	switch mode {
	case "css", "text", "auto":
	default:
		return nil, nil, fmt.Errorf("unsupported -subset-scan-mode %q (css, text or auto)", mode)
	}

	origin := map[rune]string{}
	var order []rune
	scanFile := func(path string) error {
		var cps []rune
		switch fileMode(mode, strings.ToLower(filepath.Ext(path))) {
		case "css":
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			cps = extractCSSCodepoints(string(data))
		case "text":
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			cps = extractTextCodepoints(string(data))
		default:
			return nil
		}
		for _, r := range cps {
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
