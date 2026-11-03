//go:build !noasm && !appengine && !gccgo && !nopshufb

package reedsolomon

import (
	"bytes"
	"math/rand"
	"testing"
)

// TestLeopardNEONKernels checks the GF(2^16) NEON kernels against the
// reference Go implementations for random log_m values and sizes.
func TestLeopardNEONKernels(t *testing.T) {
	initConstants()
	if multiply256LUT == nil {
		t.Skip("multiply256LUT not initialized")
	}
	rng := rand.New(rand.NewSource(1))
	for iter := 0; iter < 2000; iter++ {
		logM := ffe(rng.Intn(modulus))
		if iter < 4 {
			logM = ffe(iter)
		}
		n := 64 * (1 + rng.Intn(64))
		x := make([]byte, n)
		y := make([]byte, n)
		rng.Read(x)
		rng.Read(y)
		tbl := &multiply256LUT[logM]

		// mulgf16: x = y*m
		gotMul := make([]byte, n)
		wantMul := make([]byte, n)
		mulgf16NEON(gotMul, y, tbl)
		refMul(wantMul, y, logM)
		if !bytes.Equal(gotMul, wantMul) {
			t.Fatalf("mulgf16NEON mismatch log_m=%d n=%d", logM, n)
		}

		// mulgf16Xor: x ^= y*m
		gotXor := bytes.Clone(x)
		wantXor := bytes.Clone(x)
		mulgf16XorNEON(gotXor, y, tbl)
		refMulAdd(wantXor, y, logM)
		if !bytes.Equal(gotXor, wantXor) {
			t.Fatalf("mulgf16XorNEON mismatch log_m=%d n=%d", logM, n)
		}

		// ifftDIT2: y ^= x; x ^= y*m
		gx, gy := bytes.Clone(x), bytes.Clone(y)
		wx, wy := bytes.Clone(x), bytes.Clone(y)
		ifftDIT2NEON(gx, gy, tbl)
		sliceXorGo(wx, wy, nil)
		refMulAdd(wx, wy, logM)
		if !bytes.Equal(gx, wx) || !bytes.Equal(gy, wy) {
			t.Fatalf("ifftDIT2NEON mismatch log_m=%d n=%d", logM, n)
		}

		// fftDIT2: x ^= y*m; y ^= x
		gx, gy = bytes.Clone(x), bytes.Clone(y)
		wx, wy = bytes.Clone(x), bytes.Clone(y)
		fftDIT2NEON(gx, gy, tbl)
		refMulAdd(wx, wy, logM)
		sliceXorGo(wx, wy, nil)
		if !bytes.Equal(gx, wx) || !bytes.Equal(gy, wy) {
			t.Fatalf("fftDIT2NEON mismatch log_m=%d n=%d", logM, n)
		}
	}
}

func TestLeopardNEONKernelBoundaries(t *testing.T) {
	initConstants()
	if !defaultOptions.useNEON {
		t.Skip("NEON not available")
	}
	rng := rand.New(rand.NewSource(8))
	for _, n := range []int{0, 64, 192, 4096, 25664} {
		for _, offset := range []int{0, 1, 15, 31, 63} {
			for _, logM := range []ffe{0, 1, 2, modulus - 1} {
				for kind := range 4 {
					xbuf, ybuf := make([]byte, n+128), make([]byte, n+128)
					rng.Read(xbuf)
					rng.Read(ybuf)
					xbefore, ybefore := bytes.Clone(xbuf), bytes.Clone(ybuf)
					x, y := xbuf[offset:offset+n], ybuf[offset:offset+n]
					wx, wy := bytes.Clone(x), bytes.Clone(y)
					table := &multiply256LUT[logM]
					switch kind {
					case 0:
						mulgf16NEON(x, y, table)
						refMul(wx, wy, logM)
					case 1:
						mulgf16XorNEON(x, y, table)
						refMulAdd(wx, wy, logM)
					case 2:
						fftDIT2NEON(x, y, table)
						refMulAdd(wx, wy, logM)
						sliceXorGo(wx, wy, nil)
					case 3:
						ifftDIT2NEON(x, y, table)
						sliceXorGo(wx, wy, nil)
						refMulAdd(wx, wy, logM)
					}
					if !bytes.Equal(x, wx) || !bytes.Equal(y, wy) ||
						!bytes.Equal(xbuf[:offset], xbefore[:offset]) ||
						!bytes.Equal(xbuf[offset+n:], xbefore[offset+n:]) ||
						!bytes.Equal(ybuf[:offset], ybefore[:offset]) ||
						!bytes.Equal(ybuf[offset+n:], ybefore[offset+n:]) {
						t.Fatalf("boundary mismatch: kind=%d n=%d offset=%d log=%d", kind, n, offset, logM)
					}
				}
			}
		}
	}
}

