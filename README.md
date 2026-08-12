# woffify

woffify converts web fonts to WOFF2 from a single static binary. It reads WOFF,
TTF, OTF and EOT, subsets glyphs with HarfBuzz, and can derive the glyph set
straight from your CSS. No Python or Node runtime — one 6.6 MB binary, built for
CI pipelines and container images.

[![CI](https://github.com/Gheop/woffify/actions/workflows/ci.yml/badge.svg)](https://github.com/Gheop/woffify/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Gheop/woffify)](https://github.com/Gheop/woffify/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Quick start

Convert a folder of fonts to WOFF2 with the prebuilt image:

```bash
docker run --rm -v "$PWD:/data" ghcr.io/gheop/woffify:v0.2.2 -o /data/out /data/fonts
```

The `.woff2` files are written next to `/data/out`. Every WOFF, TTF, OTF and
EOT file in the input folder is converted.

## Installation

### Prebuilt image

Pull the static image from the container registry:

```bash
docker pull ghcr.io/gheop/woffify:v0.2.2
```

The image is also on GitLab at `registry.gitlab.com/gheop/woffify:v0.2.2`.

### Build the image from source

Build straight from the repository:

```bash
docker build -t woffify https://github.com/Gheop/woffify.git
```

### Install with Go

Install the command with Go 1.26 or newer:

```bash
go install github.com/Gheop/woffify@latest
```

This build links against system libraries. Install the C dependencies first. See
[Building from source](#building-from-source).

## Usage

Convert one file. The output goes next to the source:

```bash
woffify Font.woff
```

Convert a folder into an output folder:

```bash
woffify -o dist/fonts assets/fonts
```

Subset to a set of code points. Recurse into folders and print only errors:

```bash
woffify -r -q -subset-unicodes 0-FF,20AC,2000-206F -o dist/fonts assets/fonts
```

Subset to the exact characters in a string:

```bash
woffify -subset-text "Patu.dev — coming soon" Brand.ttf
```

Read a font from stdin and write WOFF2 to stdout. No temp files:

```bash
cat Font.woff | woffify - > Font.woff2
woffify -subset-unicodes 0-FF - < Font.ttf > Font.woff2
```

The exit code is non-zero when any conversion fails. A CI step fails cleanly.

### Subset from your sources

`-subset-scan` derives the glyph set from your source files. The subset stays in
sync with the pages, with no hand-maintained glyph list:

```bash
woffify -subset-scan public/themes -o dist/fonts assets/fonts/fa-solid-900.ttf
```

Scan modes:

- `css` reads `\fXXX` escapes in CSS `content` declarations. This is the icon-font
  case (Font Awesome, icomoon).
- `text` collects the literal characters from HTML and templates. Markup is
  stripped and HTML entities are decoded.
- `auto` (the default) picks the mode per file extension. `.css` uses css.
  `.html`, `.svg` and template files use text.

`-subset-scan` unions with `-subset-unicodes` and `-subset-text`, so you can add
glyphs that are injected at runtime. If a scan finds no code points, woffify exits
with an error instead of writing an empty font.

Print the retained code points and their origin file with `-subset-scan-report`.

## Options

| Flag | Default | Description |
|---|---|---|
| `-o <dir>` | next to each source | Output directory |
| `-r` | off | Recurse into directories |
| `-q` | off | Print only errors |
| `-j <n>` | CPU count | Number of parallel workers |
| `-subset-unicodes <set>` | — | Subset to code point ranges, e.g. `0-FF,20AC` |
| `-subset-text <string>` | — | Subset to the glyphs covering these characters |
| `-subset-scan <path>` | — | Derive the subset from files or dirs (repeatable) |
| `-subset-scan-mode <m>` | `auto` | Scan mode: `auto`, `css` or `text` |
| `-subset-scan-report` | off | Print the code points kept by the scan |
| `-drop-hints` | off | Drop hinting when subsetting |
| `-retain-gids` | off | Keep original glyph IDs when subsetting |

Code points are hex, with an optional `U+` prefix.

Input formats: `.woff`, `.ttf`, `.otf`, `.eot`. Output: `.woff2`. EOT
input is for migrating legacy IE assets. Only uncompressed EOT is read. MicroType
Express-compressed EOT is rejected with a clear message.

## How it works

```
WOFF/EOT ──(decode, pure Go)──▶ SFNT ─┐
TTF/OTF ──────────────────────────────┼─▶ hb-subset ─▶ woff2 encoder ─▶ WOFF2
                                       │   (optional)   (Brotli 11)
```

- WOFF (zlib) and EOT decoding are pure Go.
- Subsetting calls HarfBuzz `hb-subset` through cgo.
- WOFF2 encoding calls the `google/woff2` encoder through cgo.

The release binary is fully static. HarfBuzz is built minimal (subset only, no
FreeType, glib or graphite). woff2 and brotli are linked statically. The result
runs from a `scratch` image with no shared libraries.

On TTF and OTF input without subsetting, the output is byte-for-byte identical to
`woff2_compress`, because it is the same encoder at Brotli quality 11.

## Benchmarks

Measured on 180 Google Fonts as TTF, WOFF, WOFF2 and EOT plus 94 OTF (814 files),
woff2 encoder 1.0.2.

Reliability, over the whole dataset:

- every input format converts and the output decodes: **634/634** (TTF, WOFF, OTF, EOT)
- woffify(TTF) is **byte-identical** to `woff2_compress`: **180/180**
- EOT→WOFF2 is **byte-identical** to TTF→WOFF2 (lossless extraction): **180/180**
- output is deterministic — same input, same bytes — including in parallel batches

Size, cumulative over the 180 fonts:

| output | total | vs TTF source |
|---|---|---|
| TTF sources | 79.8 MB | 100% |
| WOFF2, full | 23.6 MB | 29.6% |
| WOFF2, Latin subset | 4.47 MB | 5.6% |

Full WOFF2 is within **+0.003%** of the official Google Fonts WOFF2 (same Brotli
11 encoder). A Latin subset (`0-FF,20AC,2000-206F,2122`) is **81% smaller** than
the full WOFF2.

Throughput is the encoder's, not woffify's: converting a font takes the same time
as `woff2_compress` (Brotli 11), parallelized across all cores. Subsetting is
several times faster because it shrinks the font before the Brotli step. WOFF
decoding adds about 5 ms per font (`go test -bench`), negligible next to encoding.

## Building from source

The static Docker build needs no local dependencies:

```bash
docker build -t woffify .
```

For a local dynamic build, install the C dependencies. On Fedora, all three are
packaged:

```bash
dnf install harfbuzz-devel woff2-devel brotli-devel
go build -o woffify .
go test ./...
```

Debian and Ubuntu do not package the woff2 encoder headers (`libwoff2-dev` does
not exist). Build the woff2 encoder from source, as the CI does. See
[`.github/workflows/ci.yml`](.github/workflows/ci.yml) for the exact recipe.

## Contributing

Pull requests run through CI on GitHub and GitLab: `go vet`, the test suite, and a
static image build. Keep changes covered by a test.

## Why not fontTools?

[fontTools](https://fonttools.readthedocs.io/) is the reference toolkit and does
far more than woffify — merging, variable-font instancing, TTX round-trips — and
it reads WOFF 1.0 too. On output size the two are equivalent: measured on the same
fonts, both faithful conversion and Latin subsetting land within 1% of each other.
woffify is neither smaller nor better at compressing.

The reason to reach for woffify is deployment. A minimal fontTools container image
(`python:3-alpine` + fonttools + brotli, the smallest that still writes WOFF2) is
**84 MB**. woffify ships as a **6.6 MB** static binary in a `scratch` image, with
no runtime to install or pin. Two things it adds inside that single binary:
uncompressed EOT input, which fontTools does not read, and deriving the subset
straight from your CSS (`-subset-scan`), which otherwise means adding a Node tool
like glyphhanger.

If you already have Python in your pipeline, use fontTools.

## Used in production

woffify is built for and used by [patu.dev](https://patu.dev) to convert and
subset web fonts in its asset pipeline.

## License

MIT, see [LICENSE](LICENSE). The static binary links HarfBuzz, google/woff2 and
Brotli, all under permissive MIT or MIT-style licenses.

## Release history

### v0.2.3 — Reject font collections (2026-08-12)

- Reject `.ttc` font collections with a clear message: WOFF2 has no collection format (previously mis-handled)
- Hardened over 297 varied system fonts (color/COLR, CJK, Arabic, Indic, symbols, variable, CFF): all subset and convert, and faithful output is byte-identical to `woff2_compress` on the sample

### v0.2.2 — Deterministic output (2026-07-02)

- Fix non-deterministic WOFF2 output in parallel batches, caused by an uninitialized encode buffer; output is now byte-identical to `woff2_compress` on every run
- Verified over 814 files: 634/634 convert and decode, 180/180 byte-identical to the reference

### v0.2.1 — EOT input (2026-07-02)

- Read uncompressed EOT (Embedded OpenType) files, for migrating legacy IE assets to WOFF2
- MicroType Express-compressed EOT is rejected with a clear message

### v0.2.0 — Source-scan subsetting (2026-07-02)

- `-subset-scan` derives the glyph subset from your sources, no hand-maintained glyph list
- `css` mode reads `\fXXX` escapes in CSS `content` declarations (icon fonts)
- `text` mode collects literal characters from HTML/templates
- `auto` (default) picks the mode per file extension
- `-subset-scan-report` lists the kept code points and their origin file
- Refuses to build an empty subset when a scan matches nothing

### v0.1.0 — Initial release (2026-07-02)

- Convert WOFF/TTF/OTF/TTC to WOFF2 using the `google/woff2` encoder (Brotli 11)
- Pure-Go WOFF 1.0 decoding
- Glyph subsetting via HarfBuzz `hb-subset`
- Stdin/stdout pipe mode for temp-file-free CI integration
- Parallel batch conversion of files and directories
- Single fully static binary, `scratch` container image

## README changelog

| Version | Date       | Changes                                                              |
|---------|------------|----------------------------------------------------------------------|
| 1.0.1   | 2026-08-12 | Drop .ttc from supported input formats                              |
| 1.0.0   | 2026-08-12 | Initialize changelog, restructure to Diátaxis, fix Debian/Ubuntu build deps |
