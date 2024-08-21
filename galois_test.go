/**
 * Unit tests for Galois
 *
 * Copyright 2015, Klaus Post
 * Copyright 2015, Backblaze, Inc.
 */

package reedsolomon

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mathrand "math/rand"
	"testing"
)

func TestAssociativity(t *testing.T) {
	for i := 0; i < 256; i++ {
		a := byte(i)
		for j := 0; j < 256; j++ {
			b := byte(j)
			for k := 0; k < 256; k++ {
				c := byte(k)
				x := galAdd(a, galAdd(b, c))
				y := galAdd(galAdd(a, b), c)
				if x != y {
					t.Fatal("add does not match:", x, "!=", y)
				}
				x = galMultiply(a, galMultiply(b, c))
				y = galMultiply(galMultiply(a, b), c)
				if x != y {
					t.Fatal("multiply does not match:", x, "!=", y)
				}
			}
		}
	}
}

func TestIdentity(t *testing.T) {
	for i := 0; i < 256; i++ {
		a := byte(i)
		b := galAdd(a, 0)
		if a != b {
			t.Fatal("Add zero should yield same result", a, "!=", b)
		}
		b = galMultiply(a, 1)
		if a != b {
			t.Fatal("Mul by one should yield same result", a, "!=", b)
		}
	}
}

func TestInverse(t *testing.T) {
	for i := 0; i < 256; i++ {
		a := byte(i)
		b := galAdd(0, a)
		c := galAdd(a, b)
		if c != 0 {
			t.Fatal("inverse sub/add", c, "!=", 0)
		}
		if a != 0 {
			b = galDivide(1, a)
			c = galMultiply(a, b)
			if c != 1 {
				t.Fatal("inverse div/mul", c, "!=", 1)
			}
			b2 := galOneOver(a)
			if b != b2 {
				t.Fatal("inverse div/mul", b, "!=", b2)
			}
		}
	}
}

func TestCommutativity(t *testing.T) {
	for i := 0; i < 256; i++ {
		a := byte(i)
		for j := 0; j < 256; j++ {
			b := byte(j)
			x := galAdd(a, b)
			y := galAdd(b, a)
			if x != y {
				t.Fatal(x, "!= ", y)
			}
			x = galMultiply(a, b)
			y = galMultiply(b, a)
			if x != y {
				t.Fatal(x, "!= ", y)
			}
		}
	}
}

func TestDistributivity(t *testing.T) {
	for i := 0; i < 256; i++ {
		a := byte(i)
		for j := 0; j < 256; j++ {
			b := byte(j)
			for k := 0; k < 256; k++ {
				c := byte(k)
				x := galMultiply(a, galAdd(b, c))
				y := galAdd(galMultiply(a, b), galMultiply(a, c))
				if x != y {
					t.Fatal(x, "!= ", y)
				}
			}
		}
	}
}

func TestExp(t *testing.T) {
	for i := 0; i < 256; i++ {
		a := byte(i)
		power := byte(1)
		for j := 0; j < 256; j++ {
			x := galExp(a, j)
			if x != power {
				t.Fatal(x, "!=", power)
			}
			power = galMultiply(power, a)
		}
	}
}

