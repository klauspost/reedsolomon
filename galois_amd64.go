//go:build !noasm && !appengine && !gccgo
// +build !noasm,!appengine,!gccgo

// Copyright 2015, Klaus Post, see LICENSE for details.

package reedsolomon

//go:noescape
func galMulSSSE3(low, high, in, out []byte)

//go:noescape
func galMulSSSE3Xor(low, high, in, out []byte)

//go:noescape
func galMulAVX2Xor(low, high, in, out []byte)

//go:noescape
func galMulAVX2(low, high, in, out []byte)

//go:noescape
func sSE2XorSlice(in, out []byte)

//go:noescape
func galMulAVX2Xor_64(low, high, in, out []byte)

//go:noescape
func galMulAVX2_64(low, high, in, out []byte)

//go:noescape
func sSE2XorSlice_64(in, out []byte)

// This is what the assembler routines do in blocks of 16 bytes:
/*
func galMulSSSE3(low, high, in, out []byte) {
	for n, input := range in {
		l := input & 0xf
		h := input >> 4
		out[n] = low[l] ^ high[h]
	}
}

func galMulSSSE3Xor(low, high, in, out []byte) {
	for n, input := range in {
		l := input & 0xf
		h := input >> 4
		out[n] ^= low[l] ^ high[h]
	}
}
*/

// bigSwitchover is the size where 64 bytes are processed per loop.
const bigSwitchover = 128

func galMulSlice(c byte, in, out []byte, o *options) {
	if c == 1 {
		copy(out, in)
		return
	}
	if o.useAVX2 {
		if len(in) >= bigSwitchover {
			galMulAVX2_64(mulTableLow[c][:], mulTableHigh[c][:], in, out)
			done := (len(in) >> 6) << 6
			in = in[done:]
			out = out[done:]
		}
		if len(in) > 32 {
			galMulAVX2(mulTableLow[c][:], mulTableHigh[c][:], in, out)
			done := (len(in) >> 5) << 5
			in = in[done:]
			out = out[done:]
		}
	} else if o.useSSSE3 {
		galMulSSSE3(mulTableLow[c][:], mulTableHigh[c][:], in, out)
		done := (len(in) >> 4) << 4
		in = in[done:]
		out = out[done:]
	}
	out = out[:len(in)]
	mt := mulTable[c][:256]
	for i := range in {
		out[i] = mt[in[i]]
	}
}

func galMulSliceXor(c byte, in, out []byte, o *options) {
	if c == 1 {
		sliceXor(in, out, o)
		return
	}

	if o.useAVX2 {
		if len(in) >= bigSwitchover {
			galMulAVX2Xor_64(mulTableLow[c][:], mulTableHigh[c][:], in, out)
			done := (len(in) >> 6) << 6
			in = in[done:]
			out = out[done:]
		}
		if len(in) >= 32 {
			galMulAVX2Xor(mulTableLow[c][:], mulTableHigh[c][:], in, out)
			done := (len(in) >> 5) << 5
			in = in[done:]
			out = out[done:]
		}
	} else if o.useSSSE3 {
		galMulSSSE3Xor(mulTableLow[c][:], mulTableHigh[c][:], in, out)
		done := (len(in) >> 4) << 4
		in = in[done:]
		out = out[done:]
	}
	if len(in) == 0 {
		return
	}
	out = out[:len(in)]
	mt := mulTable[c][:256]
	for i := range in {
		out[i] ^= mt[in[i]]
	}
}

// simple slice xor
func sliceXor(in, out []byte, o *options) {
	if o.useSSE2 {
		if len(in) >= bigSwitchover {
			sSE2XorSlice_64(in, out)
			done := (len(in) >> 6) << 6
			in = in[done:]
			out = out[done:]
		}
		if len(in) >= 16 {
			sSE2XorSlice(in, out)
			done := (len(in) >> 4) << 4
			in = in[done:]
			out = out[done:]
		}
	} else {
		sliceXorGo(in, out, o)
		return
	}
	out = out[:len(in)]
	for i := range in {
		out[i] ^= in[i]
	}
}

