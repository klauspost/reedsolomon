//go:build generate
// +build generate

package main

import (
	"fmt"

	"github.com/mmcloughlin/avo/attr"
	. "github.com/mmcloughlin/avo/build"
	. "github.com/mmcloughlin/avo/operand"
	"github.com/mmcloughlin/avo/reg"
)

type table256 struct {
	Lo, Hi reg.VecVirtual
}

// table128 contains memory pointers to tables
type table128 struct {
	Lo, Hi Op
}

type gf16ctx struct {
	clrMask    reg.VecVirtual
	clrMask128 reg.VecVirtual
}

func genGF16() {
	var ctx gf16ctx
	// Ported from static void IFFT_DIT2
	// https://github.com/catid/leopard/blob/master/LeopardFF16.cpp#L629
	{
		TEXT("ifftDIT2_avx2", attr.NOSPLIT, fmt.Sprintf("func(x, y []byte, table  *[8*16]uint8)"))
		tablePtr := Load(Param("table"), GP64())
		tables := [4]table256{}
		for i, t := range tables {
			t.Lo, t.Hi = YMM(), YMM()
			// Load and expand tables
			VBROADCASTI128(Mem{Base: tablePtr, Disp: i * 16}, t.Lo)
			VBROADCASTI128(Mem{Base: tablePtr, Disp: i*16 + 16*4}, t.Hi)
			tables[i] = t
		}
		bytes := Load(Param("x").Len(), GP64())
		x := Load(Param("x").Base(), GP64())
		y := Load(Param("y").Base(), GP64())
		// Generate mask
		ctx.clrMask = YMM()
		tmpMask := GP64()
		MOVQ(U32(15), tmpMask)
		MOVQ(tmpMask, ctx.clrMask.AsX())
		VPBROADCASTB(ctx.clrMask.AsX(), ctx.clrMask)

		xLo, xHi, yLo, yHi := YMM(), YMM(), YMM(), YMM()
		Label("loop")
		VMOVDQU(Mem{Base: x, Disp: 0}, xLo)
		VMOVDQU(Mem{Base: x, Disp: 32}, xHi)
		VMOVDQU(Mem{Base: y, Disp: 0}, yLo)
		VMOVDQU(Mem{Base: y, Disp: 32}, yHi)
		VPXOR(yLo, xLo, yLo)
		VPXOR(yHi, xHi, yHi)
		VMOVDQU(yLo, Mem{Base: y, Disp: 0})
		VMOVDQU(yHi, Mem{Base: y, Disp: 32})
		leoMulAdd256(ctx, xLo, xHi, yLo, yHi, tables)
		VMOVDQU(xLo, Mem{Base: x, Disp: 0})
		VMOVDQU(xHi, Mem{Base: x, Disp: 32})
		ADDQ(U8(64), x)
		ADDQ(U8(64), y)
		SUBQ(U8(64), bytes)
		JNZ(LabelRef("loop"))

		VZEROUPPER()
		RET()
	}
	{
		TEXT("fftDIT2_avx2", attr.NOSPLIT, fmt.Sprintf("func(x, y []byte, table  *[8*16]uint8)"))
		tablePtr := Load(Param("table"), GP64())
		tables := [4]table256{}
		for i, t := range tables {
			t.Lo, t.Hi = YMM(), YMM()
			// Load and expand tables
			VBROADCASTI128(Mem{Base: tablePtr, Disp: i * 16}, t.Lo)
			VBROADCASTI128(Mem{Base: tablePtr, Disp: i*16 + 16*4}, t.Hi)
			tables[i] = t
		}
		bytes := Load(Param("x").Len(), GP64())
		x := Load(Param("x").Base(), GP64())
		y := Load(Param("y").Base(), GP64())
		// Generate mask
		ctx.clrMask = YMM()
		tmpMask := GP64()
		MOVQ(U32(15), tmpMask)
		MOVQ(tmpMask, ctx.clrMask.AsX())
		VPBROADCASTB(ctx.clrMask.AsX(), ctx.clrMask)

		xLo, xHi, yLo, yHi := YMM(), YMM(), YMM(), YMM()
		Label("loop")
		VMOVDQU(Mem{Base: x, Disp: 0}, xLo)
		VMOVDQU(Mem{Base: x, Disp: 32}, xHi)
		VMOVDQU(Mem{Base: y, Disp: 0}, yLo)
		VMOVDQU(Mem{Base: y, Disp: 32}, yHi)

		leoMulAdd256(ctx, xLo, xHi, yLo, yHi, tables)
		VMOVDQU(xLo, Mem{Base: x, Disp: 0})
		VMOVDQU(xHi, Mem{Base: x, Disp: 32})

		// Reload, or we go beyond 16 regs..
		if true {
			yLo, yHi = YMM(), YMM()
			VMOVDQU(Mem{Base: y, Disp: 0}, yLo)
			VMOVDQU(Mem{Base: y, Disp: 32}, yHi)
		}

		VPXOR(yLo, xLo, yLo)
		VPXOR(yHi, xHi, yHi)
		VMOVDQU(yLo, Mem{Base: y, Disp: 0})
		VMOVDQU(yHi, Mem{Base: y, Disp: 32})
		ADDQ(U8(64), x)
		ADDQ(U8(64), y)
		SUBQ(U8(64), bytes)
		JNZ(LabelRef("loop"))

		VZEROUPPER()
		RET()
	}

	{
		TEXT("mulgf16_avx2", attr.NOSPLIT, fmt.Sprintf("func(x, y []byte, table  *[8*16]uint8)"))
		tablePtr := Load(Param("table"), GP64())
		tables := [4]table256{}
		for i, t := range tables {
			t.Lo, t.Hi = YMM(), YMM()
			// Load and expand tables
			VBROADCASTI128(Mem{Base: tablePtr, Disp: i * 16}, t.Lo)
			VBROADCASTI128(Mem{Base: tablePtr, Disp: i*16 + 16*4}, t.Hi)
			tables[i] = t
		}
		bytes := Load(Param("x").Len(), GP64())
		x := Load(Param("x").Base(), GP64())
		y := Load(Param("y").Base(), GP64())
		// Generate mask
		ctx.clrMask = YMM()
		tmpMask := GP64()
		MOVQ(U32(15), tmpMask)
		MOVQ(tmpMask, ctx.clrMask.AsX())
		VPBROADCASTB(ctx.clrMask.AsX(), ctx.clrMask)

		dataLo, dataHi := YMM(), YMM()
		Label("loop")
		VMOVDQU(Mem{Base: y, Disp: 0}, dataLo)
		VMOVDQU(Mem{Base: y, Disp: 32}, dataHi)

		prodLo, prodHi := leoMul256(ctx, dataLo, dataHi, tables)
		VMOVDQU(prodLo, Mem{Base: x, Disp: 0})
		VMOVDQU(prodHi, Mem{Base: x, Disp: 32})

		ADDQ(U8(64), x)
		ADDQ(U8(64), y)
		SUBQ(U8(64), bytes)
		JNZ(LabelRef("loop"))

		VZEROUPPER()
		RET()
	}
	// SSSE3:
	{
		TEXT("ifftDIT2_ssse3", attr.NOSPLIT, fmt.Sprintf("func(x, y []byte, table  *[8*16]uint8)"))
		tablePtr := Load(Param("table"), GP64())
		tables := [4]table128{}
		for i, t := range tables {
			// We almost have enough space for all tables.
			if i > 2 {
				t.Lo, t.Hi = Mem{Base: tablePtr, Disp: i * 16}, Mem{Base: tablePtr, Disp: i*16 + 16*4}
			} else {
				t.Lo, t.Hi = XMM(), XMM()
				MOVUPS(Mem{Base: tablePtr, Disp: i * 16}, t.Lo)
				MOVUPS(Mem{Base: tablePtr, Disp: i*16 + 16*4}, t.Hi)
			}
			tables[i] = t
		}
		// Generate mask
		zero := XMM()
		XORPS(zero, zero) // Zero, so bytes will be copied.
		fifteen, mask := GP64(), XMM()
		MOVQ(U32(0xf), fifteen)
		MOVQ(fifteen, mask)
		PSHUFB(zero, mask)
		ctx.clrMask128 = mask

		bytes := Load(Param("x").Len(), GP64())
		x := Load(Param("x").Base(), GP64())
		y := Load(Param("y").Base(), GP64())

		Label("loop")
		for i := 0; i < 2; i++ {
			xLo, xHi, yLo, yHi := XMM(), XMM(), XMM(), XMM()
			MOVUPS(Mem{Base: x, Disp: i*16 + 0}, xLo)
			MOVUPS(Mem{Base: x, Disp: i*16 + 32}, xHi)
			MOVUPS(Mem{Base: y, Disp: i*16 + 0}, yLo)
			MOVUPS(Mem{Base: y, Disp: i*16 + 32}, yHi)
			PXOR(xLo, yLo)
			PXOR(xHi, yHi)
			MOVUPS(yLo, Mem{Base: y, Disp: i*16 + 0})
			MOVUPS(yHi, Mem{Base: y, Disp: i*16 + 32})
			leoMulAdd128(ctx, xLo, xHi, yLo, yHi, tables)
			MOVUPS(xLo, Mem{Base: x, Disp: i*16 + 0})
			MOVUPS(xHi, Mem{Base: x, Disp: i*16 + 32})
		}
		ADDQ(U8(64), x)
		ADDQ(U8(64), y)
		SUBQ(U8(64), bytes)
		JNZ(LabelRef("loop"))

		RET()
	}
	{
		TEXT("fftDIT2_ssse3", attr.NOSPLIT, fmt.Sprintf("func(x, y []byte, table  *[8*16]uint8)"))
		tablePtr := Load(Param("table"), GP64())
		tables := [4]table128{}
		for i, t := range tables {
			// We almost have enough space for all tables.
			if i > 2 {
				t.Lo, t.Hi = Mem{Base: tablePtr, Disp: i * 16}, Mem{Base: tablePtr, Disp: i*16 + 16*4}
			} else {
				t.Lo, t.Hi = XMM(), XMM()
				MOVUPS(Mem{Base: tablePtr, Disp: i * 16}, t.Lo)
				MOVUPS(Mem{Base: tablePtr, Disp: i*16 + 16*4}, t.Hi)
			}
			tables[i] = t
		}
		// Generate mask
		zero := XMM()
		XORPS(zero, zero) // Zero, so bytes will be copied.
		fifteen, mask := GP64(), XMM()
		MOVQ(U32(0xf), fifteen)
		MOVQ(fifteen, mask)
		PSHUFB(zero, mask)
		ctx.clrMask128 = mask

		bytes := Load(Param("x").Len(), GP64())
		x := Load(Param("x").Base(), GP64())
		y := Load(Param("y").Base(), GP64())

		Label("loop")
		for i := 0; i < 2; i++ {
			xLo, xHi, yLo, yHi := XMM(), XMM(), XMM(), XMM()
			MOVUPS(Mem{Base: y, Disp: i*16 + 0}, yLo)
			MOVUPS(Mem{Base: y, Disp: i*16 + 32}, yHi)

			prodLo, prodHi := leoMul128(ctx, yLo, yHi, tables)

			MOVUPS(Mem{Base: x, Disp: i*16 + 0}, xLo)
			MOVUPS(Mem{Base: x, Disp: i*16 + 32}, xHi)
			PXOR(prodLo, xLo)
			PXOR(prodHi, xHi)
			MOVUPS(xLo, Mem{Base: x, Disp: i*16 + 0})
			MOVUPS(xHi, Mem{Base: x, Disp: i*16 + 32})

			PXOR(xLo, yLo)
			PXOR(xHi, yHi)
			MOVUPS(yLo, Mem{Base: y, Disp: i*16 + 0})
			MOVUPS(yHi, Mem{Base: y, Disp: i*16 + 32})

		}

		ADDQ(U8(64), x)
		ADDQ(U8(64), y)
		SUBQ(U8(64), bytes)
		JNZ(LabelRef("loop"))

		RET()
	}
	{
		TEXT("mulgf16_ssse3", attr.NOSPLIT, fmt.Sprintf("func(x, y []byte, table  *[8*16]uint8)"))
		tablePtr := Load(Param("table"), GP64())
		tables := [4]table128{}
		for i, t := range tables {
			// We have enough space for all tables.
			if i > 3 {
				t.Lo, t.Hi = Mem{Base: tablePtr, Disp: i * 16}, Mem{Base: tablePtr, Disp: i*16 + 16*4}
			} else {
				t.Lo, t.Hi = XMM(), XMM()
				MOVUPS(Mem{Base: tablePtr, Disp: i * 16}, t.Lo)
				MOVUPS(Mem{Base: tablePtr, Disp: i*16 + 16*4}, t.Hi)
			}
			tables[i] = t
		}
		bytes := Load(Param("x").Len(), GP64())
		x := Load(Param("x").Base(), GP64())
		y := Load(Param("y").Base(), GP64())
		// Generate mask
		zero := XMM()
		XORPS(zero, zero) // Zero, so bytes will be copied.
		fifteen, mask := GP64(), XMM()
		MOVQ(U32(0xf), fifteen)
		MOVQ(fifteen, mask)
		PSHUFB(zero, mask)
		ctx.clrMask128 = mask

		Label("loop")
		for i := 0; i < 2; i++ {
			dataLo, dataHi := XMM(), XMM()
			MOVUPS(Mem{Base: y, Disp: i*16 + 0}, dataLo)
			MOVUPS(Mem{Base: y, Disp: i*16 + 32}, dataHi)

			prodLo, prodHi := leoMul128(ctx, dataLo, dataHi, tables)
			MOVUPS(prodLo, Mem{Base: x, Disp: i*16 + 0})
			MOVUPS(prodHi, Mem{Base: x, Disp: i*16 + 32})
		}

		ADDQ(U8(64), x)
		ADDQ(U8(64), y)
		SUBQ(U8(64), bytes)
		JNZ(LabelRef("loop"))

		RET()
	}

}

