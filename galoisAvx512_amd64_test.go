//+build !noasm
//+build !appengine
//+build !gccgo

// Copyright 2015, Klaus Post, see LICENSE for details.
// Copyright 2019, Minio, Inc.

package reedsolomon

import (
	"bytes"
	"math/rand"
	"testing"
	"time"
)

func testGaloisAvx512Parallelx2(t *testing.T, inputSize int) {

	if !defaultOptions.useAVX512 {
		return
	}

	rand.Seed(time.Now().UnixNano())

	const size = 1024 * 1024

	in, out := make([][]byte, inputSize), make([][]byte, dimOut82)

	for i := range in {
		in[i] = make([]byte, size)
		rand.Read(in[i])
	}

	for i := range out {
		out[i] = make([]byte, size)
		rand.Read(out[i])
	}

	opts := defaultOptions
	opts.useSSSE3 = true

	matrix := [(16 + 16) * dimIn * dimOut82]byte{}
	coeffs := make([]byte, dimIn*len(out))

	for i := 0; i < dimIn*len(out); i++ {
		coeffs[i] = byte(rand.Int31n(256))
		copy(matrix[i*32:], mulTableLow[coeffs[i]][:])
		copy(matrix[i*32+16:], mulTableHigh[coeffs[i]][:])
	}

	// Do first run with clearing out any existing results
	_galMulAVX512Parallel82(in, out, &matrix, false)

	expect := make([][]byte, len(out))
	for i := range expect {
		expect[i] = make([]byte, size)
		rand.Read(expect[i])
	}

	for i := range in {
		if i == 0 {
			galMulSlice(coeffs[i], in[i], expect[0], &options{})
			galMulSlice(coeffs[dimIn+i], in[i], expect[1], &options{})
		} else {
			galMulSliceXor(coeffs[i], in[i], expect[0], &options{})
			galMulSliceXor(coeffs[dimIn+i], in[i], expect[1], &options{})
		}
	}

	for i := range out {
		if 0 != bytes.Compare(out[i], expect[i]) {
			t.Errorf("got [%d]%#v...,\n                  expected [%d]%#v...", i, out[i][:8], i, expect[i][:8])
		}
	}

	inToAdd := make([][]byte, len(in))

	for i := range inToAdd {
		inToAdd[i] = make([]byte, size)
		rand.Read(inToAdd[i])
	}

	for i := 0; i < dimIn*len(out); i++ {
		coeffs[i] = byte(rand.Int31n(256))
		copy(matrix[i*32:], mulTableLow[coeffs[i]][:])
		copy(matrix[i*32+16:], mulTableHigh[coeffs[i]][:])
	}

	// Do second run by adding to original run
	_galMulAVX512Parallel82(inToAdd, out, &matrix, true)

	for i := range in {
		galMulSliceXor(coeffs[i], inToAdd[i], expect[0], &options{})
		galMulSliceXor(coeffs[dimIn+i], inToAdd[i], expect[1], &options{})
	}

	for i := range out {
		if 0 != bytes.Compare(out[i], expect[i]) {
			t.Errorf("got [%d]%#v...,\n                  expected [%d]%#v...", i, out[i][:8], i, expect[i][:8])
		}
	}
}

func TestGaloisAvx512Parallel12(t *testing.T) { testGaloisAvx512Parallelx2(t, 1) }
func TestGaloisAvx512Parallel22(t *testing.T) { testGaloisAvx512Parallelx2(t, 2) }
func TestGaloisAvx512Parallel32(t *testing.T) { testGaloisAvx512Parallelx2(t, 3) }
func TestGaloisAvx512Parallel42(t *testing.T) { testGaloisAvx512Parallelx2(t, 4) }
func TestGaloisAvx512Parallel52(t *testing.T) { testGaloisAvx512Parallelx2(t, 5) }
func TestGaloisAvx512Parallel62(t *testing.T) { testGaloisAvx512Parallelx2(t, 6) }
func TestGaloisAvx512Parallel72(t *testing.T) { testGaloisAvx512Parallelx2(t, 7) }
func TestGaloisAvx512Parallel82(t *testing.T) { testGaloisAvx512Parallelx2(t, 8) }

