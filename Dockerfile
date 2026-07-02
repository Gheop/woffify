# Build a fully static woffify binary and ship it in a scratch image.
#
# HarfBuzz is built minimal (subset only, no freetype/glib/graphite/icu), and
# woff2 is built as a static archive; brotli comes from Alpine's -static package.
# Everything is linked into one static binary with no runtime dependencies.
FROM golang:1.26-alpine AS build
RUN apk add --no-cache build-base git meson ninja brotli-static brotli-dev

# Minimal static HarfBuzz (subset API only).
RUN git clone --depth 1 https://github.com/harfbuzz/harfbuzz /hb
WORKDIR /hb
RUN meson setup build --default-library=static --buildtype=release \
      -Dfreetype=disabled -Dglib=disabled -Dgobject=disabled -Dicu=disabled \
      -Dcairo=disabled -Dgraphite2=disabled -Dtests=disabled -Ddocs=disabled \
      -Dutilities=disabled \
    && ninja -C build src/libharfbuzz-subset.a src/libharfbuzz.a \
    && cp build/src/libharfbuzz-subset.a build/src/libharfbuzz.a /usr/lib/ \
    && mkdir -p /usr/include/harfbuzz \
    && cp src/hb*.h /usr/include/harfbuzz/ \
    && cp build/src/hb-version.h /usr/include/harfbuzz/ 2>/dev/null || true

# Static woff2 encoder.
RUN git clone --depth 1 https://github.com/google/woff2 /woff2
WORKDIR /woff2
RUN SRCS="src/woff2_enc.cc src/font.cc src/glyph.cc src/normalize.cc src/transform.cc src/table_tags.cc src/variable_length.cc src/woff2_common.cc src/woff2_out.cc"; \
    g++ -O2 -std=c++11 -Iinclude -I/usr/include -c $SRCS \
    && ar rcs /usr/lib/libwoff2enc.a *.o \
    && cp -r include/woff2 /usr/include/

WORKDIR /src
COPY go.mod ./
COPY *.go *.cc *.h ./
RUN CGO_ENABLED=1 go build -tags static \
      -ldflags='-s -w -linkmode external -extldflags "-static"' \
      -o /woffify .

FROM scratch
COPY --from=build /woffify /woffify
ENTRYPOINT ["/woffify"]
