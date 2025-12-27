package reedsolomon

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// TestDecodeIdx_InvalidExpectInputLength tests that DecodeIdx returns an error
// when expectInput length doesn't match totalShards
func TestDecodeIdx_InvalidExpectInputLength(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	dst := make([]byte, 100)
	input := make([]byte, 100)
	wrongExpectInput := make([]bool, 7) // Should be 8 (5+3)

	err = r.DecodeIdx(dst, 0, wrongExpectInput, input, 0)
	if err == nil {
		t.Fatal("expected error for wrong expectInput length")
	}
	if !strings.Contains(err.Error(), "expectInput length expected to be 8") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestDecodeIdx_InvalidDstIdx tests that DecodeIdx returns an error
// for invalid destination indices
func TestDecodeIdx_InvalidDstIdx(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	dst := make([]byte, 100)
	input := make([]byte, 100)
	expectInput := make([]bool, 8)
	for i := 0; i < 5; i++ {
		expectInput[i] = true // Mark data shards as available
	}

	testCases := []struct {
		name   string
		dstIdx int
	}{
		{"negative", -1},
		{"equal to totalShards", 8},
		{"greater than totalShards", 10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := r.DecodeIdx(dst, tc.dstIdx, expectInput, input, 0)
			if err != ErrInvShardNum {
				t.Errorf("expected ErrInvShardNum, got %v", err)
			}
		})
	}
}