// 4-way butterfly
func ifftDIT4(work [][]byte, dist int, log_m01, log_m23, log_m02 ffe, o *options) {
	if len(work[0]) == 0 {
		return
	}

	t01 := &multiply256LUT[log_m01]
	t23 := &multiply256LUT[log_m23]
	t02 := &multiply256LUT[log_m02]
	if o.useAVX512 {
		if log_m01 == modulus {
			if log_m23 == modulus {
				if log_m02 == modulus {
					ifftDIT4_avx512_7(work, dist*24, t01, t23, t02)
				} else {
					ifftDIT4_avx512_3(work, dist*24, t01, t23, t02)
				}
			} else {
				if log_m02 == modulus {
					ifftDIT4_avx512_5(work, dist*24, t01, t23, t02)
				} else {
					ifftDIT4_avx512_1(work, dist*24, t01, t23, t02)
				}
			}
		} else {
			if log_m23 == modulus {
				if log_m02 == modulus {
					ifftDIT4_avx512_6(work, dist*24, t01, t23, t02)
				} else {
					ifftDIT4_avx512_2(work, dist*24, t01, t23, t02)
				}
			} else {
				if log_m02 == modulus {
					ifftDIT4_avx512_4(work, dist*24, t01, t23, t02)
				} else {
					ifftDIT4_avx512_0(work, dist*24, t01, t23, t02)
				}
			}
		}
		return
	} else if o.useAVX2 {
		if log_m01 == modulus {
			if log_m23 == modulus {
				if log_m02 == modulus {
					ifftDIT4_avx2_7(work, dist*24, t01, t23, t02)
				} else {
					ifftDIT4_avx2_3(work, dist*24, t01, t23, t02)
				}
			} else {
				if log_m02 == modulus {
					ifftDIT4_avx2_5(work, dist*24, t01, t23, t02)
				} else {
					ifftDIT4_avx2_1(work, dist*24, t01, t23, t02)
				}
			}
		} else {
			if log_m23 == modulus {
				if log_m02 == modulus {
					ifftDIT4_avx2_6(work, dist*24, t01, t23, t02)
				} else {
					ifftDIT4_avx2_2(work, dist*24, t01, t23, t02)
				}
			} else {
				if log_m02 == modulus {
					ifftDIT4_avx2_4(work, dist*24, t01, t23, t02)
				} else {
					ifftDIT4_avx2_0(work, dist*24, t01, t23, t02)
				}
			}
		}
		return
	}
	ifftDIT4Ref(work, dist, log_m01, log_m23, log_m02, o)
}

// 4-way butterfly
func ifftDIT48(work [][]byte, dist int, log_m01, log_m23, log_m02 ffe8, o *options) {
	ifftDIT4Ref8(work, dist, log_m01, log_m23, log_m02, o)
}

func fftDIT4(work [][]byte, dist int, log_m01, log_m23, log_m02 ffe, o *options) {
	if len(work[0]) == 0 {
		return
	}

	t01 := &multiply256LUT[log_m01]
	t23 := &multiply256LUT[log_m23]
	t02 := &multiply256LUT[log_m02]
	if o.useAVX512 {
		if log_m02 == modulus {
			if log_m01 == modulus {
				if log_m23 == modulus {
					fftDIT4_avx512_7(work, dist*24, t01, t23, t02)
				} else {
					fftDIT4_avx512_3(work, dist*24, t01, t23, t02)
				}
			} else {
				if log_m23 == modulus {
					fftDIT4_avx512_5(work, dist*24, t01, t23, t02)
				} else {
					fftDIT4_avx512_1(work, dist*24, t01, t23, t02)
				}
			}
		} else {
			if log_m01 == modulus {
				if log_m23 == modulus {
					fftDIT4_avx512_6(work, dist*24, t01, t23, t02)
				} else {
					fftDIT4_avx512_2(work, dist*24, t01, t23, t02)
				}
			} else {
				if log_m23 == modulus {
					fftDIT4_avx512_4(work, dist*24, t01, t23, t02)
				} else {
					fftDIT4_avx512_0(work, dist*24, t01, t23, t02)
				}
			}
		}
		return
	} else if o.useAVX2 {
		if log_m02 == modulus {
			if log_m01 == modulus {
				if log_m23 == modulus {
					fftDIT4_avx2_7(work, dist*24, t01, t23, t02)
				} else {
					fftDIT4_avx2_3(work, dist*24, t01, t23, t02)
				}
			} else {
				if log_m23 == modulus {
					fftDIT4_avx2_5(work, dist*24, t01, t23, t02)
				} else {
					fftDIT4_avx2_1(work, dist*24, t01, t23, t02)
				}
			}
		} else {
			if log_m01 == modulus {
				if log_m23 == modulus {
					fftDIT4_avx2_6(work, dist*24, t01, t23, t02)
				} else {
					fftDIT4_avx2_2(work, dist*24, t01, t23, t02)
				}
			} else {
				if log_m23 == modulus {
					fftDIT4_avx2_4(work, dist*24, t01, t23, t02)
				} else {
					fftDIT4_avx2_0(work, dist*24, t01, t23, t02)
				}
			}
		}
		return
	}
	fftDIT4Ref(work, dist, log_m01, log_m23, log_m02, o)
}

