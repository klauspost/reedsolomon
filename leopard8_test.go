package reedsolomon

import (
	"bytes"
	"encoding/hex"
	"maps"
	"slices"
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

func TestCompareIfftDIT48GFNIvsAVX2(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.GFNI, cpuid.AVX512VL) {
		t.Skip("GFNI not supported")
	}

	// Create a simple encoder to ensure tables are initialized
	enc, err := New(4, 2, WithLeopardGF(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = enc

	// Create test shards - 4 work slices of 64 bytes each (leopard8 requirement)
	work := make([][]byte, 4)
	for i := range work {
		work[i] = make([]byte, 128)
		for j := range work[i] {
			work[i][j] = byte(i*16 + j + 1) // Simple test pattern
		}
	}

	// Test each variant
	results := make(map[int]string)
	for variant := 0; variant < 8; variant++ {
		// Create copies for GFNI and AVX2
		gfniWork := make([][]byte, len(work))
		avx2Work := make([][]byte, len(work))
		for i := range work {
			gfniWork[i] = make([]byte, len(work[i]))
			avx2Work[i] = make([]byte, len(work[i]))
			copy(gfniWork[i], work[i])
			copy(avx2Work[i], work[i])
		}

		// Call assembly functions directly
		log_m01, log_m23, log_m02 := ffe8(10), ffe8(200), ffe8(30)

		// Call GFNI variant directly
		t01 := gf2p811dMulMatricesLeo8[log_m01]
		t23 := gf2p811dMulMatricesLeo8[log_m23]
		t02 := gf2p811dMulMatricesLeo8[log_m02]

		switch variant {
		case 0:
			ifftDIT48_gfni_0(gfniWork, 24, t01, t23, t02)
		case 1:
			ifftDIT48_gfni_1(gfniWork, 24, t01, t23, t02)
		case 2:
			ifftDIT48_gfni_2(gfniWork, 24, t01, t23, t02)
		case 3:
			ifftDIT48_gfni_3(gfniWork, 24, t01, t23, t02)
		case 4:
			ifftDIT48_gfni_4(gfniWork, 24, t01, t23, t02)
		case 5:
			ifftDIT48_gfni_5(gfniWork, 24, t01, t23, t02)
		case 6:
			ifftDIT48_gfni_6(gfniWork, 24, t01, t23, t02)
		case 7:
			ifftDIT48_gfni_7(gfniWork, 24, t01, t23, t02)
		}

		// Call AVX2 variant directly
		at01 := &multiply256LUT8[log_m01]
		at23 := &multiply256LUT8[log_m23]
		at02 := &multiply256LUT8[log_m02]
		if true {
			switch variant {
			case 0:
				ifftDIT48_avx2_0(avx2Work, 24, at01, at23, at02)
			case 1:
				ifftDIT48_avx2_1(avx2Work, 24, at01, at23, at02)
			case 2:
				ifftDIT48_avx2_2(avx2Work, 24, at01, at23, at02)
			case 3:
				ifftDIT48_avx2_3(avx2Work, 24, at01, at23, at02)
			case 4:
				ifftDIT48_avx2_4(avx2Work, 24, at01, at23, at02)
			case 5:
				ifftDIT48_avx2_5(avx2Work, 24, at01, at23, at02)
			case 6:
				ifftDIT48_avx2_6(avx2Work, 24, at01, at23, at02)
			case 7:
				ifftDIT48_avx2_7(avx2Work, 24, at01, at23, at02)
			}
		} else if true {
			switch variant {
			case 0:
				ifftDIT4Ref8(avx2Work, 1, log_m01, log_m23, log_m02, &options{})
			case 1:
				ifftDIT4Ref8(avx2Work, 1, 255, log_m23, log_m02, &options{})
			case 2:
				ifftDIT4Ref8(avx2Work, 1, log_m01, 255, log_m02, &options{})
			case 3:
				ifftDIT4Ref8(avx2Work, 1, 255, 255, log_m02, &options{})
			case 4:
				ifftDIT4Ref8(avx2Work, 1, log_m01, log_m23, 255, &options{})
			case 5:
				ifftDIT4Ref8(avx2Work, 1, 255, log_m23, 255, &options{})
			case 6:
				ifftDIT4Ref8(avx2Work, 1, log_m01, 255, 255, &options{})
			case 7:
				ifftDIT4Ref8(avx2Work, 1, 255, 255, 255, &options{})
			}
		}

		// Compare results
		match := make(map[int]bool, len(gfniWork))
		for i := range gfniWork {
			if !bytes.Equal(gfniWork[i], avx2Work[i]) {
				match[i] = false
			}
		}

		if len(match) == 0 {
			results[variant] = "✅ MATCH"
			t.Logf("ifftDIT48_%d: MATCH ✅", variant)
		} else {
			results[variant] = "❌ MISMATCH"
			t.Logf("ifftDIT48_%d: MISMATCH ❌", variant)
			for _, i := range slices.Sorted(maps.Keys(match)) {
				t.Logf("  Work idx %d: avx2: %s, gfni: %s", i, hex.EncodeToString(avx2Work[i]), hex.EncodeToString(gfniWork[i]))
			}
		}
	}

	// Summary
	t.Logf("\n=== ifftDIT48 SUMMARY ===")
	for variant := 0; variant < 8; variant++ {
		t.Logf("Variant %d: %s", variant, results[variant])
	}
}

func TestCompareFftDIT48GFNIvsAVX2(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.GFNI, cpuid.AVX512VL) {
		t.Skip("GFNI not supported")
	}

	// Create a simple encoder to ensure tables are initialized
	enc, err := New(4, 2, WithLeopardGF(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = enc

	// Create test shards - 4 work slices of 64 bytes each (leopard8 requirement)
	work := make([][]byte, 4)
	for i := range work {
		work[i] = make([]byte, 128)
		for j := range work[i] {
			work[i][j] = byte(i*16 + j + 1) // Simple test pattern
		}
	}

	// Test each variant
	results := make(map[int]string)
	for variant := 0; variant < 8; variant++ {
		// Create copies for GFNI and AVX2
		gfniWork := make([][]byte, len(work))
		avx2Work := make([][]byte, len(work))
		for i := range work {
			gfniWork[i] = make([]byte, len(work[i]))
			avx2Work[i] = make([]byte, len(work[i]))
			copy(gfniWork[i], work[i])
			copy(avx2Work[i], work[i])
		}

		// Call assembly functions directly
		log_m01, log_m23, log_m02 := ffe8(1), ffe8(2), ffe8(3)

		// Call GFNI variant directly
		t01 := gf2p811dMulMatricesLeo8[log_m01]
		t23 := gf2p811dMulMatricesLeo8[log_m23]
		t02 := gf2p811dMulMatricesLeo8[log_m02]

		switch variant {
		case 0:
			fftDIT48_gfni_0(gfniWork, 24, t01, t23, t02)
		case 1:
			fftDIT48_gfni_1(gfniWork, 24, t01, t23, t02)
		case 2:
			fftDIT48_gfni_2(gfniWork, 24, t01, t23, t02)
		case 3:
			fftDIT48_gfni_3(gfniWork, 24, t01, t23, t02)
		case 4:
			fftDIT48_gfni_4(gfniWork, 24, t01, t23, t02)
		case 5:
			fftDIT48_gfni_5(gfniWork, 24, t01, t23, t02)
		case 6:
			fftDIT48_gfni_6(gfniWork, 24, t01, t23, t02)
		case 7:
			fftDIT48_gfni_7(gfniWork, 24, t01, t23, t02)
		}

		// Call AVX2 variant directly
		at01 := &multiply256LUT8[log_m01]
		at23 := &multiply256LUT8[log_m23]
		at02 := &multiply256LUT8[log_m02]

		switch variant {
		case 0:
			fftDIT48_avx2_0(avx2Work, 24, at01, at23, at02)
		case 1:
			fftDIT48_avx2_1(avx2Work, 24, at01, at23, at02)
		case 2:
			fftDIT48_avx2_2(avx2Work, 24, at01, at23, at02)
		case 3:
			fftDIT48_avx2_3(avx2Work, 24, at01, at23, at02)
		case 4:
			fftDIT48_avx2_4(avx2Work, 24, at01, at23, at02)
		case 5:
			fftDIT48_avx2_5(avx2Work, 24, at01, at23, at02)
		case 6:
			fftDIT48_avx2_6(avx2Work, 24, at01, at23, at02)
		case 7:
			fftDIT48_avx2_7(avx2Work, 24, at01, at23, at02)
		}

		// Compare results
		match := true
		for i := range gfniWork {
			if !bytes.Equal(gfniWork[i], avx2Work[i]) {
				match = false
				break
			}
		}

		if match {
			results[variant] = "✅ MATCH"
			t.Logf("fftDIT48_%d: MATCH ✅", variant)
		} else {
			results[variant] = "❌ MISMATCH"
			t.Logf("fftDIT48_%d: MISMATCH ❌", variant)
		}
	}

	// Summary
	t.Logf("\n=== fftDIT48 SUMMARY ===")
	for variant := 0; variant < 8; variant++ {
		t.Logf("Variant %d: %s", variant, results[variant])
	}
}

func TestDebugGFNI(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.GFNI, cpuid.AVX512VL) {
		t.Skip("GFNI not supported")
	}

	// Create a simple encoder to ensure tables are initialized
	enc, err := New(4, 2, WithLeopardGF(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = enc

	// Create test shards - 4 work slices of 64 bytes each
	work := make([][]byte, 4)
	for i := range work {
		work[i] = make([]byte, 64)
		for j := range work[i] {
			work[i][j] = byte(i*16 + j + 1) // Simple test pattern
		}
	}

	// Test variant 0 (no skips) - this should do all multiplications
	log_m01, log_m23, log_m02 := ffe8(1), ffe8(2), ffe8(3)

	// Create copies for GFNI, reference, and AVX2
	gfniWork := make([][]byte, len(work))
	refWork := make([][]byte, len(work))
	avx2Work := make([][]byte, len(work))
	for i := range work {
		gfniWork[i] = make([]byte, len(work[i]))
		refWork[i] = make([]byte, len(work[i]))
		avx2Work[i] = make([]byte, len(work[i]))
		copy(gfniWork[i], work[i])
		copy(refWork[i], work[i])
		copy(avx2Work[i], work[i])
	}

	// Call reference implementation
	o := &options{}
	ifftDIT4Ref8(refWork, 1, log_m01, log_m23, log_m02, o)

	// Call GFNI directly
	t01 := gf2p811dMulMatricesLeo8[log_m01]
	t23 := gf2p811dMulMatricesLeo8[log_m23]
	t02 := gf2p811dMulMatricesLeo8[log_m02]
	ifftDIT48_gfni_0(gfniWork, 24, t01, t23, t02)

	// Call AVX2 directly
	at01 := &multiply256LUT8[log_m01]
	at23 := &multiply256LUT8[log_m23]
	at02 := &multiply256LUT8[log_m02]
	ifftDIT48_avx2_0(avx2Work, 24, at01, at23, at02)

	// Compare byte by byte
	t.Logf("Comparing GFNI vs Reference vs AVX2:")
	for i := range work {
		t.Logf("\n=== Work[%d] ===", i)

		// Find first difference
		firstDiff := -1
		for j := range work[i] {
			if gfniWork[i][j] != refWork[i][j] || avx2Work[i][j] != refWork[i][j] {
				firstDiff = j
				break
			}
		}

		if firstDiff >= 0 {
			// Show a few bytes around the first difference
			start := firstDiff
			if start > 2 {
				start = firstDiff - 2
			}
			end := firstDiff + 5
			if end > len(work[i]) {
				end = len(work[i])
			}

			t.Logf("First difference at byte %d:", firstDiff)
			t.Logf("  Original: %v", work[i][start:end])
			t.Logf("  Ref:      %v", refWork[i][start:end])
			t.Logf("  AVX2:     %v", avx2Work[i][start:end])
			t.Logf("  GFNI:     %v", gfniWork[i][start:end])

			// Show the specific byte difference
			if firstDiff < len(work[i]) {
				t.Logf("  At byte %d: Orig=%02x, Ref=%02x, AVX2=%02x, GFNI=%02x",
					firstDiff, work[i][firstDiff], refWork[i][firstDiff],
					avx2Work[i][firstDiff], gfniWork[i][firstDiff])
			}
		} else if bytes.Equal(gfniWork[i], refWork[i]) && bytes.Equal(avx2Work[i], refWork[i]) {
			t.Logf("  ✅ All match!")
		}
	}

	// Check if AVX2 matches reference
	avx2MatchesRef := true
	for i := range work {
		if !bytes.Equal(avx2Work[i], refWork[i]) {
			avx2MatchesRef = false
			break
		}
	}

	// Check if GFNI matches reference
	gfniMatchesRef := true
	for i := range work {
		if !bytes.Equal(gfniWork[i], refWork[i]) {
			gfniMatchesRef = false
			break
		}
	}

	t.Logf("\n=== Summary ===")
	t.Logf("AVX2 matches Reference: %v", avx2MatchesRef)
	t.Logf("GFNI matches Reference: %v", gfniMatchesRef)

	// If GFNI doesn't match but AVX2 does, let's debug the matrix multiplication
	if !gfniMatchesRef && avx2MatchesRef {
		t.Logf("\n=== Debugging Matrix Multiplication ===")
		// Test a single multiplication with log_m01 (which is 1)
		testInput := byte(0x01)

		// Reference using mulLog8
		refResult := mulLog8(ffe8(testInput), ffe8(log_m01))

		// AVX2 table lookup
		avx2Table := &multiply256LUT8[log_m01]
		lo := testInput & 0x0f
		hi := (testInput >> 4) & 0x0f
		avx2Result := (*avx2Table)[lo] ^ (*avx2Table)[16+hi]

		// GFNI matrix
		matrix := gf2p811dMulMatricesLeo8[log_m01]
		gfniResult := applyGFNIMatrixDebug(testInput, matrix)

		t.Logf("Single byte test with log_m01=%d:", log_m01)
		t.Logf("  Input:    0x%02x", testInput)
		t.Logf("  Ref:      0x%02x (using mulLog8)", refResult)
		t.Logf("  AVX2:     0x%02x (using table lookup)", avx2Result)
		t.Logf("  GFNI:     0x%02x (using matrix 0x%016x)", gfniResult, matrix)

		// Check what multiply256LUT8 actually contains
		t.Logf("\n  multiply256LUT8[%d] first 16 bytes (low nibble): %v", log_m01, (*avx2Table)[:16])
		t.Logf("  multiply256LUT8[%d] next 16 bytes (high nibble): %v", log_m01, (*avx2Table)[16:32])
	}
}

func applyGFNIMatrixDebug(input byte, matrix uint64) byte {
	result := byte(0)
	// Apply 8x8 matrix transformation according to VGF2P8AFFINEQB spec
	for resultBit := 0; resultBit < 8; resultBit++ {
		// Get the matrix row for this result bit (byte 0 controls bit 7, etc.)
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
	// VGF2P8AFFINEQB with immediate 0x00 - no XOR needed
	return result
}
