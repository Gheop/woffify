#ifndef WOFFIFY_WOFF2ENC_H
#define WOFFIFY_WOFF2ENC_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Encode SFNT bytes to WOFF2 (Brotli quality 11, glyf/hmtx transforms on).
 * Returns a malloc'd buffer the caller must free, or NULL on failure. */
unsigned char *woffify_woff2_encode(const unsigned char *data, size_t len,
                                    size_t *out_len);

#ifdef __cplusplus
}
#endif

#endif /* WOFFIFY_WOFF2ENC_H */
