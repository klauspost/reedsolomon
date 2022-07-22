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

type gf16ctx struct {
	clrMask reg.VecVirtual
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