func testGalois(t *testing.T, o *options) {
	// These values were copied output of the Python code.
	if galMultiply(3, 4) != 12 {
		t.Fatal("galMultiply(3, 4) != 12")
	}
	if galMultiply(7, 7) != 21 {
		t.Fatal("galMultiply(7, 7) != 21")
	}
	if galMultiply(23, 45) != 41 {
		t.Fatal("galMultiply(23, 45) != 41")
	}

	// Test slices (>32 entries to test assembler -- AVX2 & NEON)
	in := []byte{0, 1, 2, 3, 4, 5, 6, 10, 50, 100, 150, 174, 201, 255, 99, 32, 67, 85, 200, 199, 198, 197, 196, 195, 194, 193, 192, 191, 190, 189, 188, 187, 186, 185}
	out := make([]byte, len(in))
	galMulSlice(25, in, out, o)
	expect := []byte{0x0, 0x19, 0x32, 0x2b, 0x64, 0x7d, 0x56, 0xfa, 0xb8, 0x6d, 0xc7, 0x85, 0xc3, 0x1f, 0x22, 0x7, 0x25, 0xfe, 0xda, 0x5d, 0x44, 0x6f, 0x76, 0x39, 0x20, 0xb, 0x12, 0x11, 0x8, 0x23, 0x3a, 0x75, 0x6c, 0x47}
	if 0 != bytes.Compare(out, expect) {
		t.Errorf("got %#v, expected %#v", out, expect)
	}
	expectXor := []byte{0x0, 0x2d, 0x5a, 0x77, 0xb4, 0x99, 0xee, 0x2f, 0x79, 0xf2, 0x7, 0x51, 0xd4, 0x19, 0x31, 0xc9, 0xf8, 0xfc, 0xf9, 0x4f, 0x62, 0x15, 0x38, 0xfb, 0xd6, 0xa1, 0x8c, 0x96, 0xbb, 0xcc, 0xe1, 0x22, 0xf, 0x78}
	galMulSliceXor(52, in, out, o)
	if 0 != bytes.Compare(out, expectXor) {
		t.Errorf("got %#v, expected %#v", out, expectXor)
	}

	galMulSlice(177, in, out, o)
	expect = []byte{0x0, 0xb1, 0x7f, 0xce, 0xfe, 0x4f, 0x81, 0x9e, 0x3, 0x6, 0xe8, 0x75, 0xbd, 0x40, 0x36, 0xa3, 0x95, 0xcb, 0xc, 0xdd, 0x6c, 0xa2, 0x13, 0x23, 0x92, 0x5c, 0xed, 0x1b, 0xaa, 0x64, 0xd5, 0xe5, 0x54, 0x9a}
	if 0 != bytes.Compare(out, expect) {
		t.Errorf("got %#v, expected %#v", out, expect)
	}

	expectXor = []byte{0x0, 0xc4, 0x95, 0x51, 0x37, 0xf3, 0xa2, 0xfb, 0xec, 0xc5, 0xd0, 0xc7, 0x53, 0x88, 0xa3, 0xa5, 0x6, 0x78, 0x97, 0x9f, 0x5b, 0xa, 0xce, 0xa8, 0x6c, 0x3d, 0xf9, 0xdf, 0x1b, 0x4a, 0x8e, 0xe8, 0x2c, 0x7d}
	galMulSliceXor(117, in, out, o)
	if 0 != bytes.Compare(out, expectXor) {
		t.Errorf("got %#v, expected %#v", out, expectXor)
	}

	if galExp(2, 2) != 4 {
		t.Fatal("galExp(2, 2) != 4")
	}
	if galExp(5, 20) != 235 {
		t.Fatal("galExp(5, 20) != 235")
	}
	if galExp(13, 7) != 43 {
		t.Fatal("galExp(13, 7) != 43")
	}
}

func TestGalois(t *testing.T) {
	// invoke with all combinations of asm instructions
	o := options{}
	o.useSSSE3, o.useAVX2 = false, false
	testGalois(t, &o)
	o.useSSSE3, o.useAVX2 = true, false
	testGalois(t, &o)
	if defaultOptions.useAVX2 {
		o.useSSSE3, o.useAVX2 = false, true
		testGalois(t, &o)
	}
}

func TestSliceGalAdd(t *testing.T) {

	lengthList := []int{16, 32, 34}
	for _, length := range lengthList {
		in := make([]byte, length)
		fillRandom(in)
		out := make([]byte, length)
		fillRandom(out)
		expect := make([]byte, length)
		for i := range expect {
			expect[i] = in[i] ^ out[i]
		}
		noSSE2 := defaultOptions
		noSSE2.useSSE2 = false
		sliceXor(in, out, &noSSE2)
		if 0 != bytes.Compare(out, expect) {
			t.Errorf("got %#v, expected %#v", out, expect)
		}
		fillRandom(out)
		for i := range expect {
			expect[i] = in[i] ^ out[i]
		}
		sliceXor(in, out, &defaultOptions)
		if 0 != bytes.Compare(out, expect) {
			t.Errorf("got %#v, expected %#v", out, expect)
		}
	}

	for i := 0; i < 256; i++ {
		a := byte(i)
		for j := 0; j < 256; j++ {
			b := byte(j)
			for k := 0; k < 256; k++ {
				c := byte(k)
				x := galAdd(a, galAdd(b, c))
				y := galAdd(galAdd(a, b), c)
				if x != y {
					t.Fatal("add does not match:", x, "!=", y)
				}
				x = galMultiply(a, galMultiply(b, c))
				y = galMultiply(galMultiply(a, b), c)
				if x != y {
					t.Fatal("multiply does not match:", x, "!=", y)
				}
			}
		}
	}
}

