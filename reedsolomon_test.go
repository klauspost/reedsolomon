/**
 * Unit tests for ReedSolomon
 *
 * Copyright 2015, Klaus Post
 * Copyright 2015, Backblaze, Inc.  All rights reserved.
 */

package reedsolomon

import (
	"bytes"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"sync"
	"testing"
)

var noSSE2 = flag.Bool("no-sse2", !defaultOptions.useSSE2, "Disable SSE2")
var noSSSE3 = flag.Bool("no-ssse3", !defaultOptions.useSSSE3, "Disable SSSE3")
var noAVX2 = flag.Bool("no-avx2", !defaultOptions.useAVX2, "Disable AVX2")
var noAVX512 = flag.Bool("no-avx512", !defaultOptions.useAVX512, "Disable AVX512")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func testOptions(o ...Option) []Option {
	if *noSSSE3 {
		o = append(o, withSSSE3(false))
	}
	if *noSSE2 {
		o = append(o, withSSE2(false))
	}
	if *noAVX2 {
		o = append(o, withAVX2(false))
	}
	if *noAVX512 {
		o = append(o, withAVX512(false))
	}
	return o
}

func isIncreasingAndContainsDataRow(indices []int) bool {
	cols := len(indices)
	for i := 0; i < cols-1; i++ {
		if indices[i] >= indices[i+1] {
			return false
		}
	}
	// Data rows are in the upper square portion of the matrix.
	return indices[0] < cols
}

func incrementIndices(indices []int, indexBound int) (valid bool) {
	for i := len(indices) - 1; i >= 0; i-- {
		indices[i]++
		if indices[i] < indexBound {
			break
		}

		if i == 0 {
			return false
		}

		indices[i] = 0
	}

	return true
}

func incrementIndicesUntilIncreasingAndContainsDataRow(
	indices []int, maxIndex int) bool {
	for {
		valid := incrementIndices(indices, maxIndex)
		if !valid {
			return false
		}

		if isIncreasingAndContainsDataRow(indices) {
			return true
		}
	}
}

func findSingularSubMatrix(m matrix) (matrix, error) {
	rows := len(m)
	cols := len(m[0])
	rowIndices := make([]int, cols)
	for incrementIndicesUntilIncreasingAndContainsDataRow(rowIndices, rows) {
		subMatrix, _ := newMatrix(cols, cols)
		for i, r := range rowIndices {
			for c := 0; c < cols; c++ {
				subMatrix[i][c] = m[r][c]
			}
		}

		_, err := subMatrix.Invert()
		if err == errSingular {
			return subMatrix, nil
		} else if err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func TestBuildMatrixPAR1Singular(t *testing.T) {
	totalShards := 8
	dataShards := 4
	m, err := buildMatrixPAR1(dataShards, totalShards)
	if err != nil {
		t.Fatal(err)
	}

	singularSubMatrix, err := findSingularSubMatrix(m)
	if err != nil {
		t.Fatal(err)
	}

	if singularSubMatrix == nil {
		t.Fatal("No singular sub-matrix found")
	}

	t.Logf("matrix %s has singular sub-matrix %s", m, singularSubMatrix)
}

func testOpts() [][]Option {
	if testing.Short() {
		return [][]Option{
			{WithPAR1Matrix()}, {WithCauchyMatrix()},
		}
	}
	opts := [][]Option{
		{WithPAR1Matrix()}, {WithCauchyMatrix()},
		{WithFastOneParityMatrix()}, {WithPAR1Matrix(), WithFastOneParityMatrix()}, {WithCauchyMatrix(), WithFastOneParityMatrix()},
		{WithMaxGoroutines(1), WithMinSplitSize(500), withSSSE3(false), withAVX2(false), withAVX512(false)},
		{WithMaxGoroutines(5000), WithMinSplitSize(50), withSSSE3(false), withAVX2(false), withAVX512(false)},
		{WithMaxGoroutines(5000), WithMinSplitSize(500000), withSSSE3(false), withAVX2(false), withAVX512(false)},
		{WithMaxGoroutines(1), WithMinSplitSize(500000), withSSSE3(false), withAVX2(false), withAVX512(false)},
		{WithAutoGoroutines(50000), WithMinSplitSize(500)},
	}
	for _, o := range opts[:] {
		if defaultOptions.useSSSE3 {
			n := make([]Option, len(o), len(o)+1)
			copy(n, o)
			n = append(n, withSSSE3(true))
			opts = append(opts, n)
		}
		if defaultOptions.useAVX2 {
			n := make([]Option, len(o), len(o)+1)
			copy(n, o)
			n = append(n, withAVX2(true))
			opts = append(opts, n)
		}
		if defaultOptions.useAVX512 {
			n := make([]Option, len(o), len(o)+1)
			copy(n, o)
			n = append(n, withAVX512(true))
			opts = append(opts, n)
		}
	}
	return opts
}

func TestEncoding(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		testEncoding(t, testOptions()...)
	})
	for i, o := range testOpts() {
		t.Run(fmt.Sprintf("opt-%d", i), func(t *testing.T) {
			testEncoding(t, o...)
		})
	}
}

// matrix sizes to test.
// note that par1 matric will fail on some combinations.
var testSizes = [][2]int{{1, 1}, {1, 2}, {3, 3}, {3, 1}, {5, 3}, {8, 4}, {10, 30}, {12, 10}, {14, 7}, {41, 17}, {49, 1}}
var testDataSizes = []int{10, 100, 1000, 10001, 100003, 1000055}
var testDataSizesShort = []int{10, 10001, 100003}

