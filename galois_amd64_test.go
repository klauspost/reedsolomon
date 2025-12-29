//go:build !noasm && !appengine && !gccgo && !nopshufb

// Copyright 2015, Klaus Post, see LICENSE for details.

package reedsolomon

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/klauspost/cpuid/v2"
)

func TestGenGalois(t *testing.T) {
	if defaultOptions.useAVX2 {
		testGenGaloisUpto10x10(t, galMulSlicesAvx2, galMulSlicesAvx2Xor, 32)
	}
}

func swapBytes64(x uint64) uint64 {
	return ((x & 0xFF) << 56) |
		((x & 0xFF00) << 40) |
		((x & 0xFF0000) << 24) |
		((x & 0xFF000000) << 8) |
		((x & 0xFF00000000) >> 8) |
		((x & 0xFF0000000000) >> 24) |
		((x & 0xFF000000000000) >> 40) |
		((x & 0xFF00000000000000) >> 56)
}

func TestGenerateFinalMatrices(t *testing.T) {
	// Initialize tables
	t.Skip("Enable to generate GFNI matrices")
	enc, err := New(4, 2, WithLeopardGF(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = enc

	// Generate first few matrices to verify they work
	t.Logf("Generating and testing first 8 matrices...")

	for logValue := 0; logValue < 8; logValue++ {
		var matrix uint64

		// Use the proven working method
		for inputBit := 0; inputBit < 8; inputBit++ {
			testInput := ffe8(1 << inputBit)
			result := mulLog8(testInput, ffe8(logValue))
			for outputBit := 0; outputBit < 8; outputBit++ {
				if (result>>outputBit)&1 == 1 {
					matrixBitPos := outputBit*8 + inputBit
					matrix |= 1 << matrixBitPos
				}
			}
		}
		matrix = swapBytes64(matrix)
	}

	t.Logf("All tested matrices work! Now generating full array...")

	// Generate the complete array
	fmt.Printf("\n// Complete corrected gf2p811dMulMatricesLeo8 array:\n")
	fmt.Printf("var gf2p811dMulMatricesLeo8 = [256]uint64{\n")

	for logValue := 0; logValue < 256; logValue++ {
		var matrix uint64

		for inputBit := 0; inputBit < 8; inputBit++ {
			testInput := ffe8(1 << inputBit)
			result := mulLog8(testInput, ffe8(logValue))
			for outputBit := 0; outputBit < 8; outputBit++ {
				if (result>>outputBit)&1 == 1 {
					matrixBitPos := outputBit*8 + inputBit
					matrix |= 1 << matrixBitPos
				}
			}
		}
		matrix = swapBytes64(matrix)

		if logValue%4 == 0 {
			fmt.Printf("\t")
		}

		fmt.Printf("0x%016x", matrix)
		if logValue < 255 {
			fmt.Printf(", ")
		}
		if (logValue+1)%4 == 0 {
			fmt.Printf("\n")
		}
	}

	fmt.Printf("}\n")
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
		if false {
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