// TestLeopardNEONMulXor8 checks the 8-way accumulate wrapper against the
// reference, including zero scalars.
func TestLeopardNEONMulXor8(t *testing.T) {
	initConstants()
	if multiply256LUT == nil {
		t.Skip("multiply256LUT not initialized")
	}
	rng := rand.New(rand.NewSource(2))
	o := defaultOptions
	for iter := 0; iter < 200; iter++ {
		n := 64 * (1 + rng.Intn(32))
		in := make([]byte, n)
		rng.Read(in)
		var scalars [8]uint16
		var got, want [8][]byte
		for k := range scalars {
			if rng.Intn(4) != 0 {
				scalars[k] = uint16(rng.Intn(1 << 16))
			}
			got[k] = make([]byte, n)
			rng.Read(got[k])
			want[k] = bytes.Clone(got[k])
		}
		mulgf16Xor8(&scalars, in, &got, &o)
		refMulAdd8x(&scalars, in, &want)
		for k := range got {
			if !bytes.Equal(got[k], want[k]) {
				t.Fatalf("mulgf16Xor8 mismatch k=%d scalar=%d", k, scalars[k])
			}
		}
	}
}

// TestLeopardNEONEncodeMatchesReference encodes and reconstructs with the
// NEON path and with NEON disabled (pure-Go reference) and requires
// byte-identical shards. Shapes include a large 4096+12288.
func TestLeopardNEONEncodeMatchesReference(t *testing.T) {
	if !defaultOptions.useNEON {
		t.Skip("NEON not available")
	}
	shapes := []struct{ data, parity, size int }{
		{4, 12, 64},
		{64, 192, 1024},
		{300, 700, 320},
		{4096, 12288, 256},
		{4096, 12288, 25664},
	}
	for _, sh := range shapes {
		neon, err := New(sh.data, sh.parity, WithLeopardGF16(true))
		if err != nil {
			t.Fatal(err)
		}
		ref, err := New(sh.data, sh.parity, WithLeopardGF16(true), WithNEON(false))
		if err != nil {
			t.Fatal(err)
		}
		rng := rand.New(rand.NewSource(int64(sh.data)))
		a := make([][]byte, sh.data+sh.parity)
		b := make([][]byte, sh.data+sh.parity)
		for i := range a {
			a[i] = make([]byte, sh.size)
			if i < sh.data {
				rng.Read(a[i])
			}
			b[i] = bytes.Clone(a[i])
		}
		if err := neon.Encode(a); err != nil {
			t.Fatal(err)
		}
		if err := ref.Encode(b); err != nil {
			t.Fatal(err)
		}
		for i := range a {
			if !bytes.Equal(a[i], b[i]) {
				t.Fatalf("shape %+v: shard %d differs between NEON and reference encode", sh, i)
			}
		}
		// Drop every other data shard and a slice of parity; reconstruct both ways.
		for i := 0; i < sh.data; i += 2 {
			a[i], b[i] = nil, nil
		}
		for i := sh.data; i < sh.data+sh.parity/3; i++ {
			a[i], b[i] = nil, nil
		}
		if err := neon.Reconstruct(a); err != nil {
			t.Fatal(err)
		}
		if err := ref.Reconstruct(b); err != nil {
			t.Fatal(err)
		}
		for i := range a {
			if !bytes.Equal(a[i], b[i]) {
				t.Fatalf("shape %+v: shard %d differs between NEON and reference reconstruct", sh, i)
			}
		}
	}
}