func testEncoding(t *testing.T, o ...Option) {
	for _, size := range testSizes {
		data, parity := size[0], size[1]
		rng := rand.New(rand.NewSource(0xabadc0cac01a))
		t.Run(fmt.Sprintf("%dx%d", data, parity), func(t *testing.T) {
			sz := testDataSizes
			if testing.Short() {
				sz = testDataSizesShort
			}
			for _, perShard := range sz {
				t.Run(fmt.Sprint(perShard), func(t *testing.T) {

					r, err := New(data, parity, testOptions(o...)...)
					if err != nil {
						t.Fatal(err)
					}
					shards := make([][]byte, data+parity)
					for s := range shards {
						shards[s] = make([]byte, perShard)
					}

					for s := 0; s < data; s++ {
						rng.Read(shards[s])
					}

					err = r.Encode(shards)
					if err != nil {
						t.Fatal(err)
					}
					ok, err := r.Verify(shards)
					if err != nil {
						t.Fatal(err)
					}
					if !ok {
						t.Fatal("Verification failed")
					}
					// Delete one in data
					idx := rng.Intn(data)
					want := shards[idx]
					shards[idx] = nil

					err = r.ReconstructData(shards)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(shards[idx], want) {
						t.Fatal("did not ReconstructData correctly")
					}

					// Delete one randomly
					idx = rng.Intn(data + parity)
					want = shards[idx]
					shards[idx] = nil
					err = r.Reconstruct(shards)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(shards[idx], want) {
						t.Fatal("did not Reconstruct correctly")
					}

					err = r.Encode(make([][]byte, 1))
					if err != ErrTooFewShards {
						t.Errorf("expected %v, got %v", ErrTooFewShards, err)
					}

					// Make one too short.
					shards[idx] = shards[idx][:perShard-1]
					err = r.Encode(shards)
					if err != ErrShardSize {
						t.Errorf("expected %v, got %v", ErrShardSize, err)
					}
				})
			}
		})

	}
}

func TestUpdate(t *testing.T) {
	for i, o := range testOpts() {
		t.Run(fmt.Sprintf("options %d", i), func(t *testing.T) {
			testUpdate(t, o...)
		})
	}
}

func testUpdate(t *testing.T, o ...Option) {
	rand.Seed(0)
	for _, size := range [][2]int{{10, 3}, {17, 2}} {
		data, parity := size[0], size[1]
		t.Run(fmt.Sprintf("%dx%d", data, parity), func(t *testing.T) {
			sz := testDataSizesShort
			if testing.Short() {
				sz = []int{50000}
			}
			for _, perShard := range sz {
				t.Run(fmt.Sprint(perShard), func(t *testing.T) {
					r, err := New(data, parity, testOptions(o...)...)
					if err != nil {
						t.Fatal(err)
					}
					shards := make([][]byte, data+parity)
					for s := range shards {
						shards[s] = make([]byte, perShard)
					}

					for s := range shards {
						fillRandom(shards[s])
					}

					err = r.Encode(shards)
					if err != nil {
						t.Fatal(err)
					}
					ok, err := r.Verify(shards)
					if err != nil {
						t.Fatal(err)
					}
					if !ok {
						t.Fatal("Verification failed")
					}

					newdatashards := make([][]byte, data)
					for s := range newdatashards {
						newdatashards[s] = make([]byte, perShard)
						fillRandom(newdatashards[s])
						err = r.Update(shards, newdatashards)
						if err != nil {
							t.Fatal(err)
						}
						shards[s] = newdatashards[s]
						ok, err := r.Verify(shards)
						if err != nil {
							t.Fatal(err)
						}
						if !ok {
							t.Fatal("Verification failed")
						}
						newdatashards[s] = nil
					}
					for s := 0; s < len(newdatashards)-1; s++ {
						newdatashards[s] = make([]byte, perShard)
						newdatashards[s+1] = make([]byte, perShard)
						fillRandom(newdatashards[s])
						fillRandom(newdatashards[s+1])
						err = r.Update(shards, newdatashards)
						if err != nil {
							t.Fatal(err)
						}
						shards[s] = newdatashards[s]
						shards[s+1] = newdatashards[s+1]
						ok, err := r.Verify(shards)
						if err != nil {
							t.Fatal(err)
						}
						if !ok {
							t.Fatal("Verification failed")
						}
						newdatashards[s] = nil
						newdatashards[s+1] = nil
					}
					for newNum := 1; newNum <= data; newNum++ {
						for s := 0; s <= data-newNum; s++ {
							for i := 0; i < newNum; i++ {
								newdatashards[s+i] = make([]byte, perShard)
								fillRandom(newdatashards[s+i])
							}
							err = r.Update(shards, newdatashards)
							if err != nil {
								t.Fatal(err)
							}
							for i := 0; i < newNum; i++ {
								shards[s+i] = newdatashards[s+i]
							}
							ok, err := r.Verify(shards)
							if err != nil {
								t.Fatal(err)
							}
							if !ok {
								t.Fatal("Verification failed")
							}
							for i := 0; i < newNum; i++ {
								newdatashards[s+i] = nil
							}
						}
					}
				})
			}
		})
	}
}

func TestReconstruct(t *testing.T) {
	testReconstruct(t)
	for i, o := range testOpts() {
		t.Run(fmt.Sprintf("options %d", i), func(t *testing.T) {
			testReconstruct(t, o...)
		})
	}
}