// 4-way butterfly
func fftDIT48(work [][]byte, dist int, log_m01, log_m23, log_m02 ffe8, o *options) {
	fftDIT4Ref8(work, dist, log_m01, log_m23, log_m02, o)
}

// 2-way butterfly forward
func fftDIT2(x, y []byte, log_m ffe, o *options) {
	if o.useAVX2 {
		if len(x) > 0 {
			tmp := &multiply256LUT[log_m]
			fftDIT2_avx2(x, y, tmp)
		}
	} else if o.useSSSE3 {
		if len(x) > 0 {
			tmp := &multiply256LUT[log_m]
			fftDIT2_ssse3(x, y, tmp)
		}
	} else {
		// Reference version:
		refMulAdd(x, y, log_m)
		sliceXor(x, y, o)
	}
}

// 2-way butterfly forward
func fftDIT28(x, y []byte, log_m ffe8, o *options) {
	// Reference version:
	mulAdd8(x, y, log_m, o)
	sliceXor(x, y, o)
}

// 2-way butterfly inverse
func ifftDIT28(x, y []byte, log_m ffe8, o *options) {
	// Reference version:
	sliceXorGo(x, y, o)
	mulAdd8(x, y, log_m, o)
}

func mulAdd8(out, in []byte, log_m ffe8, o *options) {
	if o.useAVX2 {
		t := &multiply256LUT8[log_m]
		galMulAVX2Xor_64(t[:16], t[16:32], in, out)
		done := (len(in) >> 6) << 6
		in = in[done:]
		out = out[done:]
	} else if o.useSSSE3 {
		t := &multiply256LUT8[log_m]
		galMulSSSE3Xor(t[:16], t[16:32], in, out)
		done := (len(in) >> 4) << 4
		in = in[done:]
		out = out[done:]
	}
	refMulAdd8(in, out, log_m)
}

// 2-way butterfly
func ifftDIT2(x, y []byte, log_m ffe, o *options) {
	if o.useAVX2 {
		if len(x) > 0 {
			tmp := &multiply256LUT[log_m]
			ifftDIT2_avx2(x, y, tmp)
		}
	} else if o.useSSSE3 {
		if len(x) > 0 {
			tmp := &multiply256LUT[log_m]
			ifftDIT2_ssse3(x, y, tmp)
		}
	} else {
		// Reference version:
		sliceXor(x, y, o)
		refMulAdd(x, y, log_m)
	}
}

func mulgf16(x, y []byte, log_m ffe, o *options) {
	if o.useAVX2 {
		if len(x) > 0 {
			tmp := &multiply256LUT[log_m]
			mulgf16_avx2(x, y, tmp)
		}
	} else if o.useSSSE3 {
		if len(x) > 0 {
			tmp := &multiply256LUT[log_m]
			mulgf16_ssse3(x, y, tmp)
		}
	} else {
		refMul(x, y, log_m)
	}
}

func mulgf8(out, in []byte, log_m ffe8, o *options) {
	if o.useAVX2 {
		t := &multiply256LUT8[log_m]
		galMulAVX2_64(t[:16], t[16:32], in, out)
		done := (len(in) >> 6) << 6
		in = in[done:]
		out = out[done:]
	} else if o.useSSSE3 {
		t := &multiply256LUT8[log_m]
		galMulSSSE3(t[:16], t[16:32], in, out)
		done := (len(in) >> 4) << 4
		in = in[done:]
		out = out[done:]
	}
	out = out[:len(in)]
	mt := mul8LUTs[log_m].Value[:]
	for i := range in {
		out[i] = byte(mt[in[i]])
	}
}
