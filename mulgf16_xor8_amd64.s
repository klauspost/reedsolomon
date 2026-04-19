// Fused GF(2^16) mul-XOR kernel: given a single source column `col` and 8
// scalars, produces 8 separate accumulate-XORs in one pass, reading `col`
// once per 64-byte chunk instead of 8 times.
//
// out_k[i] ^= col[i] * scalar_k   for k in [0, 8)
//
// Leopard-formatted 64-byte chunks (32 low bytes + 32 high bytes).

//go:build !appengine && !noasm && !nogen && !nopshufb && gc

#include "textflag.h"

// func mulgf16Xor8_gfni(col []byte, tables *[8 * 4]uint64, outs *[8]uintptr)
// Requires: AVX, AVX2, GFNI
// len(col) must be a positive multiple of 64.
// tables must hold 8 contiguous [4]uint64 GFNI matrices (32 bytes each).
// outs holds 8 pointers to destination buffers, each at least len(col) bytes.
// On exit the 8 destination pointers are unchanged.
TEXT ·mulgf16Xor8_gfni(SB), NOSPLIT, $0-40
	MOVQ col_base+0(FP), DX
	MOVQ col_len+8(FP), AX
	MOVQ tables+24(FP), BX
	MOVQ outs+32(FP), SI

	// Load the 8 output base pointers into R8..R15.
	MOVQ (SI), R8
	MOVQ 8(SI), R9
	MOVQ 16(SI), R10
	MOVQ 24(SI), R11
	MOVQ 32(SI), R12
	MOVQ 40(SI), R13
	MOVQ 48(SI), R14
	MOVQ 56(SI), R15