// xLo, xHi updated
func leoMulAdd256(ctx gf16ctx, xLo, xHi, yLo, yHi reg.VecVirtual, table [4]table256) {
	prodLo, prodHi := leoMul256(ctx, yLo, yHi, table)
	VPXOR(xLo, prodLo, xLo)
	VPXOR(xHi, prodHi, xHi)
}

// leoMul256 lo, hi preserved...
func leoMul256(ctx gf16ctx, lo, hi reg.VecVirtual, table [4]table256) (prodLo, prodHi reg.VecVirtual) {
	data0, data1 := YMM(), YMM()
	VPSRLQ(U8(4), lo, data1)         // data1 = lo >> 4
	VPAND(ctx.clrMask, lo, data0)    // data0 = lo&0xf
	VPAND(ctx.clrMask, data1, data1) // data 1 = data1 &0xf
	prodLo, prodHi = YMM(), YMM()
	VPSHUFB(data0, table[0].Lo, prodLo)
	VPSHUFB(data0, table[0].Hi, prodHi)
	tmpLo, tmpHi := YMM(), YMM()
	VPSHUFB(data1, table[1].Lo, tmpLo)
	VPSHUFB(data1, table[1].Hi, tmpHi)
	VPXOR(prodLo, tmpLo, prodLo)
	VPXOR(prodHi, tmpHi, prodHi)

	// Now process high
	data0, data1 = YMM(), YMM() // Realloc to break dep
	VPAND(hi, ctx.clrMask, data0)
	VPSRLQ(U8(4), hi, data1)
	VPAND(ctx.clrMask, data1, data1)

	tmpLo, tmpHi = YMM(), YMM() // Realloc to break dep
	VPSHUFB(data0, table[2].Lo, tmpLo)
	VPSHUFB(data0, table[2].Hi, tmpHi)
	VPXOR(prodLo, tmpLo, prodLo)
	VPXOR(prodHi, tmpHi, prodHi)
	VPSHUFB(data1, table[3].Lo, tmpLo)
	VPSHUFB(data1, table[3].Hi, tmpHi)
	VPXOR(prodLo, tmpLo, prodLo)
	VPXOR(prodHi, tmpHi, prodHi)
	return
}

