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
		fillRandom(shards[i])
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
				fillRandom(shards[i])
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
		fillRandom(shards[i])
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
		fillRandom(shards[i])
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

// TestDecodeIdx_AllOptions tests DecodeIdx with all encoder option combinations
func TestDecodeIdx_AllOptions(t *testing.T) {
	opts := [][]Option{
		testOptions(),
		testOptions(WithLeopardGF(true)),
		testOptions(WithLeopardGF16(true)),
		testOptions(WithJerasureMatrix()),
		testOptions(WithCauchyMatrix()),
		testOptions(WithFastOneParityMatrix()),
		testOptions(WithInversionCache(false)),
		testOptions(WithMaxGoroutines(1)),
		testOptions(WithMaxGoroutines(8)),
	}

	testCases := []struct {
		dataShards   int
		parityShards int
		shardSize    int
	}{
		{5, 3, 256}, // Use power of 2 for compatibility
		{10, 4, 256},
		{17, 3, 256},
		{3, 2, 256},
		{2, 1, 256},
		{8, 8, 256},
	}

	for optIdx, opts := range opts {
		for tcIdx, tc := range testCases {
			t.Run(fmt.Sprintf("opts_%d_case_%d_%d+%d", optIdx, tcIdx, tc.dataShards, tc.parityShards), func(t *testing.T) {
				enc, err := New(tc.dataShards, tc.parityShards, opts...)
				if err != nil {
					t.Skip("encoder creation failed:", err)
				}

				// Check if DecodeIdx is supported
				ext, ok := enc.(Extensions)
				if !ok {
					t.Skip("Extensions interface not supported")
				}

				// Test basic DecodeIdx functionality
				totalShards := tc.dataShards + tc.parityShards

				// Create and encode test data
				shards := make([][]byte, totalShards)
				for i := range shards {
					shards[i] = make([]byte, tc.shardSize)
				}

				// Fill data shards with random data
				for i := 0; i < tc.dataShards; i++ {
					fillRandom(shards[i])
				}

				err = enc.Encode(shards)
				if err != nil {
					t.Fatal("encode failed:", err)
				}

				// Save originals
				originals := make([][]byte, totalShards)
				for i := range shards {
					originals[i] = make([]byte, len(shards[i]))
					copy(originals[i], shards[i])
				}

				// Test reconstruction of data shards
				t.Run("data_reconstruction", func(t *testing.T) {
					// Remove some data shards
					damaged := make([][]byte, totalShards)
					copy(damaged, shards)

					damaged[1] = nil
					if tc.dataShards > 3 {
						damaged[tc.dataShards-1] = nil
					}

					// Set up DecodeIdx call
					dst := make([][]byte, totalShards)
					dst[1] = make([]byte, tc.shardSize)
					if tc.dataShards > 3 {
						dst[tc.dataShards-1] = make([]byte, tc.shardSize)
					}

					expectInput := make([]bool, totalShards)
					input := make([][]byte, totalShards)

					// Use first tc.dataShards available shards as input
					inputCount := 0
					for i := 0; i < totalShards && inputCount < tc.dataShards; i++ {
						if damaged[i] != nil {
							expectInput[i] = true
							input[i] = damaged[i]
							inputCount++
						}
					}

					// Call DecodeIdx
					err = ext.DecodeIdx(dst, expectInput, input)
					if err != nil {
						// Check if this is a "not supported" error (leopard cases)
						if errors.Is(err, ErrNotSupported) {
							t.Skip("DecodeIdx not supported for this encoder type")
						}
						t.Fatal("DecodeIdx failed:", err)
					}

					// Verify reconstruction
					if dst[1] != nil && !bytes.Equal(dst[1], originals[1]) {
						t.Error("data shard 1 reconstruction failed")
					}
					if tc.dataShards > 3 && dst[tc.dataShards-1] != nil && !bytes.Equal(dst[tc.dataShards-1], originals[tc.dataShards-1]) {
						t.Error("data shard reconstruction failed")
					}
				})

				// Test reconstruction of parity shards (if supported)
				t.Run("parity_reconstruction", func(t *testing.T) {
					if tc.parityShards == 0 {
						t.Skip("no parity shards")
					}

					// Remove a parity shard
					damaged := make([][]byte, totalShards)
					copy(damaged, shards)

					parityIdx := tc.dataShards
					damaged[parityIdx] = nil

					// Set up DecodeIdx call
					dst := make([][]byte, totalShards)
					dst[parityIdx] = make([]byte, tc.shardSize)

					expectInput := make([]bool, totalShards)
					input := make([][]byte, totalShards)

					// Use first tc.dataShards shards as input
					for i := 0; i < tc.dataShards; i++ {
						expectInput[i] = true
						input[i] = shards[i]
					}

					// Call DecodeIdx
					err = ext.DecodeIdx(dst, expectInput, input)
					if err != nil {
						if errors.Is(err, ErrNotSupported) {
							t.Skip("DecodeIdx not supported for this encoder type")
						}
						t.Fatal("DecodeIdx failed:", err)
					}

					// Verify reconstruction
					if !bytes.Equal(dst[parityIdx], originals[parityIdx]) {
						t.Error("parity shard reconstruction failed")
					}
				})

				// Test progressive reconstruction
				t.Run("progressive_reconstruction", func(t *testing.T) {
					if tc.dataShards < 4 {
						t.Skip("need at least 4 data shards for progressive test")
					}

					// Remove one data shard for reconstruction
					dst := make([][]byte, totalShards)
					dst[1] = make([]byte, tc.shardSize)

					expectInput := make([]bool, totalShards)
					// Mark first tc.dataShards available shards as expected
					inputCount := 0
					for i := 0; i < totalShards && inputCount < tc.dataShards; i++ {
						if i != 1 { // Skip the one we're reconstructing
							expectInput[i] = true
							inputCount++
						}
					}

					// First call - provide half the inputs (but at least 1)
					input1 := make([][]byte, totalShards)
					provided1 := 0
					target1 := tc.dataShards / 2
					if target1 < 1 {
						target1 = 1
					}
					for i := 0; i < totalShards && provided1 < target1; i++ {
						if expectInput[i] {
							input1[i] = shards[i]
							provided1++
						}
					}

					err = ext.DecodeIdx(dst, expectInput, input1)
					if err != nil {
						if errors.Is(err, ErrNotSupported) {
							t.Skip("DecodeIdx not supported for this encoder type")
						}
						t.Fatal("first DecodeIdx call failed:", err)
					}

					// Second call - provide remaining inputs
					input2 := make([][]byte, totalShards)
					provided2 := 0
					for i := 0; i < totalShards && provided1+provided2 < tc.dataShards; i++ {
						if expectInput[i] && input1[i] == nil {
							input2[i] = shards[i]
							provided2++
						}
					}

					err = ext.DecodeIdx(dst, expectInput, input2)
					if err != nil {
						if errors.Is(err, ErrNotSupported) {
							t.Skip("DecodeIdx not supported for this encoder type")
						}
						t.Fatal("second DecodeIdx call failed:", err)
					}

					// Verify reconstruction
					if !bytes.Equal(dst[1], originals[1]) {
						t.Error("progressive reconstruction of shard 1 failed")
					}
				})
			})
		}
	}
}

