#include "woff2enc.h"

#include <woff2/encode.h>

#include <cstdlib>

// Thin C wrapper around woff2::ConvertTTFToWOFF2 so cgo can call it.
// Uses the default WOFF2Params (brotli_quality=11, allow_transforms=true),
// which matches the woff2_compress reference tool byte for byte.
extern "C" unsigned char *woffify_woff2_encode(const unsigned char *data,
                                               size_t len, size_t *out_len) {
  size_t max = woff2::MaxWOFF2CompressedSize(data, len);
  // calloc, not malloc: the encoder may leave padding bytes untouched within the
  // returned range, so an uninitialized buffer makes the output non-deterministic.
  unsigned char *buf = static_cast<unsigned char *>(std::calloc(max, 1));
  if (buf == nullptr) {
    return nullptr;
  }
  size_t result_len = max;
  woff2::WOFF2Params params;
  if (!woff2::ConvertTTFToWOFF2(data, len, buf, &result_len, params)) {
    std::free(buf);
    return nullptr;
  }
  *out_len = result_len;
  return buf;
}