func leoMulAdd128(ctx gf16ctx, xLo, xHi, yLo, yHi reg.VecVirtual, table [4]table128) {
	prodLo, prodHi := leoMul128(ctx, yLo, yHi, table)
	PXOR(prodLo, xLo)
	PXOR(prodHi, xHi)
}

// leoMul128 lo, hi preseved (but likely will take extra regs to reuse)
func leoMul128(ctx gf16ctx, lo, hi reg.VecVirtual, table [4]table128) (prodLo, prodHi reg.VecVirtual) {
	data0, data1 := XMM(), XMM()
	MOVAPS(lo, data1)
	PSRLQ(U8(4), data1) // data1 = lo >> 4
	MOVAPS(lo, data0)
	PAND(ctx.clrMask128, data0) // data0 = lo&0xf
	PAND(ctx.clrMask128, data1) // data 1 = data1 &0xf
	prodLo, prodHi = XMM(), XMM()
	MOVUPS(table[0].Lo, prodLo)
	MOVUPS(table[0].Hi, prodHi)
	PSHUFB(data0, prodLo)
	PSHUFB(data0, prodHi)
	tmpLo, tmpHi := XMM(), XMM()
	MOVUPS(table[1].Lo, tmpLo)
	MOVUPS(table[1].Hi, tmpHi)
	PSHUFB(data1, tmpLo)
	PSHUFB(data1, tmpHi)
	PXOR(tmpLo, prodLo)
	PXOR(tmpHi, prodHi)

	// Now process high
	data0, data1 = XMM(), XMM() // Realloc to break dep
	MOVAPS(hi, data0)
	MOVAPS(hi, data1)
	PAND(ctx.clrMask128, data0)
	PSRLQ(U8(4), data1)
	PAND(ctx.clrMask128, data1)

	tmpLo, tmpHi = XMM(), XMM() // Realloc to break dep
	MOVUPS(table[2].Lo, tmpLo)
	MOVUPS(table[2].Hi, tmpHi)
	PSHUFB(data0, tmpLo)
	PSHUFB(data0, tmpHi)
	PXOR(tmpLo, prodLo)
	PXOR(tmpHi, prodHi)
	MOVUPS(table[3].Lo, tmpLo)
	MOVUPS(table[3].Hi, tmpHi)
	PSHUFB(data1, tmpLo)
	PSHUFB(data1, tmpHi)
	PXOR(tmpLo, prodLo)
	PXOR(tmpHi, prodHi)
	return
}
