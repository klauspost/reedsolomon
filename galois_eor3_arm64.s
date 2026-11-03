//go:build !noasm && !appengine && !gccgo && !nopshufb

// EOR3 (FEAT_SHA3) variants of the Leopard GF(2^16) NEON kernels. Same data
// and table layout as galois_leopard_arm64.s; the four nibble lookups per
// output byte are folded with two VEOR3 instead of four VEOR. Only called
// when options.useSHA3 is set.

#include "textflag.h"

#define LOAD_TABLES3(R) \
	VLD1.P 64(R), [V0.B16, V1.B16, V2.B16, V3.B16] \
	VLD1   (R), [V4.B16, V5.B16, V6.B16, V7.B16]

#define LOAD_MASK3 \
	MOVD $0x0f, R3      \
	VMOV R3, V8.B[0]    \
	VDUP V8.B[0], V8.B16

// (XLO, XHI) ^= (LO, HI) * log_m for 16 symbols. Scratch V9-V15, V31.
#define MULXOR16(LO, HI, XLO, XHI) \
	VAND  V8.B16, LO.B16, V9.B16   \
	VUSHR $4, LO.B16, V10.B16      \
	VAND  V8.B16, HI.B16, V11.B16  \
	VUSHR $4, HI.B16, V12.B16      \
	VTBL  V9.B16, [V0.B16], V13.B16  \
	VTBL  V10.B16, [V1.B16], V14.B16 \
	VTBL  V11.B16, [V2.B16], V15.B16 \
	VTBL  V12.B16, [V3.B16], V31.B16 \
	VEOR3 V13.B16, V14.B16, XLO.B16, XLO.B16 \
	VEOR3 V15.B16, V31.B16, XLO.B16, XLO.B16 \
	VTBL  V9.B16, [V4.B16], V13.B16  \
	VTBL  V10.B16, [V5.B16], V14.B16 \
	VTBL  V11.B16, [V6.B16], V15.B16 \
	VTBL  V12.B16, [V7.B16], V31.B16 \
	VEOR3 V13.B16, V14.B16, XHI.B16, XHI.B16 \
	VEOR3 V15.B16, V31.B16, XHI.B16, XHI.B16

// x (V20-V23) ^= y (V16-V19) * log_m.
#define MULXOR64 \
	MULXOR16(V16, V18, V20, V22) \
	MULXOR16(V17, V19, V21, V23)

// func ifftDIT2NEON3(x, y []byte, table *[128]uint8)
// y ^= x; x ^= y * log_m
TEXT ·ifftDIT2NEON3(SB), NOSPLIT, $0-56
	MOVD table+48(FP), R10
	LOAD_TABLES3(R10)
	LOAD_MASK3
	MOVD x_base+0(FP), R1
	MOVD x_len+8(FP), R2
	MOVD y_base+24(FP), R5
	CBZ  R2, ifft2e3done

ifft2e3loop:
	VLD1 (R1), [V20.B16, V21.B16, V22.B16, V23.B16]
	VLD1 (R5), [V16.B16, V17.B16, V18.B16, V19.B16]
	VEOR V20.B16, V16.B16, V16.B16
	VEOR V21.B16, V17.B16, V17.B16
	VEOR V22.B16, V18.B16, V18.B16
	VEOR V23.B16, V19.B16, V19.B16
	VST1.P [V16.B16, V17.B16, V18.B16, V19.B16], 64(R5)
	MULXOR64
	VST1.P [V20.B16, V21.B16, V22.B16, V23.B16], 64(R1)
	SUBS $64, R2
	BGT  ifft2e3loop

ifft2e3done:
	RET

// func fftDIT2NEON3(x, y []byte, table *[128]uint8)
// x ^= y * log_m; y ^= x
TEXT ·fftDIT2NEON3(SB), NOSPLIT, $0-56
	MOVD table+48(FP), R10
	LOAD_TABLES3(R10)
	LOAD_MASK3
	MOVD x_base+0(FP), R1
	MOVD x_len+8(FP), R2
	MOVD y_base+24(FP), R5
	CBZ  R2, fft2e3done

