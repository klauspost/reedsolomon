//go:build !noasm && !appengine && !gccgo && !nopshufb

// Copyright 2015, Klaus Post, see LICENSE for details.
// Copyright 2017, Minio, Inc.

package reedsolomon

import (
	"fmt"
)

const pshufb = true

//go:noescape
func galMulNEON(low, high, in, out []byte)

//go:noescape
func galMulXorNEON(low, high, in, out []byte)

func galMulSlice(c byte, in, out []byte, o *options) {
	if c == 1 {
		copy(out, in)
		return
	}
	var done int
	galMulNEON(mulTableLow[c][:], mulTableHigh[c][:], in, out)
	done = (len(in) >> 5) << 5

	remain := len(in) - done
	if remain > 0 {
		mt := mulTable[c][:256]
		for i := done; i < len(in); i++ {
			out[i] = mt[in[i]]
		}
	}
}

func galMulSliceXor(c byte, in, out []byte, o *options) {
	if c == 1 {
		sliceXor(in, out, o)
		return
	}
	var done int
	galMulXorNEON(mulTableLow[c][:], mulTableHigh[c][:], in, out)
	done = (len(in) >> 5) << 5

	remain := len(in) - done
	if remain > 0 {
		mt := mulTable[c][:256]
		for i := done; i < len(in); i++ {
			out[i] ^= mt[in[i]]
		}
	}
}

