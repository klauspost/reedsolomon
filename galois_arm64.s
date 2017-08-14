//+build !noasm !appengine

// Copyright 2015, Klaus Post, see LICENSE for details.
// Copyright 2017, Minio, Inc.

// Use github.com/minio/asm2plan9s on this file to assemble ARM instructions to
// the opcodes of their Plan9 equivalents

// func galMulNEON(c uint64, in, out []byte)
TEXT ·galMulNEON(SB), 7, $0
	MOVD c+0(FP), R0
	MOVD in_base+8(FP), R1
	MOVD in_len+16(FP), R2   // length of message
	MOVD out_base+32(FP), R5
	SUBS $16, R2
	BMI  complete

	// Load constants table pointer
	MOVD $·constants(SB), R3

	// and load constants into v28 & v29
	WORD $0x4c40a07c // ld1    {v28.16b-v29.16b}, [x3]

	WORD $0x4e010c1b // dup    v27.16b, w0

loop:
	// Main loop
	WORD $0x4cdf783a // ld1   {v26.4s}, [x1], #16

	// polynomial multiplication
	WORD $0x0e3be340 // pmull v0.8h,v26.8b,v27.8b
	WORD $0x4e3be346 // pmull2 v6.8h,v26.16b,v27.16b

	// first reduction
	WORD $0x0f088402 // shrn v2.8b, v0.8h, #8
	WORD $0x0f0884c8 // shrn v8.8b, v6.8h, #8
	WORD $0x0e22e383 // pmull v3.8h,v28.8b,v2.8b
	WORD $0x0e28e389 // pmull v9.8h,v28.8b,v8.8b
	WORD $0x6e201c60 // eor v0.16b,v3.16b,v0.16b
	WORD $0x6e261d26 // eor v6.16b,v9.16b,v6.16b

	// second reduction
	WORD $0x0f088404 // shrn v4.8b, v0.8h, #8
	WORD $0x0f0884ca // shrn v10.8b, v6.8h, #8
	WORD $0x6e241c44 // eor v4.16b,v2.16b,v4.16b
	WORD $0x6e2a1d0a // eor v10.16b,v8.16b,v10.16b
	WORD $0x0e24e385 // pmull v5.8h,v28.8b,v4.8b
	WORD $0x0e2ae38b // pmull v11.8h,v28.8b,v10.8b
	WORD $0x6e201ca0 // eor v0.16b,v5.16b,v0.16b
	WORD $0x6e261d61 // eor v1.16b,v11.16b,v6.16b

    // combine results
	WORD $0x4e1d2000 // tbl v0.16b,{v0.16b,v1.16b},v29.16b

	// Store result
	WORD $0x4c9f7ca0 // st1    {v0.2d}, [x5], #16

	SUBS $16, R2
	BPL  loop

complete:
	RET

// func galMulXorNEON(c uint64, in, out []byte)
TEXT ·galMulXorNEON(SB), 7, $0
	MOVD c+0(FP), R0
	MOVD in_base+8(FP), R1
	MOVD in_len+16(FP), R2   // length of message
	MOVD out_base+32(FP), R5
	SUBS $16, R2
	BMI  completeXor

	// Load constants table pointer
	MOVD $·constants(SB), R3

	// and load constants into v28 & v29
	WORD $0x4c40a07c // ld1    {v28.16b-v29.16b}, [x3]

	WORD $0x4e010c1b // dup    v27.16b, w0

loopXor:
	// Main loop
	WORD $0x4cdf783a // ld1   {v26.4s}, [x1], #16

	// polynomial multiplication
	WORD $0x0e3be340 // pmull v0.8h,v26.8b,v27.8b
	WORD $0x4e3be346 // pmull2 v6.8h,v26.16b,v27.16b

	// first reduction
	WORD $0x0f088402 // shrn v2.8b, v0.8h, #8
	WORD $0x0f0884c8 // shrn v8.8b, v6.8h, #8
	WORD $0x0e22e383 // pmull v3.8h,v28.8b,v2.8b
	WORD $0x0e28e389 // pmull v9.8h,v28.8b,v8.8b
	WORD $0x6e201c60 // eor v0.16b,v3.16b,v0.16b
	WORD $0x6e261d26 // eor v6.16b,v9.16b,v6.16b

	// second reduction
	WORD $0x0f088404 // shrn v4.8b, v0.8h, #8
	WORD $0x0f0884ca // shrn v10.8b, v6.8h, #8
	WORD $0x6e241c44 // eor v4.16b,v2.16b,v4.16b
	WORD $0x6e2a1d0a // eor v10.16b,v8.16b,v10.16b
	WORD $0x0e24e385 // pmull v5.8h,v28.8b,v4.8b
	WORD $0x0e2ae38b // pmull v11.8h,v28.8b,v10.8b
	WORD $0x6e201ca0 // eor v0.16b,v5.16b,v0.16b
	WORD $0x6e261d61 // eor v1.16b,v11.16b,v6.16b

    // combine results
	WORD $0x4e1d2000 // tbl v0.16b,{v0.16b,v1.16b},v29.16b

	// Load & xor result
	WORD $0x4c4078a1 // ld1   {v1.4s}, [x5]
	WORD $0x6e211c00 // eor v0.16b,v0.16b,v1.16b
	WORD $0x4c9f7ca0 // st1   {v0.2d}, [x5], #16

	SUBS $16, R2
	BPL  loopXor

completeXor:
	RET

// Constants
DATA ·constants+0x0(SB)/8, $0x1d1d1d1d1d1d1d1d
DATA ·constants+0x8(SB)/8, $0x1d1d1d1d1d1d1d1d
DATA ·constants+0x10(SB)/8, $0x0e0c0a0806040200
DATA ·constants+0x18(SB)/8, $0x1e1c1a1816141210

GLOBL ·constants(SB), 8, $32
