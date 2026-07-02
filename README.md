# woffify

Convert WOFF, TTF and OTF fonts to WOFF2, with optional glyph subsetting. One
self-contained static binary, no Python or Node runtime, made for CI pipelines
and container clusters.

## What it does

`google/woff2` (`woff2_compress`) is the reference WOFF2 encoder, but it only
takes TTF/OTF input and does no subsetting. woffify wraps the same encoder and
adds the two things a web font pipeline actually needs:

- **WOFF 1.0 input.** WOFF is decoded to SFNT in pure Go, so you can recompress
  existing `.woff` files. This is lossless: the result matches encoding from the
  original TTF.
- **Subsetting.** HarfBuzz (`hb-subset`) drops the glyphs you don't need, which
  is the biggest size win for the web. On text fonts a Latin subset is routinely
  80–95% smaller than the full font.

On TTF/OTF input without subsetting, woffify's output is **byte-for-byte
identical** to `woff2_compress` (same encoder, Brotli quality 11).

Input: `.woff`, `.ttf`, `.otf`, `.ttc`. Output: `.woff2`.

## Install

Pull the prebuilt static image (~6 MB), build it yourself, or install with Go:

```bash
# prebuilt image
docker run --rm -v "$PWD:/data" ghcr.io/gheop/woffify -o /data/out /data/fonts

# or build the image from source
docker build -t woffify https://github.com/Gheop/woffify.git

# or install with Go (needs the harfbuzz/woff2/brotli dev packages, see Building)
go install github.com/Gheop/woffify@latest
```

## Usage

```bash
# single file
woffify Font.woff

# a whole directory to an output directory
woffify -o dist/fonts assets/fonts

# Latin subset, recursive, quiet
woffify -r -q -subset-unicodes 0-FF,20AC,2000-206F dist/fonts assets/fonts

# subset to exactly the characters in a string
woffify -subset-text "Patu.dev — coming soon" Brand.ttf

# pipe mode: read a font from stdin, write WOFF2 to stdout (no temp files)
cat Font.woff | woffify - > Font.woff2
woffify -subset-unicodes 0-FF - < Font.ttf > Font.woff2
```

Options:

```
-o <dir>                output directory (default: next to each source)
-r                      recurse into directories
-q                      only print errors
-j <n>                  parallel workers (default: CPU count)
-subset-unicodes <set>  subset to code point ranges, e.g. 0-FF,20AC,2000-206F
-subset-text <string>   subset to the glyphs covering these characters
-drop-hints             drop hinting when subsetting
-retain-gids            keep original glyph IDs when subsetting
```

Code points are hex, with an optional `U+` prefix. The exit code is non-zero if
any conversion fails, so a CI step fails cleanly.

Batch runs use every CPU core by default:

```bash
woffify -j "$(nproc)" -subset-unicodes 0-FF -o dist assets/fonts
```

## How it works

```
WOFF ──(zlib decode, pure Go)──▶ SFNT ─┐
TTF/OTF ───────────────────────────────┼─▶ hb-subset ─▶ woff2 encoder ─▶ WOFF2
                                        │   (optional)   (Brotli 11)
```

- WOFF decoding is pure Go (`compress/zlib`).
- Subsetting calls HarfBuzz `hb-subset` via cgo.
- WOFF2 encoding calls the `google/woff2` encoder via cgo.

The release binary is fully static: HarfBuzz is built minimal (subset only, no
FreeType/glib/graphite), woff2 and brotli are linked statically, and the result
runs from a `scratch` image with no shared libraries.

## Benchmarks

Measured on 180 Google Fonts (paired TTF and WOFF), Intel Core Ultra 7 155H
(22 threads), woff2 encoder 1.0.2.

Reliability:

- woffify TTF→WOFF2 identical to `woff2_compress`: **180/180**
- WOFF→WOFF2 output valid (decodes back): **180/180**
- Latin subset succeeds: **180/180**

Size, cumulative over the 180 fonts:

| output | total | vs TTF source |
|---|---|---|
| TTF sources | 79.8 MB | 100% |
| WOFF2, full | 23.6 MB | 29.6% |
| WOFF2, Latin subset | 4.47 MB | 5.6% |

The full WOFF2 output is within **+0.003%** of the official Google Fonts WOFF2
files (same Brotli 11 encoder). A Latin subset (`0-FF,20AC,2000-206F,2122`) is
**81% smaller** than the full WOFF2.

Throughput, all 180 fonts, parallel over 22 cores:

| task | wall time |
|---|---|
| full, from TTF | 53 s |
| full, from WOFF | 39 s |
| Latin subset, from TTF | 2.3 s |

Subsetting is faster than full conversion because it shrinks the font before the
Brotli 11 step, which dominates. WOFF decoding on its own costs about 5 ms per
font (`go test -bench`), negligible next to encoding.

## Building

Static binary in a `scratch` image (recommended):

```bash
docker build -t woffify .
```

Local dynamic build for development (needs the dev packages for harfbuzz-subset,
woff2 and brotli):

```bash
# Debian/Ubuntu: apt install libharfbuzz-dev libwoff2-dev libbrotli-dev
# Fedora:        dnf install harfbuzz-devel woff2-devel brotli-devel
go build -o woffify .
go test ./...
```

## License

MIT, see [LICENSE](LICENSE).

The static binary links HarfBuzz, google/woff2 and Brotli, all under permissive
MIT/MIT-style licenses.

## Changelog

### v0.1.0 — Initial release (2026-07-02)

- Convert WOFF/TTF/OTF/TTC to WOFF2 using the `google/woff2` encoder (Brotli 11)
- Pure-Go WOFF 1.0 decoding, so `.woff` files can be recompressed
- Glyph subsetting via HarfBuzz `hb-subset`
- Stdin/stdout pipe mode (`woffify -`) for temp-file-free CI integration
- Parallel batch conversion of files and directories
- Single fully static binary, `scratch` container image
