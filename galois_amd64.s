//+build !noasm !appengine

// Copyright 2015, Klaus Post, see LICENSE for details.

// Based on http://www.snia.org/sites/default/files2/SDC2013/presentations/NewThinking/EthanMiller_Screaming_Fast_Galois_Field%20Arithmetic_SIMD%20Instructions.pdf
// and http://jerasure.org/jerasure/gf-complete/tree/master

// func galMulSSSE3Xor(low, high, in, out []byte)
TEXT ·galMulSSSE3Xor(SB), 7, $0
    MOVQ    low+0(FP),SI        // SI: &low
    MOVQ    high+24(FP),DX      // DX: &high
    MOVOU  (SI), X6             // X6 low
    MOVOU  (DX), X7             // X7: high
    MOVQ    $15, BX             // BX: low mask
    MOVQ    BX, X8
    PXOR    X5, X5    
    MOVQ    in+48(FP),SI        // R11: &in
    MOVQ    in_len+56(FP),R9    // R9: len(in)
    MOVQ    out+72(FP), DX      // DX: &out
    PSHUFB  X5, X8              // X8: lomask (unpacked)
    SHRQ    $4, R9              // len(in) / 16
    CMPQ    R9 ,$0
    JEQ     done_xor
loopback_xor:
    MOVOU  (SI),X0   // in[x]
    MOVOU  (DX),X4   // out[x]
    MOVOU   X0, X1   // in[x]
    MOVOU   X6, X2   // low copy
    MOVOU   X7, X3   // high copy
    PSRLQ   $4, X1   // X1: high input
    PAND    X8, X0   // X0: low input
    PAND    X8, X1   // X0: high input
    PSHUFB  X0, X2   // X2: mul low part
    PSHUFB  X1, X3   // X3: mul high part
    PXOR    X2, X3   // X3: Result
    PXOR    X4, X3   // X3: Result xor existing out
    MOVOU   X3, (DX) // Store
    ADDQ    $16, SI  // in+=16
    ADDQ    $16, DX  // out+=16 
    SUBQ    $1, R9
    JNZ     loopback_xor
done_xor:
    RET

// func galMulSSSE3(low, high, in, out []byte)
TEXT ·galMulSSSE3(SB), 7, $0
    MOVQ    low+0(FP),SI        // SI: &low
    MOVQ    high+24(FP),DX      // DX: &high
    MOVOU   (SI), X6            // X6 low
    MOVOU   (DX), X7            // X7: high
    MOVQ    $15, BX             // BX: low mask
    MOVQ    BX, X8
    PXOR    X5, X5    
    MOVQ    in+48(FP),SI        // R11: &in
    MOVQ    in_len+56(FP),R9    // R9: len(in)
    MOVQ    out+72(FP), DX      // DX: &out
    PSHUFB  X5, X8              // X8: lomask (unpacked)
    SHRQ    $4, R9              // len(in) / 16
    CMPQ    R9 ,$0
    JEQ     done
loopback:
    MOVOU  (SI),X0   // in[x]
    MOVOU   X0, X1   // in[x]
    MOVOU   X6, X2   // low copy
    MOVOU   X7, X3   // high copy
    PSRLQ   $4, X1   // X1: high input
    PAND    X8, X0   // X0: low input
    PAND    X8, X1   // X0: high input
    PSHUFB  X0, X2   // X2: mul low part
    PSHUFB  X1, X3   // X3: mul high part
    PXOR    X2, X3   // X3: Result
    MOVOU   X3, (DX) // Store
    ADDQ    $16, SI  // in+=16
    ADDQ    $16, DX  // out+=16 
    SUBQ    $1, R9
    JNZ     loopback
done:
    RET