func testReconstruct(t *testing.T, o ...Option) {
	perShard := 50000
	r, err := New(10, 3, testOptions(o...)...)
	if err != nil {
		t.Fatal(err)
	}
	shards := make([][]byte, 13)
	for s := range shards {
		shards[s] = make([]byte, perShard)
	}

	rand.Seed(0)
	for s := 0; s < 13; s++ {
		fillRandom(shards[s])
	}

	err = r.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Reconstruct with all shards present
	err = r.Reconstruct(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Reconstruct with 10 shards present. Use pre-allocated memory for one of them.
	shards[0] = nil
	shards[7] = nil
	shard11 := shards[11]
	shards[11] = shard11[:0]
	fillRandom(shard11)

	err = r.Reconstruct(shards)
	if err != nil {
		t.Fatal(err)
	}

	ok, err := r.Verify(shards)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Verification failed")
	}

	if &shard11[0] != &shards[11][0] {
		t.Errorf("Shard was not reconstructed into pre-allocated memory")
	}

	// Reconstruct with 9 shards present (should fail)
	shards[0] = nil
	shards[4] = nil
	shards[7] = nil
	shards[11] = nil

	err = r.Reconstruct(shards)
	if err != ErrTooFewShards {
		t.Errorf("expected %v, got %v", ErrTooFewShards, err)
	}

	err = r.Reconstruct(make([][]byte, 1))
	if err != ErrTooFewShards {
		t.Errorf("expected %v, got %v", ErrTooFewShards, err)
	}
	err = r.Reconstruct(make([][]byte, 13))
	if err != ErrShardNoData {
		t.Errorf("expected %v, got %v", ErrShardNoData, err)
	}
}

func TestReconstructData(t *testing.T) {
	testReconstructData(t)
	for i, o := range testOpts() {
		t.Run(fmt.Sprintf("options %d", i), func(t *testing.T) {
			testReconstruct(t, o...)
		})
	}
}

func testReconstructData(t *testing.T, o ...Option) {
	perShard := 100000
	r, err := New(8, 5, testOptions(o...)...)
	if err != nil {
		t.Fatal(err)
	}
	shards := make([][]byte, 13)
	for s := range shards {
		shards[s] = make([]byte, perShard)
	}

	rand.Seed(0)
	for s := 0; s < 13; s++ {
		fillRandom(shards[s])
	}

	err = r.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Reconstruct with all shards present
	err = r.ReconstructData(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Reconstruct with 10 shards present. Use pre-allocated memory for one of them.
	shards[0] = nil
	shards[2] = nil
	shard4 := shards[4]
	shards[4] = shard4[:0]
	fillRandom(shard4)

	err = r.ReconstructData(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Since all parity shards are available, verification will succeed
	ok, err := r.Verify(shards)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Verification failed")
	}

	if &shard4[0] != &shards[4][0] {
		t.Errorf("Shard was not reconstructed into pre-allocated memory")
	}

	// Reconstruct with 6 data and 4 parity shards
	shards[0] = nil
	shards[2] = nil
	shards[12] = nil

	err = r.ReconstructData(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Verification will fail now due to absence of a parity block
	_, err = r.Verify(shards)
	if err != ErrShardSize {
		t.Errorf("expected %v, got %v", ErrTooFewShards, err)
	}

	// Reconstruct with 7 data and 1 parity shards
	shards[0] = nil
	shards[9] = nil
	shards[10] = nil
	shards[11] = nil
	shards[12] = nil

	err = r.ReconstructData(shards)
	if err != nil {
		t.Fatal(err)
	}

	_, err = r.Verify(shards)
	if err != ErrShardSize {
		t.Errorf("expected %v, got %v", ErrTooFewShards, err)
	}

	// Reconstruct with 6 data and 1 parity shards (should fail)
	shards[0] = nil
	shards[1] = nil
	shards[9] = nil
	shards[10] = nil
	shards[11] = nil
	shards[12] = nil

	err = r.ReconstructData(shards)
	if err != ErrTooFewShards {
		t.Errorf("expected %v, got %v", ErrTooFewShards, err)
	}

	err = r.ReconstructData(make([][]byte, 1))
	if err != ErrTooFewShards {
		t.Errorf("expected %v, got %v", ErrTooFewShards, err)
	}
	err = r.ReconstructData(make([][]byte, 13))
	if err != ErrShardNoData {
		t.Errorf("expected %v, got %v", ErrShardNoData, err)
	}
}

func TestReconstructPAR1Singular(t *testing.T) {
	perShard := 50
	r, err := New(4, 4, testOptions(WithPAR1Matrix())...)
	if err != nil {
		t.Fatal(err)
	}
	shards := make([][]byte, 8)
	for s := range shards {
		shards[s] = make([]byte, perShard)
	}

	rand.Seed(0)
	for s := 0; s < 8; s++ {
		fillRandom(shards[s])
	}

	err = r.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Reconstruct with only the last data shard present, and the
	// first, second, and fourth parity shard present (based on
	// the result of TestBuildMatrixPAR1Singular). This should
	// fail.
	shards[0] = nil
	shards[1] = nil
	shards[2] = nil
	shards[6] = nil

	err = r.Reconstruct(shards)
	if err != errSingular {
		t.Fatal(err)
		t.Errorf("expected %v, got %v", errSingular, err)
	}
}

func TestVerify(t *testing.T) {
	testVerify(t)
	for i, o := range testOpts() {
		t.Run(fmt.Sprintf("options %d", i), func(t *testing.T) {
			testVerify(t, o...)
		})
	}
}

func testVerify(t *testing.T, o ...Option) {
	perShard := 33333
	r, err := New(10, 4, testOptions(o...)...)
	if err != nil {
		t.Fatal(err)
	}
	shards := make([][]byte, 14)
	for s := range shards {
		shards[s] = make([]byte, perShard)
	}

	rand.Seed(0)
	for s := 0; s < 10; s++ {
		fillRandom(shards[s])
	}

	err = r.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := r.Verify(shards)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Verification failed")
	}

	// Put in random data. Verification should fail
	fillRandom(shards[10])
	ok, err = r.Verify(shards)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("Verification did not fail")
	}
	// Re-encode
	err = r.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}
	// Fill a data segment with random data
	fillRandom(shards[0])
	ok, err = r.Verify(shards)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("Verification did not fail")
	}

	_, err = r.Verify(make([][]byte, 1))
	if err != ErrTooFewShards {
		t.Errorf("expected %v, got %v", ErrTooFewShards, err)
	}

	_, err = r.Verify(make([][]byte, 14))
	if err != ErrShardNoData {
		t.Errorf("expected %v, got %v", ErrShardNoData, err)
	}
}

