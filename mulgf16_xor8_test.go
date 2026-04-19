package reedsolomon

import (
	"math/rand/v2"
	"testing"
)

// naiveGF16MulSliceXor8 computes out_k[i] ^= scalars[k] * in[i] over
// Leopard-formatted chunks using the scalar GF16Mul. Used as correctness
// oracle for GF16MulSliceXor8.
func naiveGF16MulSliceXor8(ll LowLevel, scalars *[8]uint16, in []byte, outs *[8][]byte) {
	for off := 0; off < len(in); off += 64 {
		for j := 0; j < 32; j++ {
			inSym := uint16(in[off+32+j])<<8 | uint16(in[off+j])
			for k := 0; k < 8; k++ {
				dst := outs[k]
				outSym := uint16(dst[off+32+j])<<8 | uint16(dst[off+j])
				res := outSym ^ ll.GF16Mul(scalars[k], inSym)
				dst[off+j] = byte(res)
				dst[off+32+j] = byte(res >> 8)
			}
		}
	}
}

func TestGF16MulSliceXor8(t *testing.T) {
	var ll LowLevel
	initConstants()

	sizes := []int{64, 128, 256, 2048}
	r := rand.New(rand.NewPCG(7, 42))

	for _, sz := range sizes {
		in := make([]byte, sz)
		for i := range in {
			in[i] = byte(r.IntN(256))
		}
		// Include at least one zero scalar to cover the no-op short-circuit.
		var scalars [8]uint16
		for k := range scalars {
			scalars[k] = uint16(r.IntN(65536))
		}
		scalars[3] = 0

		outs, ref := newOutputPair(sz, r)
		ll.GF16MulSliceXor8(&scalars, in, &outs)
		naiveGF16MulSliceXor8(ll, &scalars, in, &ref)
		assertOutputsEqual(t, sz, outs, ref)
	}
}

// TestGF16MulSliceXor8PanicsOnLengthMismatch checks that the precondition
// (every outs[k] has len(in)) is enforced at the entry of the function, so
// callers that violate it get a clear panic instead of memory corruption in
// the asm kernel.
func TestGF16MulSliceXor8PanicsOnLengthMismatch(t *testing.T) {
	var ll LowLevel
	initConstants()

	sz := 64
	in := make([]byte, sz)
	scalars := [8]uint16{1, 2, 3, 4, 5, 6, 7, 8}

	var outs [8][]byte
	for k := 0; k < 8; k++ {
		outs[k] = make([]byte, sz)
	}
	outs[3] = outs[3][:sz-1] // wrong length

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on mismatched outs length, got nil")
		}
	}()
	ll.GF16MulSliceXor8(&scalars, in, &outs)
}

// TestMulgf16Xor exercises both internal paths of the mulgf16Xor kernel
// that GF16MulSliceXor8 relies on when the fused GFNI kernel is not taken:
// the AVX2 scalar-broadcast path (useAVX2=true) and the pure-Go refMulAdd
// path (useAVX2=false, matching the noasm build).
func TestMulgf16Xor(t *testing.T) {
	initConstants()

	for _, tc := range []struct {
		name    string
		useAVX2 bool
	}{
		{"avx2", true},
		{"scalar", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.useAVX2 && !defaultOptions.useAVX2 {
				t.Skip("host does not support AVX2")
			}
			opts := defaultOptions
			opts.useAVX2 = tc.useAVX2

			r := rand.New(rand.NewPCG(9, 11))
			sz := 256
			in := make([]byte, sz)
			for i := range in {
				in[i] = byte(r.IntN(256))
			}
			scalars := [8]uint16{0x1234, 0, 0xFFFF, 1, 2, 0xABCD, 0x5555, 0xAAAA}

			outs, ref := newOutputPair(sz, r)
			for k, c := range scalars {
				if c == 0 {
					continue
				}
				mulgf16Xor(outs[k], in, logLUT[ffe(c)], &opts)
			}

			var ll LowLevel
			naiveGF16MulSliceXor8(ll, &scalars, in, &ref)
			assertOutputsEqual(t, sz, outs, ref)
		})
	}
}

// newOutputPair returns two identical pairs of 8 destination buffers of the
// requested size, each pre-filled with random bytes.
func newOutputPair(sz int, r *rand.Rand) (a, b [8][]byte) {
	for k := 0; k < 8; k++ {
		a[k] = make([]byte, sz)
		b[k] = make([]byte, sz)
		for i := 0; i < sz; i++ {
			v := byte(r.IntN(256))
			a[k][i] = v
			b[k][i] = v
		}
	}
	return a, b
}

func assertOutputsEqual(t *testing.T, sz int, got, want [8][]byte) {
	t.Helper()
	for k := 0; k < 8; k++ {
		for i := 0; i < sz; i++ {
			if got[k][i] != want[k][i] {
				t.Fatalf("size=%d k=%d byte %d: got=%02x want=%02x",
					sz, k, i, got[k][i], want[k][i])
			}
		}
	}
}