func testGenGalois(t *testing.T, matrixRows [][]byte, size, start, stop int, f func(matrix []byte, in, out [][]byte, start, stop int) int, vectorLength int) {

	// reference versions
	galMulSliceRef := func(c byte, in, out []byte) {
		out = out[:len(in)]
		mt := mulTable[c][:256]
		for n, input := range in {
			out[n] = mt[input]
		}
	}
	galMulSliceXorRef := func(c byte, in, out []byte) {
		out = out[:len(in)]
		mt := mulTable[c][:256]
		for n, input := range in {
			out[n] ^= mt[input]
		}
	}

	outputs := make([][]byte, len(matrixRows))
	for i := range outputs {
		outputs[i] = make([]byte, size)
		if _, err := rand.Read(outputs[i]); err != nil {
			t.Fatalf("error: %v", err)
			return
		}
	}
	inputs := make([][]byte, len(matrixRows[0]))
	for i := range inputs {
		inputs[i] = make([]byte, size)
		if _, err := rand.Read(inputs[i]); err != nil {
			t.Fatalf("error: %v", err)
			return
		}
	}

	m := genCodeGenMatrix(matrixRows, len(inputs), 0, len(outputs), vectorLength, nil)

	end := start + f(m, inputs, outputs, start, stop)
	if end != stop {
		t.Errorf("got %#v, expected %#v", end, stop)
	}

	wanteds := make([][]byte, len(outputs))
	for i := range wanteds {
		wanteds[i] = make([]byte, size)
		galMulSliceRef(matrixRows[i][0], inputs[0], wanteds[i])
		for j := 1; j < len(matrixRows[i]); j++ {
			galMulSliceXorRef(matrixRows[i][j], inputs[j], wanteds[i])
		}
	}

	for i := range outputs {
		if !bytes.Equal(outputs[i][start:stop], wanteds[i][start:stop]) {
			t.Errorf("testGenGalois(%dx%d): got %#v, expected %#v", len(inputs), len(outputs), outputs[i][start:stop], wanteds[i][start:stop])
			fmt.Printf("output[%d]\n", i)
			fmt.Print(hex.Dump(outputs[i][start:stop]))
			fmt.Printf("wanted[%d]\n", i)
			fmt.Print(hex.Dump(wanteds[i][start:stop]))
		}
	}
}

func testGenGaloisXor(t *testing.T, matrixRows [][]byte, size, start, stop int, f func(matrix []byte, in, out [][]byte, start, stop int) int, vectorLength int) {

	// reference version
	galMulSliceXorRef := func(c byte, in, out []byte) {
		out = out[:len(in)]
		mt := mulTable[c][:256]
		for n, input := range in {
			out[n] ^= mt[input]
		}
	}

	outputs := make([][]byte, len(matrixRows))
	wanteds := make([][]byte, len(outputs))
	for i := range outputs {
		outputs[i] = make([]byte, size)
		wanteds[i] = make([]byte, size)

		// For Xor tests, prefill both outputs and wanted with identical values
		copy(outputs[i], bytes.Repeat([]byte{byte(i)}, size))
		copy(wanteds[i], outputs[i])
	}
	inputs := make([][]byte, len(matrixRows[0]))
	for i := range inputs {
		inputs[i] = make([]byte, size)
		if _, err := rand.Read(inputs[i]); err != nil {
			t.Fatalf("error: %v", err)
			return
		}
	}

	m := genCodeGenMatrix(matrixRows, len(inputs), 0, len(outputs), vectorLength, nil)

	end := start + f(m, inputs, outputs, start, stop)
	if end != stop {
		t.Errorf("got %#v, expected %#v", end, stop)
	}

	for i := range wanteds {
		for j := 0; j < len(matrixRows[i]); j++ {
			galMulSliceXorRef(matrixRows[i][j], inputs[j], wanteds[i])
		}
	}

	for i := range outputs {
		if !bytes.Equal(outputs[i][start:stop], wanteds[i][start:stop]) {
			t.Errorf("testGenGaloisXor(%dx%d): got %#v, expected %#v", len(inputs), len(outputs), outputs[i][start:stop], wanteds[i][start:stop])
			fmt.Printf("output[%d]\n", i)
			fmt.Print(hex.Dump(outputs[i][start:stop]))
			fmt.Printf("wanted[%d]\n", i)
			fmt.Print(hex.Dump(wanteds[i][start:stop]))
		}
	}
}

