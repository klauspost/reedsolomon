//go:build generate
// +build generate

// Copyright 2022+, Klaus Post. See LICENSE for details.

package main

import (
	"fmt"

	"github.com/mmcloughlin/avo/attr"
	. "github.com/mmcloughlin/avo/build"
	. "github.com/mmcloughlin/avo/operand"
	"github.com/mmcloughlin/avo/reg"
)

type gf8ctx struct {
	clrMask    reg.VecVirtual
	clrMask128 reg.VecVirtual
	avx512     bool
}

func genGF8() {
	var ctx gf8ctx
	// Ported from static void IFFT_DIT2
	// https://github.com/catid/leopard/blob/master/LeopardFF8.cpp#L599
	if true {
		// Disabled: somehow wrong..
		TEXT("ifftDIT28_avx2", attr.NOSPLIT, fmt.Sprintf("func(x, y []byte, table *[2*16]uint8)"))
		Pragma("noescape")
		tablePtr := Load(Param("table"), GP64())
		var tables table256
		tables.Lo, tables.Hi = YMM(), YMM()
		// Load and expand tables
		VBROADCASTI128(Mem{Base: tablePtr, Disp: 0}, tables.Lo)
		VBROADCASTI128(Mem{Base: tablePtr, Disp: 16}, tables.Hi)

		bytes := Load(Param("x").Len(), GP64())
		x := Load(Param("x").Base(), GP64())
		y := Load(Param("y").Base(), GP64())
		// Generate mask
		ctx.clrMask = YMM()
		tmpMask := GP64()
		MOVQ(U32(15), tmpMask)
		MOVQ(tmpMask, ctx.clrMask.AsX())
		VPBROADCASTB(ctx.clrMask.AsX(), ctx.clrMask)

		x0, x1, y0, y1 := YMM(), YMM(), YMM(), YMM()
		Label("loop")
		VMOVDQU(Mem{Base: x, Disp: 0}, x0)
		VMOVDQU(Mem{Base: x, Disp: 32}, x1)
		VMOVDQU(Mem{Base: y, Disp: 0}, y0)
		VMOVDQU(Mem{Base: y, Disp: 32}, y1)

		// Update y and store
		VPXOR(y0, x0, y0)
		VPXOR(y1, x1, y1)
		VMOVDQU(y0, Mem{Base: y, Disp: 0})
		VMOVDQU(y1, Mem{Base: y, Disp: 32})

		// Update x and store
		leo8MulAdd256(ctx, x0, y0, tables)
		leo8MulAdd256(ctx, x1, y1, tables)
		VMOVDQU(x0, Mem{Base: x, Disp: 0})
		VMOVDQU(x1, Mem{Base: x, Disp: 32})

		// Move on
		ADDQ(U8(64), x)
		ADDQ(U8(64), y)
		SUBQ(U8(64), bytes)
		JA(LabelRef("loop"))

		VZEROUPPER()
		RET()
	}
	// https://github.com/catid/leopard/blob/master/LeopardFF8.cpp#L1323
	if true {
		// Disabled: somehow wrong..
		TEXT("fftDIT28_avx2", attr.NOSPLIT, fmt.Sprintf("func(x, y []byte, table *[2*16]uint8)"))
		Pragma("noescape")
		tablePtr := Load(Param("table"), GP64())
		var tables table256
		tables.Lo, tables.Hi = YMM(), YMM()
		// Load and expand tables
		VBROADCASTI128(Mem{Base: tablePtr, Disp: 0}, tables.Lo)
		VBROADCASTI128(Mem{Base: tablePtr, Disp: 16}, tables.Hi)

		bytes := Load(Param("x").Len(), GP64())
		x := Load(Param("x").Base(), GP64())
		y := Load(Param("y").Base(), GP64())
		// Generate mask
		ctx.clrMask = YMM()
		tmpMask := GP64()
		MOVQ(U32(15), tmpMask)
		MOVQ(tmpMask, ctx.clrMask.AsX())
		VPBROADCASTB(ctx.clrMask.AsX(), ctx.clrMask)

		x0, x1, y0, y1 := YMM(), YMM(), YMM(), YMM()
		Label("loop")
		VMOVDQU(Mem{Base: x, Disp: 0}, x0)
		VMOVDQU(Mem{Base: x, Disp: 32}, x1)
		VMOVDQU(Mem{Base: y, Disp: 0}, y0)
		VMOVDQU(Mem{Base: y, Disp: 32}, y1)

		leo8MulAdd256(ctx, x0, y0, tables)
		leo8MulAdd256(ctx, x1, y1, tables)
		VMOVDQU(x0, Mem{Base: x, Disp: 0})
		VMOVDQU(x1, Mem{Base: x, Disp: 32})

		VPXOR(y0, x0, y0)
		VPXOR(y1, x1, y1)
		VMOVDQU(y0, Mem{Base: y, Disp: 0})
		VMOVDQU(y1, Mem{Base: y, Disp: 32})
		ADDQ(U8(64), x)
		ADDQ(U8(64), y)
		SUBQ(U8(64), bytes)
		JA(LabelRef("loop"))

		VZEROUPPER()
		RET()
	}
}

// x updated, y preserved...
func leo8MulAdd256(ctx gf8ctx, x, y reg.VecVirtual, table table256) {
	Comment("LEO_MULADD_256")
	lo, hi := YMM(), YMM()

	VPAND(y, ctx.clrMask, lo)
	VPSRLQ(U8(4), y, hi)
	table.prepare()
	VPSHUFB(lo, table.Lo, lo)

	// Do high
	VPAND(hi, ctx.clrMask, hi)
	VPSHUFB(hi, table.Hi, hi)
	if ctx.avx512 {
		VPTERNLOGD(U8(0x96), lo, hi, x)
	} else {
		VPXOR3way(lo, hi, x)
	}
}