// TestDecodeIdx_DestinationAlreadyFilled tests that DecodeIdx returns an error
// when trying to decode into an already filled shard
func TestDecodeIdx_DestinationAlreadyFilled(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	dst := make([]byte, 100)
	input := make([]byte, 100)
	expectInput := make([]bool, 8)
	for i := 0; i < 5; i++ {
		expectInput[i] = true
	}
	expectInput[7] = true // Mark destination as already filled

	err = r.DecodeIdx(dst, 7, expectInput, input, 0)
	if err == nil {
		t.Fatal("expected error for already filled destination")
	}
	if !strings.Contains(err.Error(), "destination shard already filled") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestDecodeIdx_InvalidInputIdx tests that DecodeIdx returns an error
// when inputIdx is not in expectInput
func TestDecodeIdx_InvalidInputIdx(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	dst := make([]byte, 100)
	input := make([]byte, 100)
	expectInput := make([]bool, 8)
	for i := 0; i < 5; i++ {
		expectInput[i] = true
	}
	// expectInput[3] is false, but we'll try to use it as inputIdx

	err = r.DecodeIdx(dst, 7, expectInput, input, 6)
	if err != ErrInvShardNum {
		t.Errorf("expected ErrInvShardNum for invalid inputIdx, got %v", err)
	}
}

// TestDecodeIdx_TooFewShards tests that DecodeIdx returns an error
// when there are not enough shards to reconstruct
func TestDecodeIdx_TooFewShards(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	dst := make([]byte, 100)
	input := make([]byte, 100)
	expectInput := make([]bool, 8)
	// Only mark 4 shards as available (need at least 5)
	for i := 0; i < 4; i++ {
		expectInput[i] = true
	}

	err = r.DecodeIdx(dst, 7, expectInput, input, 0)
	if err != ErrTooFewShards {
		t.Errorf("expected ErrTooFewShards, got %v", err)
	}
}

// TestDecodeIdx_MismatchedBufferSizes tests that DecodeIdx returns an error
// when dst and input have different sizes
func TestDecodeIdx_MismatchedBufferSizes(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	dst := make([]byte, 100)
	input := make([]byte, 50) // Different size
	expectInput := make([]bool, 8)
	for i := 0; i < 5; i++ {
		expectInput[i] = true
	}

	err = r.DecodeIdx(dst, 7, expectInput, input, 0)
	if err != ErrInvalidShardSize {
		t.Errorf("expected ErrInvalidShardSize, got %v", err)
	}
}

// TestDecodeIdx_XORMode tests the XOR mode when inputIdx < 0
func TestDecodeIdx_XORMode(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	dst := []byte{1, 2, 3, 4, 5}
	input := []byte{10, 20, 30, 40, 50}
	expected := []byte{1^10, 2^20, 3^30, 4^40, 5^50} // XOR results: 11, 22, 29, 44, 55

	expectInput := make([]bool, 8)
	for i := 0; i < 5; i++ {
		expectInput[i] = true
	}

	// Make a copy of dst to preserve original for comparison
	dstCopy := make([]byte, len(dst))
	copy(dstCopy, dst)

	err = r.DecodeIdx(dstCopy, 7, expectInput, input, -1)
	if err != nil {
		t.Fatalf("unexpected error in XOR mode: %v", err)
	}

	if !bytes.Equal(dstCopy, expected) {
		t.Errorf("XOR result mismatch: got %v, want %v", dstCopy, expected)
	}
}

// TestDecodeIdx_ProgressiveDecode tests progressive decoding with multiple calls
func TestDecodeIdx_ProgressiveDecode(t *testing.T) {
	dataShards := 5
	parityShards := 3
	shardSize := 1000

	enc, err := New(dataShards, parityShards, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	// Create original data
	shards := make([][]byte, dataShards+parityShards)
	for i := 0; i < dataShards; i++ {
		shards[i] = make([]byte, shardSize)
		fillRandomDecodeIdx(shards[i])
	}
	for i := dataShards; i < dataShards+parityShards; i++ {
		shards[i] = make([]byte, shardSize)
	}

	// Encode
	err = enc.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Save original data shard for comparison
	originalShard := make([]byte, shardSize)
	copy(originalShard, shards[0])

	// Set up expectInput - we'll use shards 1,2,3,4,5 to reconstruct shard 0
	expectInput := make([]bool, dataShards+parityShards)
	availableShards := []int{1, 2, 3, 4, 5}
	for _, idx := range availableShards {
		expectInput[idx] = true
	}

	// Initialize destination buffer with zeros
	dst := make([]byte, shardSize)

	// Progressive decode - feed one shard at a time
	for i, shardIdx := range availableShards {
		err = r.DecodeIdx(dst, 0, expectInput, shards[shardIdx], shardIdx)
		if err != nil {
			t.Fatalf("DecodeIdx failed on input %d (shard %d): %v", i, shardIdx, err)
		}
	}

	// Verify the reconstructed shard matches the original
	if !bytes.Equal(dst, originalShard) {
		t.Error("Progressive decode produced incorrect result")
	}
}

// TestDecodeIdx_IntegrationWithFullCycle tests DecodeIdx in a complete encode/decode cycle
func TestDecodeIdx_IntegrationWithFullCycle(t *testing.T) {
	testCases := []struct {
		name         string
		dataShards   int
		parityShards int
	}{
		{"5+3", 5, 3},
		{"10+4", 10, 4},
		{"2+2", 2, 2},
		{"17+3", 17, 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			enc, err := New(tc.dataShards, tc.parityShards, testOptions()...)
			if err != nil {
				t.Fatal(err)
			}
			r := enc.(*reedSolomon)

			shardSize := 1000
			totalShards := tc.dataShards + tc.parityShards

			// Create and encode data
			shards := make([][]byte, totalShards)
			for i := 0; i < tc.dataShards; i++ {
				shards[i] = make([]byte, shardSize)
				fillRandomDecodeIdx(shards[i])
			}
			for i := tc.dataShards; i < totalShards; i++ {
				shards[i] = make([]byte, shardSize)
			}

			err = enc.Encode(shards)
			if err != nil {
				t.Fatal(err)
			}

			// Save original for verification
			originalShards := make([][]byte, totalShards)
			for i := range shards {
				originalShards[i] = make([]byte, shardSize)
				copy(originalShards[i], shards[i])
			}

			// Test reconstruction of ALL shards (both data and parity)
			for targetShard := 0; targetShard < totalShards; targetShard++ {
				// Skip one shard at a time and use DecodeIdx to reconstruct it
				expectInput := make([]bool, totalShards)
				inputShards := make([]int, 0, tc.dataShards)

				// Select which shards to use (all except target, up to dataShards count)
				count := 0
				for i := 0; i < totalShards && count < tc.dataShards; i++ {
					if i != targetShard {
						expectInput[i] = true
						inputShards = append(inputShards, i)
						count++
					}
				}

				// Decode progressively
				dst := make([]byte, shardSize)
				for _, inputIdx := range inputShards {
					err = r.DecodeIdx(dst, targetShard, expectInput, shards[inputIdx], inputIdx)
					if err != nil {
						t.Fatalf("Failed to decode shard %d using inputs %v: %v",
							targetShard, inputShards, err)
					}
				}

				// Verify
				if !bytes.Equal(dst, originalShards[targetShard]) {
					t.Errorf("Shard %d reconstruction failed", targetShard)
				}
			}
		})
	}
}

// TestDecodeIdx_MinimalShards tests decoding with exactly dataShards inputs
func TestDecodeIdx_MinimalShards(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	shardSize := 100
	shards := make([][]byte, 8)
	for i := 0; i < 5; i++ {
		shards[i] = make([]byte, shardSize)
		fillRandomDecodeIdx(shards[i])
	}
	for i := 5; i < 8; i++ {
		shards[i] = make([]byte, shardSize)
	}

	err = enc.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Use exactly 5 shards (minimum required) to reconstruct data shard 0
	// We'll use shards 1,2,3,4,5 to reconstruct shard 0
	expectInput := make([]bool, 8)
	for i := 1; i < 6; i++ {
		expectInput[i] = true
	}

	originalShard := make([]byte, shardSize)
	copy(originalShard, shards[0])

	dst := make([]byte, shardSize)
	for i := 1; i < 6; i++ {
		err = r.DecodeIdx(dst, 0, expectInput, shards[i], i)
		if err != nil {
			t.Fatalf("Failed with minimal shards at input %d: %v", i, err)
		}
	}

	if !bytes.Equal(dst, originalShard) {
		t.Error("Minimal shards reconstruction failed")
	}
}

// TestDecodeIdx_ParityOnly tests reconstruction using only parity shards
func TestDecodeIdx_ParityOnly(t *testing.T) {
	enc, err := New(3, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	shardSize := 100
	shards := make([][]byte, 6)
	for i := 0; i < 3; i++ {
		shards[i] = make([]byte, shardSize)
		fillRandomDecodeIdx(shards[i])
	}
	for i := 3; i < 6; i++ {
		shards[i] = make([]byte, shardSize)
	}

	err = enc.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Use only parity shards (indices 3, 4, 5) to reconstruct data shard 0
	expectInput := make([]bool, 6)
	expectInput[3] = true
	expectInput[4] = true
	expectInput[5] = true

	originalShard := make([]byte, shardSize)
	copy(originalShard, shards[0])

	dst := make([]byte, shardSize)
	for i := 3; i < 6; i++ {
		err = r.DecodeIdx(dst, 0, expectInput, shards[i], i)
		if err != nil {
			t.Fatalf("Failed with parity shards at input %d: %v", i, err)
		}
	}

	if !bytes.Equal(dst, originalShard) {
		t.Error("Parity-only reconstruction failed")
	}
}

// TestDecodeIdx_MixedDataAndParity tests reconstruction with mixed data and parity shards
func TestDecodeIdx_MixedDataAndParity(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	shardSize := 100
	shards := make([][]byte, 8)
	for i := 0; i < 5; i++ {
		shards[i] = make([]byte, shardSize)
		fillRandomDecodeIdx(shards[i])
	}
	for i := 5; i < 8; i++ {
		shards[i] = make([]byte, shardSize)
	}

	err = enc.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Use mix of data shards (0, 1) and parity shards (5, 6, 7) to reconstruct shard 2
	expectInput := make([]bool, 8)
	mixedShards := []int{0, 1, 5, 6, 7}
	for _, idx := range mixedShards {
		expectInput[idx] = true
	}

	originalShard := make([]byte, shardSize)
	copy(originalShard, shards[2])

	dst := make([]byte, shardSize)
	for _, shardIdx := range mixedShards {
		err = r.DecodeIdx(dst, 2, expectInput, shards[shardIdx], shardIdx)
		if err != nil {
			t.Fatalf("Failed with mixed shards at input %d: %v", shardIdx, err)
		}
	}

	if !bytes.Equal(dst, originalShard) {
		t.Error("Mixed data/parity reconstruction failed")
	}
}

// TestDecodeIdx_XORMerge tests merging two partial decodings using XOR mode
func TestDecodeIdx_XORMerge(t *testing.T) {
	enc, err := New(4, 2, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	shardSize := 100
	shards := make([][]byte, 6)
	for i := 0; i < 4; i++ {
		shards[i] = make([]byte, shardSize)
		fillRandomDecodeIdx(shards[i])
	}
	for i := 4; i < 6; i++ {
		shards[i] = make([]byte, shardSize)
	}

	err = enc.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Reconstruct data shard 0 using two different sets and merge
	// First partial decode using shards 1, 2, 4
	expectInput1 := make([]bool, 6)
	expectInput1[1] = true
	expectInput1[2] = true
	expectInput1[3] = true
	expectInput1[4] = true

	dst1 := make([]byte, shardSize)
	err = r.DecodeIdx(dst1, 0, expectInput1, shards[1], 1)
	if err != nil {
		t.Fatal(err)
	}
	err = r.DecodeIdx(dst1, 0, expectInput1, shards[2], 2)
	if err != nil {
		t.Fatal(err)
	}

	// Second partial decode using shards 3, 4
	dst2 := make([]byte, shardSize)
	err = r.DecodeIdx(dst2, 0, expectInput1, shards[3], 3)
	if err != nil {
		t.Fatal(err)
	}
	err = r.DecodeIdx(dst2, 0, expectInput1, shards[4], 4)
	if err != nil {
		t.Fatal(err)
	}

	// Merge using XOR mode (inputIdx < 0)
	err = r.DecodeIdx(dst1, 0, expectInput1, dst2, -1)
	if err != nil {
		t.Fatal(err)
	}

	// Verify result
	if !bytes.Equal(dst1, shards[0]) {
		t.Error("XOR merge produced incorrect result")
	}
}

// Helper function to fill random data for DecodeIdx tests
func fillRandomDecodeIdx(p []byte) {
	for i := 0; i < len(p); i += 7 {
		val := randomInt63DecodeIdx()
		for j := 0; i+j < len(p) && j < 7; j++ {
			p[i+j] = byte(val)
			val >>= 8
		}
	}
}

// TestDecodeIdx_ParityReconstruction specifically tests parity shard reconstruction
func TestDecodeIdx_ParityReconstruction(t *testing.T) {
	const dataShards = 5
	const parityShards = 3
	const totalShards = dataShards + parityShards
	const shardSize = 100

	enc, err := New(dataShards, parityShards)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	// Create and encode data
	shards := make([][]byte, totalShards)
	for i := 0; i < dataShards; i++ {
		shards[i] = make([]byte, shardSize)
		fillRandomDecodeIdx(shards[i])
	}
	for i := dataShards; i < totalShards; i++ {
		shards[i] = make([]byte, shardSize)
	}

	err = enc.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Save original for verification
	originalShards := make([][]byte, totalShards)
	for i := range shards {
		originalShards[i] = make([]byte, shardSize)
		copy(originalShards[i], shards[i])
	}

	// Test reconstruction of each parity shard
	for targetShard := dataShards; targetShard < totalShards; targetShard++ {
		t.Run(fmt.Sprintf("parity_%d", targetShard-dataShards), func(t *testing.T) {
			// Use first dataShards shards (all data shards)
			expectInput := make([]bool, totalShards)
			for i := 0; i < dataShards; i++ {
				expectInput[i] = true
			}

			// Decode parity shard progressively
			dst := make([]byte, shardSize)
			for inputIdx := 0; inputIdx < dataShards; inputIdx++ {
				err = r.DecodeIdx(dst, targetShard, expectInput, shards[inputIdx], inputIdx)
				if err != nil {
					t.Fatalf("Failed to decode parity shard %d using input %d: %v",
						targetShard-dataShards, inputIdx, err)
				}
			}

			// Verify
			if !bytes.Equal(dst, originalShards[targetShard]) {
				t.Errorf("Parity shard %d reconstruction mismatch", targetShard-dataShards)
			}
		})
	}
}

// TestDecodeIdx_ParityFromMixedShards tests reconstructing parity from mixed data/parity shards
func TestDecodeIdx_ParityFromMixedShards(t *testing.T) {
	const dataShards = 5
	const parityShards = 3
	const totalShards = dataShards + parityShards
	const shardSize = 100

	enc, err := New(dataShards, parityShards)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	// Create and encode data
	shards := make([][]byte, totalShards)
	for i := 0; i < dataShards; i++ {
		shards[i] = make([]byte, shardSize)
		fillRandomDecodeIdx(shards[i])
	}
	for i := dataShards; i < totalShards; i++ {
		shards[i] = make([]byte, shardSize)
	}

	err = enc.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Save original for verification
	originalShards := make([][]byte, totalShards)
	for i := range shards {
		originalShards[i] = make([]byte, shardSize)
		copy(originalShards[i], shards[i])
	}

	// Test reconstructing last parity shard using first 3 data + first 2 parity
	targetShard := totalShards - 1 // last parity shard
	expectInput := make([]bool, totalShards)
	expectInput[0] = true // data 0
	expectInput[1] = true // data 1
	expectInput[2] = true // data 2
	expectInput[dataShards] = true   // parity 0
	expectInput[dataShards+1] = true // parity 1

	// Decode progressively
	dst := make([]byte, shardSize)
	inputIndices := []int{0, 1, 2, dataShards, dataShards + 1}
	for _, inputIdx := range inputIndices {
		err = r.DecodeIdx(dst, targetShard, expectInput, shards[inputIdx], inputIdx)
		if err != nil {
			t.Fatalf("Failed to decode parity shard %d using input %d: %v",
				targetShard, inputIdx, err)
		}
	}

	// Verify
	if !bytes.Equal(dst, originalShards[targetShard]) {
		t.Errorf("Parity shard reconstruction from mixed shards failed")
	}
}

// Simple pseudo-random number generator for consistent test data
var randomSeedDecodeIdx int64 = 1

func randomInt63DecodeIdx() int64 {
	randomSeedDecodeIdx = randomSeedDecodeIdx*1103515245 + 12345
	return (randomSeedDecodeIdx / 65536) % 0x7FFFFFFFFFFFFFFF
}