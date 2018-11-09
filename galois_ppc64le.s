//+build !noasm !appengine

// Copyright 2015, Klaus Post, see LICENSE for details.
// Copyright 2018, Minio, Inc.

#define LOW  R2
#define HIGH R3
#define IN   R4
#define LEN  R5
#define OUT  R6

#define X6 VS34
#define X7 VS35
#define MSG VS36
#define MSG_HI VS37

#define ROTATE   VS41
#define ROTATE_  V9
#define MASK     VS43
#define MASK_    V11
#define FLIP     VS44
#define FLIP_    V12

#define CONSTANTS R11

// func galMulPpc(low, high, in, out []byte)
TEXT ·galMulPpc(SB), 7, $0
    MOVD low+0(FP), LOW
    MOVD high+24(FP), HIGH

    MOVD in+48(FP), IN      // R11: &in
    MOVD in_len+56(FP), LEN // R9: len(in)
    MOVD out+72(FP), OUT    // DX: &out

    LXVD2X   (LOW)(R0), X6
    LXVD2X   (HIGH)(R0), X7
    XXPERMDI X6, X6, $2, X6
    XXPERMDI X7, X7, $2, X7

    MOVD     $16, R10
    MOVD     $32, R12

    MOVD     $·constants(SB), CONSTANTS
    LXVD2X   (CONSTANTS)(R0), ROTATE
    XXPERMDI ROTATE, ROTATE, $2, ROTATE

    LXVD2X   (CONSTANTS)(R10), MASK
    XXPERMDI MASK, MASK, $2, MASK

    LXVD2X   (CONSTANTS)(R12), FLIP
    XXPERMDI FLIP, FLIP, $2, FLIP

    VPERM    V2, V31, FLIP_, V2
    VPERM    V3, V31, FLIP_, V3

    MOVD     $0, R10

loop:
    LXVD2X   (IN)(R10), MSG
    XXPERMDI MSG, MSG, $2, MSG

    VSRB     V4, ROTATE_, V5
    VAND     V4, MASK_, V4
    VPERM    V2, V31, V4, V4
    VPERM    V3, V31, V5, V5

    VXOR      V4, V5, V4

    XXPERMDI MSG, MSG, $2, MSG
    STXVD2X  MSG, (OUT)(R10)

    ADD     $16, R10, R10
    CMP	LEN, R10
    BGT	loop

complete:
    RET

DATA ·constants+0x0(SB)/8, $0x0404040404040404
DATA ·constants+0x8(SB)/8, $0x0404040404040404
DATA ·constants+0x10(SB)/8, $0x0f0f0f0f0f0f0f0f
DATA ·constants+0x18(SB)/8, $0x0f0f0f0f0f0f0f0f
DATA ·constants+0x20(SB)/8, $0x0706050403020100
DATA ·constants+0x28(SB)/8, $0x0f0e0d0c0b0a0908

GLOBL ·constants(SB), 8, $48

// func galMulPpcXor(c uint64, in, out []byte)
TEXT ·galMulPpcXor(SB), 7, $0
completeXor:
	RET
