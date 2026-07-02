package main

// #include <hb.h>
// #include <hb-subset.h>
// #include <stdlib.h>
import "C"

import (
	"fmt"
	"unsafe"
)

// unicodeRange is an inclusive range of code points to keep.
type unicodeRange struct{ first, last rune }

// subsetOptions controls which glyphs and data hb-subset keeps.
type subsetOptions struct {
	// unicodes is the set of code point ranges to retain. If empty, no
	// subsetting is performed and the input is returned unchanged.
	unicodes []unicodeRange
	// dropHints removes TrueType hinting instructions and related tables.
	dropHints bool
	// retainGids keeps the original glyph IDs instead of renumbering them.
	retainGids bool
}

// active reports whether subsetting should run.
func (o subsetOptions) active() bool { return len(o.unicodes) > 0 }

// subsetSFNT reduces a font to the requested code points using HarfBuzz.
// It takes SFNT bytes and returns SFNT bytes.
func subsetSFNT(sfnt []byte, opts subsetOptions) ([]byte, error) {
	if len(sfnt) == 0 {
		return nil, fmt.Errorf("empty SFNT data")
	}

	// DUPLICATE makes HarfBuzz copy the bytes immediately, so it never holds a
	// Go pointer past this call (required by the cgo pointer-passing rules; a
	// retained Go pointer corrupts under GC pressure with many parallel workers).
	blob := C.hb_blob_create(
		(*C.char)(unsafe.Pointer(&sfnt[0])), C.uint(len(sfnt)),
		C.HB_MEMORY_MODE_DUPLICATE, nil, nil,
	)
	defer C.hb_blob_destroy(blob)

	face := C.hb_face_create(blob, 0)
	defer C.hb_face_destroy(face)

	input := C.hb_subset_input_create_or_fail()
	if input == nil {
		return nil, fmt.Errorf("hb_subset_input_create_or_fail failed")
	}
	defer C.hb_subset_input_destroy(input)

	unicodeSet := C.hb_subset_input_unicode_set(input)
	for _, rg := range opts.unicodes {
		C.hb_set_add_range(unicodeSet, C.hb_codepoint_t(rg.first), C.hb_codepoint_t(rg.last))
	}

	flags := C.uint(C.HB_SUBSET_FLAGS_DEFAULT)
	if opts.dropHints {
		flags |= C.uint(C.HB_SUBSET_FLAGS_NO_HINTING)
	}
	if opts.retainGids {
		flags |= C.uint(C.HB_SUBSET_FLAGS_RETAIN_GIDS)
	}
	C.hb_subset_input_set_flags(input, flags)

	result := C.hb_subset_or_fail(face, input)
	if result == nil {
		return nil, fmt.Errorf("hb_subset_or_fail failed")
	}
	defer C.hb_face_destroy(result)

	outBlob := C.hb_face_reference_blob(result)
	defer C.hb_blob_destroy(outBlob)

	var n C.uint
	data := C.hb_blob_get_data(outBlob, &n)
	if data == nil || n == 0 {
		return nil, fmt.Errorf("subsetting produced no data")
	}
	return C.GoBytes(unsafe.Pointer(data), C.int(n)), nil
}