// TestDecodeIdx_ExcessValidShards tests DecodeIdx when expectInput marks more than dataShards as true
func TestDecodeIdx_ExcessValidShards(t *testing.T) {
	enc, err := New(5, 3, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := enc.(*reedSolomon)

	const shardSize = 256
	const dataShards = 5
	const parityShards = 3
	const totalShards = dataShards + parityShards

	// Create and encode test data
	shards := make([][]byte, totalShards)
	for i := range shards {
		shards[i] = make([]byte, shardSize)
	}

	// Fill data shards with random data
	for i := 0; i < dataShards; i++ {
		fillRandom(shards[i])
	}

	err = enc.Encode(shards)
	if err != nil {
		t.Fatal("encode failed:", err)
	}

	// Save originals for verification
	originals := make([][]byte, totalShards)
	for i := range shards {
		originals[i] = make([]byte, len(shards[i]))
		copy(originals[i], shards[i])
	}

	// Test: expectInput marks 7 shards as valid (more than dataShards=5)
	// This tests the scenario where more shards are marked as available than needed
	t.Run("excess_valid_shards_data_reconstruction", func(t *testing.T) {
		// Set up reconstruction of shard 1
		dst := make([][]byte, totalShards)
		dst[1] = make([]byte, shardSize)

		// Mark 7 shards as expected (more than dataShards=5)
		expectInput := make([]bool, totalShards)
		expectInput[0] = true // data shard
		expectInput[2] = true // data shard
		expectInput[3] = true // data shard
		expectInput[4] = true // data shard
		expectInput[5] = true // parity shard
		expectInput[6] = true // parity shard
		expectInput[7] = true // parity shard

		input := make([][]byte, totalShards)

		// Test 1: Provide only the first dataShards (5) valid shards
		// This should work since matrix is built from first 5 valid shards
		input[0] = shards[0]
		input[2] = shards[2]
		input[3] = shards[3]
		input[4] = shards[4]
		input[5] = shards[5]

		err = r.DecodeIdx(dst, expectInput, input)
		if err != nil {
			t.Fatal("DecodeIdx failed with first 5 valid shards:", err)
		}

		// Verify reconstruction
		if !bytes.Equal(dst[1], originals[1]) {
			t.Error("data shard 1 reconstruction failed")
		}

		// Test 2: Try to provide shards beyond the first dataShards
		// This should fail since matrix was only built from first 5 valid shards
		dst2 := make([][]byte, totalShards)
		dst2[1] = make([]byte, shardSize)

		input2 := make([][]byte, totalShards)
		input2[6] = shards[6] // This is the 6th valid shard, beyond dataShards
		input2[7] = shards[7] // This is the 7th valid shard, beyond dataShards

		err = r.DecodeIdx(dst2, expectInput, input2)
		if err == nil {
			t.Error("Expected error when providing shards beyond first dataShards valid positions, but got none")
		}
		if !strings.Contains(err.Error(), "not in the first") {
			t.Errorf("Expected error about 'not in the first' valid shards, got: %v", err)
		}
	})

	// Test with parity reconstruction
	t.Run("excess_valid_shards_parity_reconstruction", func(t *testing.T) {
		// Set up reconstruction of parity shard 5
		dst := make([][]byte, totalShards)
		dst[5] = make([]byte, shardSize)

		// Mark 6 shards as expected (more than dataShards=5)
		expectInput := make([]bool, totalShards)
		expectInput[0] = true // data shard
		expectInput[1] = true // data shard
		expectInput[2] = true // data shard
		expectInput[3] = true // data shard
		expectInput[4] = true // data shard
		expectInput[6] = true // parity shard (6th valid shard, beyond dataShards)

		input := make([][]byte, totalShards)

		// Provide all data shards (first 5 valid)
		input[0] = shards[0]
		input[1] = shards[1]
		input[2] = shards[2]
		input[3] = shards[3]
		input[4] = shards[4]

		err = r.DecodeIdx(dst, expectInput, input)
		if err != nil {
			t.Fatal("DecodeIdx failed for parity reconstruction with valid data shards:", err)
		}

		// Verify reconstruction
		if !bytes.Equal(dst[5], originals[5]) {
			t.Error("parity shard 5 reconstruction failed")
		}

		// Test providing the excess shard (should fail)
		dst2 := make([][]byte, totalShards)
		dst2[5] = make([]byte, shardSize)

		input2 := make([][]byte, totalShards)
		input2[6] = shards[6] // This is beyond the first dataShards valid positions

		err = r.DecodeIdx(dst2, expectInput, input2)
		if err == nil {
			t.Error("Expected error when providing excess valid shard for parity reconstruction")
		}
	})

	// Test edge case: exactly dataShards valid shards
	t.Run("exactly_dataShards_valid", func(t *testing.T) {
		// Set up reconstruction of shard 1
		dst := make([][]byte, totalShards)
		dst[1] = make([]byte, shardSize)

		// Mark exactly dataShards (5) as expected
		expectInput := make([]bool, totalShards)
		expectInput[0] = true
		expectInput[2] = true
		expectInput[3] = true
		expectInput[4] = true
		expectInput[5] = true // exactly the 5th (last allowed)

		input := make([][]byte, totalShards)
		input[0] = shards[0]
		input[2] = shards[2]
		input[3] = shards[3]
		input[4] = shards[4]
		input[5] = shards[5]

		err = r.DecodeIdx(dst, expectInput, input)
		if err != nil {
			t.Fatal("DecodeIdx failed with exactly dataShards valid shards:", err)
		}

		// Verify reconstruction
		if !bytes.Equal(dst[1], originals[1]) {
			t.Error("reconstruction failed with exactly dataShards valid shards")
		}
	})
}
