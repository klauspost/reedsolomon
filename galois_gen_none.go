//go:build !(amd64 || arm64) || noasm || appengine || gccgo || nogen

package reedsolomon

const (
	codeGen              = false
	codeGenMaxGoroutines = 8
	codeGenMaxInputs     = 1
	codeGenMaxOutputs    = 1
	minCodeGenSize       = 1
)

func (r *reedSolomon) hasCodeGen(_ int, _, _ int) (_, _ *func(matrix []byte, in, out [][]byte, start, stop int) int, ok bool) {
	return nil, nil, false
}

func galMulSlicesGFNI(matrix []uint64, in, out [][]byte, start, stop int) int {
	panic("codegen not available")
}

func galMulSlicesGFNIXor(matrix []uint64, in, out [][]byte, start, stop int) int {
	panic("codegen not available")
}

func galMulSlicesAvxGFNI(matrix []uint64, in, out [][]byte, start, stop int) int {
	panic("codegen not available")
}

func galMulSlicesAvxGFNIXor(matrix []uint64, in, out [][]byte, start, stop int) int {
	panic("codegen not available")
}
