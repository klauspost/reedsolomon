package reedsolomon

import (
	"bytes"
	"testing"

	"github.com/klauspost/cpuid/v2"
)

func TestAddMod8(t *testing.T) {
	type testCase struct {
		x        ffe8
		y        ffe8
		expected ffe8
	}
	testCases := []testCase{
		{x: ffe8(1), y: ffe8(2), expected: ffe8(3)},
		{x: ffe8(253), y: ffe8(1), expected: ffe8(254)},
		{x: ffe8(254), y: ffe8(2), expected: ffe8(1)},
		{x: ffe8(255), y: ffe8(1), expected: ffe8(1)},
		// it is expected that the following tests cases return modulus and that
		// callers of addMod will convert it to 0.
		{x: ffe8(254), y: ffe8(1), expected: ffe8(255)},
		{x: ffe8(255), y: ffe8(0), expected: ffe8(255)},
		{x: ffe8(255), y: ffe8(255), expected: ffe8(255)},
	}
	for _, tc := range testCases {
		got := addMod8(tc.x, tc.y)
		if tc.expected != got {
			t.Errorf("expected %v, got %v", tc.expected, got)
		}
	}
}

func TestGFNIMultiplication(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.GFNI, cpuid.AVX512VL) {
		t.Skip("GFNI not supported")
	}

	// Create a simple encoder to ensure tables are initialized
	enc, err := New(4, 2, WithLeopardGF(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = enc

	// Test simple multiplication: multiply a single byte using both methods
	testValue := byte(0x42)
	multiplier := ffe8(5) // Use a non-trivial multiplier

	// Create test data - single byte repeated 64 times for SIMD alignment
	testData := make([]byte, 64)
	for i := range testData {
		testData[i] = testValue
	}

	// Test with AVX2 lookup table method
	avx2Result := make([]byte, 64)
	copy(avx2Result, testData)
	avx2Table := &multiply256LUT8[multiplier]

	// We need a simple way to call the multiplication - let's use the direct table lookup
	for i := range avx2Result {
		lo := avx2Result[i] & 0x0f
		hi := (avx2Result[i] >> 4) & 0x0f
		// First 16 bytes are for low nibble, second 16 bytes for high nibble
		avx2Result[i] = (*avx2Table)[lo] ^ (*avx2Table)[16+hi]
	}

	// Test with GFNI matrix method
	gfniResult := make([]byte, 64)
	copy(gfniResult, testData)
	gfniMatrix := gf2p811dMulMatricesLeo8[multiplier]

	// Manually compute what VGF2P8AFFINEQB should produce
	for i := range gfniResult {
		input := gfniResult[i]
		result := byte(0)

		// Apply 8x8 matrix transformation according to VGF2P8AFFINEQB spec
		// The matrix is stored with byte 0 controlling bit 7, byte 1 controlling bit 6, etc.
		matrix := gfniMatrix
		for resultBit := 0; resultBit < 8; resultBit++ {
			// Get the matrix row for this result bit (note: bit 7-i indexing)
			row := byte((matrix >> (8 * (7 - resultBit))) & 0xff)
			// Compute dot product of input with this row
			dotProduct := byte(0)
			for inputBit := 0; inputBit < 8; inputBit++ {
				if (input>>inputBit)&1 == 1 && (row>>inputBit)&1 == 1 {
					dotProduct ^= 1
				}
			}
			result |= dotProduct << resultBit
		}
		gfniResult[i] = result
	}

	// Verify which result is correct using the reference implementation
	correctResult := mulLog8(ffe8(testValue), multiplier)

	// Compare results
	if !bytes.Equal(avx2Result, gfniResult) {
		t.Errorf("GFNI multiplication doesn't match AVX2")
		t.Logf("Input: %02x, Multiplier: %d", testValue, multiplier)
		t.Logf("AVX2 result: %02x", avx2Result[0])
		t.Logf("GFNI result: %02x", gfniResult[0])
		t.Logf("Correct result: %02x", correctResult)

		if avx2Result[0] == byte(correctResult) {
			t.Logf("AVX2 is correct, GFNI is wrong")
		} else if gfniResult[0] == byte(correctResult) {
			t.Logf("GFNI is correct, AVX2 is wrong")
		} else {
			t.Logf("Both are wrong!")
		}

		// Show first few bytes for debugging
		for i := 0; i < 8; i++ {
			t.Logf("Byte %d: AVX2=%02x GFNI=%02x", i, avx2Result[i], gfniResult[i])
		}

		// Debug the GFNI matrix
		t.Logf("GFNI matrix for multiplier %d: %016x", multiplier, gfniMatrix)
	}

	// NOTE: This test demonstrates the issue - it should be used to verify the fix
	t.Logf("This test shows GFNI vs AVX2 differences - used for debugging VGF2P8AFFINEQB immediate values")
}
