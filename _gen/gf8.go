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
}

func genGF8() {
	var ctx gf8ctx
	// Ported from static void IFFT_DIT2
	// https://github.com/catid/leopard/blob/master/LeopardFF8.cpp#L599
	if pshufb {
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
	if pshufb {
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

	x := [8]int{}
	for skipMask := range x[:] {
		if !pshufb {
			break
		}
		{
			var suffix = "avx2_" + fmt.Sprint(skipMask)
			TEXT("ifftDIT48_"+suffix, attr.NOSPLIT, fmt.Sprintf("func(work [][]byte, dist int, t01, t23, t02 *[2*16]uint8)"))
			Pragma("noescape")
			var t01, t23, t02 table256
			// Load and expand tables

			if (skipMask & 1) == 0 {
				tablePtr := Load(Param("t01"), GP64())
				t01.Lo, t01.Hi = YMM(), YMM()
				// We need one register when loading all.
				if skipMask == 0 {
					t01.loadLo128 = &Mem{Base: tablePtr, Disp: 0}
				} else {
					VBROADCASTI128(Mem{Base: tablePtr, Disp: 0}, t01.Lo)
				}
				VBROADCASTI128(Mem{Base: tablePtr, Disp: 16}, t01.Hi)
			}
			if (skipMask & 2) == 0 {
				tablePtr := Load(Param("t23"), GP64())
				t23.Lo, t23.Hi = YMM(), YMM()
				VBROADCASTI128(Mem{Base: tablePtr, Disp: 0}, t23.Lo)
				VBROADCASTI128(Mem{Base: tablePtr, Disp: 16}, t23.Hi)
			}
			if (skipMask & 4) == 0 {
				tablePtr := Load(Param("t02"), GP64())
				t02.Lo, t02.Hi = YMM(), YMM()
				VBROADCASTI128(Mem{Base: tablePtr, Disp: 0}, t02.Lo)
				VBROADCASTI128(Mem{Base: tablePtr, Disp: 16}, t02.Hi)
			}
			dist := Load(Param("dist"), GP64())

			var work [4]reg.GPVirtual
			workTable := Load(Param("work").Base(), GP64()) // &work[0]
			bytes := GP64()
			MOVQ(Mem{Base: workTable, Disp: 8}, bytes)

			offset := GP64()
			XORQ(offset, offset)
			for i := range work {
				work[i] = GP64()
				// work[i] = &workTable[dist*i]
				MOVQ(Mem{Base: workTable, Index: offset, Scale: 1}, work[i])
				if i < len(work)-1 {
					ADDQ(dist, offset)
				}
			}

			// Generate mask
			ctx.clrMask = YMM()
			tmpMask := GP64()
			MOVQ(U32(15), tmpMask)
			MOVQ(tmpMask, ctx.clrMask.AsX())
			VPBROADCASTB(ctx.clrMask.AsX(), ctx.clrMask)

			Label("loop")
			var workReg [4]reg.VecVirtual
			var workReg2 [4]reg.VecVirtual

			workReg[0] = YMM()
			workReg[1] = YMM()
			workReg2[0] = YMM()
			workReg2[1] = YMM()

			VMOVDQU(Mem{Base: work[0], Disp: 0}, workReg[0])
			VMOVDQU(Mem{Base: work[1], Disp: 0}, workReg[1])
			VMOVDQU(Mem{Base: work[0], Disp: 32}, workReg2[0])
			VMOVDQU(Mem{Base: work[1], Disp: 32}, workReg2[1])

			// work1_reg = _mm256_xor_si256(work0_reg, work1_reg);
			VPXOR(workReg[1], workReg[0], workReg[1])
			VPXOR(workReg2[1], workReg2[0], workReg2[1])
			if (skipMask & 1) == 0 {
				t01.prepare()
				leo8MulAdd256(ctx, workReg[0], workReg[1], t01)
				leo8MulAdd256(ctx, workReg2[0], workReg2[1], t01)
			}

			workReg[2] = YMM()
			workReg[3] = YMM()
			workReg2[2] = YMM()
			workReg2[3] = YMM()
			VMOVDQU(Mem{Base: work[2], Disp: 0}, workReg[2])
			VMOVDQU(Mem{Base: work[3], Disp: 0}, workReg[3])
			VMOVDQU(Mem{Base: work[2], Disp: 32}, workReg2[2])
			VMOVDQU(Mem{Base: work[3], Disp: 32}, workReg2[3])

			//work3_reg = _mm256_xor_si256(work2_reg, work3_reg)
			VPXOR(workReg[2], workReg[3], workReg[3])
			VPXOR(workReg2[2], workReg2[3], workReg2[3])
			if (skipMask & 2) == 0 {
				leo8MulAdd256(ctx, workReg[2], workReg[3], t23)
				leo8MulAdd256(ctx, workReg2[2], workReg2[3], t23)
			}

			// Second layer:
			// work2_reg = _mm256_xor_si256(work0_reg, work2_reg);
			// work3_reg = _mm256_xor_si256(work1_reg, work3_reg);
			VPXOR(workReg[0], workReg[2], workReg[2])
			VPXOR(workReg[1], workReg[3], workReg[3])
			VPXOR(workReg2[0], workReg2[2], workReg2[2])
			VPXOR(workReg2[1], workReg2[3], workReg2[3])

			if (skipMask & 4) == 0 {
				leo8MulAdd256(ctx, workReg[0], workReg[2], t02)
				leo8MulAdd256(ctx, workReg[1], workReg[3], t02)
				leo8MulAdd256(ctx, workReg2[0], workReg2[2], t02)
				leo8MulAdd256(ctx, workReg2[1], workReg2[3], t02)
			}

			// Store + Next loop:
			for i := range work {
				VMOVDQU(workReg[i], Mem{Base: work[i], Disp: 0})
				VMOVDQU(workReg2[i], Mem{Base: work[i], Disp: 32})
				ADDQ(U8(64), work[i])
			}

			SUBQ(U8(64), bytes)
			JA(LabelRef("loop"))

			VZEROUPPER()
			RET()
		}
		{
			var suffix = "avx2_" + fmt.Sprint(skipMask)
			TEXT("fftDIT48_"+suffix, attr.NOSPLIT, fmt.Sprintf("func(work [][]byte, dist int, t01, t23, t02 *[2*16]uint8)"))
			Pragma("noescape")
			var t01, t23, t02 table256
			// Load and expand tables

			if (skipMask & 2) == 0 {
				tablePtr := Load(Param("t01"), GP64())
				t01.Lo, t01.Hi = YMM(), YMM()
				if skipMask == 0 {
					t01.loadLo128 = &Mem{Base: tablePtr, Disp: 0}
				} else {
					// We need additional registers
					VBROADCASTI128(Mem{Base: tablePtr, Disp: 0}, t01.Lo)
				}
				VBROADCASTI128(Mem{Base: tablePtr, Disp: 16}, t01.Hi)
			}
			if (skipMask & 4) == 0 {
				tablePtr := Load(Param("t23"), GP64())
				t23.Lo, t23.Hi = YMM(), YMM()
				if skipMask == 0 {
					t23.loadLo128 = &Mem{Base: tablePtr, Disp: 0}
				} else {
					VBROADCASTI128(Mem{Base: tablePtr, Disp: 0}, t23.Lo)
				}
				VBROADCASTI128(Mem{Base: tablePtr, Disp: 16}, t23.Hi)
			}
			if (skipMask & 1) == 0 {
				tablePtr := Load(Param("t02"), GP64())

				t02.Lo, t02.Hi = YMM(), YMM()
				VBROADCASTI128(Mem{Base: tablePtr, Disp: 0}, t02.Lo)
				VBROADCASTI128(Mem{Base: tablePtr, Disp: 16}, t02.Hi)
			}
			dist := Load(Param("dist"), GP64())

			var work [4]reg.GPVirtual
			workTable := Load(Param("work").Base(), GP64()) // &work[0]
			bytes := GP64()
			MOVQ(Mem{Base: workTable, Disp: 8}, bytes)

			offset := GP64()
			XORQ(offset, offset)
			for i := range work {
				work[i] = GP64()
				// work[i] = &workTable[dist*i]
				MOVQ(Mem{Base: workTable, Index: offset, Scale: 1}, work[i])
				if i < len(work)-1 {
					ADDQ(dist, offset)
				}
			}

			// Generate mask
			ctx.clrMask = YMM()
			tmpMask := GP64()
			MOVQ(U32(15), tmpMask)
			MOVQ(tmpMask, ctx.clrMask.AsX())
			VPBROADCASTB(ctx.clrMask.AsX(), ctx.clrMask)

			Label("loop")
			var workReg [4]reg.VecVirtual
			var workReg2 [4]reg.VecVirtual

			for i := range workReg {
				workReg[i] = YMM()
				workReg2[i] = YMM()
			}

			VMOVDQU(Mem{Base: work[0], Disp: 0}, workReg[0])
			VMOVDQU(Mem{Base: work[0], Disp: 32}, workReg2[0])
			VMOVDQU(Mem{Base: work[2], Disp: 0}, workReg[2])
			VMOVDQU(Mem{Base: work[2], Disp: 32}, workReg2[2])
			VMOVDQU(Mem{Base: work[1], Disp: 0}, workReg[1])
			VMOVDQU(Mem{Base: work[1], Disp: 32}, workReg2[1])
			VMOVDQU(Mem{Base: work[3], Disp: 0}, workReg[3])
			VMOVDQU(Mem{Base: work[3], Disp: 32}, workReg2[3])

			// work1_reg = _mm256_xor_si256(work0_reg, work1_reg);
			if (skipMask & 1) == 0 {
				leo8MulAdd256(ctx, workReg[0], workReg[2], t02)
				leo8MulAdd256(ctx, workReg2[0], workReg2[2], t02)

				leo8MulAdd256(ctx, workReg[1], workReg[3], t02)
				leo8MulAdd256(ctx, workReg2[1], workReg2[3], t02)
			}
			// work2_reg = _mm256_xor_si256(work0_reg, work2_reg);
			// work3_reg = _mm256_xor_si256(work1_reg, work3_reg);
			VPXOR(workReg[0], workReg[2], workReg[2])
			VPXOR(workReg[1], workReg[3], workReg[3])
			VPXOR(workReg2[0], workReg2[2], workReg2[2])
			VPXOR(workReg2[1], workReg2[3], workReg2[3])

			// Second layer:
			if (skipMask & 2) == 0 {
				t01.prepare()
				leo8MulAdd256(ctx, workReg[0], workReg[1], t01)
				leo8MulAdd256(ctx, workReg2[0], workReg2[1], t01)
			}
			//work1_reg = _mm256_xor_si256(work0_reg, work1_reg);
			VPXOR(workReg[1], workReg[0], workReg[1])
			VPXOR(workReg2[1], workReg2[0], workReg2[1])

			if (skipMask & 4) == 0 {
				t23.prepare()
				leo8MulAdd256(ctx, workReg[2], workReg[3], t23)
				leo8MulAdd256(ctx, workReg2[2], workReg2[3], t23)
			}
			// work3_reg = _mm256_xor_si256(work2_reg, work3_reg);
			VPXOR(workReg[2], workReg[3], workReg[3])
			VPXOR(workReg2[2], workReg2[3], workReg2[3])

			// Store + Next loop:
			for i := range work {
				VMOVDQU(workReg[i], Mem{Base: work[i], Disp: 0})
				VMOVDQU(workReg2[i], Mem{Base: work[i], Disp: 32})
				ADDQ(U8(64), work[i])
			}

			SUBQ(U8(64), bytes)
			JA(LabelRef("loop"))

			VZEROUPPER()
			RET()
		}
	}

	// GFNI
	for skipMask := range x[:] {
		{
			var suffix = "gfni_" + fmt.Sprint(skipMask)
			TEXT("ifftDIT48_"+suffix, attr.NOSPLIT, fmt.Sprintf("func(work [][]byte, dist int, t01, t23, t02 uint64)"))
			Pragma("noescape")
			var t01, t23, t02 table512 = ZMM(), ZMM(), ZMM()
			// Load and expand tables

			if (skipMask & 1) == 0 {
				tablePtr, _ := Param("t01").Resolve()
				VBROADCASTF32X2(tablePtr.Addr, t01)
			}
			if (skipMask & 2) == 0 {
				tablePtr, _ := Param("t23").Resolve()
				VBROADCASTF32X2(tablePtr.Addr, t23)
			}
			if (skipMask & 4) == 0 {
				tablePtr, _ := Param("t02").Resolve()
				VBROADCASTF32X2(tablePtr.Addr, t02)
			}
			dist := Load(Param("dist"), GP64())

			var work [4]reg.GPVirtual
			workTable := Load(Param("work").Base(), GP64()) // &work[0]
			bytes := GP64()
			MOVQ(Mem{Base: workTable, Disp: 8}, bytes)

			offset := GP64()
			XORQ(offset, offset)
			for i := range work {
				work[i] = GP64()
				// work[i] = &workTable[dist*i]
				MOVQ(Mem{Base: workTable, Index: offset, Scale: 1}, work[i])
				if i < len(work)-1 {
					ADDQ(dist, offset)
				}
			}

			Label("loop")
			var workReg [4]reg.VecVirtual
			for i := range workReg[:] {
				workReg[i] = ZMM()
				VMOVDQU64(Mem{Base: work[i], Disp: 0}, workReg[i])
			}

			// work1_reg = _mm256_xor_si256(work0_reg, work1_reg);
			VXORPD(workReg[1], workReg[0], workReg[1])
			if (skipMask & 1) == 0 {
				leo8MulAdd512(ctx, workReg[0], workReg[1], t01, nil)
			}

			//work3_reg = _mm256_xor_si256(work2_reg, work3_reg)
			VXORPD(workReg[2], workReg[3], workReg[3])
			if (skipMask & 2) == 0 {
				leo8MulAdd512(ctx, workReg[2], workReg[3], t23, workReg[0])
			} else {
				// Merged above when run...
				VXORPD(workReg[0], workReg[2], workReg[2])
			}

			// Second layer:
			// work2_reg = _mm256_xor_si256(work0_reg, work2_reg);
			// work3_reg = _mm256_xor_si256(work1_reg, work3_reg);
			VXORPD(workReg[1], workReg[3], workReg[3])

			if (skipMask & 4) == 0 {
				leo8MulAdd512(ctx, workReg[0], workReg[2], t02, nil)
				leo8MulAdd512(ctx, workReg[1], workReg[3], t02, nil)
			}

			// Store + Next loop:
			for i := range work {
				VMOVDQU64(workReg[i], Mem{Base: work[i], Disp: 0})
				ADDQ(U8(64), work[i])
			}

			SUBQ(U8(64), bytes)
			JA(LabelRef("loop"))

			VZEROUPPER()
			RET()
		}
		{
			var suffix = "gfni_" + fmt.Sprint(skipMask)
			TEXT("fftDIT48_"+suffix, attr.NOSPLIT, fmt.Sprintf("func(work [][]byte, dist int, t01, t23, t02 uint64)"))
			Pragma("noescape")
			var t01, t23, t02 table512 = ZMM(), ZMM(), ZMM()
			// Load and expand tables

			if (skipMask & 2) == 0 {
				tablePtr, _ := Param("t01").Resolve()
				VBROADCASTF32X2(tablePtr.Addr, t01)
			}
			if (skipMask & 4) == 0 {
				tablePtr, _ := Param("t23").Resolve()
				VBROADCASTF32X2(tablePtr.Addr, t23)
			}
			if (skipMask & 1) == 0 {
				tablePtr, _ := Param("t02").Resolve()
				VBROADCASTF32X2(tablePtr.Addr, t02)
			}
			dist := Load(Param("dist"), GP64())

			var work [4]reg.GPVirtual
			workTable := Load(Param("work").Base(), GP64()) // &work[0]
			bytes := GP64()
			MOVQ(Mem{Base: workTable, Disp: 8}, bytes)

			offset := GP64()
			XORQ(offset, offset)
			for i := range work {
				work[i] = GP64()
				// work[i] = &workTable[dist*i]
				MOVQ(Mem{Base: workTable, Index: offset, Scale: 1}, work[i])
				if i < len(work)-1 {
					ADDQ(dist, offset)
				}
			}

			Label("loop")
			var workReg [4]reg.VecVirtual

			for i := range workReg {
				workReg[i] = ZMM()
				VMOVDQU64(Mem{Base: work[i], Disp: 0}, workReg[i])
			}

			// work1_reg = _mm256_xor_si256(work0_reg, work1_reg);
			if (skipMask & 1) == 0 {
				leo8MulAdd512(ctx, workReg[0], workReg[2], t02, nil)
				leo8MulAdd512(ctx, workReg[1], workReg[3], t02, nil)
			}
			// work2_reg = _mm256_xor_si256(work0_reg, work2_reg);
			// work3_reg = _mm256_xor_si256(work1_reg, work3_reg);
			VXORPD(workReg[0], workReg[2], workReg[2])
			VXORPD(workReg[1], workReg[3], workReg[3])

			// Second layer:
			if (skipMask & 2) == 0 {
				leo8MulAdd512(ctx, workReg[0], workReg[1], t01, nil)
			}
			//work1_reg = _mm256_xor_si256(work0_reg, work1_reg);
			VXORPD(workReg[1], workReg[0], workReg[1])

			if (skipMask & 4) == 0 {
				leo8MulAdd512(ctx, workReg[2], workReg[3], t23, nil)
			}
			// work3_reg = _mm256_xor_si256(work2_reg, work3_reg);
			VXORPD(workReg[2], workReg[3], workReg[3])

			// Store + Next loop:
			for i := range work {
				VMOVDQU64(workReg[i], Mem{Base: work[i], Disp: 0})
				ADDQ(U8(64), work[i])
			}

			SUBQ(U8(64), bytes)
			JA(LabelRef("loop"))

			VZEROUPPER()
			RET()
		}
	}

}

// x updated, y preserved...
func leo8MulAdd256(ctx gf8ctx, x, y reg.VecVirtual, table table256) {
	Comment("LEO_MULADD_256")
	lo, hi := YMM(), YMM()

	VPAND(y, ctx.clrMask, lo)
	VPSRLQ(U8(4), y, hi)
	VPSHUFB(lo, table.Lo, lo)

	// Do high
	VPAND(hi, ctx.clrMask, hi)
	VPSHUFB(hi, table.Hi, hi)
	VPXOR3way(lo, hi, x)
}

// multiply y with table and xor result into x.
func leo8MulAdd512(ctx gf8ctx, x reg.VecVirtual, y reg.VecVirtual, table table512, z reg.VecVirtual) {
	Comment("LEO_MULADD_512")
	tmp := ZMM()
	VGF2P8AFFINEQB(U8(0), table, y, tmp)
	if z == nil {
		VXORPD(x, tmp, x)
	} else {
		VPTERNLOGD(U8(0x96), tmp, z, x)
	}
}
