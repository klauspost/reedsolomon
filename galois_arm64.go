//go:build !noasm && !appengine && !gccgo
// +build !noasm,!appengine,!gccgo

// Copyright 2015, Klaus Post, see LICENSE for details.
// Copyright 2017, Minio, Inc.

package reedsolomon

//go:noescape
func galMulNEON(low, high, in, out []byte)

//go:noescape
func galMulXorNEON(low, high, in, out []byte)

//go:noescape
func galXorNEON(in, out []byte)

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

// simple slice xor
func sliceXor(in, out []byte, o *options) {

	galXorNEON(in, out)
	done := (len(in) >> 5) << 5

	remain := len(in) - done
	if remain > 0 {
		for i := done; i < len(in); i++ {
			out[i] ^= in[i]
		}
	}
}

// 2-way butterfly forward
func fftDIT2(x, y []byte, log_m ffe, o *options) {
	// Reference version:
	refMulAdd(x, y, log_m)
	// 64 byte aligned, always full.
	galXorNEON(x, y)
}

// 2-way butterfly
func ifftDIT2(x, y []byte, log_m ffe, o *options) {
	// 64 byte aligned, always full.
	galXorNEON(x, y)
	// Reference version:
	refMulAdd(x, y, log_m)
}

func mulgf16(x, y []byte, log_m ffe, o *options) {
	refMul(x, y, log_m)
}
