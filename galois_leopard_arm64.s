//go:build !noasm && !appengine && !gccgo && !nopshufb

// NEON kernels for the Leopard GF(2^16) butterflies. Data layout per 64-byte
// block: bytes 0-31 are the low bytes of 32 symbols, bytes 32-63 the high
// bytes. table is the 8x16-byte multiply256LUT entry for log_m: the first
// four tables produce the low output byte from the lo-lo, lo-hi, hi-lo and
// hi-hi input nibbles, the last four the high output byte.

#include "textflag.h"

// V0-V3: low-output tables; V4-V7: high-output tables (per input nibble
// lo-lo, lo-hi, hi-lo, hi-hi).
#define LOAD_TABLES(R) \
	VLD1.P 64(R), [V0.B16, V1.B16, V2.B16, V3.B16] \
	VLD1   (R), [V4.B16, V5.B16, V6.B16, V7.B16]

#define LOAD_MASK \
	MOVD $0x0f, R3      \
	VMOV R3, V8.B[0]    \
	VDUP V8.B[0], V8.B16

// (OUTLO, OUTHI) = (LO, HI) * log_m for 16 symbols; T1-T3 are scratch.
#define MUL16(LO, HI, OUTLO, OUTHI, T1, T2, T3) \
	VAND  V8.B16, LO.B16, T1.B16        \
	VUSHR $4, LO.B16, T2.B16            \
	VTBL  T1.B16, [V0.B16], OUTLO.B16   \
	VTBL  T1.B16, [V4.B16], OUTHI.B16   \
	VTBL  T2.B16, [V1.B16], T3.B16      \
	VEOR  T3.B16, OUTLO.B16, OUTLO.B16  \
	VTBL  T2.B16, [V5.B16], T3.B16      \
	VEOR  T3.B16, OUTHI.B16, OUTHI.B16  \
	VAND  V8.B16, HI.B16, T1.B16        \
	VUSHR $4, HI.B16, T2.B16            \
	VTBL  T1.B16, [V2.B16], T3.B16      \
	VEOR  T3.B16, OUTLO.B16, OUTLO.B16  \
	VTBL  T1.B16, [V6.B16], T3.B16      \
	VEOR  T3.B16, OUTHI.B16, OUTHI.B16  \
	VTBL  T2.B16, [V3.B16], T3.B16      \
	VEOR  T3.B16, OUTLO.B16, OUTLO.B16  \
	VTBL  T2.B16, [V7.B16], T3.B16      \
	VEOR  T3.B16, OUTHI.B16, OUTHI.B16

// Multiply the 32 symbols in V16-V19 (lo0, lo1, hi0, hi1) into V24-V27
// with the same layout (prodlo0, prodlo1, prodhi0, prodhi1).
#define MUL64 \
	MUL16(V16, V18, V24, V26, V28, V29, V30) \
	MUL16(V17, V19, V25, V27, V28, V29, V30)

// x (V20-V23) ^= product (V24-V27).
#define XOR_PRODUCT_INTO_X \
	VEOR V24.B16, V20.B16, V20.B16 \
	VEOR V25.B16, V21.B16, V21.B16 \
	VEOR V26.B16, V22.B16, V22.B16 \
	VEOR V27.B16, V23.B16, V23.B16

// func ifftDIT2NEON(x, y []byte, table *[128]uint8)
// y ^= x; x ^= y * log_m
TEXT ·ifftDIT2NEON(SB), NOSPLIT, $0-56
	MOVD table+48(FP), R10
	LOAD_TABLES(R10)
	LOAD_MASK
	MOVD x_base+0(FP), R1
	MOVD x_len+8(FP), R2
	MOVD y_base+24(FP), R5
	CBZ  R2, ifft2done

ifft2loop:
	VLD1 (R1), [V20.B16, V21.B16, V22.B16, V23.B16]
	VLD1 (R5), [V16.B16, V17.B16, V18.B16, V19.B16]
	VEOR V20.B16, V16.B16, V16.B16
	VEOR V21.B16, V17.B16, V17.B16
	VEOR V22.B16, V18.B16, V18.B16
	VEOR V23.B16, V19.B16, V19.B16
	VST1.P [V16.B16, V17.B16, V18.B16, V19.B16], 64(R5)
	MUL64
	XOR_PRODUCT_INTO_X
	VST1.P [V20.B16, V21.B16, V22.B16, V23.B16], 64(R1)
	SUBS $64, R2
	BGT  ifft2loop

ifft2done:
	RET

// func fftDIT2NEON(x, y []byte, table *[128]uint8)
// x ^= y * log_m; y ^= x
TEXT ·fftDIT2NEON(SB), NOSPLIT, $0-56
	MOVD table+48(FP), R10
	LOAD_TABLES(R10)
	LOAD_MASK
	MOVD x_base+0(FP), R1
	MOVD x_len+8(FP), R2
	MOVD y_base+24(FP), R5
	CBZ  R2, fft2done

fft2loop:
	VLD1 (R5), [V16.B16, V17.B16, V18.B16, V19.B16]
	VLD1 (R1), [V20.B16, V21.B16, V22.B16, V23.B16]
	MUL64
	XOR_PRODUCT_INTO_X
	VST1.P [V20.B16, V21.B16, V22.B16, V23.B16], 64(R1)
	VEOR V20.B16, V16.B16, V16.B16
	VEOR V21.B16, V17.B16, V17.B16
	VEOR V22.B16, V18.B16, V18.B16
	VEOR V23.B16, V19.B16, V19.B16
	VST1.P [V16.B16, V17.B16, V18.B16, V19.B16], 64(R5)
	SUBS $64, R2
	BGT  fft2loop

fft2done:
	RET

// func mulgf16NEON(x, y []byte, table *[128]uint8)
// x = y * log_m
TEXT ·mulgf16NEON(SB), NOSPLIT, $0-56
	MOVD table+48(FP), R10
	LOAD_TABLES(R10)
	LOAD_MASK
	MOVD x_base+0(FP), R1
	MOVD x_len+8(FP), R2
	MOVD y_base+24(FP), R5
	CBZ  R2, muldone

mulloop:
	VLD1.P 64(R5), [V16.B16, V17.B16, V18.B16, V19.B16]
	MUL64
	VST1.P [V24.B16, V25.B16, V26.B16, V27.B16], 64(R1)
	SUBS $64, R2
	BGT  mulloop

muldone:
	RET

// func mulgf16XorNEON(x, y []byte, table *[128]uint8)
// x ^= y * log_m
TEXT ·mulgf16XorNEON(SB), NOSPLIT, $0-56
	MOVD table+48(FP), R10
	LOAD_TABLES(R10)
	LOAD_MASK
	MOVD x_base+0(FP), R1
	MOVD x_len+8(FP), R2
	MOVD y_base+24(FP), R5
	CBZ  R2, mulxordone

mulxorloop:
	VLD1.P 64(R5), [V16.B16, V17.B16, V18.B16, V19.B16]
	VLD1 (R1), [V20.B16, V21.B16, V22.B16, V23.B16]
	MUL64
	XOR_PRODUCT_INTO_X
	VST1.P [V20.B16, V21.B16, V22.B16, V23.B16], 64(R1)
	SUBS $64, R2
	BGT  mulxorloop

mulxordone:
	RET
