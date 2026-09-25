# Build a fully static woffify binary and ship it in a scratch image.
#
# HarfBuzz is built minimal (subset only, no freetype/glib/graphite/icu), and
# woff2 and brotli are built as static archives.
# Everything is linked into one static binary with no runtime dependencies.
FROM golang:1.26-alpine@sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628 AS build
RUN apk add --no-cache build-base git meson ninja cmake

# Static brotli, pinned to a release + verified SHA. Built from source at -O2:
# the Alpine package encodes 5-6% slower at quality 11 for identical bytes.
ARG BROTLI_TAG=v1.2.0
ARG BROTLI_SHA=028fb5a23661f123017c060daa546b55cf4bde29
RUN git clone --depth 1 --branch "$BROTLI_TAG" https://github.com/google/brotli /brotli \
    && test "$(git -C /brotli rev-parse HEAD)" = "$BROTLI_SHA" \
    && cmake -S /brotli -B /brotli/build -DCMAKE_BUILD_TYPE=None -DCMAKE_C_FLAGS=-O2 \
       -DBUILD_SHARED_LIBS=OFF -DBROTLI_DISABLE_TESTS=ON -DCMAKE_INSTALL_PREFIX=/usr \
    && cmake --build /brotli/build -j \
    && cmake --install /brotli/build

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

# Static woff2 encoder, pinned to a release + verified SHA, plus the upstream
# fix for google/woff2#191 (encoder DoS on untrusted fonts), which has no
# release yet. Only that fix: later upstream commits change the encoded bytes.
ARG WOFF2_TAG=v1.0.2
ARG WOFF2_SHA=1bccf208bca986e53a647dfe4811322adb06ecf8
COPY patches/woff2-monotonic-endpts.patch /patches/
RUN git clone --depth 1 --branch "$WOFF2_TAG" https://github.com/google/woff2 /woff2 \
    && test "$(git -C /woff2 rev-parse HEAD)" = "$WOFF2_SHA" \
    && git -C /woff2 apply /patches/woff2-monotonic-endpts.patch
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
