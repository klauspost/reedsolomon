//+build !noasm !appengine !gccgo

// Copyright 2015, Klaus Post, see LICENSE for details.
// Copyright 2017, Minio, Inc.

// Use github.com/minio/asm2plan9s on this file to assemble ARM instructions to
// the opcodes of their Plan9 equivalents

// func galMulNEON(low, high, in, out []byte)
TEXT ·galMulNEON(SB), 7, $0
	MOVD in_base+48(FP),  R1
	MOVD in_len+56(FP),   R2 // length of message
	MOVD out_base+72(FP), R5
	SUBS $32, R2
	BMI  complete

	MOVD low+0(FP),   R10 // R10: &low
	MOVD high+24(FP), R11 // R11: &high
	WORD $0x4c407146 // ld1 {v6.16b}, [x10]
	WORD $0x4c407167 // ld1 {v7.16b}, [x11]

	MOVD $0x0f, R3
	WORD $0x4e010c68 // dup v8.16b, w3

loop:
	// Main loop
	WORD $0x4cdfa020 // ld1  {v0.16b-v1.16b}, [x1], #32

	// Get low input and high input
	WORD $0x6f0c040a // ushr v10.16b, v0.16b, #4
	WORD $0x6f0c042b // ushr v11.16b, v1.16b, #4
	WORD $0x4e281c00 // and  v0.16b, v0.16b, v8.16b
	WORD $0x4e281c21 // and  v1.16b, v1.16b, v8.16b

	// Mul low part and mul high part
	WORD $0x4e0000c4 // tbl  v4.16b, {v6.16b}, v0.16b
	WORD $0x4e0a00e5 // tbl  v5.16b, {v7.16b}, v10.16b
	WORD $0x4e0100ce // tbl  v14.16b, {v6.16b}, v1.16b
	WORD $0x4e0b00ef // tbl  v15.16b, {v7.16b}, v11.16b

	// Combine results
	WORD $0x6e251c84 // eor  v4.16b, v4.16b,  v5.16b
	WORD $0x6e2f1dc5 // eor  v5.16b, v14.16b, v15.16b

	// Store result
	WORD $0x4c9faca4 // st1  {v4.2d-v5.2d}, [x5], #32

	SUBS $32, R2
	BPL  loop

complete:
	RET


// func galMulXorNEON(low, high, in, out []byte)
TEXT ·galMulXorNEON(SB), 7, $0
	MOVD in_base+48(FP),  R1
	MOVD in_len+56(FP),   R2 // length of message
	MOVD out_base+72(FP), R5
	SUBS $32, R2
	BMI  completeXor

	MOVD low+0(FP),   R10 // R10: &low
	MOVD high+24(FP), R11 // R11: &high
	WORD $0x4c407146 // ld1 {v6.16b}, [x10]
	WORD $0x4c407167 // ld1 {v7.16b}, [x11]

	MOVD $0x0f, R3
	WORD $0x4e010c68 // dup v8.16b, w3

loopXor:
	// Main loop
	WORD $0x4cdfa020 // ld1  {v0.16b-v1.16b}, [x1], #32
	WORD $0x4c40a0b4 // ld1  {v20.16b-v21.16b}, [x5]

	// Get low input and high input
	WORD $0x6f0c040a // ushr v10.16b, v0.16b, #4
	WORD $0x6f0c042b // ushr v11.16b, v1.16b, #4
	WORD $0x4e281c00 // and  v0.16b, v0.16b, v8.16b
	WORD $0x4e281c21 // and  v1.16b, v1.16b, v8.16b

	// Mul low part and mul high part
	WORD $0x4e0000c4 // tbl  v4.16b, {v6.16b}, v0.16b
	WORD $0x4e0a00e5 // tbl  v5.16b, {v7.16b}, v10.16b
	WORD $0x4e0100ce // tbl  v14.16b, {v6.16b}, v1.16b
	WORD $0x4e0b00ef // tbl  v15.16b, {v7.16b}, v11.16b

	// Combine results
	WORD $0x6e251c84 // eor  v4.16b, v4.16b,  v5.16b
	WORD $0x6e2f1dc5 // eor  v5.16b, v14.16b, v15.16b
	WORD $0x6e341c84 // eor  v4.16b, v4.16b,  v20.16b
	WORD $0x6e351ca5 // eor  v5.16b, v5.16b,  v21.16b

	// Store result
	WORD $0x4c9faca4 // st1  {v4.2d-v5.2d}, [x5], #32

	SUBS $32, R2
	BPL  loopXor

completeXor:
	RET

// func galXorNEON(in, out []byte)
TEXT ·galXorNEON(SB), 7, $0
	MOVD in_base+0(FP),  R1
	MOVD in_len+8(FP),   R2 // length of message
	MOVD out_base+24(FP), R5
	SUBS $32, R2
	BMI  completeXor

loopXor:
	// Main loop
	WORD $0x4cdfa020 // ld1  {v0.16b-v1.16b}, [x1], #32
	WORD $0x4c40a0b4 // ld1  {v20.16b-v21.16b}, [x5]

	WORD $0x6e341c04 // eor  v4.16b, v0.16b,  v20.16b
	WORD $0x6e351c25 // eor  v5.16b, v1.16b,  v21.16b

	// Store result
	WORD $0x4c9faca4 // st1  {v4.2d-v5.2d}, [x5], #32

	SUBS $32, R2
	BPL  loopXor

completeXor:
	RET
