//+build !noasm
//+build !appengine
//+build !gccgo

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
	out = out[:len(in)]
	mt := mulTable[c][:256]
	for i := range in {
		out[i] ^= mt[in[i]]
	}
}

// slice galois add
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
	}
	out = out[:len(in)]
	for i := range in {
		out[i] ^= in[i]
	}
}

const maxAvx2Inputs = 11
const maxAvx2Outputs = 11

func galMulSlicesAvx2(matrixRows [][]byte, in, out [][]byte) {
	if len(in) > maxAvx2Inputs {
		panic("max input exceeded")
	}
	if len(out) > maxAvx2Outputs {
		panic("max output exceeded")
	}
	switch len(in) {
	case 2:
		switch len(out) {
		case 2:
			mulAvxTwo_2x2([4][16]byte{
				mulTableLow[matrixRows[0][0]],
				mulTableLow[matrixRows[1][0]],
				mulTableLow[matrixRows[0][1]],
				mulTableLow[matrixRows[1][1]],
			}, [4][16]byte{
				mulTableHigh[matrixRows[0][0]],
				mulTableHigh[matrixRows[1][0]],
				mulTableHigh[matrixRows[0][1]],
				mulTableHigh[matrixRows[1][1]],
			},
				[2][]byte{in[0], in[1]},
				[2][]byte{out[0], out[1]},
			)
		}
	case 4:
		switch len(out) {
		case 2:
			/*
				mulAvxTwo_4x2([8][16]byte{
					mulTableLow[matrixRows[0][0]],
					mulTableLow[matrixRows[1][0]],
					mulTableLow[matrixRows[0][1]],
					mulTableLow[matrixRows[1][1]],
					mulTableLow[matrixRows[0][2]],
					mulTableLow[matrixRows[1][2]],
					mulTableLow[matrixRows[0][3]],
					mulTableLow[matrixRows[1][3]],
				}, [8][16]byte{
					mulTableHigh[matrixRows[0][0]],
					mulTableHigh[matrixRows[1][0]],
					mulTableHigh[matrixRows[0][1]],
					mulTableHigh[matrixRows[1][1]],
					mulTableHigh[matrixRows[0][2]],
					mulTableHigh[matrixRows[1][2]],
					mulTableHigh[matrixRows[0][3]],
					mulTableHigh[matrixRows[1][3]],
				},
					[4][]byte{in[0], in[1], in[2], in[3]},
					[2][]byte{out[0], out[1]},
				)
			*/
		case 4:
			/*
				mulAvx2_4_4([16][16]byte{
					mulTableLow[matrixRows[0][0]],
					mulTableLow[matrixRows[1][0]],
					mulTableLow[matrixRows[2][0]],
					mulTableLow[matrixRows[3][0]],
					mulTableLow[matrixRows[0][1]],
					mulTableLow[matrixRows[1][1]],
					mulTableLow[matrixRows[2][1]],
					mulTableLow[matrixRows[3][1]],
					mulTableLow[matrixRows[0][2]],
					mulTableLow[matrixRows[1][2]],
					mulTableLow[matrixRows[2][2]],
					mulTableLow[matrixRows[3][2]],
					mulTableLow[matrixRows[0][3]],
					mulTableLow[matrixRows[1][3]],
					mulTableLow[matrixRows[2][3]],
					mulTableLow[matrixRows[3][3]],
				}, [16][16]byte{
					mulTableHigh[matrixRows[0][0]],
					mulTableHigh[matrixRows[1][0]],
					mulTableHigh[matrixRows[2][0]],
					mulTableHigh[matrixRows[3][0]],
					mulTableHigh[matrixRows[0][1]],
					mulTableHigh[matrixRows[1][1]],
					mulTableHigh[matrixRows[2][1]],
					mulTableHigh[matrixRows[3][1]],
					mulTableHigh[matrixRows[0][2]],
					mulTableHigh[matrixRows[1][2]],
					mulTableHigh[matrixRows[2][2]],
					mulTableHigh[matrixRows[3][2]],
					mulTableHigh[matrixRows[0][3]],
					mulTableHigh[matrixRows[1][3]],
					mulTableHigh[matrixRows[2][3]],
					mulTableHigh[matrixRows[3][3]],
				},
					[4][]byte{in[0], in[1], in[2], in[3]},
					[4][]byte{out[0], out[1], out[2], out[3]},
				)
			*/
		}
	case 5:
		switch len(out) {
		case 1:
			mulAvxTwo_5x1([5][16]byte{
				mulTableLow[matrixRows[0][0]],
				mulTableLow[matrixRows[0][1]],
				mulTableLow[matrixRows[0][2]],
				mulTableLow[matrixRows[0][3]],
				mulTableLow[matrixRows[0][4]],
			}, [5][16]byte{
				mulTableHigh[matrixRows[0][0]],
				mulTableHigh[matrixRows[0][1]],
				mulTableHigh[matrixRows[0][2]],
				mulTableHigh[matrixRows[0][3]],
				mulTableHigh[matrixRows[0][4]],
			},
				[5][]byte{in[0], in[1], in[2], in[3], in[4]},
				[1][]byte{out[0]},
			)
		}
	case 10:
		switch len(out) {
		case 1:
			/*
				mulAvxTwo_10x1([10][16]byte{
					mulTableLow[matrixRows[0][0]],
					mulTableLow[matrixRows[0][1]],
					mulTableLow[matrixRows[0][2]],
					mulTableLow[matrixRows[0][3]],
					mulTableLow[matrixRows[0][4]],
					mulTableLow[matrixRows[0][5]],
					mulTableLow[matrixRows[0][6]],
					mulTableLow[matrixRows[0][7]],
					mulTableLow[matrixRows[0][8]],
					mulTableLow[matrixRows[0][9]],
				}, [10][16]byte{
					mulTableHigh[matrixRows[0][0]],
					mulTableHigh[matrixRows[0][1]],
					mulTableHigh[matrixRows[0][2]],
					mulTableHigh[matrixRows[0][3]],
					mulTableHigh[matrixRows[0][4]],
					mulTableHigh[matrixRows[0][5]],
					mulTableHigh[matrixRows[0][6]],
					mulTableHigh[matrixRows[0][7]],
					mulTableHigh[matrixRows[0][8]],
					mulTableHigh[matrixRows[0][9]],
				},
					[10][]byte{in[0], in[1], in[2], in[3], in[4], in[5], in[6], in[7], in[8], in[9]},
					[1][]byte{out[0]},
				)

			*/
		}
	}
}
