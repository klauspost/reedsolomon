//go:build !noasm && !appengine && !gccgo && !nopshufb

package reedsolomon

//go:noescape
func mulSve_10x1_64(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x1_64Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x2_64(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x2_64Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x3_64(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x3_64Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x4(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x4Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x5(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x5Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x6(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x6Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x7(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x7Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x8(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x8Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x9(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x9Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x10(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulSve_10x10Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x1_64(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x1_64Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x2_64(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x2_64Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x3_64(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x3_64Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x4(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x4Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x5(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x5Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x6(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x6Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x7(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x7Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x8(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x8Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x9(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x9Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x10(matrix []byte, in [][]byte, out [][]byte, start int, n int)

//go:noescape
func mulNeon_10x10Xor(matrix []byte, in [][]byte, out [][]byte, start int, n int)
