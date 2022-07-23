//go:build (!amd64 || noasm || appengine || gccgo) && (!arm64 || noasm || appengine || gccgo) && (!ppc64le || noasm || appengine || gccgo)
// +build !amd64 noasm appengine gccgo
// +build !arm64 noasm appengine gccgo
// +build !ppc64le noasm appengine gccgo

// Copyright 2015, Klaus Post, see LICENSE for details.

package reedsolomon

func galMulSlice(c byte, in, out []byte, o *options) {
	out = out[:len(in)]
	if c == 1 {
		copy(out, in)
		return
	}
	mt := mulTable[c][:256]
	for n, input := range in {
		out[n] = mt[input]
	}
}

func galMulSliceXor(c byte, in, out []byte, o *options) {
	out = out[:len(in)]
	if c == 1 {
		sliceXor(in, out, o)
		return
	}
	mt := mulTable[c][:256]
	for n, input := range in {
		out[n] ^= mt[input]
	}
}

// simple slice xor
func sliceXor(in, out []byte, o *options) {
	sliceXorGo(in, out, o)
}

func init() {
	defaultOptions.useAVX512 = false
}

// 2-way butterfly forward
func fftDIT2(x, y []byte, log_m ffe, o *options) {
	// Reference version:
	refMulAdd(x, y, log_m)
	sliceXorGo(x, y, o)
}

// 2-way butterfly inverse
func ifftDIT2(x, y []byte, log_m ffe, o *options) {
	// Reference version:
	sliceXorGo(x, y, o)
	refMulAdd(x, y, log_m)
}

func mulgf16(x, y []byte, log_m ffe, o *options) {
	refMul(x, y, log_m)
}