func TestOneEncode(t *testing.T) {
	codec, err := New(5, 5, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	shards := [][]byte{
		{0, 1},
		{4, 5},
		{2, 3},
		{6, 7},
		{8, 9},
		{0, 0},
		{0, 0},
		{0, 0},
		{0, 0},
		{0, 0},
	}
	codec.Encode(shards)
	if shards[5][0] != 12 || shards[5][1] != 13 {
		t.Fatal("shard 5 mismatch")
	}
	if shards[6][0] != 10 || shards[6][1] != 11 {
		t.Fatal("shard 6 mismatch")
	}
	if shards[7][0] != 14 || shards[7][1] != 15 {
		t.Fatal("shard 7 mismatch")
	}
	if shards[8][0] != 90 || shards[8][1] != 91 {
		t.Fatal("shard 8 mismatch")
	}
	if shards[9][0] != 94 || shards[9][1] != 95 {
		t.Fatal("shard 9 mismatch")
	}

	ok, err := codec.Verify(shards)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("did not verify")
	}
	shards[8][0]++
	ok, err = codec.Verify(shards)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("verify did not fail as expected")
	}

}

func fillRandom(p []byte) {
	for i := 0; i < len(p); i += 7 {
		val := rand.Int63()
		for j := 0; i+j < len(p) && j < 7; j++ {
			p[i+j] = byte(val)
			val >>= 8
		}
	}
}

