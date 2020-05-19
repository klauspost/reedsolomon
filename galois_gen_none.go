//+build !amd64 noasm appengine gccgo nogen

package reedsolomon

const maxAvx2Inputs = 0
const maxAvx2Outputs = 0
const avx2CodeGen = false

func galMulSlicesAvx2(matrixRows [][]byte, in, out [][]byte, start, stop int) {
	panic("avx2 codegen not available")
}
