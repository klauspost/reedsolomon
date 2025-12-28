package reedsolomon

import (
	"bytes"
	"errors"
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

	dst := make([][]byte, 8)
	input := make([][]byte, 8)
	wrongExpectInput := make([]bool, 7) // Should be 8 (5+3)

	err = r.DecodeIdx(dst, wrongExpectInput, input)
	if err == nil {
		t.Fatal("expected error for wrong expectInput length")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got: %v", err)
	}
	if !strings.Contains(err.Error(), "expectInput length") {
		t.Errorf("error should mention expectInput length: %v", err)
	}
}

// TestDecodeIdx_WrongSliceLengths tests that DecodeIdx returns an error
// when dst or input don't have the correct length
func TestDecodeIdx_WrongSliceLengths(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	expectInput := make([]bool, 8)
	for i := 0; i < 5; i++ {
		expectInput[i] = true
	}

	// Test wrong dst length
	dst := make([][]byte, 7)
	input := make([][]byte, 8)
	err = r.DecodeIdx(dst, expectInput, input)
	if err == nil {
		t.Fatal("expected error for wrong dst length")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for wrong dst length, got: %v", err)
	}

	// Test wrong input length
	dst = make([][]byte, 8)
	input = make([][]byte, 7)
	err = r.DecodeIdx(dst, expectInput, input)
	if err == nil {
		t.Fatal("expected error for wrong input length")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for wrong input length, got: %v", err)
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

	dst := make([][]byte, 8)
	input := make([][]byte, 8)
	expectInput := make([]bool, 8)
	for i := 0; i < 5; i++ {
		expectInput[i] = true
	}

	// Mark shard 7 as expected but also provide it in dst
	expectInput[7] = true
	dst[7] = make([]byte, 100)

	err = r.DecodeIdx(dst, expectInput, input)
	if err == nil {
		t.Fatal("expected error for already filled destination")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got: %v", err)
	}
	if !strings.Contains(err.Error(), "should be nil") {
		t.Errorf("error should mention dst should be nil: %v", err)
	}
}

// TestDecodeIdx_UnexpectedInput tests that DecodeIdx returns an error
// when input is provided at an index not marked in expectInput
func TestDecodeIdx_UnexpectedInput(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	dst := make([][]byte, 8)
	input := make([][]byte, 8)
	expectInput := make([]bool, 8)
	for i := 0; i < 5; i++ {
		expectInput[i] = true
	}

	// Provide input at index 7 which is not marked as expected
	input[7] = make([]byte, 100)
	input[0] = make([]byte, 100)
	dst[6] = make([]byte, 100) // Decode into shard 6

	err = r.DecodeIdx(dst, expectInput, input)
	if err == nil {
		t.Fatal("expected error for unexpected input")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got: %v", err)
	}
	if !strings.Contains(err.Error(), "unexpected input") {
		t.Errorf("error should mention unexpected input: %v", err)
	}
}

// TestDecodeIdx_TooFewShards tests that DecodeIdx returns an error
// when there are too few shards marked in expectInput
func TestDecodeIdx_TooFewShards(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	dst := make([][]byte, 8)
	input := make([][]byte, 8)
	expectInput := make([]bool, 8)
	// Only mark 4 shards as expected (need at least 5)
	for i := 0; i < 4; i++ {
		expectInput[i] = true
	}

	err = r.DecodeIdx(dst, expectInput, input)
	if !errors.Is(err, ErrTooFewShards) {
		t.Errorf("expected ErrTooFewShards, got %v", err)
	}
}

// TestDecodeIdx_MismatchedBufferSizes tests that DecodeIdx returns an error
// when buffer sizes don't match
func TestDecodeIdx_MismatchedBufferSizes(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	dst := make([][]byte, 8)
	input := make([][]byte, 8)
	expectInput := make([]bool, 8)
	for i := 0; i < 5; i++ {
		expectInput[i] = true
	}

	// Set up mismatched sizes - reconstruct shards 5 and 6 with different sizes
	dst[5] = make([]byte, 100)
	dst[6] = make([]byte, 200)   // Different size
	input[0] = make([]byte, 100) // Provide input at index 0

	err = r.DecodeIdx(dst, expectInput, input)
	if !errors.Is(err, ErrInvalidShardSize) {
		t.Errorf("expected ErrInvalidShardSize, got %v", err)
	}
}

// TestDecodeIdx_MergeMode tests merging mode (expectInput == nil)
func TestDecodeIdx_MergeMode(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	dst := make([][]byte, 8)
	input := make([][]byte, 8)

	// Set up data to merge
	dst[0] = []byte{1, 2, 3, 4}
	input[0] = []byte{5, 6, 7, 8}

	dst[3] = []byte{10, 20, 30, 40}
	input[3] = []byte{50, 60, 70, 80}

	err = r.DecodeIdx(dst, nil, input)
	if err != nil {
		t.Fatal(err)
	}

	// Check XOR results
	expected0 := []byte{1 ^ 5, 2 ^ 6, 3 ^ 7, 4 ^ 8}
	if !bytes.Equal(dst[0], expected0) {
		t.Errorf("merge failed for shard 0: got %v, expected %v", dst[0], expected0)
	}

	expected3 := []byte{10 ^ 50, 20 ^ 60, 30 ^ 70, 40 ^ 80}
	if !bytes.Equal(dst[3], expected3) {
		t.Errorf("merge failed for shard 3: got %v, expected %v", dst[3], expected3)
	}
}

// TestDecodeIdx_ProgressiveDecode tests progressive decoding of shards
func TestDecodeIdx_ProgressiveDecode(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	// Create and encode data
	shards := make([][]byte, 8)
	for i := 0; i < 5; i++ {
		shards[i] = make([]byte, 100)
		fillRandomDecodeIdx(shards[i])
	}
	for i := 5; i < 8; i++ {
		shards[i] = make([]byte, 100)
	}

	err = enc.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Save originals
	originals := make([][]byte, 8)
	for i := range shards {
		originals[i] = make([]byte, 100)
		copy(originals[i], shards[i])
	}

	// Progressive decode: reconstruct shards 0 and 7 using shards 1-5
	dst := make([][]byte, 8)
	dst[0] = make([]byte, 100) // Reconstruct data shard 0
	dst[7] = make([]byte, 100) // Reconstruct parity shard 7

	expectInput := make([]bool, 8)
	for i := 1; i <= 5; i++ {
		expectInput[i] = true
	}

	// First call with shards 1-3
	input := make([][]byte, 8)
	for i := 1; i <= 3; i++ {
		input[i] = shards[i]
	}

	err = r.DecodeIdx(dst, expectInput, input)
	if err != nil {
		t.Fatal(err)
	}

	// Second call with shards 4-5
	input2 := make([][]byte, 8)
	for i := 4; i <= 5; i++ {
		input2[i] = shards[i]
	}

	err = r.DecodeIdx(dst, expectInput, input2)
	if err != nil {
		t.Fatal(err)
	}

	// Verify reconstructed shards
	if !bytes.Equal(dst[0], originals[0]) {
		t.Error("data shard 0 reconstruction failed")
	}
	if !bytes.Equal(dst[7], originals[7]) {
		t.Error("parity shard 7 reconstruction failed")
	}
}

// TestDecodeIdx_IntegrationWithFullCycle tests full encode/decode cycle
func TestDecodeIdx_IntegrationWithFullCycle(t *testing.T) {
	testCases := []struct {
		dataShards   int
		parityShards int
	}{
		{5, 3},
		{10, 4},
		{2, 2},
		{17, 3},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d+%d", tc.dataShards, tc.parityShards), func(t *testing.T) {
			enc, err := New(tc.dataShards, tc.parityShards, testOptions()...)
			if err != nil {
				t.Fatal(err)
			}
			r := enc.(*reedSolomon)

			totalShards := tc.dataShards + tc.parityShards
			shardSize := 100

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
				// Prepare dst with only the target shard to reconstruct
				dst := make([][]byte, totalShards)
				dst[targetShard] = make([]byte, shardSize)

				// Mark first dataShards shards as expected (excluding target if it's one of them)
				expectInput := make([]bool, totalShards)
				count := 0
				for i := 0; i < totalShards && count < tc.dataShards; i++ {
					if i != targetShard {
						expectInput[i] = true
						count++
					}
				}

				// Provide all expected inputs in one call
				input := make([][]byte, totalShards)
				for i := 0; i < totalShards; i++ {
					if expectInput[i] {
						input[i] = shards[i]
					}
				}

				err = r.DecodeIdx(dst, expectInput, input)
				if err != nil {
					t.Fatalf("Failed to decode shard %d: %v", targetShard, err)
				}

				// Verify
				if !bytes.Equal(dst[targetShard], originalShards[targetShard]) {
					t.Errorf("Shard %d reconstruction mismatch", targetShard)
				}
			}
		})
	}
}