// Test early abort for galMulARCH_?x?_* routines
func testGenGaloisEarlyAbort(t *testing.T, matrixRows [][]byte, size int, f func(matrix []byte, in, out [][]byte, start, stop int) int) {
	outputs := make([][]byte, len(matrixRows))
	inputs := make([][]byte, len(matrixRows[0]))

	start := 0
	start += f(nil, inputs, outputs, 0, size)
	if start != 0 {
		t.Errorf("got %#v, expected %#v", start, 0)
	}
}

func testGenGaloisUpto10x10(t *testing.T, f, fXor func(matrix []byte, in, out [][]byte, start, stop int) int, vectorLength int) {

	for output := 1; output <= codeGenMaxOutputs; output++ {
		for input := 1; input <= codeGenMaxInputs; input++ {
			matrixRows := make([][]byte, input)
			for i := range matrixRows {
				matrixRows[i] = make([]byte, output)
				for j := range matrixRows[i] {
					matrixRows[i][j] = byte(mathrand.Intn(16))
				}
			}

			size, stepsize := 32, 32
			if input <= 3 {
				size, stepsize = 64, 64 // 3x? are all _64 versions
			}

			// test early abort
			testGenGaloisEarlyAbort(t, matrixRows, size-1, f)
			testGenGaloisEarlyAbort(t, matrixRows, size-1, fXor)
			const limit = 1024
			for ; size < limit; size += stepsize {
				// test full range
				testGenGalois(t, matrixRows, size, 0, size, f, vectorLength)
				testGenGaloisXor(t, matrixRows, size, 0, size, fXor, vectorLength)

				if size >= stepsize*2 && size < limit-stepsize*2 {
					start := stepsize
					stop := size - start
					// test partial range
					testGenGalois(t, matrixRows, size, start, stop, f, vectorLength)
					testGenGaloisXor(t, matrixRows, size, start, stop, fXor, vectorLength)
				}
			}
		}
	}
}

func benchmarkGalois(b *testing.B, size int) {
	in := make([]byte, size)
	out := make([]byte, size)

	o := options{}
	o.useSSSE3, o.useAVX2 = !*noSSSE3, !*noAVX2

	b.SetBytes(int64(size))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		galMulSlice(25, in[:], out[:], &o)
	}
}

func BenchmarkGalois128K(b *testing.B) {
	benchmarkGalois(b, 128*1024)
}

func BenchmarkGalois1M(b *testing.B) {
	benchmarkGalois(b, 1024*1024)
}

func benchmarkGaloisXor(b *testing.B, size int) {
	in := make([]byte, size)
	out := make([]byte, size)

	o := options{}
	o.useSSSE3, o.useAVX2 = !*noSSSE3, !*noAVX2

	b.SetBytes(int64(size))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		galMulSliceXor(177, in[:], out[:], &o)
	}
}

func BenchmarkGaloisXor128K(b *testing.B) {
	benchmarkGaloisXor(b, 128*1024)
}

func BenchmarkGaloisXor1M(b *testing.B) {
	benchmarkGaloisXor(b, 1024*1024)
}