func BenchmarkLeopardGF16Encode4096x12288(b *testing.B) {
	for _, neon := range []bool{false, true} {
		name := "ref"
		if neon {
			name = "neon"
		}
		b.Run(name, func(b *testing.B) {
			enc, err := New(4096, 12288, WithLeopardGF16(true), WithNEON(neon))
			if err != nil {
				b.Fatal(err)
			}
			const size = 25600
			shards := make([][]byte, 16384)
			for i := range shards {
				shards[i] = make([]byte, size)
			}
			rand.New(rand.NewSource(1)).Read(shards[0])
			b.SetBytes(int64(4096 * size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := enc.Encode(shards); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Leopard GF16 encode runs on one goroutine regardless of the goroutine
// options; this benchmark documents that they have no effect on its speed.
func BenchmarkLeopardGF16EncodeGoroutineOpts(b *testing.B) {
	for _, opt := range []struct {
		name string
		opts []Option
	}{
		{"default", nil},
		{"maxGoroutines16", []Option{WithMaxGoroutines(16)}},
		{"autoGoroutines25600", []Option{WithAutoGoroutines(25600)}},
	} {
		b.Run(opt.name, func(b *testing.B) {
			enc, err := New(4096, 12288, append([]Option{WithLeopardGF16(true)}, opt.opts...)...)
			if err != nil {
				b.Fatal(err)
			}
			const size = 25600
			shards := make([][]byte, 16384)
			for i := range shards {
				shards[i] = make([]byte, size)
			}
			rand.New(rand.NewSource(1)).Read(shards[0])
			b.SetBytes(int64(4096 * size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := enc.Encode(shards); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestLeopardNEONMulXor8InputAlias checks that mulgf16Xor8 stays
// byte-identical to refMulAdd8x when an output aliases the input. The
// per-scalar NEON loop multiplies in by each scalar in turn, so an aliasing
// output would mutate in before later scalars read it; mulgf16Xor8 detects
// the overlap and falls back to the reference.
func TestLeopardNEONMulXor8InputAlias(t *testing.T) {
	initConstants()
	if !defaultOptions.useNEON {
		t.Skip("NEON not available")
	}
	o := defaultOptions
	rng := rand.New(rand.NewSource(7))
	for _, n := range []int{64, 1024, 4096} {
		for _, aliasK := range []int{0, 3, 7} {
			inSeed := make([]byte, n)
			rng.Read(inSeed)
			var outSeed [8][]byte
			var scalars [8]uint16
			for k := range scalars {
				scalars[k] = uint16(1 + rng.Intn(0xffff))
				outSeed[k] = make([]byte, n)
				rng.Read(outSeed[k])
			}
			build := func() ([]byte, [8][]byte) {
				in := bytes.Clone(inSeed)
				var outs [8][]byte
				for k := range outs {
					if k == aliasK {
						outs[k] = in // alias the input
					} else {
						outs[k] = bytes.Clone(outSeed[k])
					}
				}
				return in, outs
			}
			gotIn, got := build()
			wantIn, want := build()
			mulgf16Xor8(&scalars, gotIn, &got, &o)
			refMulAdd8x(&scalars, wantIn, &want)
			for k := range got {
				if !bytes.Equal(got[k], want[k]) {
					t.Fatalf("n=%d aliasK=%d out[%d] mismatch", n, aliasK, k)
				}
			}
		}
	}
}

// withSHA3 toggles the EOR3 kernels; test-only.
func withSHA3(enabled bool) Option {
	return func(o *options) {
		o.useSHA3 = enabled && defaultOptions.useSHA3
	}
}

func goldenArchVariants() []struct {
	name string
	opts []Option
	fast bool
} {
	return []struct {
		name string
		opts []Option
		fast bool
	}{
		{"noSHA3", []Option{withSHA3(false)}, true},
	}
}

type gf16Kernel struct {
	name string
	fn   func(x, y []byte, table *[128]uint8)
	ref  func(x, y []byte, logM ffe)
}

// gf16Kernels lists every NEON GF16 kernel with its reference; the EOR3
// variants are included only when the CPU supports SHA3.
func gf16Kernels() []gf16Kernel {
	refMulXor := func(x, y []byte, logM ffe) { refMulAdd(x, y, logM) }
	refFFT := func(x, y []byte, logM ffe) { refMulAdd(x, y, logM); sliceXorGo(x, y, nil) }
	refIFFT := func(x, y []byte, logM ffe) { sliceXorGo(x, y, nil); refMulAdd(x, y, logM) }
	ks := []gf16Kernel{
		{"mulgf16NEON", mulgf16NEON, func(x, y []byte, logM ffe) { refMul(x, y, logM) }},
		{"mulgf16XorNEON", mulgf16XorNEON, refMulXor},
		{"fftDIT2NEON", fftDIT2NEON, refFFT},
		{"ifftDIT2NEON", ifftDIT2NEON, refIFFT},
	}
	if defaultOptions.useSHA3 {
		ks = append(ks,
			gf16Kernel{"mulgf16NEON3", mulgf16NEON3, func(x, y []byte, logM ffe) { refMul(x, y, logM) }},
			gf16Kernel{"mulgf16XorNEON3", mulgf16XorNEON3, refMulXor},
			gf16Kernel{"fftDIT2NEON3", fftDIT2NEON3, refFFT},
			gf16Kernel{"ifftDIT2NEON3", ifftDIT2NEON3, refIFFT},
		)
	}
	return ks
}

// TestLeopardGF16KernelsVsReference checks every kernel against its
// reference for random log_m, sizes and unaligned offsets, and that bytes
// outside the given slices are untouched.
func TestLeopardGF16KernelsVsReference(t *testing.T) {
	initConstants()
	if !defaultOptions.useNEON {
		t.Skip("NEON not available")
	}
	rng := rand.New(rand.NewSource(11))
	for _, k := range gf16Kernels() {
		t.Run(k.name, func(t *testing.T) {
			for iter := 0; iter < 400; iter++ {
				logM := ffe(rng.Intn(modulus))
				switch iter {
				case 0, 1, 2:
					logM = ffe(iter)
				case 3:
					logM = modulus - 1
				}
				n := 64 * rng.Intn(48)
				offset := []int{0, 1, 15, 31, 63}[rng.Intn(5)]
				xbuf, ybuf := make([]byte, n+128), make([]byte, n+128)
				rng.Read(xbuf)
				rng.Read(ybuf)
				xbefore, ybefore := bytes.Clone(xbuf), bytes.Clone(ybuf)
				x, y := xbuf[offset:offset+n], ybuf[offset:offset+n]
				wx, wy := bytes.Clone(x), bytes.Clone(y)
				k.fn(x, y, &multiply256LUT[logM])
				k.ref(wx, wy, logM)
				if !bytes.Equal(x, wx) || !bytes.Equal(y, wy) {
					t.Fatalf("mismatch log_m=%d n=%d offset=%d", logM, n, offset)
				}
				if !bytes.Equal(xbuf[:offset], xbefore[:offset]) || !bytes.Equal(xbuf[offset+n:], xbefore[offset+n:]) ||
					!bytes.Equal(ybuf[:offset], ybefore[:offset]) || !bytes.Equal(ybuf[offset+n:], ybefore[offset+n:]) {
					t.Fatalf("wrote outside slice: log_m=%d n=%d offset=%d", logM, n, offset)
				}
			}
		})
	}
}