loop_mulgf16Xor8_gfni:
	VMOVDQU (DX), Y0
	VMOVDQU 32(DX), Y1

	// k = 0
	VBROADCASTSD   0(BX), Y2
	VBROADCASTSD   8(BX), Y3
	VBROADCASTSD   16(BX), Y4
	VBROADCASTSD   24(BX), Y5
	VGF2P8AFFINEQB $0x00, Y2, Y0, Y6
	VGF2P8AFFINEQB $0x00, Y3, Y1, Y7
	VGF2P8AFFINEQB $0x00, Y4, Y0, Y8
	VGF2P8AFFINEQB $0x00, Y5, Y1, Y9
	VPXOR          Y6, Y7, Y6
	VPXOR          Y8, Y9, Y8
	VPXOR          (R8), Y6, Y6
	VPXOR          32(R8), Y8, Y8
	VMOVDQU        Y6, (R8)
	VMOVDQU        Y8, 32(R8)

	// k = 1
	VBROADCASTSD   32(BX), Y2
	VBROADCASTSD   40(BX), Y3
	VBROADCASTSD   48(BX), Y4
	VBROADCASTSD   56(BX), Y5
	VGF2P8AFFINEQB $0x00, Y2, Y0, Y6
	VGF2P8AFFINEQB $0x00, Y3, Y1, Y7
	VGF2P8AFFINEQB $0x00, Y4, Y0, Y8
	VGF2P8AFFINEQB $0x00, Y5, Y1, Y9
	VPXOR          Y6, Y7, Y6
	VPXOR          Y8, Y9, Y8
	VPXOR          (R9), Y6, Y6
	VPXOR          32(R9), Y8, Y8
	VMOVDQU        Y6, (R9)
	VMOVDQU        Y8, 32(R9)

	// k = 2
	VBROADCASTSD   64(BX), Y2
	VBROADCASTSD   72(BX), Y3
	VBROADCASTSD   80(BX), Y4
	VBROADCASTSD   88(BX), Y5
	VGF2P8AFFINEQB $0x00, Y2, Y0, Y6
	VGF2P8AFFINEQB $0x00, Y3, Y1, Y7
	VGF2P8AFFINEQB $0x00, Y4, Y0, Y8
	VGF2P8AFFINEQB $0x00, Y5, Y1, Y9
	VPXOR          Y6, Y7, Y6
	VPXOR          Y8, Y9, Y8
	VPXOR          (R10), Y6, Y6
	VPXOR          32(R10), Y8, Y8
	VMOVDQU        Y6, (R10)
	VMOVDQU        Y8, 32(R10)

	// k = 3
	VBROADCASTSD   96(BX), Y2
	VBROADCASTSD   104(BX), Y3
	VBROADCASTSD   112(BX), Y4
	VBROADCASTSD   120(BX), Y5
	VGF2P8AFFINEQB $0x00, Y2, Y0, Y6
	VGF2P8AFFINEQB $0x00, Y3, Y1, Y7
	VGF2P8AFFINEQB $0x00, Y4, Y0, Y8
	VGF2P8AFFINEQB $0x00, Y5, Y1, Y9
	VPXOR          Y6, Y7, Y6
	VPXOR          Y8, Y9, Y8
	VPXOR          (R11), Y6, Y6
	VPXOR          32(R11), Y8, Y8
	VMOVDQU        Y6, (R11)
	VMOVDQU        Y8, 32(R11)

	// k = 4
	VBROADCASTSD   128(BX), Y2
	VBROADCASTSD   136(BX), Y3
	VBROADCASTSD   144(BX), Y4
	VBROADCASTSD   152(BX), Y5
	VGF2P8AFFINEQB $0x00, Y2, Y0, Y6
	VGF2P8AFFINEQB $0x00, Y3, Y1, Y7
	VGF2P8AFFINEQB $0x00, Y4, Y0, Y8
	VGF2P8AFFINEQB $0x00, Y5, Y1, Y9
	VPXOR          Y6, Y7, Y6
	VPXOR          Y8, Y9, Y8
	VPXOR          (R12), Y6, Y6
	VPXOR          32(R12), Y8, Y8
	VMOVDQU        Y6, (R12)
	VMOVDQU        Y8, 32(R12)

	// k = 5
	VBROADCASTSD   160(BX), Y2
	VBROADCASTSD   168(BX), Y3
	VBROADCASTSD   176(BX), Y4
	VBROADCASTSD   184(BX), Y5
	VGF2P8AFFINEQB $0x00, Y2, Y0, Y6
	VGF2P8AFFINEQB $0x00, Y3, Y1, Y7
	VGF2P8AFFINEQB $0x00, Y4, Y0, Y8
	VGF2P8AFFINEQB $0x00, Y5, Y1, Y9
	VPXOR          Y6, Y7, Y6
	VPXOR          Y8, Y9, Y8
	VPXOR          (R13), Y6, Y6
	VPXOR          32(R13), Y8, Y8
	VMOVDQU        Y6, (R13)
	VMOVDQU        Y8, 32(R13)

	// k = 6
	VBROADCASTSD   192(BX), Y2
	VBROADCASTSD   200(BX), Y3
	VBROADCASTSD   208(BX), Y4
	VBROADCASTSD   216(BX), Y5
	VGF2P8AFFINEQB $0x00, Y2, Y0, Y6
	VGF2P8AFFINEQB $0x00, Y3, Y1, Y7
	VGF2P8AFFINEQB $0x00, Y4, Y0, Y8
	VGF2P8AFFINEQB $0x00, Y5, Y1, Y9
	VPXOR          Y6, Y7, Y6
	VPXOR          Y8, Y9, Y8
	VPXOR          (R14), Y6, Y6
	VPXOR          32(R14), Y8, Y8
	VMOVDQU        Y6, (R14)
	VMOVDQU        Y8, 32(R14)

	// k = 7
	VBROADCASTSD   224(BX), Y2
	VBROADCASTSD   232(BX), Y3
	VBROADCASTSD   240(BX), Y4
	VBROADCASTSD   248(BX), Y5
	VGF2P8AFFINEQB $0x00, Y2, Y0, Y6
	VGF2P8AFFINEQB $0x00, Y3, Y1, Y7
	VGF2P8AFFINEQB $0x00, Y4, Y0, Y8
	VGF2P8AFFINEQB $0x00, Y5, Y1, Y9
	VPXOR          Y6, Y7, Y6
	VPXOR          Y8, Y9, Y8
	VPXOR          (R15), Y6, Y6
	VPXOR          32(R15), Y8, Y8
	VMOVDQU        Y6, (R15)
	VMOVDQU        Y8, 32(R15)

	// Advance all pointers by 64.
	ADDQ $64, R8
	ADDQ $64, R9
	ADDQ $64, R10
	ADDQ $64, R11
	ADDQ $64, R12
	ADDQ $64, R13
	ADDQ $64, R14
	ADDQ $64, R15
	ADDQ $64, DX
	SUBQ $64, AX
	JNZ  loop_mulgf16Xor8_gfni

	VZEROUPPER
	RET