func benchmarkEncode(b *testing.B, dataShards, parityShards, shardSize int) {
	r, err := New(dataShards, parityShards, testOptions(WithAutoGoroutines(shardSize))...)
	if err != nil {
		b.Fatal(err)
	}
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
		err = r.Encode(shards)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncode10x2x10000(b *testing.B) {
	benchmarkEncode(b, 10, 2, 10000)
}

func BenchmarkEncode100x20x10000(b *testing.B) {
	benchmarkEncode(b, 100, 20, 10000)
}

func BenchmarkEncode17x3x1M(b *testing.B) {
	benchmarkEncode(b, 17, 3, 1024*1024)
}

// Benchmark 10 data shards and 4 parity shards with 16MB each.
func BenchmarkEncode10x4x16M(b *testing.B) {
	benchmarkEncode(b, 10, 4, 16*1024*1024)
}

// Benchmark 5 data shards and 2 parity shards with 1MB each.
func BenchmarkEncode5x2x1M(b *testing.B) {
	benchmarkEncode(b, 5, 2, 1024*1024)
}

// Benchmark 1 data shards and 2 parity shards with 1MB each.
func BenchmarkEncode10x2x1M(b *testing.B) {
	benchmarkEncode(b, 10, 2, 1024*1024)
}

// Benchmark 10 data shards and 4 parity shards with 1MB each.
func BenchmarkEncode10x4x1M(b *testing.B) {
	benchmarkEncode(b, 10, 4, 1024*1024)
}

// Benchmark 50 data shards and 20 parity shards with 1MB each.
func BenchmarkEncode50x20x1M(b *testing.B) {
	benchmarkEncode(b, 50, 20, 1024*1024)
}

// Benchmark 17 data shards and 3 parity shards with 16MB each.
func BenchmarkEncode17x3x16M(b *testing.B) {
	benchmarkEncode(b, 17, 3, 16*1024*1024)
}

func BenchmarkEncode_8x4x8M(b *testing.B)   { benchmarkEncode(b, 8, 4, 8*1024*1024) }
func BenchmarkEncode_12x4x12M(b *testing.B) { benchmarkEncode(b, 12, 4, 12*1024*1024) }
func BenchmarkEncode_16x4x16M(b *testing.B) { benchmarkEncode(b, 16, 4, 16*1024*1024) }
func BenchmarkEncode_16x4x32M(b *testing.B) { benchmarkEncode(b, 16, 4, 32*1024*1024) }
func BenchmarkEncode_16x4x64M(b *testing.B) { benchmarkEncode(b, 16, 4, 64*1024*1024) }

func BenchmarkEncode_8x5x8M(b *testing.B)  { benchmarkEncode(b, 8, 5, 8*1024*1024) }
func BenchmarkEncode_8x6x8M(b *testing.B)  { benchmarkEncode(b, 8, 6, 8*1024*1024) }
func BenchmarkEncode_8x7x8M(b *testing.B)  { benchmarkEncode(b, 8, 7, 8*1024*1024) }
func BenchmarkEncode_8x9x8M(b *testing.B)  { benchmarkEncode(b, 8, 9, 8*1024*1024) }
func BenchmarkEncode_8x10x8M(b *testing.B) { benchmarkEncode(b, 8, 10, 8*1024*1024) }
func BenchmarkEncode_8x11x8M(b *testing.B) { benchmarkEncode(b, 8, 11, 8*1024*1024) }

func BenchmarkEncode_8x8x05M(b *testing.B) { benchmarkEncode(b, 8, 8, 1*1024*1024/2) }
func BenchmarkEncode_8x8x1M(b *testing.B)  { benchmarkEncode(b, 8, 8, 1*1024*1024) }
func BenchmarkEncode_8x8x8M(b *testing.B)  { benchmarkEncode(b, 8, 8, 8*1024*1024) }
func BenchmarkEncode_8x8x32M(b *testing.B) { benchmarkEncode(b, 8, 8, 32*1024*1024) }

func BenchmarkEncode_24x8x24M(b *testing.B) { benchmarkEncode(b, 24, 8, 24*1024*1024) }
func BenchmarkEncode_24x8x48M(b *testing.B) { benchmarkEncode(b, 24, 8, 48*1024*1024) }

func benchmarkVerify(b *testing.B, dataShards, parityShards, shardSize int) {
	r, err := New(dataShards, parityShards, testOptions(WithAutoGoroutines(shardSize))...)
	if err != nil {
		b.Fatal(err)
	}
	shards := make([][]byte, parityShards+dataShards)
	for s := range shards {
		shards[s] = make([]byte, shardSize)
	}

	rand.Seed(0)
	for s := 0; s < dataShards; s++ {
		fillRandom(shards[s])
	}
	err = r.Encode(shards)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(shardSize * dataShards))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = r.Verify(shards)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark 10 data slices with 2 parity slices holding 10000 bytes each
func BenchmarkVerify10x2x10000(b *testing.B) {
	benchmarkVerify(b, 10, 2, 10000)
}

// Benchmark 50 data slices with 5 parity slices holding 100000 bytes each
func BenchmarkVerify50x5x50000(b *testing.B) {
	benchmarkVerify(b, 50, 5, 100000)
}

// Benchmark 10 data slices with 2 parity slices holding 1MB bytes each
func BenchmarkVerify10x2x1M(b *testing.B) {
	benchmarkVerify(b, 10, 2, 1024*1024)
}

// Benchmark 5 data slices with 2 parity slices holding 1MB bytes each
func BenchmarkVerify5x2x1M(b *testing.B) {
	benchmarkVerify(b, 5, 2, 1024*1024)
}

// Benchmark 10 data slices with 4 parity slices holding 1MB bytes each
func BenchmarkVerify10x4x1M(b *testing.B) {
	benchmarkVerify(b, 10, 4, 1024*1024)
}

// Benchmark 5 data slices with 2 parity slices holding 1MB bytes each
func BenchmarkVerify50x20x1M(b *testing.B) {
	benchmarkVerify(b, 50, 20, 1024*1024)
}

// Benchmark 10 data slices with 4 parity slices holding 16MB bytes each
func BenchmarkVerify10x4x16M(b *testing.B) {
	benchmarkVerify(b, 10, 4, 16*1024*1024)
}

func corruptRandom(shards [][]byte, dataShards, parityShards int) {
	shardsToCorrupt := rand.Intn(parityShards) + 1
	for i := 0; i < shardsToCorrupt; i++ {
		n := rand.Intn(dataShards + parityShards)
		shards[n] = shards[n][:0]
	}
}

func benchmarkReconstruct(b *testing.B, dataShards, parityShards, shardSize int) {
	r, err := New(dataShards, parityShards, testOptions(WithAutoGoroutines(shardSize))...)
	if err != nil {
		b.Fatal(err)
	}
	shards := make([][]byte, parityShards+dataShards)
	for s := range shards {
		shards[s] = make([]byte, shardSize)
	}

	rand.Seed(0)
	for s := 0; s < dataShards; s++ {
		fillRandom(shards[s])
	}
	err = r.Encode(shards)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(shardSize * dataShards))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		corruptRandom(shards, dataShards, parityShards)

		err = r.Reconstruct(shards)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark 10 data slices with 2 parity slices holding 10000 bytes each
func BenchmarkReconstruct10x2x10000(b *testing.B) {
	benchmarkReconstruct(b, 10, 2, 10000)
}

// Benchmark 50 data slices with 5 parity slices holding 100000 bytes each
func BenchmarkReconstruct50x5x50000(b *testing.B) {
	benchmarkReconstruct(b, 50, 5, 100000)
}

// Benchmark 10 data slices with 2 parity slices holding 1MB bytes each
func BenchmarkReconstruct10x2x1M(b *testing.B) {
	benchmarkReconstruct(b, 10, 2, 1024*1024)
}

// Benchmark 5 data slices with 2 parity slices holding 1MB bytes each
func BenchmarkReconstruct5x2x1M(b *testing.B) {
	benchmarkReconstruct(b, 5, 2, 1024*1024)
}

// Benchmark 10 data slices with 4 parity slices holding 1MB bytes each
func BenchmarkReconstruct10x4x1M(b *testing.B) {
	benchmarkReconstruct(b, 10, 4, 1024*1024)
}

// Benchmark 5 data slices with 2 parity slices holding 1MB bytes each
func BenchmarkReconstruct50x20x1M(b *testing.B) {
	benchmarkReconstruct(b, 50, 20, 1024*1024)
}

// Benchmark 10 data slices with 4 parity slices holding 16MB bytes each
func BenchmarkReconstruct10x4x16M(b *testing.B) {
	benchmarkReconstruct(b, 10, 4, 16*1024*1024)
}

func corruptRandomData(shards [][]byte, dataShards, parityShards int) {
	shardsToCorrupt := rand.Intn(parityShards) + 1
	for i := 1; i <= shardsToCorrupt; i++ {
		n := rand.Intn(dataShards)
		shards[n] = shards[n][:0]
	}
}

func benchmarkReconstructData(b *testing.B, dataShards, parityShards, shardSize int) {
	r, err := New(dataShards, parityShards, testOptions(WithAutoGoroutines(shardSize))...)
	if err != nil {
		b.Fatal(err)
	}
	shards := make([][]byte, parityShards+dataShards)
	for s := range shards {
		shards[s] = make([]byte, shardSize)
	}

	rand.Seed(0)
	for s := 0; s < dataShards; s++ {
		fillRandom(shards[s])
	}
	err = r.Encode(shards)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(shardSize * dataShards))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		corruptRandomData(shards, dataShards, parityShards)

		err = r.ReconstructData(shards)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark 10 data slices with 2 parity slices holding 10000 bytes each
func BenchmarkReconstructData10x2x10000(b *testing.B) {
	benchmarkReconstructData(b, 10, 2, 10000)
}

// Benchmark 50 data slices with 5 parity slices holding 100000 bytes each
func BenchmarkReconstructData50x5x50000(b *testing.B) {
	benchmarkReconstructData(b, 50, 5, 100000)
}

// Benchmark 10 data slices with 2 parity slices holding 1MB bytes each
func BenchmarkReconstructData10x2x1M(b *testing.B) {
	benchmarkReconstructData(b, 10, 2, 1024*1024)
}

// Benchmark 5 data slices with 2 parity slices holding 1MB bytes each
func BenchmarkReconstructData5x2x1M(b *testing.B) {
	benchmarkReconstructData(b, 5, 2, 1024*1024)
}

// Benchmark 10 data slices with 4 parity slices holding 1MB bytes each
func BenchmarkReconstructData10x4x1M(b *testing.B) {
	benchmarkReconstructData(b, 10, 4, 1024*1024)
}

// Benchmark 5 data slices with 2 parity slices holding 1MB bytes each
func BenchmarkReconstructData50x20x1M(b *testing.B) {
	benchmarkReconstructData(b, 50, 20, 1024*1024)
}

// Benchmark 10 data slices with 4 parity slices holding 16MB bytes each
func BenchmarkReconstructData10x4x16M(b *testing.B) {
	benchmarkReconstructData(b, 10, 4, 16*1024*1024)
}

func benchmarkReconstructP(b *testing.B, dataShards, parityShards, shardSize int) {
	r, err := New(dataShards, parityShards, testOptions(WithMaxGoroutines(1))...)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(shardSize * dataShards))
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		shards := make([][]byte, parityShards+dataShards)
		for s := range shards {
			shards[s] = make([]byte, shardSize)
		}

		rand.Seed(0)
		for s := 0; s < dataShards; s++ {
			fillRandom(shards[s])
		}
		err = r.Encode(shards)
		if err != nil {
			b.Fatal(err)
		}

		for pb.Next() {
			corruptRandom(shards, dataShards, parityShards)

			err = r.Reconstruct(shards)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Benchmark 10 data slices with 2 parity slices holding 10000 bytes each
func BenchmarkReconstructP10x2x10000(b *testing.B) {
	benchmarkReconstructP(b, 10, 2, 10000)
}

// Benchmark 10 data slices with 5 parity slices holding 20000 bytes each
func BenchmarkReconstructP10x5x20000(b *testing.B) {
	benchmarkReconstructP(b, 10, 5, 20000)
}

func TestEncoderReconstruct(t *testing.T) {
	testEncoderReconstruct(t)
	for _, o := range testOpts() {
		testEncoderReconstruct(t, o...)
	}
}

func testEncoderReconstruct(t *testing.T, o ...Option) {
	// Create some sample data
	var data = make([]byte, 250000)
	fillRandom(data)

	// Create 5 data slices of 50000 elements each
	enc, err := New(5, 3, testOptions(o...)...)
	if err != nil {
		t.Fatal(err)
	}
	shards, err := enc.Split(data)
	if err != nil {
		t.Fatal(err)
	}
	err = enc.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Check that it verifies
	ok, err := enc.Verify(shards)
	if !ok || err != nil {
		t.Fatal("not ok:", ok, "err:", err)
	}

	// Delete a shard
	shards[0] = nil

	// Should reconstruct
	err = enc.Reconstruct(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Check that it verifies
	ok, err = enc.Verify(shards)
	if !ok || err != nil {
		t.Fatal("not ok:", ok, "err:", err)
	}

	// Recover original bytes
	buf := new(bytes.Buffer)
	err = enc.Join(buf, shards, len(data))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), data) {
		t.Fatal("recovered bytes do not match")
	}

	// Corrupt a shard
	shards[0] = nil
	shards[1][0], shards[1][500] = 75, 75

	// Should reconstruct (but with corrupted data)
	err = enc.Reconstruct(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Check that it verifies
	ok, err = enc.Verify(shards)
	if ok || err != nil {
		t.Fatal("error or ok:", ok, "err:", err)
	}

	// Recovered data should not match original
	buf.Reset()
	err = enc.Join(buf, shards, len(data))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(buf.Bytes(), data) {
		t.Fatal("corrupted data matches original")
	}
}

func TestSplitJoin(t *testing.T) {
	var data = make([]byte, 250000)
	rand.Seed(0)
	fillRandom(data)

	enc, _ := New(5, 3, testOptions()...)
	shards, err := enc.Split(data)
	if err != nil {
		t.Fatal(err)
	}

	_, err = enc.Split([]byte{})
	if err != ErrShortData {
		t.Errorf("expected %v, got %v", ErrShortData, err)
	}

	buf := new(bytes.Buffer)
	err = enc.Join(buf, shards, 50)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), data[:50]) {
		t.Fatal("recovered data does match original")
	}

	err = enc.Join(buf, [][]byte{}, 0)
	if err != ErrTooFewShards {
		t.Errorf("expected %v, got %v", ErrTooFewShards, err)
	}

	err = enc.Join(buf, shards, len(data)+1)
	if err != ErrShortData {
		t.Errorf("expected %v, got %v", ErrShortData, err)
	}

	shards[0] = nil
	err = enc.Join(buf, shards, len(data))
	if err != ErrReconstructRequired {
		t.Errorf("expected %v, got %v", ErrReconstructRequired, err)
	}
}

