//+build !noasm
//+build !appengine

// Copyright 2015, Klaus Post, see LICENSE for details.

package reedsolomon

import (
	"github.com/klauspost/cpuid"
)

func galMulSSE3(low, high, in, out []byte)
func galMulSSE3Xor(low, high, in, out []byte)

// This is what the assembler rountes does in blocks of 16 bytes:
/*
func galMulSSE3(low, high, in, out []byte) {
	for n, input := range in {
		l := input & 0xf
		h := input >> 4
		out[n] = low[l] ^ high[h]
	}
}

func galMulSSE3Xor(low, high, in, out []byte) {
	for n, input := range in {
		l := input & 0xf
		h := input >> 4
		out[n] ^= low[l] ^ high[h]
	}
}
*/

func galMulSlice(c byte, in, out []byte) {
	var done int
	if cpuid.CPU.SSE3() {
		galMulSSE3(mulTableLow[c][:], mulTableHigh[c][:], in, out)
		done = (len(in) >> 4) << 4
	}
	remain := len(in) - done
	if remain > 0 {
		mt := mulTable[c]
		for i := done; i < len(in); i++ {
			out[i] = mt[in[i]]
		}
	}
}

func galMulSliceXor(c byte, in, out []byte) {
	var done int
	if cpuid.CPU.SSE3() {
		galMulSSE3Xor(mulTableLow[c][:], mulTableHigh[c][:], in, out)
		done = (len(in) >> 4) << 4
	}
	remain := len(in) - done
	if remain > 0 {
		mt := mulTable[c]
		for i := done; i < len(in); i++ {
			out[i] ^= mt[in[i]]
		}
	}
}
