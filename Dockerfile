# Build a fully static woffify binary and ship it in a scratch image.
#
# HarfBuzz is built minimal (subset only, no freetype/glib/graphite/icu), and
# woff2 is built as a static archive; brotli comes from Alpine's -static package.
# Everything is linked into one static binary with no runtime dependencies.
FROM golang:1.26-alpine@sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628 AS build
RUN apk add --no-cache build-base git meson ninja brotli-static brotli-dev

# Minimal static HarfBuzz (subset API only), pinned to a release + verified SHA.
ARG HB_TAG=14.4.0
ARG HB_SHA=36cb489cb02ce4b92099669ba9f9bea348eff93f
RUN git clone --depth 1 --branch "$HB_TAG" https://github.com/harfbuzz/harfbuzz /hb \
    && test "$(git -C /hb rev-parse HEAD)" = "$HB_SHA"
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

# Static woff2 encoder, pinned to a release + verified SHA.
ARG WOFF2_TAG=v1.0.2
ARG WOFF2_SHA=1bccf208bca986e53a647dfe4811322adb06ecf8
RUN git clone --depth 1 --branch "$WOFF2_TAG" https://github.com/google/woff2 /woff2 \
    && test "$(git -C /woff2 rev-parse HEAD)" = "$WOFF2_SHA"
WORKDIR /woff2
RUN SRCS="src/woff2_enc.cc src/font.cc src/glyph.cc src/normalize.cc src/transform.cc src/table_tags.cc src/variable_length.cc src/woff2_common.cc src/woff2_out.cc"; \
    g++ -O2 -std=c++11 -include cstdint -Iinclude -I/usr/include -c $SRCS \
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