func testGaloisAvx512Parallelx4(t *testing.T, inputSize int) {

	if !defaultOptions.useAVX512 {
		return
	}

	rand.Seed(time.Now().UnixNano())

	const size = 1024 * 1024

	in, out := make([][]byte, inputSize), make([][]byte, dimOut84)

	for i := range in {
		in[i] = make([]byte, size)
		rand.Read(in[i])
	}

	for i := range out {
		out[i] = make([]byte, size)
		rand.Read(out[i])
	}

	opts := defaultOptions
	opts.useSSSE3 = true

	matrix := [(16 + 16) * dimIn * dimOut84]byte{}
	coeffs := make([]byte, dimIn*len(out))

	for i := 0; i < dimIn*len(out); i++ {
		coeffs[i] = byte(rand.Int31n(256))
		copy(matrix[i*32:], mulTableLow[coeffs[i]][:])
		copy(matrix[i*32+16:], mulTableHigh[coeffs[i]][:])
	}

	// Do first run with clearing out any existing results
	_galMulAVX512Parallel84(in, out, &matrix, false)

	expect := make([][]byte, 4)
	for i := range expect {
		expect[i] = make([]byte, size)
		rand.Read(expect[i])
	}

	for i := range in {
		if i == 0 {
			galMulSlice(coeffs[i], in[i], expect[0], &options{})
			galMulSlice(coeffs[dimIn+i], in[i], expect[1], &options{})
			galMulSlice(coeffs[dimIn*2+i], in[i], expect[2], &options{})
			galMulSlice(coeffs[dimIn*3+i], in[i], expect[3], &options{})
		} else {
			galMulSliceXor(coeffs[i], in[i], expect[0], &options{})
			galMulSliceXor(coeffs[dimIn+i], in[i], expect[1], &options{})
			galMulSliceXor(coeffs[dimIn*2+i], in[i], expect[2], &options{})
			galMulSliceXor(coeffs[dimIn*3+i], in[i], expect[3], &options{})
		}
	}

	for i := range out {
		if 0 != bytes.Compare(out[i], expect[i]) {
			t.Errorf("got [%d]%#v...,\n                  expected [%d]%#v...", i, out[i][:8], i, expect[i][:8])
		}
	}

	inToAdd := make([][]byte, len(in))

	for i := range inToAdd {
		inToAdd[i] = make([]byte, size)
		rand.Read(inToAdd[i])
	}

	for i := 0; i < dimIn*len(out); i++ {
		coeffs[i] = byte(rand.Int31n(256))
		copy(matrix[i*32:], mulTableLow[coeffs[i]][:])
		copy(matrix[i*32+16:], mulTableHigh[coeffs[i]][:])
	}

	// Do second run by adding to original run
	_galMulAVX512Parallel84(inToAdd, out, &matrix, true)

	for i := range in {
		galMulSliceXor(coeffs[i], inToAdd[i], expect[0], &options{})
		galMulSliceXor(coeffs[dimIn+i], inToAdd[i], expect[1], &options{})
		galMulSliceXor(coeffs[dimIn*2+i], inToAdd[i], expect[2], &options{})
		galMulSliceXor(coeffs[dimIn*3+i], inToAdd[i], expect[3], &options{})
	}

	for i := range out {
		if 0 != bytes.Compare(out[i], expect[i]) {
			t.Errorf("got [%d]%#v...,\n                  expected [%d]%#v...", i, out[i][:8], i, expect[i][:8])
		}
	}
}

func TestGaloisAvx512Parallel14(t *testing.T) { testGaloisAvx512Parallelx4(t, 1) }
func TestGaloisAvx512Parallel24(t *testing.T) { testGaloisAvx512Parallelx4(t, 2) }
func TestGaloisAvx512Parallel34(t *testing.T) { testGaloisAvx512Parallelx4(t, 3) }
func TestGaloisAvx512Parallel44(t *testing.T) { testGaloisAvx512Parallelx4(t, 4) }
func TestGaloisAvx512Parallel54(t *testing.T) { testGaloisAvx512Parallelx4(t, 5) }
func TestGaloisAvx512Parallel64(t *testing.T) { testGaloisAvx512Parallelx4(t, 6) }
func TestGaloisAvx512Parallel74(t *testing.T) { testGaloisAvx512Parallelx4(t, 7) }
func TestGaloisAvx512Parallel84(t *testing.T) { testGaloisAvx512Parallelx4(t, 8) }

