//go:build !amd64 || appengine || noasm || nogen || nopshufb || !gc

package reedsolomon

import "fmt"

// GF16MulSliceXor8 is the portable scalar fallback. See the amd64 variant
// for full semantics.
func (l LowLevel) GF16MulSliceXor8(scalars *[8]uint16, in []byte, outs *[8][]byte) {
	if len(in) == 0 {
		return
	}
	if len(in)%64 != 0 {
		panic(fmt.Sprintf("reedsolomon: GF16MulSliceXor8: len(in)=%d must be a multiple of 64", len(in)))
	}
	for k := range outs {
		if len(outs[k]) != len(in) {
			panic(fmt.Sprintf("reedsolomon: GF16MulSliceXor8: outs[%d] has len %d, expected %d", k, len(outs[k]), len(in)))
		}
	}
	initConstants()
	opts := l.options()
	for k, c := range scalars {
		if c == 0 {
			continue
		}
		mulgf16Xor(outs[k], in, logLUT[ffe(c)], opts)
	}
}
