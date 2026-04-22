package reedsolomon

import (
	"fmt"
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

	sizes := []int{64, 128, 256, 512, 2048, 8192}

	scalarSets := [][8]uint16{
		{0x1234, 0, 0xFFFF, 1, 2, 0xABCD, 0x5555, 0xAAAA},
		{0, 0, 0, 0, 0, 0, 0, 0},
		{0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF},
		{1, 1, 1, 1, 1, 1, 1, 1},
		{0, 1, 0, 1, 0, 1, 0, 1},
	}

	for si, scalars := range scalarSets {
		for _, sz := range sizes {
			r := rand.New(rand.NewPCG(uint64(si*1000+sz), 42))
			in := make([]byte, sz)
			for i := range in {
				in[i] = byte(r.IntN(256))
			}

			outs, ref := newOutputPair(sz, r)
			ll.GF16MulSliceXor8(&scalars, in, &outs)
			naiveGF16MulSliceXor8(ll, &scalars, in, &ref)
			assertOutputsEqual(t, sz, outs, ref)
		}
	}
}

// TestGF16MulSliceXor8PanicsOnLengthMismatch checks that the precondition
// (every outs[k] has len(in)) is enforced at the entry of the function, so
// callers that violate it get a clear panic instead of memory corruption in
// the asm kernel.  It also verifies that no output buffer is mutated before
// the panic (the validation must happen before any work).
func TestGF16MulSliceXor8PanicsOnLengthMismatch(t *testing.T) {
	var ll LowLevel
	initConstants()

	sz := 64
	in := make([]byte, sz)
	for i := range in {
		in[i] = 0xAA
	}
	scalars := [8]uint16{1, 2, 3, 4, 5, 6, 7, 8}

	var outs [8][]byte
	for k := 0; k < 8; k++ {
		outs[k] = make([]byte, sz)
		for i := range outs[k] {
			outs[k][i] = byte(k + 1)
		}
	}
	outs[3] = outs[3][:sz-1] // wrong length

	// Snapshot every buffer except the shortened one.
	var snap [8][]byte
	for k := 0; k < 8; k++ {
		snap[k] = make([]byte, len(outs[k]))
		copy(snap[k], outs[k])
	}

	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		ll.GF16MulSliceXor8(&scalars, in, &outs)
	}()

	if !panicked {
		t.Fatal("expected panic on mismatched outs length, got nil")
	}

	for k := 0; k < 8; k++ {
		if len(outs[k]) != len(snap[k]) {
			continue // outs[3] was shortened intentionally
		}
		for i := range outs[k] {
			if outs[k][i] != snap[k][i] {
				t.Fatalf("outs[%d][%d] mutated: got=%02x want=%02x", k, i, outs[k][i], snap[k][i])
			}
		}
	}
}

// TestMulgf16Xor8 exercises each internal implementation of the fused 8-output
// mulgf16Xor8 kernel: AVX2, GFNI (AVX+GFNI), AVX512+GFNI, and pure-Go
// reference. Each path is compared against the scalar naiveGF16MulSliceXor8
// oracle. This is the test that would have caught the load-from-wrong-output
// bug in the GFNI assembly.
func TestMulgf16Xor8(t *testing.T) {
	initConstants()

	type impl struct {
		name string
		opts options
	}
	impls := []impl{
		{"pure_go", options{}},
	}
	if defaultOptions.useAVX2 {
		impls = append(impls, impl{"avx2", options{useAVX2: true}})
	}
	if defaultOptions.useAvxGNFI {
		impls = append(impls, impl{"gfni", options{useAvxGNFI: true}})
	}
	if defaultOptions.useAvx512GFNI {
		impls = append(impls, impl{"avx512gfni", options{useAvx512GFNI: true, useAVX512: true}})
	}

	sizes := []int{64, 128, 256, 512, 2048, 8192}

	scalarSets := [][8]uint16{
		{0x1234, 0, 0xFFFF, 1, 2, 0xABCD, 0x5555, 0xAAAA},
		{0, 0, 0, 0, 0, 0, 0, 0},
		{0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF},
		{1, 1, 1, 1, 1, 1, 1, 1},
		{0, 1, 0, 1, 0, 1, 0, 1},
	}

	for _, im := range impls {
		t.Run(im.name, func(t *testing.T) {
			for si, scalars := range scalarSets {
				for _, sz := range sizes {
					t.Run(fmt.Sprintf("scalars%d/sz%d", si, sz), func(t *testing.T) {
						r := rand.New(rand.NewPCG(uint64(si*1000+sz), 99))
						in := make([]byte, sz)
						for i := range in {
							in[i] = byte(r.IntN(256))
						}

						outs, ref := newOutputPair(sz, r)
						o := im.opts
						mulgf16Xor8(&scalars, in, &outs, &o)

						var ll LowLevel
						naiveGF16MulSliceXor8(ll, &scalars, in, &ref)
						assertOutputsEqual(t, sz, outs, ref)
					})
				}
			}
		})
	}
}

