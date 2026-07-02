package main

// #include <stdlib.h>
// #include "woff2enc.h"
import "C"

import (
	"fmt"
	"unsafe"
)

// encodeWOFF2 compresses SFNT bytes to WOFF2 using the woff2 reference encoder
// (Brotli quality 11). The output is byte-for-byte identical to woff2_compress.
func encodeWOFF2(sfnt []byte) ([]byte, error) {
	if len(sfnt) == 0 {
		return nil, fmt.Errorf("empty SFNT data")
	}
	var n C.size_t
	p := C.woffify_woff2_encode(
		(*C.uchar)(unsafe.Pointer(&sfnt[0])),
		C.size_t(len(sfnt)),
		&n,
	)
	if p == nil {
		return nil, fmt.Errorf("woff2 encoding failed (not a valid font?)")
	}
	defer C.free(unsafe.Pointer(p))
	return C.GoBytes(unsafe.Pointer(p), C.int(n)), nil
}