fft2e3loop:
	VLD1 (R5), [V16.B16, V17.B16, V18.B16, V19.B16]
	VLD1 (R1), [V20.B16, V21.B16, V22.B16, V23.B16]
	MULXOR64
	VST1.P [V20.B16, V21.B16, V22.B16, V23.B16], 64(R1)
	VEOR V20.B16, V16.B16, V16.B16
	VEOR V21.B16, V17.B16, V17.B16
	VEOR V22.B16, V18.B16, V18.B16
	VEOR V23.B16, V19.B16, V19.B16
	VST1.P [V16.B16, V17.B16, V18.B16, V19.B16], 64(R5)
	SUBS $64, R2
	BGT  fft2e3loop

fft2e3done:
	RET

// func mulgf16XorNEON3(x, y []byte, table *[128]uint8)
// x ^= y * log_m
TEXT ·mulgf16XorNEON3(SB), NOSPLIT, $0-56
	MOVD table+48(FP), R10
	LOAD_TABLES3(R10)
	LOAD_MASK3
	MOVD x_base+0(FP), R1
	MOVD x_len+8(FP), R2
	MOVD y_base+24(FP), R5
	CBZ  R2, mulxore3done

mulxore3loop:
	VLD1.P 64(R5), [V16.B16, V17.B16, V18.B16, V19.B16]
	VLD1 (R1), [V20.B16, V21.B16, V22.B16, V23.B16]
	MULXOR64
	VST1.P [V20.B16, V21.B16, V22.B16, V23.B16], 64(R1)
	SUBS $64, R2
	BGT  mulxore3loop

mulxore3done:
	RET

// (OUTLO, OUTHI) = (LO, HI) * log_m for 16 symbols. Scratch V9-V15.
#define MUL16_3(LO, HI, OUTLO, OUTHI) \
	VAND  V8.B16, LO.B16, V9.B16   \
	VUSHR $4, LO.B16, V10.B16      \
	VAND  V8.B16, HI.B16, V11.B16  \
	VUSHR $4, HI.B16, V12.B16      \
	VTBL  V9.B16, [V0.B16], V13.B16  \
	VTBL  V10.B16, [V1.B16], V14.B16 \
	VTBL  V11.B16, [V2.B16], V15.B16 \
	VEOR3 V13.B16, V14.B16, V15.B16, OUTLO.B16 \
	VTBL  V12.B16, [V3.B16], V13.B16 \
	VEOR  V13.B16, OUTLO.B16, OUTLO.B16 \
	VTBL  V9.B16, [V4.B16], V13.B16  \
	VTBL  V10.B16, [V5.B16], V14.B16 \
	VTBL  V11.B16, [V6.B16], V15.B16 \
	VEOR3 V13.B16, V14.B16, V15.B16, OUTHI.B16 \
	VTBL  V12.B16, [V7.B16], V13.B16 \
	VEOR  V13.B16, OUTHI.B16, OUTHI.B16

// func mulgf16NEON3(x, y []byte, table *[128]uint8)
// x = y * log_m
TEXT ·mulgf16NEON3(SB), NOSPLIT, $0-56
	MOVD table+48(FP), R10
	LOAD_TABLES3(R10)
	LOAD_MASK3
	MOVD x_base+0(FP), R1
	MOVD x_len+8(FP), R2
	MOVD y_base+24(FP), R5
	CBZ  R2, mul3done

mul3loop:
	VLD1.P 64(R5), [V16.B16, V17.B16, V18.B16, V19.B16]
	MUL16_3(V16, V18, V24, V26)
	MUL16_3(V17, V19, V25, V27)
	VST1.P [V24.B16, V25.B16, V26.B16, V27.B16], 64(R1)
	SUBS $64, R2
	BGT  mul3loop

mul3done:
	RET