func testCodeSomeShardsAvx512WithLength(t *testing.T, ds, ps, l int) {

	if !defaultOptions.useAVX512 {
		return
	}

	var data = make([]byte, l)
	fillRandom(data)
	enc, _ := New(ds, ps)
	r := enc.(*reedSolomon) // need to access private methods
	shards, _ := enc.Split(data)

	// Fill shards to encode with garbage
	for i := r.DataShards; i < r.DataShards+r.ParityShards; i++ {
		rand.Read(shards[i])
	}

	r.codeSomeShardsAvx512(r.parity, shards[:r.DataShards], shards[r.DataShards:], r.ParityShards, len(shards[0]))

	correct, _ := r.Verify(shards)
	if !correct {
		t.Errorf("Verification of encoded shards failed")
	}
}

func testCodeSomeShardsAvx512(t *testing.T, ds, ps int) {

	if !defaultOptions.useAVX512 {
		return
	}

	for l := 1; l <= 8192; l++ {
		testCodeSomeShardsAvx512WithLength(t, ds, ps, l)
	}
}

func TestCodeSomeShardsAvx512_8x2(t *testing.T)  { testCodeSomeShardsAvx512(t, 8, 2) }
func TestCodeSomeShardsAvx512_1x4(t *testing.T)  { testCodeSomeShardsAvx512(t, 1, 4) }
func TestCodeSomeShardsAvx512_2x4(t *testing.T)  { testCodeSomeShardsAvx512(t, 2, 4) }
func TestCodeSomeShardsAvx512_3x4(t *testing.T)  { testCodeSomeShardsAvx512(t, 3, 4) }
func TestCodeSomeShardsAvx512_4x4(t *testing.T)  { testCodeSomeShardsAvx512(t, 4, 4) }
func TestCodeSomeShardsAvx512_5x4(t *testing.T)  { testCodeSomeShardsAvx512(t, 5, 4) }
func TestCodeSomeShardsAvx512_6x4(t *testing.T)  { testCodeSomeShardsAvx512(t, 6, 4) }
func TestCodeSomeShardsAvx512_7x4(t *testing.T)  { testCodeSomeShardsAvx512(t, 7, 4) }
func TestCodeSomeShardsAvx512_8x4(t *testing.T)  { testCodeSomeShardsAvx512(t, 8, 4) }
func TestCodeSomeShardsAvx512_9x4(t *testing.T)  { testCodeSomeShardsAvx512(t, 9, 4) }
func TestCodeSomeShardsAvx512_10x4(t *testing.T) { testCodeSomeShardsAvx512(t, 10, 4) }
func TestCodeSomeShardsAvx512_12x4(t *testing.T) { testCodeSomeShardsAvx512(t, 12, 4) }
func TestCodeSomeShardsAvx512_16x4(t *testing.T) { testCodeSomeShardsAvx512(t, 16, 4) }
func TestCodeSomeShardsAvx512_3x6(t *testing.T)  { testCodeSomeShardsAvx512(t, 3, 6) }
func TestCodeSomeShardsAvx512_8x6(t *testing.T)  { testCodeSomeShardsAvx512(t, 8, 6) }
func TestCodeSomeShardsAvx512_8x7(t *testing.T)  { testCodeSomeShardsAvx512(t, 8, 7) }
func TestCodeSomeShardsAvx512_3x8(t *testing.T)  { testCodeSomeShardsAvx512(t, 3, 8) }
func TestCodeSomeShardsAvx512_8x8(t *testing.T)  { testCodeSomeShardsAvx512(t, 8, 8) }
func TestCodeSomeShardsAvx512_5x10(t *testing.T) { testCodeSomeShardsAvx512(t, 5, 10) }
func TestCodeSomeShardsAvx512_8x10(t *testing.T) { testCodeSomeShardsAvx512(t, 8, 10) }
func TestCodeSomeShardsAvx512_9x10(t *testing.T) { testCodeSomeShardsAvx512(t, 9, 10) }

