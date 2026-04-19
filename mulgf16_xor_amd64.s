// AVX2 scalar-broadcast GF(2^16) mul-accumulate: out[:] ^= in[:] * scalar.
// Used as the inner kernel by the non-GFNI fallback path of
// GF16MulSliceXor8 — GFNI hosts take the fused 8-scalar kernel instead.
// Leopard-formatted 64-byte chunks (32 low bytes + 32 high bytes).

//go:build !appengine && !noasm && !nogen && !nopshufb && gc

#include "textflag.h"

// func mulgf16Xor_avx2(x []byte, y []byte, table *[128]uint8)
// Requires: AVX, AVX2, SSE2
TEXT ·mulgf16Xor_avx2(SB), NOSPLIT, $0-56
	MOVQ           table+48(FP), AX
	VBROADCASTI128 (AX), Y0
	VBROADCASTI128 64(AX), Y1
	VBROADCASTI128 16(AX), Y2
	VBROADCASTI128 80(AX), Y3
	VBROADCASTI128 32(AX), Y4
	VBROADCASTI128 96(AX), Y5
	VBROADCASTI128 48(AX), Y6
	VBROADCASTI128 112(AX), Y7
	MOVQ           x_len+8(FP), AX
	MOVQ           x_base+0(FP), CX
	MOVQ           y_base+24(FP), DX
	MOVQ           $0x0000000f, BX
	MOVQ           BX, X8
	VPBROADCASTB   X8, Y8

loop_mulgf16Xor_avx2:
	VMOVDQU (DX), Y9
	VMOVDQU 32(DX), Y10
	VPSRLQ  $0x04, Y9, Y11
	VPAND   Y8, Y9, Y9
	VPAND   Y8, Y11, Y11
	VPSHUFB Y9, Y0, Y12
	VPSHUFB Y9, Y1, Y9
	VPSHUFB Y11, Y2, Y13
	VPSHUFB Y11, Y3, Y11
	VPXOR   Y12, Y13, Y12
	VPXOR   Y9, Y11, Y9
	VPAND   Y10, Y8, Y11
	VPSRLQ  $0x04, Y10, Y10
	VPAND   Y8, Y10, Y10
	VPSHUFB Y11, Y4, Y13
	VPSHUFB Y11, Y5, Y11
	VPXOR   Y12, Y13, Y12
	VPXOR   Y9, Y11, Y9
	VPSHUFB Y10, Y6, Y13
	VPSHUFB Y10, Y7, Y11
	VPXOR   Y12, Y13, Y12
	VPXOR   Y9, Y11, Y9
	VPXOR   (CX), Y12, Y12
	VPXOR   32(CX), Y9, Y9
	VMOVDQU Y12, (CX)
	VMOVDQU Y9, 32(CX)
	ADDQ    $0x40, CX
	ADDQ    $0x40, DX
	SUBQ    $0x40, AX
	JNZ     loop_mulgf16Xor_avx2
	VZEROUPPER
	RET
