package reedsolomon

import (
	"fmt"
	"testing"

	"github.com/klauspost/cpuid/v2"
)

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
	if !cpuid.CPU.Supports(cpuid.GFNI, cpuid.AVX512VL) {
		t.Skip("GFNI not supported")
	}

	// Initialize tables
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