// galMulSlicesSve
func galMulSlicesSve(matrix []byte, in, out [][]byte, start, stop int) int {
	n := stop - start

	// fmt.Println(len(in), len(out))
	switch len(out) {
	case 1:
		mulSve_10x1_64(matrix, in, out, start, n)
		return n & (maxInt - 63)
	case 2:
		mulSve_10x2_64(matrix, in, out, start, n)
		return n & (maxInt - 63)
	case 3:
		mulSve_10x3_64(matrix, in, out, start, n)
		return n & (maxInt - 63)
	case 4:
		mulSve_10x4(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 5:
		mulSve_10x5(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 6:
		mulSve_10x6(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 7:
		mulSve_10x7(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 8:
		mulSve_10x8(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 9:
		mulSve_10x9(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 10:
		mulSve_10x10(matrix, in, out, start, n)
		return n & (maxInt - 31)
	}
	panic(fmt.Sprintf("ARM SVE: unhandled size: %dx%d", len(in), len(out)))
}

// galMulSlicesSveXor
func galMulSlicesSveXor(matrix []byte, in, out [][]byte, start, stop int) int {
	n := (stop - start)

	switch len(out) {
	case 1:
		mulSve_10x1_64Xor(matrix, in, out, start, n)
		return n & (maxInt - 63)
	case 2:
		mulSve_10x2_64Xor(matrix, in, out, start, n)
		return n & (maxInt - 63)
	case 3:
		mulSve_10x3_64Xor(matrix, in, out, start, n)
		return n & (maxInt - 63)
	case 4:
		mulSve_10x4Xor(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 5:
		mulSve_10x5Xor(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 6:
		mulSve_10x6Xor(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 7:
		mulSve_10x7Xor(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 8:
		mulSve_10x8Xor(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 9:
		mulSve_10x9Xor(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 10:
		mulSve_10x10Xor(matrix, in, out, start, n)
		return n & (maxInt - 31)
	}
	panic(fmt.Sprintf("ARM SVE: unhandled size: %dx%d", len(in), len(out)))
}

// galMulSlicesNeon
func galMulSlicesNeon(matrix []byte, in, out [][]byte, start, stop int) int {
	n := stop - start

	switch len(out) {
	case 1:
		mulNeon_10x1_64(matrix, in, out, start, n)
		return n & (maxInt - 63)
	case 2:
		mulNeon_10x2_64(matrix, in, out, start, n)
		return n & (maxInt - 63)
	case 3:
		mulNeon_10x3_64(matrix, in, out, start, n)
		return n & (maxInt - 63)
	case 4:
		mulNeon_10x4(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 5:
		mulNeon_10x5(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 6:
		mulNeon_10x6(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 7:
		mulNeon_10x7(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 8:
		mulNeon_10x8(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 9:
		mulNeon_10x9(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 10:
		mulNeon_10x10(matrix, in, out, start, n)
		return n & (maxInt - 31)
	}
	panic(fmt.Sprintf("ARM NEON: unhandled size: %dx%d", len(in), len(out)))
}

// galMulSlicesNeonXor
func galMulSlicesNeonXor(matrix []byte, in, out [][]byte, start, stop int) int {
	n := (stop - start)

	switch len(out) {
	case 1:
		mulNeon_10x1_64Xor(matrix, in, out, start, n)
		return n & (maxInt - 63)
	case 2:
		mulNeon_10x2_64Xor(matrix, in, out, start, n)
		return n & (maxInt - 63)
	case 3:
		mulNeon_10x3_64Xor(matrix, in, out, start, n)
		return n & (maxInt - 63)
	case 4:
		mulNeon_10x4Xor(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 5:
		mulNeon_10x5Xor(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 6:
		mulNeon_10x6Xor(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 7:
		mulNeon_10x7Xor(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 8:
		mulNeon_10x8Xor(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 9:
		mulNeon_10x9Xor(matrix, in, out, start, n)
		return n & (maxInt - 31)
	case 10:
		mulNeon_10x10Xor(matrix, in, out, start, n)
		return n & (maxInt - 31)
	}
	panic(fmt.Sprintf("ARM NEON: unhandled size: %dx%d", len(in), len(out)))
}

// 4-way butterfly
func ifftDIT4(work [][]byte, dist int, log_m01, log_m23, log_m02 ffe, o *options) {
	ifftDIT4Ref(work, dist, log_m01, log_m23, log_m02, o)
}

// 4-way butterfly
func ifftDIT48(work [][]byte, dist int, log_m01, log_m23, log_m02 ffe8, o *options) {
	ifftDIT4Ref8(work, dist, log_m01, log_m23, log_m02, o)
}

// 4-way butterfly
func fftDIT4(work [][]byte, dist int, log_m01, log_m23, log_m02 ffe, o *options) {
	fftDIT4Ref(work, dist, log_m01, log_m23, log_m02, o)
}

// 4-way butterfly
func fftDIT48(work [][]byte, dist int, log_m01, log_m23, log_m02 ffe8, o *options) {
	fftDIT4Ref8(work, dist, log_m01, log_m23, log_m02, o)
}

// 2-way butterfly forward
func fftDIT2(x, y []byte, log_m ffe, o *options) {
	// Reference version:
	refMulAdd(x, y, log_m)
	// 64 byte aligned, always full.
	xorSliceNEON(x, y)
}

// 2-way butterfly forward
func fftDIT28(x, y []byte, log_m ffe8, o *options) {
	// Reference version:
	mulAdd8(x, y, log_m, o)
	sliceXor(x, y, o)
}

// 2-way butterfly
func ifftDIT2(x, y []byte, log_m ffe, o *options) {
	// 64 byte aligned, always full.
	xorSliceNEON(x, y)
	// Reference version:
	refMulAdd(x, y, log_m)
}

// 2-way butterfly inverse
func ifftDIT28(x, y []byte, log_m ffe8, o *options) {
	// Reference version:
	sliceXor(x, y, o)
	mulAdd8(x, y, log_m, o)
}

func mulgf16(x, y []byte, log_m ffe, o *options) {
	refMul(x, y, log_m)
}

func mulAdd8(out, in []byte, log_m ffe8, o *options) {
	t := &multiply256LUT8[log_m]
	galMulXorNEON(t[:16], t[16:32], in, out)
	done := (len(in) >> 5) << 5
	in = in[done:]
	if len(in) > 0 {
		out = out[done:]
		refMulAdd8(in, out, log_m)
	}
}

func mulgf8(out, in []byte, log_m ffe8, o *options) {
	var done int
	t := &multiply256LUT8[log_m]
	galMulNEON(t[:16], t[16:32], in, out)
	done = (len(in) >> 5) << 5

	remain := len(in) - done
	if remain > 0 {
		mt := mul8LUTs[log_m].Value[:]
		for i := done; i < len(in); i++ {
			out[i] ^= byte(mt[in[i]])
		}
	}
}