// TestDecodeIdx_MultipleShards tests reconstructing multiple shards in one call
func TestDecodeIdx_MultipleShards(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	// Create and encode data
	shards := make([][]byte, 8)
	for i := 0; i < 5; i++ {
		shards[i] = make([]byte, 100)
		fillRandomDecodeIdx(shards[i])
	}
	for i := 5; i < 8; i++ {
		shards[i] = make([]byte, 100)
	}

	err = enc.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Save originals
	originals := make([][]byte, 8)
	for i := range shards {
		originals[i] = make([]byte, 100)
		copy(originals[i], shards[i])
	}

	// Reconstruct shards 0, 2, and 7 using shards 1, 3, 4, 5, 6
	dst := make([][]byte, 8)
	dst[0] = make([]byte, 100) // data shard
	dst[2] = make([]byte, 100) // data shard
	dst[7] = make([]byte, 100) // parity shard

	expectInput := make([]bool, 8)
	expectInput[1] = true
	expectInput[3] = true
	expectInput[4] = true
	expectInput[5] = true
	expectInput[6] = true

	input := make([][]byte, 8)
	input[1] = shards[1]
	input[3] = shards[3]
	input[4] = shards[4]
	input[5] = shards[5]
	input[6] = shards[6]

	err = r.DecodeIdx(dst, expectInput, input)
	if err != nil {
		t.Fatal(err)
	}

	// Verify all reconstructed shards
	if !bytes.Equal(dst[0], originals[0]) {
		t.Error("data shard 0 reconstruction failed")
	}
	if !bytes.Equal(dst[2], originals[2]) {
		t.Error("data shard 2 reconstruction failed")
	}
	if !bytes.Equal(dst[7], originals[7]) {
		t.Error("parity shard 7 reconstruction failed")
	}
}

