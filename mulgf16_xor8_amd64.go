//go:build !appengine && !noasm && !nogen && !nopshufb && gc

package reedsolomon

import (
	"fmt"
	"runtime"
	"unsafe"
)

//go:noescape
func mulgf16Xor8_gfni(col []byte, tables *[8 * 4]uint64, outs *[8]uintptr)

// GF16MulSliceXor8 fuses 8 scalar-broadcast GF(2^16) mul-XOR accumulates
// that share the same input slice `in`:
//
//	out_k[i] ^= scalars[k] * in[i]   for k in [0, 8)
//
// All 8 destination slices must share length with in, a non-zero multiple
// of 64 bytes (Leopard's chunk size: 32 low bytes + 32 high bytes per chunk).
// A zero scalar contributes nothing and is treated normally; callers do not
// need to filter them.
//
// On GFNI hosts the fused kernel loads each input chunk into registers once
// and applies all 8 scalars with unrolled VGF2P8AFFINEQB sequences. Other
// amd64 hosts fall back to 8 independent AVX2 mul-XORs; non-amd64 / no-asm
// builds fall back to the scalar refMulAdd.
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

	if opts.useAvxGNFI && gf2p811dMulMatrices16 != nil {
		var tables [8 * 4]uint64
		anyNonZero := false
		for k, c := range scalars {
			if c == 0 {
				// All-zero matrix: VGF2P8AFFINEQB yields zero, XOR is a no-op.
				continue
			}
			anyNonZero = true
			copy(tables[k*4:k*4+4], gf2p811dMulMatrices16[logLUT[ffe(c)]][:])
		}
		if !anyNonZero {
			return
		}
		if raceEnabled {
			raceReadSlice(in)
			for k := range outs {
				raceWriteSlice(outs[k])
			}
		}
		var ptrs [8]uintptr
		for k := range outs {
			ptrs[k] = uintptr(unsafe.Pointer(&outs[k][0]))
		}
		mulgf16Xor8_gfni(in, &tables, &ptrs)
		// outs holds the only Go-visible references to the destination
		// backing arrays; ptrs stores only uintptr so GC cannot trace
		// through it. Keep outs live across the asm call.
		runtime.KeepAlive(outs)
		return
	}

	// Fallback: per-scalar mul-XOR, reusing the AVX2/scalar mulgf16Xor.
	for k, c := range scalars {
		if c == 0 {
			continue
		}
		mulgf16Xor(outs[k], in, logLUT[ffe(c)], opts)
	}
}