// TestMulgf16Xor8CrossValidate runs all available implementations on the
// same inputs and verifies they produce identical results.
func TestMulgf16Xor8CrossValidate(t *testing.T) {
	initConstants()

	type impl struct {
		name string
		opts options
	}
	impls := []impl{
		{"pure_go", options{}},
	}
	if defaultOptions.useAVX2 {
		impls = append(impls, impl{"avx2", options{useAVX2: true}})
	}
	if defaultOptions.useAvxGNFI {
		impls = append(impls, impl{"gfni", options{useAvxGNFI: true}})
	}
	if defaultOptions.useAvx512GFNI {
		impls = append(impls, impl{"avx512gfni", options{useAvx512GFNI: true, useAVX512: true}})
	}
	if len(impls) < 2 {
		t.Skip("need at least 2 implementations to cross-validate")
	}

	r := rand.New(rand.NewPCG(31, 71))
	sizes := []int{64, 128, 512, 4096}

	for _, sz := range sizes {
		in := make([]byte, sz)
		for i := range in {
			in[i] = byte(r.IntN(256))
		}
		var scalars [8]uint16
		for k := range scalars {
			scalars[k] = uint16(r.IntN(65536))
		}
		scalars[5] = 0

		// Build identical starting output sets for each impl.
		base := make([][8][]byte, len(impls))
		var seedOuts [8][]byte
		for k := 0; k < 8; k++ {
			seedOuts[k] = make([]byte, sz)
			for i := range seedOuts[k] {
				seedOuts[k][i] = byte(r.IntN(256))
			}
		}
		for idx := range impls {
			for k := 0; k < 8; k++ {
				base[idx][k] = make([]byte, sz)
				copy(base[idx][k], seedOuts[k])
			}
		}

		// Run each impl.
		for idx := range impls {
			o := impls[idx].opts
			mulgf16Xor8(&scalars, in, &base[idx], &o)
		}

		// Compare all against the first.
		for idx := 1; idx < len(impls); idx++ {
			for k := 0; k < 8; k++ {
				for i := 0; i < sz; i++ {
					if base[0][k][i] != base[idx][k][i] {
						t.Fatalf("sz=%d %s vs %s: outs[%d][%d] got=%02x want=%02x",
							sz, impls[idx].name, impls[0].name,
							k, i, base[idx][k][i], base[0][k][i])
					}
				}
			}
		}
	}
}