func TestCodeSomeShardsAvx512_Manyx4(t *testing.T) {

	if !defaultOptions.useAVX512 {
		return
	}

	for inputs := 1; inputs <= 200; inputs++ {
		testCodeSomeShardsAvx512WithLength(t, inputs, 4, 1024+33)
	}
}

func TestCodeSomeShardsAvx512_ManyxMany(t *testing.T) {

	if !defaultOptions.useAVX512 {
		return
	}

	for outputs := 1; outputs <= 32; outputs++ {
		for inputs := 1; inputs <= 32; inputs++ {
			testCodeSomeShardsAvx512WithLength(t, inputs, outputs, 1024+33)
		}
	}
}

func benchmarkAvx512Encode(b *testing.B, dataShards, parityShards, shardSize int) {

	if !defaultOptions.useAVX512 {
		return
	}

	enc, err := New(dataShards, parityShards)
	if err != nil {
		b.Fatal(err)
	}
	r := enc.(*reedSolomon) // need to access private methods
	shards := make([][]byte, dataShards+parityShards)
	for s := range shards {
		shards[s] = make([]byte, shardSize)
	}

	rand.Seed(0)
	for s := 0; s < dataShards; s++ {
		fillRandom(shards[s])
	}

	b.SetBytes(int64(shardSize * dataShards))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Do the coding.
		r.codeSomeShardsAvx512(r.parity, shards[0:r.DataShards], shards[r.DataShards:], r.ParityShards, len(shards[0]))
	}
}

// Benchmark various combination of data shards and parity shards for AVX512 accelerated code
func BenchmarkEncodeAvx512_8x4x8M(b *testing.B)   { benchmarkAvx512Encode(b, 8, 4, 8*1024*1024) }
func BenchmarkEncodeAvx512_12x4x12M(b *testing.B) { benchmarkAvx512Encode(b, 12, 4, 12*1024*1024) }
func BenchmarkEncodeAvx512_16x4x16M(b *testing.B) { benchmarkAvx512Encode(b, 16, 4, 16*1024*1024) }
func BenchmarkEncodeAvx512_16x4x32M(b *testing.B) { benchmarkAvx512Encode(b, 16, 4, 32*1024*1024) }
func BenchmarkEncodeAvx512_16x4x64M(b *testing.B) { benchmarkAvx512Encode(b, 16, 4, 64*1024*1024) }

func BenchmarkEncodeAvx512_8x5x8M(b *testing.B)  { benchmarkAvx512Encode(b, 8, 5, 8*1024*1024) }
func BenchmarkEncodeAvx512_8x6x8M(b *testing.B)  { benchmarkAvx512Encode(b, 8, 6, 8*1024*1024) }
func BenchmarkEncodeAvx512_8x7x8M(b *testing.B)  { benchmarkAvx512Encode(b, 8, 7, 8*1024*1024) }
func BenchmarkEncodeAvx512_8x9x8M(b *testing.B)  { benchmarkAvx512Encode(b, 8, 9, 8*1024*1024) }
func BenchmarkEncodeAvx512_8x10x8M(b *testing.B) { benchmarkAvx512Encode(b, 8, 10, 8*1024*1024) }
func BenchmarkEncodeAvx512_8x11x8M(b *testing.B) { benchmarkAvx512Encode(b, 8, 11, 8*1024*1024) }

func BenchmarkEncodeAvx512_8x8x05M(b *testing.B) { benchmarkAvx512Encode(b, 8, 8, 1*1024*1024/2) }
func BenchmarkEncodeAvx512_8x8x1M(b *testing.B)  { benchmarkAvx512Encode(b, 8, 8, 1*1024*1024) }
func BenchmarkEncodeAvx512_8x8x8M(b *testing.B)  { benchmarkAvx512Encode(b, 8, 8, 8*1024*1024) }
func BenchmarkEncodeAvx512_8x8x32M(b *testing.B) { benchmarkAvx512Encode(b, 8, 8, 32*1024*1024) }

func BenchmarkEncodeAvx512_24x8x24M(b *testing.B) { benchmarkAvx512Encode(b, 24, 8, 24*1024*1024) }
func BenchmarkEncodeAvx512_24x8x48M(b *testing.B) { benchmarkAvx512Encode(b, 24, 8, 48*1024*1024) }
