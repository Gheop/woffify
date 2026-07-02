// Command woffify converts WOFF, TTF and OTF fonts to WOFF2, with optional
// glyph subsetting. It is a single self-contained binary: WOFF decoding is done
// in pure Go, subsetting uses HarfBuzz (hb-subset) and WOFF2 encoding uses the
// woff2 reference encoder (Brotli 11), both linked in via cgo.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

var inputExts = map[string]bool{
	".woff": true, ".ttf": true, ".otf": true, ".ttc": true,
}

func main() {
	outDir := flag.String("o", "", "output directory (default: next to each source)")
	recursive := flag.Bool("r", false, "recurse into directories")
	quiet := flag.Bool("q", false, "only print errors")
	jobs := flag.Int("j", runtime.NumCPU(), "number of parallel workers")
	unicodes := flag.String("subset-unicodes", "", "subset to these code point ranges, e.g. 0-FF,20AC,2000-206F")
	text := flag.String("subset-text", "", "subset to the glyphs covering these characters")
	dropHints := flag.Bool("drop-hints", false, "drop hinting when subsetting")
	retainGids := flag.Bool("retain-gids", false, "keep original glyph IDs when subsetting")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: woffify [options] <file|dir>...\n")
		fmt.Fprintf(os.Stderr, "       woffify [options] -   (read font from stdin, write WOFF2 to stdout)\n\n")
		fmt.Fprintf(os.Stderr, "Convert WOFF/TTF/OTF to WOFF2, with optional subsetting.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	opts, err := buildSubsetOptions(*unicodes, *text, *dropHints, *retainGids)
	if err != nil {
		fmt.Fprintf(os.Stderr, "woffify: %v\n", err)
		os.Exit(2)
	}

	// Pipe mode: `woffify -` reads a font from stdin and writes WOFF2 to stdout.
	if flag.NArg() == 1 && flag.Arg(0) == "-" {
		errOut := muteCStderr()
		if err := convertStream(os.Stdin, os.Stdout, opts); err != nil {
			fmt.Fprintf(errOut, "woffify: %v\n", err)
			os.Exit(1)
		}
		return
	}

	inputs, err := collect(flag.Args(), *recursive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "woffify: %v\n", err)
		os.Exit(1)
	}

	errOut := muteCStderr()
	failed := run(inputs, *outDir, *jobs, *quiet, opts, errOut)
	if failed > 0 {
		fmt.Fprintf(errOut, "woffify: %d/%d conversion(s) failed\n", failed, len(inputs))
		os.Exit(1)
	}
}

// run converts every input with a pool of workers and returns the failure count.
func run(inputs []string, outDir string, jobs int, quiet bool, opts subsetOptions, errOut *os.File) int32 {
	if jobs < 1 {
		jobs = 1
	}
	var (
		failed  int32
		printMu sync.Mutex
		wg      sync.WaitGroup
	)
	queue := make(chan string)

	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for in := range queue {
				line, err := convert(in, outputPath(in, outDir), opts)
				printMu.Lock()
				if err != nil {
					fmt.Fprintf(errOut, "woffify: %s: %v\n", in, err)
					atomic.AddInt32(&failed, 1)
				} else if !quiet {
					fmt.Println(line)
				}
				printMu.Unlock()
			}
		}()
	}
	for _, in := range inputs {
		queue <- in
	}
	close(queue)
	wg.Wait()
	return failed
}

// transform runs the decode -> subset -> encode pipeline on font bytes and
// returns the WOFF2 bytes.
func transform(data []byte, opts subsetOptions) ([]byte, error) {
	sfnt, err := toSFNT(data)
	if err != nil {
		return nil, err
	}
	if opts.active() {
		sfnt, err = subsetSFNT(sfnt, opts)
		if err != nil {
			return nil, err
		}
	}
	return encodeWOFF2(sfnt)
}

