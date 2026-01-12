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

func TestCompareIfftDIT2GF16GFNIvsAVX2(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.GFNI, cpuid.AVX) {
		t.Skip("AVX+GFNI not supported")
	}

	// Create a simple encoder to ensure tables are initialized
	enc, err := New(300, 100, WithLeopardGF16(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = enc

	if gf2p811dMulMatrices16 == nil {
		t.Skip("GFNI tables not initialized")
	}

	// Test with various log_m values
	testCases := []ffe{0, 1, 100, 1000, 10000, 30000, 65535}

	for _, logM := range testCases {
		t.Run(fmt.Sprintf("logM=%d", logM), func(t *testing.T) {
			// Create test data - 128 bytes (64 16-bit elements)
			x := make([]byte, 128)
			y := make([]byte, 128)
			for i := range x {
				x[i] = byte(i + 1)
				y[i] = byte(i*2 + 3)
			}

			// Make copies for both implementations
			xGFNI := make([]byte, len(x))
			yGFNI := make([]byte, len(y))
			copy(xGFNI, x)
			copy(yGFNI, y)

			xAVX2 := make([]byte, len(x))
			yAVX2 := make([]byte, len(y))
			copy(xAVX2, x)
			copy(yAVX2, y)

			// Compute Go reference (correct result)
			// Data layout is SPLIT: [0:32] = lo bytes, [32:64] = hi bytes of 32 elements
			// Then [64:96] = lo bytes, [96:128] = hi bytes of next 32 elements
			xRef := make([]byte, len(x))
			yRef := make([]byte, len(y))
			copy(xRef, x)
			copy(yRef, y)
			// Process 64 bytes at a time (32 elements)
			for chunk := 0; chunk < 2; chunk++ {
				base := chunk * 64
				for i := 0; i < 32; i++ {
					xElem := ffe(x[base+i]) | (ffe(x[base+32+i]) << 8)
					yElem := ffe(y[base+i]) | (ffe(y[base+32+i]) << 8)
					yNew := xElem ^ yElem
					xNew := xElem ^ mulLog(yNew, logM)
					xRef[base+i] = byte(xNew)
					xRef[base+32+i] = byte(xNew >> 8)
					yRef[base+i] = byte(yNew)
					yRef[base+32+i] = byte(yNew >> 8)
				}
			}

			// Run GFNI version
			gfniTable := &gf2p811dMulMatrices16[logM]
			ifftDIT2_gfni(xGFNI, yGFNI, gfniTable)

			// Run AVX2 version
			avx2Table := &multiply256LUT[logM]
			ifftDIT2_avx2(xAVX2, yAVX2, avx2Table)

			// Compare against reference
			gfniOK := bytes.Equal(xGFNI, xRef) && bytes.Equal(yGFNI, yRef)
			avx2OK := bytes.Equal(xAVX2, xRef) && bytes.Equal(yAVX2, yRef)

			if !gfniOK || !avx2OK {
				t.Errorf("logM=%d: GFNI correct=%v, AVX2 correct=%v", logM, gfniOK, avx2OK)
				for i := 0; i < 8; i++ {
					t.Logf("  x[%d]: Ref=0x%02x, GFNI=0x%02x, AVX2=0x%02x", i*2, xRef[i*2], xGFNI[i*2], xAVX2[i*2])
				}
			}
		})
	}
}

func TestCompareFftDIT2GF16GFNIvsAVX2(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.GFNI, cpuid.AVX) {
		t.Skip("AVX+GFNI not supported")
	}

	// Create a simple encoder to ensure tables are initialized
	enc, err := New(300, 100, WithLeopardGF16(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = enc

	if gf2p811dMulMatrices16 == nil {
		t.Skip("GFNI tables not initialized")
	}

	// Test with various log_m values
	testCases := []ffe{0, 1, 100, 1000, 10000, 30000, 65535}

	for _, logM := range testCases {
		t.Run(fmt.Sprintf("logM=%d", logM), func(t *testing.T) {
			// Create test data - 128 bytes (64 16-bit elements)
			x := make([]byte, 128)
			y := make([]byte, 128)
			for i := range x {
				x[i] = byte(i + 1)
				y[i] = byte(i*2 + 3)
			}

			// Make copies for both implementations
			xGFNI := make([]byte, len(x))
			yGFNI := make([]byte, len(y))
			copy(xGFNI, x)
			copy(yGFNI, y)

			xAVX2 := make([]byte, len(x))
			yAVX2 := make([]byte, len(y))
			copy(xAVX2, x)
			copy(yAVX2, y)

			// Run GFNI version
			gfniTable := &gf2p811dMulMatrices16[logM]
			fftDIT2_gfni(xGFNI, yGFNI, gfniTable)

			// Run AVX2 version
			avx2Table := &multiply256LUT[logM]
			fftDIT2_avx2(xAVX2, yAVX2, avx2Table)

			// Compare results
			if !bytes.Equal(xGFNI, xAVX2) {
				t.Errorf("x mismatch for logM=%d", logM)
				for i := 0; i < len(xGFNI) && i < 32; i++ {
					if xGFNI[i] != xAVX2[i] {
						t.Logf("  x[%d]: GFNI=0x%02x, AVX2=0x%02x", i, xGFNI[i], xAVX2[i])
					}
				}
			}
			if !bytes.Equal(yGFNI, yAVX2) {
				t.Errorf("y mismatch for logM=%d", logM)
				for i := 0; i < len(yGFNI) && i < 32; i++ {
					if yGFNI[i] != yAVX2[i] {
						t.Logf("  y[%d]: GFNI=0x%02x, AVX2=0x%02x", i, yGFNI[i], yAVX2[i])
					}
				}
			}
		})
	}
}

func TestCompareIfftDIT4GF16GFNIvsAVX2(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.GFNI, cpuid.AVX) {
		t.Skip("AVX+GFNI not supported")
	}

	enc, err := New(300, 100, WithLeopardGF16(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = enc

	if gf2p811dMulMatrices16 == nil {
		t.Skip("GFNI tables not initialized")
	}

	// Test all 8 variants (skipMask 0-7)
	results := make([]bool, 8)
	for variant := 0; variant < 8; variant++ {
		t.Run(fmt.Sprintf("variant=%d", variant), func(t *testing.T) {
			// Create work slices - 4 slices of 64 bytes each (32 x 16-bit elements)
			work := make([][]byte, 4)
			for i := range work {
				work[i] = make([]byte, 64)
				for j := range work[i] {
					work[i][j] = byte(i*17 + j + 1)
				}
			}

			// Make copies
			gfniWork := make([][]byte, 4)
			avx2Work := make([][]byte, 4)
			for i := range work {
				gfniWork[i] = make([]byte, len(work[i]))
				avx2Work[i] = make([]byte, len(work[i]))
				copy(gfniWork[i], work[i])
				copy(avx2Work[i], work[i])
			}

			// Set log_m values based on variant
			log_m01, log_m23, log_m02 := ffe(10), ffe(200), ffe(3000)
			if variant&1 != 0 {
				log_m01 = modulus
			}
			if variant&2 != 0 {
				log_m23 = modulus
			}
			if variant&4 != 0 {
				log_m02 = modulus
			}

			// Call GFNI version
			g01 := &gf2p811dMulMatrices16[log_m01]
			g23 := &gf2p811dMulMatrices16[log_m23]
			g02 := &gf2p811dMulMatrices16[log_m02]
			switch variant {
			case 0:
				ifftDIT4_gfni_0(gfniWork, 24, g01, g23, g02)
			case 1:
				ifftDIT4_gfni_1(gfniWork, 24, g01, g23, g02)
			case 2:
				ifftDIT4_gfni_2(gfniWork, 24, g01, g23, g02)
			case 3:
				ifftDIT4_gfni_3(gfniWork, 24, g01, g23, g02)
			case 4:
				ifftDIT4_gfni_4(gfniWork, 24, g01, g23, g02)
			case 5:
				ifftDIT4_gfni_5(gfniWork, 24, g01, g23, g02)
			case 6:
				ifftDIT4_gfni_6(gfniWork, 24, g01, g23, g02)
			case 7:
				ifftDIT4_gfni_7(gfniWork, 24, g01, g23, g02)
			}

			// Call AVX2 version
			t01 := &multiply256LUT[log_m01]
			t23 := &multiply256LUT[log_m23]
			t02 := &multiply256LUT[log_m02]
			switch variant {
			case 0:
				ifftDIT4_avx2_0(avx2Work, 24, t01, t23, t02)
			case 1:
				ifftDIT4_avx2_1(avx2Work, 24, t01, t23, t02)
			case 2:
				ifftDIT4_avx2_2(avx2Work, 24, t01, t23, t02)
			case 3:
				ifftDIT4_avx2_3(avx2Work, 24, t01, t23, t02)
			case 4:
				ifftDIT4_avx2_4(avx2Work, 24, t01, t23, t02)
			case 5:
				ifftDIT4_avx2_5(avx2Work, 24, t01, t23, t02)
			case 6:
				ifftDIT4_avx2_6(avx2Work, 24, t01, t23, t02)
			case 7:
				ifftDIT4_avx2_7(avx2Work, 24, t01, t23, t02)
			}

			// Compare results
			match := true
			for i := range work {
				if !bytes.Equal(gfniWork[i], avx2Work[i]) {
					match = false
					t.Errorf("work[%d] mismatch for variant %d", i, variant)
					for j := 0; j < len(gfniWork[i]) && j < 16; j++ {
						if gfniWork[i][j] != avx2Work[i][j] {
							t.Logf("  [%d][%d]: GFNI=0x%02x, AVX2=0x%02x", i, j, gfniWork[i][j], avx2Work[i][j])
						}
					}
				}
			}
			results[variant] = match
			if match {
				t.Logf("ifftDIT4_gfni_%d: MATCH", variant)
			}
		})
	}
}

func TestCompareFftDIT4GF16GFNIvsAVX2(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.GFNI, cpuid.AVX) {
		t.Skip("AVX+GFNI not supported")
	}

	enc, err := New(300, 100, WithLeopardGF16(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = enc

	if gf2p811dMulMatrices16 == nil {
		t.Skip("GFNI tables not initialized")
	}

	// Test all 8 variants (skipMask 0-7)
	results := make([]bool, 8)
	for variant := 0; variant < 8; variant++ {
		t.Run(fmt.Sprintf("variant=%d", variant), func(t *testing.T) {
			// Create work slices - 4 slices of 64 bytes each (32 x 16-bit elements)
			work := make([][]byte, 4)
			for i := range work {
				work[i] = make([]byte, 64)
				for j := range work[i] {
					work[i][j] = byte(i*17 + j + 1)
				}
			}

			// Make copies
			gfniWork := make([][]byte, 4)
			avx2Work := make([][]byte, 4)
			for i := range work {
				gfniWork[i] = make([]byte, len(work[i]))
				avx2Work[i] = make([]byte, len(work[i]))
				copy(gfniWork[i], work[i])
				copy(avx2Work[i], work[i])
			}

			// Set log_m values based on variant - fftDIT4 has different bit mapping
			log_m01, log_m23, log_m02 := ffe(10), ffe(200), ffe(3000)
			if variant&2 != 0 {
				log_m01 = modulus
			}
			if variant&4 != 0 {
				log_m23 = modulus
			}
			if variant&1 != 0 {
				log_m02 = modulus
			}

			// Call GFNI version
			g01 := &gf2p811dMulMatrices16[log_m01]
			g23 := &gf2p811dMulMatrices16[log_m23]
			g02 := &gf2p811dMulMatrices16[log_m02]
			switch variant {
			case 0:
				fftDIT4_gfni_0(gfniWork, 24, g01, g23, g02)
			case 1:
				fftDIT4_gfni_1(gfniWork, 24, g01, g23, g02)
			case 2:
				fftDIT4_gfni_2(gfniWork, 24, g01, g23, g02)
			case 3:
				fftDIT4_gfni_3(gfniWork, 24, g01, g23, g02)
			case 4:
				fftDIT4_gfni_4(gfniWork, 24, g01, g23, g02)
			case 5:
				fftDIT4_gfni_5(gfniWork, 24, g01, g23, g02)
			case 6:
				fftDIT4_gfni_6(gfniWork, 24, g01, g23, g02)
			case 7:
				fftDIT4_gfni_7(gfniWork, 24, g01, g23, g02)
			}

			// Call AVX2 version
			t01 := &multiply256LUT[log_m01]
			t23 := &multiply256LUT[log_m23]
			t02 := &multiply256LUT[log_m02]
			switch variant {
			case 0:
				fftDIT4_avx2_0(avx2Work, 24, t01, t23, t02)
			case 1:
				fftDIT4_avx2_1(avx2Work, 24, t01, t23, t02)
			case 2:
				fftDIT4_avx2_2(avx2Work, 24, t01, t23, t02)
			case 3:
				fftDIT4_avx2_3(avx2Work, 24, t01, t23, t02)
			case 4:
				fftDIT4_avx2_4(avx2Work, 24, t01, t23, t02)
			case 5:
				fftDIT4_avx2_5(avx2Work, 24, t01, t23, t02)
			case 6:
				fftDIT4_avx2_6(avx2Work, 24, t01, t23, t02)
			case 7:
				fftDIT4_avx2_7(avx2Work, 24, t01, t23, t02)
			}

			// Compare results
			match := true
			for i := range work {
				if !bytes.Equal(gfniWork[i], avx2Work[i]) {
					match = false
					t.Errorf("work[%d] mismatch for variant %d", i, variant)
					for j := 0; j < len(gfniWork[i]) && j < 16; j++ {
						if gfniWork[i][j] != avx2Work[i][j] {
							t.Logf("  [%d][%d]: GFNI=0x%02x, AVX2=0x%02x", i, j, gfniWork[i][j], avx2Work[i][j])
						}
					}
				}
			}
			results[variant] = match
			if match {
				t.Logf("fftDIT4_gfni_%d: MATCH", variant)
			}
		})
	}
}

func TestCompareIfftDIT4GF16GFNIAvx512vsAVX2(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.GFNI, cpuid.AVX512VL) {
		t.Skip("AVX512+GFNI not supported")
	}

	enc, err := New(300, 100, WithLeopardGF16(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = enc

	if gf2p811dMulMatrices16 == nil {
		t.Skip("GFNI tables not initialized")
	}

	// Test variant 0 (all multiplications)
	for variant := 0; variant < 8; variant++ {
		t.Run(fmt.Sprintf("variant=%d", variant), func(t *testing.T) {
			// Create work slices - 4 slices of 128 bytes each (64 x 16-bit elements)
			work := make([][]byte, 4)
			for i := range work {
				work[i] = make([]byte, 128)
				for j := range work[i] {
					work[i][j] = byte(i*17 + j + 1)
				}
			}

			// Make copies
			avx512Work := make([][]byte, 4)
			avx2Work := make([][]byte, 4)
			for i := range work {
				avx512Work[i] = make([]byte, len(work[i]))
				avx2Work[i] = make([]byte, len(work[i]))
				copy(avx512Work[i], work[i])
				copy(avx2Work[i], work[i])
			}

			log_m01, log_m23, log_m02 := ffe(10), ffe(200), ffe(3000)
			if variant&1 != 0 {
				log_m01 = modulus
			}
			if variant&2 != 0 {
				log_m23 = modulus
			}
			if variant&4 != 0 {
				log_m02 = modulus
			}

			g01 := &gf2p811dMulMatrices16[log_m01]
			g23 := &gf2p811dMulMatrices16[log_m23]
			g02 := &gf2p811dMulMatrices16[log_m02]

			// Call AVX-512 GFNI version
			switch variant {
			case 0:
				ifftDIT4_gfni_avx512_0(avx512Work, 24, g01, g23, g02)
			case 1:
				ifftDIT4_gfni_avx512_1(avx512Work, 24, g01, g23, g02)
			case 2:
				ifftDIT4_gfni_avx512_2(avx512Work, 24, g01, g23, g02)
			case 3:
				ifftDIT4_gfni_avx512_3(avx512Work, 24, g01, g23, g02)
			case 4:
				ifftDIT4_gfni_avx512_4(avx512Work, 24, g01, g23, g02)
			case 5:
				ifftDIT4_gfni_avx512_5(avx512Work, 24, g01, g23, g02)
			case 6:
				ifftDIT4_gfni_avx512_6(avx512Work, 24, g01, g23, g02)
			case 7:
				ifftDIT4_gfni_avx512_7(avx512Work, 24, g01, g23, g02)
			}

			// Call AVX2 GFNI version twice (processes 64 bytes each)
			switch variant {
			case 0:
				ifftDIT4_gfni_0(avx2Work, 24, g01, g23, g02)
			case 1:
				ifftDIT4_gfni_1(avx2Work, 24, g01, g23, g02)
			case 2:
				ifftDIT4_gfni_2(avx2Work, 24, g01, g23, g02)
			case 3:
				ifftDIT4_gfni_3(avx2Work, 24, g01, g23, g02)
			case 4:
				ifftDIT4_gfni_4(avx2Work, 24, g01, g23, g02)
			case 5:
				ifftDIT4_gfni_5(avx2Work, 24, g01, g23, g02)
			case 6:
				ifftDIT4_gfni_6(avx2Work, 24, g01, g23, g02)
			case 7:
				ifftDIT4_gfni_7(avx2Work, 24, g01, g23, g02)
			}

			// Compare results
			match := true
			for i := range work {
				if !bytes.Equal(avx512Work[i], avx2Work[i]) {
					match = false
					t.Errorf("work[%d] mismatch for variant %d", i, variant)
					for j := 0; j < len(avx512Work[i]) && j < 32; j++ {
						if avx512Work[i][j] != avx2Work[i][j] {
							t.Logf("  [%d][%d]: AVX512=0x%02x, AVX2=0x%02x", i, j, avx512Work[i][j], avx2Work[i][j])
						}
					}
				}
			}
			if match {
				t.Logf("ifftDIT4_gfni_avx512_%d: MATCH", variant)
			}
		})
	}
}

func TestCompareFftDIT4GF16GFNIAvx512vsAVX2(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.GFNI, cpuid.AVX512VL) {
		t.Skip("AVX512+GFNI not supported")
	}

	enc, err := New(300, 100, WithLeopardGF16(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = enc

	if gf2p811dMulMatrices16 == nil {
		t.Skip("GFNI tables not initialized")
	}

	for variant := 0; variant < 8; variant++ {
		t.Run(fmt.Sprintf("variant=%d", variant), func(t *testing.T) {
			work := make([][]byte, 4)
			for i := range work {
				work[i] = make([]byte, 128)
				for j := range work[i] {
					work[i][j] = byte(i*17 + j + 1)
				}
			}

			avx512Work := make([][]byte, 4)
			avx2Work := make([][]byte, 4)
			for i := range work {
				avx512Work[i] = make([]byte, len(work[i]))
				avx2Work[i] = make([]byte, len(work[i]))
				copy(avx512Work[i], work[i])
				copy(avx2Work[i], work[i])
			}

			log_m01, log_m23, log_m02 := ffe(10), ffe(200), ffe(3000)
			if variant&1 != 0 {
				log_m02 = modulus
			}
			if variant&2 != 0 {
				log_m01 = modulus
			}
			if variant&4 != 0 {
				log_m23 = modulus
			}

			g01 := &gf2p811dMulMatrices16[log_m01]
			g23 := &gf2p811dMulMatrices16[log_m23]
			g02 := &gf2p811dMulMatrices16[log_m02]

			switch variant {
			case 0:
				fftDIT4_gfni_avx512_0(avx512Work, 24, g01, g23, g02)
				fftDIT4_gfni_0(avx2Work, 24, g01, g23, g02)
			case 1:
				fftDIT4_gfni_avx512_1(avx512Work, 24, g01, g23, g02)
				fftDIT4_gfni_1(avx2Work, 24, g01, g23, g02)
			case 2:
				fftDIT4_gfni_avx512_2(avx512Work, 24, g01, g23, g02)
				fftDIT4_gfni_2(avx2Work, 24, g01, g23, g02)
			case 3:
				fftDIT4_gfni_avx512_3(avx512Work, 24, g01, g23, g02)
				fftDIT4_gfni_3(avx2Work, 24, g01, g23, g02)
			case 4:
				fftDIT4_gfni_avx512_4(avx512Work, 24, g01, g23, g02)
				fftDIT4_gfni_4(avx2Work, 24, g01, g23, g02)
			case 5:
				fftDIT4_gfni_avx512_5(avx512Work, 24, g01, g23, g02)
				fftDIT4_gfni_5(avx2Work, 24, g01, g23, g02)
			case 6:
				fftDIT4_gfni_avx512_6(avx512Work, 24, g01, g23, g02)
				fftDIT4_gfni_6(avx2Work, 24, g01, g23, g02)
			case 7:
				fftDIT4_gfni_avx512_7(avx512Work, 24, g01, g23, g02)
				fftDIT4_gfni_7(avx2Work, 24, g01, g23, g02)
			}

			match := true
			for i := 0; i < 4; i++ {
				if !bytes.Equal(avx512Work[i], avx2Work[i]) {
					match = false
					t.Errorf("fftDIT4_gfni_avx512_%d: work[%d] mismatch", variant, i)
					for j := 0; j < len(avx512Work[i]) && j < 32; j++ {
						if avx512Work[i][j] != avx2Work[i][j] {
							t.Logf("  [%d][%d]: AVX512=0x%02x, AVX2=0x%02x", i, j, avx512Work[i][j], avx2Work[i][j])
						}
					}
				}
			}
			if match {
				t.Logf("fftDIT4_gfni_avx512_%d: MATCH", variant)
			}
		})
	}
}

// TestGF16MulSimple tests a simple case of GFNI multiplication to debug
func TestGF16MulSimple(t *testing.T) {
	if !cpuid.CPU.Has(cpuid.GFNI) {
		t.Skip("GFNI not supported")
	}

	enc, err := New(300, 100, WithLeopardGF16(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = enc

	if gf2p811dMulMatrices16 == nil {
		t.Skip("GFNI tables not initialized")
	}

	// Test with identity (logM=0)
	// For identity: mul(x, 0) = x (since exp(log(x) + 0) = x)
	logM := ffe(0)

	// Simple test: process 64 bytes (32 GF16 elements per half)
	// Use element values 0x0001, 0x0002, etc.
	y := make([]byte, 64)
	for i := 0; i < 32; i++ {
		y[i*2] = byte(i + 1)   // lo byte
		y[i*2+1] = byte(0)     // hi byte = 0, so element = i+1
	}
	x := make([]byte, 64)
	for i := range x {
		x[i] = 0 // x = 0
	}

	// Expected: y = x XOR y = y (since x=0)
	// Expected: x = x XOR mul(y, 0) = mul(y, 0) = y
	xGFNI := make([]byte, 64)
	yGFNI := make([]byte, 64)
	copy(xGFNI, x)
	copy(yGFNI, y)

	gfniTable := &gf2p811dMulMatrices16[logM]
	ifftDIT2_gfni(xGFNI, yGFNI, gfniTable)

	// Check yGFNI should equal original y (since x was 0)
	for i := 0; i < 64; i++ {
		if yGFNI[i] != y[i] {
			t.Errorf("y[%d]: got 0x%02x, expected 0x%02x", i, yGFNI[i], y[i])
		}
	}

	// Check xGFNI should equal original y (x = 0 XOR mul(y, identity) = y)
	for i := 0; i < 64; i++ {
		if xGFNI[i] != y[i] {
			t.Errorf("x[%d]: got 0x%02x, expected 0x%02x (y[%d])", i, xGFNI[i], y[i], i)
		}
	}
}

// TestGF16MulNonIdentity tests GFNI assembly for non-identity logM using split layout
func TestGF16MulNonIdentity(t *testing.T) {
	if !cpuid.CPU.Has(cpuid.GFNI) {
		t.Skip("GFNI not supported")
	}

	enc, err := New(300, 100, WithLeopardGF16(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = enc

	if gf2p811dMulMatrices16 == nil {
		t.Skip("GFNI tables not initialized")
	}

	logM := ffe(100)

	// Data layout is SPLIT: [0:32] = lo bytes, [32:64] = hi bytes of 32 elements
	// Set y to element 1 in first position (split layout)
	y := make([]byte, 64)
	y[0] = 1  // lo byte of element 0
	y[32] = 0 // hi byte of element 0

	// Set x to zeros
	x := make([]byte, 64)

	// Expected: after ifftDIT2:
	// y_new = x XOR y = y (since x=0)
	// x_new = x XOR mul(y, logM) = mul(y, logM) = mul(element1, logM)
	expectedMul := mulLog(ffe(1), logM)
	t.Logf("mulLog(1, %d) = 0x%04x", logM, expectedMul)

	xGFNI := make([]byte, 64)
	yGFNI := make([]byte, 64)
	copy(xGFNI, x)
	copy(yGFNI, y)

	gfniTable := &gf2p811dMulMatrices16[logM]
	t.Logf("Matrices: A=0x%016x B=0x%016x C=0x%016x D=0x%016x",
		gfniTable[0], gfniTable[1], gfniTable[2], gfniTable[3])

	ifftDIT2_gfni(xGFNI, yGFNI, gfniTable)

	// x should have mul(1, logM) at element 0 (split layout)
	gotResult := ffe(xGFNI[0]) | (ffe(xGFNI[32]) << 8)
	t.Logf("GFNI result: 0x%04x (expected 0x%04x)", gotResult, expectedMul)
	t.Logf("xGFNI lo bytes: %02x, hi bytes: %02x", xGFNI[0], xGFNI[32])

	if gotResult != expectedMul {
		t.Errorf("GFNI mul mismatch: got 0x%04x, expected 0x%04x", gotResult, expectedMul)
	}

	// Test all 32 elements with split layout
	t.Log("Testing all 32 elements (split layout)...")
	for i := range y {
		y[i] = 0
	}
	// Set elements 0-31 to 1, 2, 3, ..., 32 in split layout
	for i := 0; i < 32; i++ {
		y[i] = byte(i + 1)    // lo bytes
		y[32+i] = 0           // hi bytes
	}

	for i := range x {
		x[i] = 0
	}
	copy(xGFNI, x)
	copy(yGFNI, y)
	ifftDIT2_gfni(xGFNI, yGFNI, gfniTable)

	// Check each element (split layout)
	errors := 0
	for i := 0; i < 32; i++ {
		elem := ffe(i + 1)
		expected := mulLog(elem, logM)
		got := ffe(xGFNI[i]) | (ffe(xGFNI[32+i]) << 8)
		if got != expected {
			t.Errorf("Element %d: mul(%d, %d) got 0x%04x, expected 0x%04x", i, elem, logM, got, expected)
			errors++
		}
		if errors > 10 {
			break
		}
	}
}

// TestGF16MatrixGeneration tests that the GFNI matrices produce correct results
// when applied in pure Go (without assembly).
func TestGF16MatrixGeneration(t *testing.T) {
	if !cpuid.CPU.Has(cpuid.GFNI) {
		t.Skip("GFNI not supported")
	}

	// Initialize tables
	enc, err := New(300, 100, WithLeopardGF16(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = enc

	if gf2p811dMulMatrices16 == nil {
		t.Skip("GFNI tables not initialized")
	}

	// applyGFNIMatrix applies one 8x8 GFNI matrix to a byte
	// VGF2P8AFFINEQB: for output bit j, uses matrix byte (7-j), XORs bits where matrix[k] AND input[k]
	applyMatrix := func(matrix uint64, input byte) byte {
		var result byte
		for outBit := 0; outBit < 8; outBit++ {
			parity := byte(0)
			for inBit := 0; inBit < 8; inBit++ {
				// VGF2P8AFFINEQB uses byte (7-outBit), bit inBit
				matrixBit := (matrix >> ((7-outBit)*8 + inBit)) & 1
				inputBit := (input >> inBit) & 1
				parity ^= byte(matrixBit & uint64(inputBit))
			}
			result |= parity << outBit
		}
		return result
	}

	// Test a few log values
	testLogMs := []int{0, 1, 100, 1000, 10000, 30000, 65535}
	for _, logM := range testLogMs {
		t.Run(fmt.Sprintf("logM=%d", logM), func(t *testing.T) {
			matrices := gf2p811dMulMatrices16[logM]
			A, B, C, D := matrices[0], matrices[1], matrices[2], matrices[3]

			// Test sample 16-bit inputs
			testInputs := []ffe{0, 1, 2, 255, 256, 0x1234, 0xFFFF}
			for _, input := range testInputs {
				// Expected result from mulLog
				expected := mulLog(input, ffe(logM))

				// Apply GFNI matrices
				inLo := byte(input)
				inHi := byte(input >> 8)
				outLo := applyMatrix(A, inLo) ^ applyMatrix(B, inHi)
				outHi := applyMatrix(C, inLo) ^ applyMatrix(D, inHi)
				gfniResult := ffe(outLo) | (ffe(outHi) << 8)

				if gfniResult != expected {
					t.Errorf("input=0x%04x logM=%d: GFNI=0x%04x expected=0x%04x",
						input, logM, gfniResult, expected)
				}
			}
		})
	}
}
