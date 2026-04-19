//go:build !appengine && !noasm && !nogen && !nopshufb && gc

package reedsolomon

//go:noescape
func mulgf16Xor_avx2(x []byte, y []byte, table *[128]uint8)

// mulgf16Xor does out[:] ^= in[:] * scalar, where scalar is given by its log.
// Slices must have equal length that is a non-zero multiple of 64.
//
// There is no GFNI variant here on purpose: the fused 8-scalar kernel
// (mulgf16Xor8_gfni) covers the GFNI fast path, so this function is only
// reached on hosts without GFNI. AVX2 is the best we can do there.
func mulgf16Xor(out, in []byte, log_m ffe, o *options) {
	if len(out) == 0 {
		return
	}
	if o.useAVX2 {
		tmp := &multiply256LUT[log_m]
		if raceEnabled {
			raceReadSlice(in)
			raceWriteSlice(out)
		}
		mulgf16Xor_avx2(out, in, tmp)
		return
	}
	refMulAdd(out, in, log_m)
}