func TestCodeSomeShards(t *testing.T) {
	var data = make([]byte, 250000)
	fillRandom(data)
	enc, _ := New(5, 3, testOptions()...)
	r := enc.(*reedSolomon) // need to access private methods
	shards, _ := enc.Split(data)

	old := runtime.GOMAXPROCS(1)
	r.codeSomeShards(r.parity, shards[:r.DataShards], shards[r.DataShards:], r.ParityShards, len(shards[0]))

	// hopefully more than 1 CPU
	runtime.GOMAXPROCS(runtime.NumCPU())
	r.codeSomeShards(r.parity, shards[:r.DataShards], shards[r.DataShards:], r.ParityShards, len(shards[0]))

	// reset MAXPROCS, otherwise testing complains
	runtime.GOMAXPROCS(old)
}

func TestStandardMatrices(t *testing.T) {
	if testing.Short() || runtime.GOMAXPROCS(0) < 4 {
		// Runtime ~15s.
		t.Skip("Skipping slow matrix check")
	}
	var wg sync.WaitGroup
	wg.Add(256 - 1)
	for i := 1; i < 256; i++ {
		go func(i int) {
			// i == n.o. datashards
			defer wg.Done()
			var shards = make([][]byte, 255)
			for p := range shards {
				v := byte(i)
				shards[p] = []byte{v}
			}
			rng := rand.New(rand.NewSource(0))
			for j := 1; j < 256; j++ {
				// j == n.o. parity shards
				if i+j > 255 {
					continue
				}
				sh := shards[:i+j]
				r, err := New(i, j, testOptions(WithFastOneParityMatrix())...)
				if err != nil {
					// We are not supposed to write to t from goroutines.
					t.Fatal("creating matrix size", i, j, ":", err)
				}
				err = r.Encode(sh)
				if err != nil {
					t.Fatal("encoding", i, j, ":", err)
				}
				for k := 0; k < j; k++ {
					// Remove random shard.
					r := int(rng.Int63n(int64(i + j)))
					sh[r] = sh[r][:0]
				}
				err = r.Reconstruct(sh)
				if err != nil {
					t.Fatal("reconstructing", i, j, ":", err)
				}
				ok, err := r.Verify(sh)
				if err != nil {
					t.Fatal("verifying", i, j, ":", err)
				}
				if !ok {
					t.Fatal(i, j, ok)
				}
				for k := range sh {
					if k == i {
						// Only check data shards
						break
					}
					if sh[k][0] != byte(i) {
						t.Fatal("does not match", i, j, k, sh[0], sh[k])
					}
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestCauchyMatrices(t *testing.T) {
	if testing.Short() || runtime.GOMAXPROCS(0) < 4 {
		// Runtime ~15s.
		t.Skip("Skipping slow matrix check")
	}
	var wg sync.WaitGroup
	wg.Add(256 - 1)
	for i := 1; i < 256; i++ {
		go func(i int) {
			// i == n.o. datashards
			defer wg.Done()
			var shards = make([][]byte, 255)
			for p := range shards {
				v := byte(i)
				shards[p] = []byte{v}
			}
			rng := rand.New(rand.NewSource(0))
			for j := 1; j < 256; j++ {
				// j == n.o. parity shards
				if i+j > 255 {
					continue
				}
				sh := shards[:i+j]
				r, err := New(i, j, testOptions(WithCauchyMatrix(), WithFastOneParityMatrix())...)
				if err != nil {
					// We are not supposed to write to t from goroutines.
					t.Fatal("creating matrix size", i, j, ":", err)
				}
				err = r.Encode(sh)
				if err != nil {
					t.Fatal("encoding", i, j, ":", err)
				}
				for k := 0; k < j; k++ {
					// Remove random shard.
					r := int(rng.Int63n(int64(i + j)))
					sh[r] = sh[r][:0]
				}
				err = r.Reconstruct(sh)
				if err != nil {
					t.Fatal("reconstructing", i, j, ":", err)
				}
				ok, err := r.Verify(sh)
				if err != nil {
					t.Fatal("verifying", i, j, ":", err)
				}
				if !ok {
					t.Fatal(i, j, ok)
				}
				for k := range sh {
					if k == i {
						// Only check data shards
						break
					}
					if sh[k][0] != byte(i) {
						t.Fatal("does not match", i, j, k, sh[0], sh[k])
					}
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestPar1Matrices(t *testing.T) {
	if testing.Short() || runtime.GOMAXPROCS(0) < 4 {
		// Runtime ~15s.
		t.Skip("Skipping slow matrix check")
	}
	var wg sync.WaitGroup
	wg.Add(256 - 1)
	for i := 1; i < 256; i++ {
		go func(i int) {
			// i == n.o. datashards
			defer wg.Done()
			var shards = make([][]byte, 255)
			for p := range shards {
				v := byte(i)
				shards[p] = []byte{v}
			}
			rng := rand.New(rand.NewSource(0))
			for j := 1; j < 256; j++ {
				// j == n.o. parity shards
				if i+j > 255 {
					continue
				}
				sh := shards[:i+j]
				r, err := New(i, j, testOptions(WithPAR1Matrix())...)
				if err != nil {
					// We are not supposed to write to t from goroutines.
					t.Fatal("creating matrix size", i, j, ":", err)
				}
				err = r.Encode(sh)
				if err != nil {
					t.Fatal("encoding", i, j, ":", err)
				}
				for k := 0; k < j; k++ {
					// Remove random shard.
					r := int(rng.Int63n(int64(i + j)))
					sh[r] = sh[r][:0]
				}
				err = r.Reconstruct(sh)
				if err != nil {
					if err == errSingular {
						t.Logf("Singular: %d (data), %d (parity)", i, j)
						for p := range sh {
							if len(sh[p]) == 0 {
								shards[p] = []byte{byte(i)}
							}
						}
						continue
					}
					t.Fatal("reconstructing", i, j, ":", err)
				}
				ok, err := r.Verify(sh)
				if err != nil {
					t.Fatal("verifying", i, j, ":", err)
				}
				if !ok {
					t.Fatal(i, j, ok)
				}
				for k := range sh {
					if k == i {
						// Only check data shards
						break
					}
					if sh[k][0] != byte(i) {
						t.Fatal("does not match", i, j, k, sh[0], sh[k])
					}
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestNew(t *testing.T) {
	tests := []struct {
		data, parity int
		err          error
	}{
		{127, 127, nil},
		{128, 128, nil},
		{255, 1, nil},
		{256, 256, ErrMaxShardNum},

		{0, 1, ErrInvShardNum},
		{1, 0, ErrInvShardNum},
		{256, 1, ErrMaxShardNum},

		// overflow causes r.Shards to be negative
		{256, int(^uint(0) >> 1), errInvalidRowSize},
	}
	for _, test := range tests {
		_, err := New(test.data, test.parity, testOptions()...)
		if err != test.err {
			t.Errorf("New(%v, %v): expected %v, got %v", test.data, test.parity, test.err, err)
		}
	}
}

// Benchmark 10 data shards and 4 parity shards and 160MB data.
func BenchmarkSplit10x4x160M(b *testing.B) {
	benchmarkSplit(b, 10, 4, 160*1024*1024)
}

// Benchmark 5 data shards and 2 parity shards with 5MB data.
func BenchmarkSplit5x2x5M(b *testing.B) {
	benchmarkSplit(b, 5, 2, 5*1024*1024)
}

// Benchmark 1 data shards and 2 parity shards with 1MB data.
func BenchmarkSplit10x2x1M(b *testing.B) {
	benchmarkSplit(b, 10, 2, 1024*1024)
}

// Benchmark 10 data shards and 4 parity shards with 10MB data.
func BenchmarkSplit10x4x10M(b *testing.B) {
	benchmarkSplit(b, 10, 4, 10*1024*1024)
}

// Benchmark 50 data shards and 20 parity shards with 50MB data.
func BenchmarkSplit50x20x50M(b *testing.B) {
	benchmarkSplit(b, 50, 20, 50*1024*1024)
}

// Benchmark 17 data shards and 3 parity shards with 272MB data.
func BenchmarkSplit17x3x272M(b *testing.B) {
	benchmarkSplit(b, 17, 3, 272*1024*1024)
}

func benchmarkSplit(b *testing.B, shards, parity, dataSize int) {
	r, err := New(shards, parity, testOptions(WithAutoGoroutines(dataSize))...)
	if err != nil {
		b.Fatal(err)
	}

	data := make([]byte, dataSize)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = r.Split(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkParallel(b *testing.B, dataShards, parityShards, shardSize int) {
	// Run max 1 goroutine per operation.
	r, err := New(dataShards, parityShards, testOptions(WithMaxGoroutines(1))...)
	if err != nil {
		b.Fatal(err)
	}
	c := runtime.GOMAXPROCS(0)

	// Note that concurrency also affects total data size and will make caches less effective.
	b.Log("Total data:", (c*dataShards*shardSize)>>20, "MiB", "parity:", (c*parityShards*shardSize)>>20, "MiB")
	// Create independent shards
	shardsCh := make(chan [][]byte, c)
	for i := 0; i < c; i++ {
		rand.Seed(int64(i))
		shards := make([][]byte, dataShards+parityShards)
		for s := range shards {
			shards[s] = make([]byte, shardSize)
		}
		for s := 0; s < dataShards; s++ {
			fillRandom(shards[s])
		}
		shardsCh <- shards
	}

	b.SetBytes(int64(shardSize * dataShards))
	b.SetParallelism(c)
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			shards := <-shardsCh
			err = r.Encode(shards)
			if err != nil {
				b.Fatal(err)
			}
			shardsCh <- shards
		}
	})
}

func BenchmarkParallel_8x8x64K(b *testing.B)   { benchmarkParallel(b, 8, 8, 64<<10) }
func BenchmarkParallel_8x8x05M(b *testing.B)   { benchmarkParallel(b, 8, 8, 512<<10) }
func BenchmarkParallel_20x10x05M(b *testing.B) { benchmarkParallel(b, 20, 10, 512<<10) }
func BenchmarkParallel_8x8x1M(b *testing.B)    { benchmarkParallel(b, 8, 8, 1<<20) }
func BenchmarkParallel_8x8x8M(b *testing.B)    { benchmarkParallel(b, 8, 8, 8<<20) }
func BenchmarkParallel_8x8x32M(b *testing.B)   { benchmarkParallel(b, 8, 8, 32<<20) }

func BenchmarkParallel_8x3x1M(b *testing.B) { benchmarkParallel(b, 8, 3, 1<<20) }
func BenchmarkParallel_8x4x1M(b *testing.B) { benchmarkParallel(b, 8, 4, 1<<20) }
func BenchmarkParallel_8x5x1M(b *testing.B) { benchmarkParallel(b, 8, 5, 1<<20) }
