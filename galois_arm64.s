//+build !noasm !appengine !gccgo

// Copyright 2015, Klaus Post, see LICENSE for details.
// Copyright 2017, Minio, Inc.

// func galMulNEON(low, high, in, out []byte)
TEXT ·galMulNEON(SB), 7, $0
	MOVD in_base+48(FP), R1
	MOVD in_len+56(FP), R2   // length of message
	MOVD out_base+72(FP), R5
	SUBS $32, R2
	BMI  complete

	MOVD low+0(FP), R10   // R10: &low
	MOVD high+24(FP), R11 // R11: &high
	VLD1 (R10), [V6.B16]
	VLD1 (R11), [V7.B16]

	//
	// Use an extra instruction below since `VDUP R3, V8.B16` generates assembler error
	// WORD $0x4e010c68 // dup v8.16b, w3
	//
	MOVD $0x0f, R3
	VMOV R3, V8.B[0]
	VDUP V8.B[0], V8.B16

loop:
	// Main loop
	VLD1.P 32(R1), [V0.B16, V1.B16]

	// Get low input and high input
	VUSHR $4, V0.B16, V10.B16
	VUSHR $4, V1.B16, V11.B16
	VAND  V8.B16, V0.B16, V0.B16
	VAND  V8.B16, V1.B16, V1.B16

	// Mul low part and mul high part
	VTBL V0.B16, [V6.B16], V4.B16
	VTBL V10.B16, [V7.B16], V5.B16
	VTBL V1.B16, [V6.B16], V14.B16
	VTBL V11.B16, [V7.B16], V15.B16

	// Combine results
	VEOR V5.B16, V4.B16, V4.B16
	VEOR V15.B16, V14.B16, V5.B16

	// Store result
	VST1.P [V4.D2, V5.D2], 32(R5)

	SUBS $32, R2
	BPL  loop

complete:
	RET

// func galMulXorNEON(low, high, in, out []byte)
TEXT ·galMulXorNEON(SB), 7, $0
	MOVD in_base+48(FP), R1
	MOVD in_len+56(FP), R2   // length of message
	MOVD out_base+72(FP), R5
	SUBS $32, R2
	BMI  completeXor

	MOVD low+0(FP), R10   // R10: &low
	MOVD high+24(FP), R11 // R11: &high
	VLD1 (R10), [V6.B16]
	VLD1 (R11), [V7.B16]

	//
	// Use an extra instruction below since `VDUP R3, V8.B16` generates assembler error
	// WORD $0x4e010c68 // dup v8.16b, w3
	//
	MOVD $0x0f, R3
	VMOV R3, V8.B[0]
	VDUP V8.B[0], V8.B16

loopXor:
	// Main loop
	VLD1.P 32(R1), [V0.B16, V1.B16]
	VLD1   (R5), [V20.B16, V21.B16]

	// Get low input and high input
	VUSHR $4, V0.B16, V10.B16
	VUSHR $4, V1.B16, V11.B16
	VAND  V8.B16, V0.B16, V0.B16
	VAND  V8.B16, V1.B16, V1.B16

	// Mul low part and mul high part
	VTBL V0.B16, [V6.B16], V4.B16
	VTBL V10.B16, [V7.B16], V5.B16
	VTBL V1.B16, [V6.B16], V14.B16
	VTBL V11.B16, [V7.B16], V15.B16

	// Combine results
	VEOR V5.B16, V4.B16, V4.B16
	VEOR V15.B16, V14.B16, V5.B16
	VEOR V20.B16, V4.B16, V4.B16
	VEOR V21.B16, V5.B16, V5.B16

	// Store result
	VST1.P [V4.D2, V5.D2], 32(R5)

	SUBS $32, R2
	BPL  loopXor

completeXor:
	RET

// func galXorNEON(in, out []byte)
TEXT ·galXorNEON(SB), 7, $0
	MOVD in_base+0(FP), R1
	MOVD in_len+8(FP), R2    // length of message
	MOVD out_base+24(FP), R5
	SUBS $32, R2
	BMI  completeXor

loopXor:
	// Main loop
	VLD1.P 32(R1), [V0.B16, V1.B16]
	VLD1   (R5), [V20.B16, V21.B16]

	VEOR V20.B16, V0.B16, V4.B16
	VEOR V21.B16, V1.B16, V5.B16

	// Store result
	VST1.P [V4.D2, V5.D2], 32(R5)

	SUBS $32, R2
	BPL  loopXor

completeXor:
	RET

