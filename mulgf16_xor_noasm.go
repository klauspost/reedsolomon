//go:build !amd64 || appengine || noasm || nogen || nopshufb || !gc

package reedsolomon

// mulgf16Xor does out[:] ^= in[:] * scalar via the scalar reference path.
func mulgf16Xor(out, in []byte, log_m ffe, _ *options) {
	if len(out) == 0 {
		return
	}
	refMulAdd(out, in, log_m)
}