// convert reads one font file, transforms it and writes the WOFF2 output. It
// returns a one-line size report on success.
func convert(in, out string, opts subsetOptions) (string, error) {
	data, err := os.ReadFile(in)
	if err != nil {
		return "", err
	}
	woff2, err := transform(data, opts)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(out, woff2, 0o644); err != nil {
		return "", err
	}
	return sizeReport(in, out, len(data), len(woff2)), nil
}

// convertStream reads a font from r, transforms it and writes WOFF2 to w. This
// backs the `woffify -` pipe mode (no temp files).
func convertStream(r io.Reader, w io.Writer, opts subsetOptions) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	woff2, err := transform(data, opts)
	if err != nil {
		return err
	}
	_, err = w.Write(woff2)
	return err
}

// toSFNT returns the SFNT bytes of a font. WOFF 1.0 is decoded; TTF/OTF are
// returned unchanged; WOFF2 is rejected as it is already the target format.
func toSFNT(data []byte) ([]byte, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("file too short")
	}
	switch binary.BigEndian.Uint32(data) {
	case woffSignature: // "wOFF"
		sfnt, _, err := decodeWOFF(data)
		return sfnt, err
	case 0x774F4632: // "wOF2"
		return nil, fmt.Errorf("already WOFF2")
	default: // "OTTO", 0x00010000, "true", "ttcf"
		return data, nil
	}
}

// collect expands the arguments into a list of font files.
func collect(args []string, recursive bool) ([]string, error) {
	var out []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, arg)
			continue
		}
		err = filepath.WalkDir(arg, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if path != arg && !recursive {
					return filepath.SkipDir
				}
				return nil
			}
			if inputExts[strings.ToLower(filepath.Ext(path))] {
				out = append(out, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// outputPath computes the .woff2 path for a source file.
func outputPath(in, outDir string) string {
	base := strings.TrimSuffix(filepath.Base(in), filepath.Ext(in)) + ".woff2"
	if outDir == "" {
		return filepath.Join(filepath.Dir(in), base)
	}
	return filepath.Join(outDir, base)
}

// buildSubsetOptions turns the CLI subset flags into a subsetOptions value.
func buildSubsetOptions(unicodes, text string, dropHints, retainGids bool) (subsetOptions, error) {
	opts := subsetOptions{dropHints: dropHints, retainGids: retainGids}
	if unicodes != "" {
		ranges, err := parseUnicodeRanges(unicodes)
		if err != nil {
			return opts, err
		}
		opts.unicodes = append(opts.unicodes, ranges...)
	}
	for _, r := range text {
		opts.unicodes = append(opts.unicodes, unicodeRange{r, r})
	}
	if (dropHints || retainGids) && !opts.active() {
		return opts, fmt.Errorf("-drop-hints/-retain-gids require -subset-unicodes or -subset-text")
	}
	return opts, nil
}

// parseUnicodeRanges parses "0-FF,20AC,2000-206F" into ranges. Values are hex,
// with an optional U+ prefix.
func parseUnicodeRanges(s string) ([]unicodeRange, error) {
	var ranges []unicodeRange
	for _, tok := range strings.Split(s, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		lo, hi, found := strings.Cut(tok, "-")
		first, err := parseCodePoint(lo)
		if err != nil {
			return nil, err
		}
		last := first
		if found {
			last, err = parseCodePoint(hi)
			if err != nil {
				return nil, err
			}
		}
		if last < first {
			return nil, fmt.Errorf("invalid range %q", tok)
		}
		ranges = append(ranges, unicodeRange{first, last})
	}
	if len(ranges) == 0 {
		return nil, fmt.Errorf("no code points in %q", s)
	}
	return ranges, nil
}

func parseCodePoint(s string) (rune, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "U+"), "u+")
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid code point %q", s)
	}
	return rune(v), nil
}

func sizeReport(in, out string, srcLen, dstLen int) string {
	var pct float64
	if srcLen > 0 {
		pct = 100 * float64(srcLen-dstLen) / float64(srcLen)
	}
	return fmt.Sprintf("%s -> %s  (%s -> %s, %+.0f%%)",
		in, out, human(srcLen), human(dstLen), -pct)
}

func human(n int) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := int64(n) / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}