// TestDecodeIdx_MergeTwoPartialDecodings tests merging partial decodings
func TestDecodeIdx_MergeTwoPartialDecodings(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	// Create and encode data
	shards := make([][]byte, 8)
	for i := 0; i < 5; i++ {
		shards[i] = make([]byte, 100)
		fillRandomDecodeIdx(shards[i])
	}
	for i := 5; i < 8; i++ {
		shards[i] = make([]byte, 100)
	}

	err = enc.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Save original
	original := make([]byte, 100)
	copy(original, shards[0])

	// First partial decode using shards 1-3
	dst1 := make([][]byte, 8)
	dst1[0] = make([]byte, 100)

	expectInput1 := make([]bool, 8)
	for i := 1; i <= 5; i++ {
		expectInput1[i] = true
	}

	input1 := make([][]byte, 8)
	for i := 1; i <= 3; i++ {
		input1[i] = shards[i]
	}

	err = r.DecodeIdx(dst1, expectInput1, input1)
	if err != nil {
		t.Fatal(err)
	}

	// Second partial decode using shards 4-5
	dst2 := make([][]byte, 8)
	dst2[0] = make([]byte, 100)

	input2 := make([][]byte, 8)
	for i := 4; i <= 5; i++ {
		input2[i] = shards[i]
	}

	err = r.DecodeIdx(dst2, expectInput1, input2)
	if err != nil {
		t.Fatal(err)
	}

	// Merge the two partial decodings
	err = r.DecodeIdx(dst1, nil, dst2)
	if err != nil {
		t.Fatal(err)
	}

	// Verify merged result
	if !bytes.Equal(dst1[0], original) {
		t.Error("merged decoding does not match original")
	}
}

// Simple pseudo-random number generator for consistent test data
var randomSeedDecodeIdx int64 = 1

func randomInt63DecodeIdx() int64 {
	randomSeedDecodeIdx = randomSeedDecodeIdx*1103515245 + 12345
	return (randomSeedDecodeIdx / 65536) % 0x7FFFFFFFFFFFFFFF
}

func fillRandomDecodeIdx(b []byte) {
	for i := range b {
		b[i] = byte(randomInt63DecodeIdx() & 0xFF)
	}
}