// TestMulgf16Xor exercises both internal paths of the mulgf16Xor kernel
// that GF16MulSliceXor8 relies on when the fused GFNI kernel is not taken:
// the AVX2 scalar-broadcast path (useAVX2=true) and the pure-Go refMulAdd
// path (useAVX2=false, matching the noasm build).
func TestMulgf16Xor(t *testing.T) {
	initConstants()

	type impl struct {
		name string
		opts options
	}
	impls := []impl{
		{"scalar", options{}},
		{"avx2", options{useAVX2: true}},
		{"ssse3", options{useSSSE3: true}},
		{"gfni", options{useAvxGNFI: true}},
	}

	for _, tc := range impls {
		t.Run(tc.name, func(t *testing.T) {
			switch tc.name {
			case "avx2":
				if !defaultOptions.useAVX2 {
					t.Skip("host does not support AVX2")
				}
			case "ssse3":
				if !defaultOptions.useSSSE3 {
					t.Skip("host does not support SSSE3")
				}
			case "gfni":
				if !defaultOptions.useAvxGNFI {
					t.Skip("host does not support AVX+GFNI")
				}
			}

			r := rand.New(rand.NewPCG(9, 11))
			sz := 256
			in := make([]byte, sz)
			for i := range in {
				in[i] = byte(r.IntN(256))
			}
			scalars := [8]uint16{0x1234, 0, 0xFFFF, 1, 2, 0xABCD, 0x5555, 0xAAAA}

			outs, ref := newOutputPair(sz, r)
			opts := tc.opts
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

func BenchmarkGF16MulSliceXor8Impls(b *testing.B) {
	initConstants()

	r := rand.New(rand.NewPCG(7, 42))
	var scalars [8]uint16
	for k := range scalars {
		scalars[k] = uint16(r.IntN(65535) + 1)
	}

	type impl struct {
		name string
		opts options
	}
	impls := []impl{
		{"pure_go", options{}},
		{"per-scalar_ssse3", options{useSSSE3: true}},
		{"per-scalar_avx2", options{useAVX2: true}},
		{"per-scalar_gfni", options{useAvxGNFI: true}},
	}
	if defaultOptions.useAVX2 {
		impls = append(impls, impl{"fused_avx2", options{useAVX2: true}})
	}
	if defaultOptions.useAvxGNFI {
		impls = append(impls, impl{"fused_gfni", options{useAvxGNFI: true}})
	}
	if defaultOptions.useAvx512GFNI {
		impls = append(impls, impl{"fused_avx512gfni", options{useAvx512GFNI: true}})
	}

	sizes := []int{64, 1024, 8192, 64 * 1024, 1024 * 1024, 64 * 1024 * 1024}

	for _, sz := range sizes {
		in := make([]byte, sz)
		for i := range in {
			in[i] = byte(r.IntN(256))
		}

		for _, im := range impls {
			skip := false
			switch im.name {
			case "per-scalar_ssse3":
				skip = !defaultOptions.useSSSE3
			case "per-scalar_avx2":
				skip = !defaultOptions.useAVX2
			case "per-scalar_gfni":
				skip = !defaultOptions.useAvxGNFI
			}
			if skip {
				continue
			}
			var outs [8][]byte
			for k := range outs {
				outs[k] = make([]byte, sz)
			}

			fused := false
			switch im.name {
			case "fused_avx2", "fused_gfni", "fused_avx512gfni":
				fused = true
			}

			b.Run(fmt.Sprintf("%d/%s", sz, im.name), func(b *testing.B) {
				b.SetBytes(int64(sz) * 8)
				o := im.opts
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if fused {
						mulgf16Xor8(&scalars, in, &outs, &o)
					} else {
						for k, c := range scalars {
							if c == 0 {
								continue
							}
							mulgf16Xor(outs[k], in, logLUT[ffe(c)], &o)
						}
					}
				}
			})
		}
	}
}

func BenchmarkGF16MulSliceXor8(b *testing.B) {
	var ll LowLevel
	initConstants()

	r := rand.New(rand.NewPCG(7, 42))
	var scalars [8]uint16
	for k := range scalars {
		scalars[k] = uint16(r.IntN(65535) + 1)
	}

	for _, sz := range []int{64, 1024, 8192, 16384, 32768, 64 * 1024, 1024 * 1024, 64 * 1024 * 1024} {
		in := make([]byte, sz)
		for i := range in {
			in[i] = byte(r.IntN(256))
		}
		var outs [8][]byte
		for k := range outs {
			outs[k] = make([]byte, sz)
		}

		b.Run(fmt.Sprintf("%d", sz), func(b *testing.B) {
			b.SetBytes(int64(sz) * 8)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ll.GF16MulSliceXor8(&scalars, in, &outs)
			}
		})
	}
}

func TestGF16MulSliceXor(t *testing.T) {
	var ll LowLevel
	initConstants()

	r := rand.New(rand.NewPCG(13, 37))
	sizes := []int{64, 128, 256, 2048}

	for _, sz := range sizes {
		in := make([]byte, sz)
		for i := range in {
			in[i] = byte(r.IntN(256))
		}

		for _, scalar := range []uint16{0, 1, 2, 0xFFFF, 0x1234} {
			out := make([]byte, sz)
			ref := make([]byte, sz)
			// Pre-fill with identical random data.
			for i := range out {
				v := byte(r.IntN(256))
				out[i] = v
				ref[i] = v
			}

			ll.GF16MulSliceXor(scalar, in, out)

			// Compute reference: ref[i] ^= scalar * in[i]
			for off := 0; off < sz; off += 64 {
				for j := 0; j < 32; j++ {
					inSym := uint16(in[off+32+j])<<8 | uint16(in[off+j])
					outSym := uint16(ref[off+32+j])<<8 | uint16(ref[off+j])
					res := outSym ^ ll.GF16Mul(scalar, inSym)
					ref[off+j] = byte(res)
					ref[off+32+j] = byte(res >> 8)
				}
			}

			for i := range out {
				if out[i] != ref[i] {
					t.Fatalf("size=%d scalar=0x%04x byte %d: got=%02x want=%02x",
						sz, scalar, i, out[i], ref[i])
				}
			}
		}
	}
}

func TestGF16MulSliceXorPanics(t *testing.T) {
	var ll LowLevel
	initConstants()

	t.Run("bad_alignment", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic")
			}
		}()
		ll.GF16MulSliceXor(1, make([]byte, 63), make([]byte, 63))
	})

	t.Run("length_mismatch", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic")
			}
		}()
		ll.GF16MulSliceXor(1, make([]byte, 64), make([]byte, 128))
	})
}

func BenchmarkGF16MulSliceXor(b *testing.B) {
	var ll LowLevel
	initConstants()

	r := rand.New(rand.NewPCG(7, 42))
	scalar := uint16(0x1234)

	for _, sz := range []int{64, 1024, 8192, 64 * 1024, 1024 * 1024, 64 * 1024 * 1024} {
		in := make([]byte, sz)
		for i := range in {
			in[i] = byte(r.IntN(256))
		}
		out := make([]byte, sz)

		b.Run(fmt.Sprintf("%d", sz), func(b *testing.B) {
			b.SetBytes(int64(sz))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ll.GF16MulSliceXor(scalar, in, out)
			}
		})
	}
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
