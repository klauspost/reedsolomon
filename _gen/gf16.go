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

type table256 struct {
	Lo, Hi               Op
	loadLo128, loadHi128 *Mem
	loadLo256, loadHi256 *Mem
	useZmmLo, useZmmHi   *reg.VecPhysical
}

type table512 Op

func (t *table256) prepare() {
	t.prepareLo()
	t.prepareHi()
}

func (t *table256) prepareHi() {
	if t.loadHi128 != nil {
		t.Hi = YMM()
		// Load and expand tables
		VBROADCASTI128(*t.loadHi128, t.Hi)
	}
	if t.loadHi256 != nil {
		t.Hi = YMM()
		// Load and expand tables
		VMOVDQU(*t.loadHi256, t.Hi)
	}
	if t.useZmmHi != nil {
		r := *t.useZmmHi
		t.Hi = r.AsY()
	}
}

func (t *table256) prepareLo() {
	if t.loadLo128 != nil {
		t.Lo = YMM()
		// Load and expand tables
		VBROADCASTI128(*t.loadLo128, t.Lo)
	}
	if t.loadLo256 != nil {
		t.Lo = YMM()
		// Load and expand tables
		VMOVDQU(*t.loadLo256, t.Lo)
	}
	if t.useZmmLo != nil {
		r := *t.useZmmLo
		t.Lo = r.AsY()
	}
}

// table128 contains memory pointers to tables
type table128 struct {
	Lo, Hi Op
}

type gf16ctx struct {
	clrMask    reg.VecVirtual
	clrMask128 reg.VecVirtual
	avx512     bool
}

func genGF16() {
	var ctx gf16ctx
	// Ported from static void IFFT_DIT2
	// https://github.com/catid/leopard/blob/master/LeopardFF16.cpp#L629
	if pshufb {
		TEXT("ifftDIT2_avx2", attr.NOSPLIT, fmt.Sprintf("func(x, y []byte, table  *[8*16]uint8)"))
		Pragma("noescape")
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
	if pshufb {
		TEXT("fftDIT2_avx2", attr.NOSPLIT, fmt.Sprintf("func(x, y []byte, table  *[8*16]uint8)"))
		Pragma("noescape")
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

	if pshufb {
		TEXT("mulgf16_avx2", attr.NOSPLIT, fmt.Sprintf("func(x, y []byte, table  *[8*16]uint8)"))
		Pragma("noescape")
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
	for _, withDst := range []bool{false, true} {
		dstString := ""
		dstSuf := ""
		if withDst {
			dstString = "dst, "
			dstSuf = "dst_"
		}
		for _, avx512 := range []bool{true, false} {
			if !pshufb {
				continue
			}
			x := [8]int{}
			for skipMask := range x[:] {
				// AVX-512 only uses more registers for tables.
				var suffix = "avx2_" + dstSuf + fmt.Sprint(skipMask)
				if avx512 {
					suffix = "avx512_" + dstSuf + fmt.Sprint(skipMask)
				}
				ctx.avx512 = avx512
				extZMMs := []reg.VecPhysical{reg.Z16, reg.Z17, reg.Z18, reg.Z19, reg.Z20, reg.Z21, reg.Z22, reg.Z23, reg.Z24, reg.Z25, reg.Z26, reg.Z27, reg.Z28, reg.Z29, reg.Z30, reg.Z31}
				{
					TEXT("ifftDIT4_"+suffix, attr.NOSPLIT, fmt.Sprintf("func(%swork [][]byte, dist int, table01 *[8*16]uint8, table23 *[8*16]uint8, table02 *[8*16]uint8)", dstString))
					Pragma("noescape")
					Comment("dist must be multiplied by 24 (size of slice header)")

					// Unpack tables to stack. Slower.
					const unpackTables = false

					table01Ptr := Load(Param("table01"), GP64())
					table23Ptr := Load(Param("table23"), GP64())
					table02Ptr := Load(Param("table02"), GP64())

					// Prepare table pointers.
					table01 := [4]table256{}
					table23 := [4]table256{}
					table02 := [4]table256{}
					if avx512 {
						usedZmm := 0
						fill := func(t *[4]table256, ptr reg.Register) {
							for i := range table01 {
								t := &t[i]
								if len(extZMMs)-usedZmm >= 2 {
									tmpLo, tmpHi := YMM(), YMM()
									t.useZmmLo, t.useZmmHi = &extZMMs[usedZmm], &extZMMs[usedZmm+1]
									usedZmm += 2
									// Load and expand tables
									VBROADCASTI128(Mem{Base: ptr, Disp: i * 16}, tmpLo)
									VBROADCASTI128(Mem{Base: ptr, Disp: i*16 + 16*4}, tmpHi)
									VMOVAPS(tmpLo.AsZ(), *t.useZmmLo)
									VMOVAPS(tmpHi.AsZ(), *t.useZmmHi)
								} else {
									t.loadLo128 = &Mem{Base: ptr, Disp: i * 16}
									t.loadHi128 = &Mem{Base: ptr, Disp: i*16 + 16*4}
								}
							}
						}
						if (skipMask & 4) == 0 {
							fill(&table02, table02Ptr)
						}
						if (skipMask & 1) == 0 {
							fill(&table01, table01Ptr)
						}
						if (skipMask & 2) == 0 {
							fill(&table23, table23Ptr)
						}
					}
					for i := range table01 {
						if avx512 {
							continue
						}

						if unpackTables {
							toStack := func(m Mem) *Mem {
								stack := AllocLocal(32)
								y := YMM()
								VBROADCASTI128(m, y)
								VMOVDQU(y, stack)
								return &stack
							}

							table01[i].loadLo256 = toStack(Mem{Base: table01Ptr, Disp: i * 16})
							table23[i].loadLo256 = toStack(Mem{Base: table23Ptr, Disp: i * 16})
							table02[i].loadLo256 = toStack(Mem{Base: table02Ptr, Disp: i * 16})

							table01[i].loadHi256 = toStack(Mem{Base: table01Ptr, Disp: i*16 + 16*4})
							table23[i].loadHi256 = toStack(Mem{Base: table23Ptr, Disp: i*16 + 16*4})
							table02[i].loadHi256 = toStack(Mem{Base: table02Ptr, Disp: i*16 + 16*4})
						} else {
							table01[i].loadLo128 = &Mem{Base: table01Ptr, Disp: i * 16}
							table23[i].loadLo128 = &Mem{Base: table23Ptr, Disp: i * 16}
							table02[i].loadLo128 = &Mem{Base: table02Ptr, Disp: i * 16}

							table01[i].loadHi128 = &Mem{Base: table01Ptr, Disp: i*16 + 16*4}
							table23[i].loadHi128 = &Mem{Base: table23Ptr, Disp: i*16 + 16*4}
							table02[i].loadHi128 = &Mem{Base: table02Ptr, Disp: i*16 + 16*4}
						}
					}
					// Generate mask
					ctx.clrMask = YMM()
					tmpMask := GP64()
					MOVQ(U32(15), tmpMask)
					MOVQ(tmpMask, ctx.clrMask.AsX())
					VPBROADCASTB(ctx.clrMask.AsX(), ctx.clrMask)

					dist := Load(Param("dist"), GP64())

					// Pointers to each "work"
					var work [4]reg.GPVirtual
					workTable := Load(Param("work").Base(), GP64()) // &work[0]
					bytes := GP64()

					// Load length of work[0]
					MOVQ(Mem{Base: workTable, Disp: 8}, bytes)

					var dst [4]reg.GPVirtual
					dstTable := GP64()
					if withDst {
						Load(Param("dst").Base(), dstTable) // &dst[0]
					}
					for i := range work {
						work[i] = GP64()
						// work[i] = &workTable[dist*i]
						MOVQ(Mem{Base: workTable}, work[i])
						if i < len(work)-1 {
							ADDQ(dist, workTable)
						}
						if withDst {
							dst[i] = GP64()
							MOVQ(Mem{Base: dstTable}, dst[i])
							if i < len(work)-1 {
								ADDQ(dist, dstTable)
							}
						}
					}
					var workRegLo [4]reg.VecVirtual
					var workRegHi [4]reg.VecVirtual

					workRegLo[0], workRegHi[0] = YMM(), YMM()
					workRegLo[1], workRegHi[1] = YMM(), YMM()

					Label("loop_ifft4_" + suffix)
					VMOVDQU(Mem{Base: work[0], Disp: 0}, workRegLo[0])
					VMOVDQU(Mem{Base: work[0], Disp: 32}, workRegHi[0])
					VMOVDQU(Mem{Base: work[1], Disp: 0}, workRegLo[1])
					VMOVDQU(Mem{Base: work[1], Disp: 32}, workRegHi[1])

					// First layer:
					VPXOR(workRegLo[0], workRegLo[1], workRegLo[1])
					VPXOR(workRegHi[0], workRegHi[1], workRegHi[1])

					// Test bit 0
					if (skipMask & 1) == 0 {
						leoMulAdd256(ctx, workRegLo[0], workRegHi[0], workRegLo[1], workRegHi[1], table01)
					}
					workRegLo[2], workRegHi[2] = YMM(), YMM()
					workRegLo[3], workRegHi[3] = YMM(), YMM()
					VMOVDQU(Mem{Base: work[2], Disp: 0}, workRegLo[2])
					VMOVDQU(Mem{Base: work[2], Disp: 32}, workRegHi[2])
					VMOVDQU(Mem{Base: work[3], Disp: 0}, workRegLo[3])
					VMOVDQU(Mem{Base: work[3], Disp: 32}, workRegHi[3])

					VPXOR(workRegLo[2], workRegLo[3], workRegLo[3])
					VPXOR(workRegHi[2], workRegHi[3], workRegHi[3])

					// Test bit 1
					if (skipMask & 2) == 0 {
						leoMulAdd256(ctx, workRegLo[2], workRegHi[2], workRegLo[3], workRegHi[3], table23)
					}

					// Second layer:
					VPXOR(workRegLo[0], workRegLo[2], workRegLo[2])
					VPXOR(workRegHi[0], workRegHi[2], workRegHi[2])
					VPXOR(workRegLo[1], workRegLo[3], workRegLo[3])
					VPXOR(workRegHi[1], workRegHi[3], workRegHi[3])

					// Test bit 2
					if (skipMask & 4) == 0 {
						leoMulAdd256(ctx, workRegLo[0], workRegHi[0], workRegLo[2], workRegHi[2], table02)
						leoMulAdd256(ctx, workRegLo[1], workRegHi[1], workRegLo[3], workRegHi[3], table02)
					}

					// Store + Next loop:
					for i := range work {
						if withDst {
							VMOVDQU(workRegLo[i], Mem{Base: dst[i], Disp: 0})
							VMOVDQU(workRegHi[i], Mem{Base: dst[i], Disp: 32})
							ADDQ(U8(64), dst[i])
						} else {
							VMOVDQU(workRegLo[i], Mem{Base: work[i], Disp: 0})
							VMOVDQU(workRegHi[i], Mem{Base: work[i], Disp: 32})
						}
						ADDQ(U8(64), work[i])
					}

					SUBQ(U8(64), bytes)
					JNZ(LabelRef("loop_ifft4_" + suffix))

					VZEROUPPER()
					RET()
				}
				// fftDIT4 does not need a dst variant
				if !withDst {
					TEXT("fftDIT4_"+suffix, attr.NOSPLIT, fmt.Sprintf("func(work [][]byte, dist int, table01 *[8*16]uint8, table23 *[8*16]uint8, table02 *[8*16]uint8)"))
					Pragma("noescape")
					Comment("dist must be multiplied by 24 (size of slice header)")

					// Unpack tables to stack. Slower.
					const unpackTables = false

					table01Ptr := Load(Param("table01"), GP64())
					table23Ptr := Load(Param("table23"), GP64())
					table02Ptr := Load(Param("table02"), GP64())

					// Prepare table pointers.
					table01 := [4]table256{}
					table23 := [4]table256{}
					table02 := [4]table256{}
					if avx512 {
						usedZmm := 0
						fill := func(t *[4]table256, ptr reg.Register) {
							for i := range table01 {
								t := &t[i]
								if len(extZMMs)-usedZmm >= 2 {
									tmpLo, tmpHi := YMM(), YMM()
									t.useZmmLo, t.useZmmHi = &extZMMs[usedZmm], &extZMMs[usedZmm+1]
									usedZmm += 2
									// Load and expand tables
									VBROADCASTI128(Mem{Base: ptr, Disp: i * 16}, tmpLo)
									VBROADCASTI128(Mem{Base: ptr, Disp: i*16 + 16*4}, tmpHi)
									VMOVAPS(tmpLo.AsZ(), *t.useZmmLo)
									VMOVAPS(tmpHi.AsZ(), *t.useZmmHi)
								} else {
									t.loadLo128 = &Mem{Base: ptr, Disp: i * 16}
									t.loadHi128 = &Mem{Base: ptr, Disp: i*16 + 16*4}
								}
							}
						}
						if (skipMask & 1) == 0 {
							fill(&table02, table02Ptr)
						}
						if (skipMask & 2) == 0 {
							fill(&table01, table01Ptr)
						}
						if (skipMask & 4) == 0 {
							fill(&table23, table23Ptr)
						}
					}
					for i := range table01 {
						if avx512 {
							continue
						}
						if unpackTables {
							toStack := func(m Mem) *Mem {
								stack := AllocLocal(32)
								y := YMM()
								VBROADCASTI128(m, y)
								VMOVDQU(y, stack)
								return &stack
							}

							table01[i].loadLo256 = toStack(Mem{Base: table01Ptr, Disp: i * 16})
							table23[i].loadLo256 = toStack(Mem{Base: table23Ptr, Disp: i * 16})
							table02[i].loadLo256 = toStack(Mem{Base: table02Ptr, Disp: i * 16})

							table01[i].loadHi256 = toStack(Mem{Base: table01Ptr, Disp: i*16 + 16*4})
							table23[i].loadHi256 = toStack(Mem{Base: table23Ptr, Disp: i*16 + 16*4})
							table02[i].loadHi256 = toStack(Mem{Base: table02Ptr, Disp: i*16 + 16*4})
						} else {
							table01[i].loadLo128 = &Mem{Base: table01Ptr, Disp: i * 16}
							table23[i].loadLo128 = &Mem{Base: table23Ptr, Disp: i * 16}
							table02[i].loadLo128 = &Mem{Base: table02Ptr, Disp: i * 16}

							table01[i].loadHi128 = &Mem{Base: table01Ptr, Disp: i*16 + 16*4}
							table23[i].loadHi128 = &Mem{Base: table23Ptr, Disp: i*16 + 16*4}
							table02[i].loadHi128 = &Mem{Base: table02Ptr, Disp: i*16 + 16*4}
						}
					}
					// Generate mask
					ctx.clrMask = YMM()
					tmpMask := GP64()
					MOVQ(U32(15), tmpMask)
					MOVQ(tmpMask, ctx.clrMask.AsX())
					VPBROADCASTB(ctx.clrMask.AsX(), ctx.clrMask)

					dist := Load(Param("dist"), GP64())

					// Pointers to each "work"
					var work [4]reg.GPVirtual
					workTable := Load(Param("work").Base(), GP64()) // &work[0]
					bytes := GP64()

					// Load length of work[0]
					MOVQ(Mem{Base: workTable, Disp: 8}, bytes)

					for i := range work {
						work[i] = GP64()
						// work[i] = &workTable[dist*i]
						MOVQ(Mem{Base: workTable}, work[i])
						if i < len(work)-1 {
							ADDQ(dist, workTable)
						}
					}
					var workRegLo [4]reg.VecVirtual
					var workRegHi [4]reg.VecVirtual

					workRegLo[0], workRegHi[0] = YMM(), YMM()
					workRegLo[1], workRegHi[1] = YMM(), YMM()
					workRegLo[2], workRegHi[2] = YMM(), YMM()
					workRegLo[3], workRegHi[3] = YMM(), YMM()

					Label("loop")
					VMOVDQU(Mem{Base: work[0], Disp: 0}, workRegLo[0])
					VMOVDQU(Mem{Base: work[0], Disp: 32}, workRegHi[0])
					VMOVDQU(Mem{Base: work[2], Disp: 0}, workRegLo[2])
					VMOVDQU(Mem{Base: work[2], Disp: 32}, workRegHi[2])

					VMOVDQU(Mem{Base: work[1], Disp: 0}, workRegLo[1])
					VMOVDQU(Mem{Base: work[1], Disp: 32}, workRegHi[1])
					VMOVDQU(Mem{Base: work[3], Disp: 0}, workRegLo[3])
					VMOVDQU(Mem{Base: work[3], Disp: 32}, workRegHi[3])

					// First layer:

					// Test bit 0
					if (skipMask & 1) == 0 {
						leoMulAdd256(ctx, workRegLo[0], workRegHi[0], workRegLo[2], workRegHi[2], table02)
						leoMulAdd256(ctx, workRegLo[1], workRegHi[1], workRegLo[3], workRegHi[3], table02)
					}

					VPXOR(workRegLo[0], workRegLo[2], workRegLo[2])
					VPXOR(workRegHi[0], workRegHi[2], workRegHi[2])
					VPXOR(workRegLo[1], workRegLo[3], workRegLo[3])
					VPXOR(workRegHi[1], workRegHi[3], workRegHi[3])

					// Second layer:
					// Test bit 1
					if (skipMask & 2) == 0 {
						leoMulAdd256(ctx, workRegLo[0], workRegHi[0], workRegLo[1], workRegHi[1], table01)
					}
					VPXOR(workRegLo[0], workRegLo[1], workRegLo[1])
					VPXOR(workRegHi[0], workRegHi[1], workRegHi[1])

					// Store...
					for i := range work[:2] {
						VMOVDQU(workRegLo[i], Mem{Base: work[i], Disp: 0})
						VMOVDQU(workRegHi[i], Mem{Base: work[i], Disp: 32})
						ADDQ(U8(64), work[i])
					}

					// Test bit 2
					if (skipMask & 4) == 0 {
						leoMulAdd256(ctx, workRegLo[2], workRegHi[2], workRegLo[3], workRegHi[3], table23)
					}
					VPXOR(workRegLo[2], workRegLo[3], workRegLo[3])
					VPXOR(workRegHi[2], workRegHi[3], workRegHi[3])

					// Store + Next loop:
					for i := range work[2:] {
						i := i + 2
						VMOVDQU(workRegLo[i], Mem{Base: work[i], Disp: 0})
						VMOVDQU(workRegHi[i], Mem{Base: work[i], Disp: 32})
						ADDQ(U8(64), work[i])
					}

					SUBQ(U8(64), bytes)
					JNZ(LabelRef("loop"))

					VZEROUPPER()
					RET()
				}
			}
		}
	}

	// GFNI versions of ifftDIT4 and fftDIT4
	// Load tables on-demand in the loop to avoid running out of YMM registers
	for _, withDstGfni := range []bool{false, true} {
		dstStringGfni := ""
		dstSufGfni := ""
		if withDstGfni {
			dstStringGfni = "dst, "
			dstSufGfni = "dst_"
		}
		for _, avx512 := range []bool{false, true} {
			for skipMask := 0; skipMask < 8; skipMask++ {
				suffix := "gfni_" + dstSufGfni + fmt.Sprint(skipMask)
				if avx512 {
					// AVX512 just uses the extra registers to avoid the load+broadcast
					suffix = "gfni_avx512_" + dstSufGfni + fmt.Sprint(skipMask)
				}
				// ifftDIT4_gfni_*
				{
					TEXT("ifftDIT4_"+suffix, attr.NOSPLIT, fmt.Sprintf("func(%swork [][]byte, dist int, table01 *[4]uint64, table23 *[4]uint64, table02 *[4]uint64)", dstStringGfni))
					Pragma("noescape")
					Comment("dist must be multiplied by 24 (size of slice header)")

					table01Ptr := Load(Param("table01"), GP64())
					table23Ptr := Load(Param("table23"), GP64())
					table02Ptr := Load(Param("table02"), GP64())

					var table02 [4]reg.Register
					if skipMask&4 == 0 {
						for i := range table02 {
							table02[i] = YMM().AsY()
							VBROADCASTSD(Mem{Base: table02Ptr, Disp: i * 8}, table02[i])
						}
					}
					var table01 [4]reg.Register
					var table23 [4]reg.Register
					if avx512 {
						table01 = [4]reg.Register{reg.Z16.AsY(), reg.Z17.AsY(), reg.Z18.AsY(), reg.Z19.AsY()}
						table23 = [4]reg.Register{reg.Z20.AsY(), reg.Z21.AsY(), reg.Z22.AsY(), reg.Z23.AsY()}
						if skipMask&1 == 0 {
							for i := range table01 {
								VBROADCASTSD(Mem{Base: table01Ptr, Disp: i * 8}, table01[i])
							}
						}
						if skipMask&2 == 0 {
							for i := range table23 {
								VBROADCASTSD(Mem{Base: table23Ptr, Disp: i * 8}, table23[i])
							}
						}
					}

					dist := Load(Param("dist"), GP64())

					var work [4]reg.GPVirtual
					workTable := Load(Param("work").Base(), GP64())
					bytes := GP64()

					MOVQ(Mem{Base: workTable, Disp: 8}, bytes)

					var dst [4]reg.GPVirtual
					dstTable := GP64()
					if withDstGfni {
						Load(Param("dst").Base(), dstTable) // &dst[0]
					}
					for i := range work {
						work[i] = GP64()
						// work[i] = &workTable[dist*i]
						MOVQ(Mem{Base: workTable}, work[i])
						if i < len(work)-1 {
							ADDQ(dist, workTable)
						}
						if withDstGfni {
							dst[i] = GP64()
							MOVQ(Mem{Base: dstTable}, dst[i])
							if i < len(work)-1 {
								ADDQ(dist, dstTable)
							}
						}
					}

					var workRegLo [4]reg.VecVirtual
					var workRegHi [4]reg.VecVirtual

					workRegLo[0], workRegHi[0] = YMM(), YMM()
					workRegLo[1], workRegHi[1] = YMM(), YMM()

					Label("loop_ifft4_" + suffix)
					VMOVDQU(Mem{Base: work[0], Disp: 0}, workRegLo[0])
					VMOVDQU(Mem{Base: work[0], Disp: 32}, workRegHi[0])
					VMOVDQU(Mem{Base: work[1], Disp: 0}, workRegLo[1])
					VMOVDQU(Mem{Base: work[1], Disp: 32}, workRegHi[1])

					// First layer: y = x XOR y
					VPXOR(workRegLo[0], workRegLo[1], workRegLo[1])
					VPXOR(workRegHi[0], workRegHi[1], workRegHi[1])

					if (skipMask & 1) == 0 {
						if avx512 {
							leoMulAdd256_gfni_avx2(workRegLo[0], workRegHi[0], workRegLo[1], workRegHi[1], table01, true)
						} else {
							leoMulAdd256_gfni_mem(workRegLo[0], workRegHi[0], workRegLo[1], workRegHi[1], table01Ptr)
						}
					}

					workRegLo[2], workRegHi[2] = YMM(), YMM()
					workRegLo[3], workRegHi[3] = YMM(), YMM()
					VMOVDQU(Mem{Base: work[2], Disp: 0}, workRegLo[2])
					VMOVDQU(Mem{Base: work[2], Disp: 32}, workRegHi[2])
					VMOVDQU(Mem{Base: work[3], Disp: 0}, workRegLo[3])
					VMOVDQU(Mem{Base: work[3], Disp: 32}, workRegHi[3])

					VPXOR(workRegLo[2], workRegLo[3], workRegLo[3])
					VPXOR(workRegHi[2], workRegHi[3], workRegHi[3])

					if (skipMask & 2) == 0 {
						if avx512 {
							leoMulAdd256_gfni_avx2(workRegLo[2], workRegHi[2], workRegLo[3], workRegHi[3], table23, true)
						} else {
							leoMulAdd256_gfni_mem(workRegLo[2], workRegHi[2], workRegLo[3], workRegHi[3], table23Ptr)
						}
					}

					// Second layer
					VPXOR(workRegLo[0], workRegLo[2], workRegLo[2])
					VPXOR(workRegHi[0], workRegHi[2], workRegHi[2])
					VPXOR(workRegLo[1], workRegLo[3], workRegLo[3])
					VPXOR(workRegHi[1], workRegHi[3], workRegHi[3])

					if (skipMask & 4) == 0 {
						leoMulAdd256_gfni_avx2(workRegLo[0], workRegHi[0], workRegLo[2], workRegHi[2], table02, avx512)
						leoMulAdd256_gfni_avx2(workRegLo[1], workRegHi[1], workRegLo[3], workRegHi[3], table02, avx512)
					}

					// Store + Next loop
					for i := range work {
						if withDstGfni {
							VMOVDQU(workRegLo[i], Mem{Base: dst[i], Disp: 0})
							VMOVDQU(workRegHi[i], Mem{Base: dst[i], Disp: 32})
							ADDQ(U8(64), dst[i])
						} else {
							VMOVDQU(workRegLo[i], Mem{Base: work[i], Disp: 0})
							VMOVDQU(workRegHi[i], Mem{Base: work[i], Disp: 32})
						}
						ADDQ(U8(64), work[i])
					}

					SUBQ(U8(64), bytes)
					JNZ(LabelRef("loop_ifft4_" + suffix))

					VZEROUPPER()
					RET()
				}

				// fftDIT4_gfni_* does not need dst variant
				if !withDstGfni {
					// fftDIT4_gfni_*
					TEXT("fftDIT4_"+suffix, attr.NOSPLIT, fmt.Sprintf("func(work [][]byte, dist int, table01 *[4]uint64, table23 *[4]uint64, table02 *[4]uint64)"))
					Pragma("noescape")
					Comment("dist must be multiplied by 24 (size of slice header)")

					table01Ptr := Load(Param("table01"), GP64())
					table23Ptr := Load(Param("table23"), GP64())
					table02Ptr := Load(Param("table02"), GP64())

					var table02 [4]reg.Register
					if (skipMask & 1) == 0 {
						for i := range table02 {
							table02[i] = YMM().AsY()
							VBROADCASTSD(Mem{Base: table02Ptr, Disp: i * 8}, table02[i])
						}
					}
					var table01 [4]reg.Register
					var table23 [4]reg.Register
					if avx512 {
						table01 = [4]reg.Register{reg.Z16.AsY(), reg.Z17.AsY(), reg.Z18.AsY(), reg.Z19.AsY()}
						table23 = [4]reg.Register{reg.Z20.AsY(), reg.Z21.AsY(), reg.Z22.AsY(), reg.Z23.AsY()}

						if skipMask&2 == 0 {
							for i := range table01 {
								VBROADCASTSD(Mem{Base: table01Ptr, Disp: i * 8}, table01[i])
							}
						}
						if skipMask&4 == 0 {
							for i := range table23 {
								VBROADCASTSD(Mem{Base: table23Ptr, Disp: i * 8}, table23[i])
							}
						}
					}
					dist := Load(Param("dist"), GP64())

					var work [4]reg.GPVirtual
					workTable := Load(Param("work").Base(), GP64())
					bytes := GP64()

					MOVQ(Mem{Base: workTable, Disp: 8}, bytes)

					for i := range work {
						work[i] = GP64()
						// work[i] = &workTable[dist*i]
						MOVQ(Mem{Base: workTable}, work[i])
						if i < len(work)-1 {
							ADDQ(dist, workTable)
						}
					}

					var workRegLo [4]reg.VecVirtual
					var workRegHi [4]reg.VecVirtual

					workRegLo[0], workRegHi[0] = YMM(), YMM()
					workRegLo[1], workRegHi[1] = YMM(), YMM()
					workRegLo[2], workRegHi[2] = YMM(), YMM()
					workRegLo[3], workRegHi[3] = YMM(), YMM()

					Label("loop_fft4_" + suffix)
					VMOVDQU(Mem{Base: work[0], Disp: 0}, workRegLo[0])
					VMOVDQU(Mem{Base: work[0], Disp: 32}, workRegHi[0])
					VMOVDQU(Mem{Base: work[2], Disp: 0}, workRegLo[2])
					VMOVDQU(Mem{Base: work[2], Disp: 32}, workRegHi[2])

					VMOVDQU(Mem{Base: work[1], Disp: 0}, workRegLo[1])
					VMOVDQU(Mem{Base: work[1], Disp: 32}, workRegHi[1])
					VMOVDQU(Mem{Base: work[3], Disp: 0}, workRegLo[3])
					VMOVDQU(Mem{Base: work[3], Disp: 32}, workRegHi[3])

					// First layer
					if (skipMask & 1) == 0 {
						leoMulAdd256_gfni_avx2(workRegLo[0], workRegHi[0], workRegLo[2], workRegHi[2], table02, avx512)
						leoMulAdd256_gfni_avx2(workRegLo[1], workRegHi[1], workRegLo[3], workRegHi[3], table02, avx512)
					}

					VPXOR(workRegLo[0], workRegLo[2], workRegLo[2])
					VPXOR(workRegHi[0], workRegHi[2], workRegHi[2])
					VPXOR(workRegLo[1], workRegLo[3], workRegLo[3])
					VPXOR(workRegHi[1], workRegHi[3], workRegHi[3])

					// Second layer
					if (skipMask & 2) == 0 {
						if avx512 {
							leoMulAdd256_gfni_avx2(workRegLo[0], workRegHi[0], workRegLo[1], workRegHi[1], table01, true)
						} else {
							leoMulAdd256_gfni_mem(workRegLo[0], workRegHi[0], workRegLo[1], workRegHi[1], table01Ptr)
						}
					}
					VPXOR(workRegLo[0], workRegLo[1], workRegLo[1])
					VPXOR(workRegHi[0], workRegHi[1], workRegHi[1])

					// Store work[0] and work[1]
					for i := range work[:2] {
						VMOVDQU(workRegLo[i], Mem{Base: work[i], Disp: 0})
						VMOVDQU(workRegHi[i], Mem{Base: work[i], Disp: 32})
						ADDQ(U8(64), work[i])
					}

					if (skipMask & 4) == 0 {
						if avx512 {
							leoMulAdd256_gfni_avx2(workRegLo[2], workRegHi[2], workRegLo[3], workRegHi[3], table23, true)
						} else {
							leoMulAdd256_gfni_mem(workRegLo[2], workRegHi[2], workRegLo[3], workRegHi[3], table23Ptr)
						}
					}
					VPXOR(workRegLo[2], workRegLo[3], workRegLo[3])
					VPXOR(workRegHi[2], workRegHi[3], workRegHi[3])

					// Store work[2] and work[3] + Next loop
					for i := range work[2:] {
						i := i + 2
						VMOVDQU(workRegLo[i], Mem{Base: work[i], Disp: 0})
						VMOVDQU(workRegHi[i], Mem{Base: work[i], Disp: 32})
						ADDQ(U8(64), work[i])
					}

					SUBQ(U8(64), bytes)
					JNZ(LabelRef("loop_fft4_" + suffix))

					VZEROUPPER()
					RET()
				}
			}
		}
	}

	// SSSE3:
	ctx.avx512 = false
	if pshufb {
		TEXT("ifftDIT2_ssse3", attr.NOSPLIT, fmt.Sprintf("func(x, y []byte, table  *[8*16]uint8)"))
		Pragma("noescape")
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
	if pshufb {
		TEXT("fftDIT2_ssse3", attr.NOSPLIT, fmt.Sprintf("func(x, y []byte, table  *[8*16]uint8)"))
		Pragma("noescape")
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
	if pshufb {
		TEXT("mulgf16_ssse3", attr.NOSPLIT, fmt.Sprintf("func(x, y []byte, table  *[8*16]uint8)"))
		Pragma("noescape")
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

	// GFNI version of ifftDIT2
	// Data layout is SPLIT: x[0:32] = lo bytes, x[32:64] = hi bytes of 32 elements
	{
		TEXT("ifftDIT2_gfni", attr.NOSPLIT, fmt.Sprintf("func(x, y []byte, table *[4]uint64)"))
		Pragma("noescape")

		// Load 4 GFNI matrices and broadcast to YMM registers
		tablePtr := Load(Param("table"), GP64())
		var tables [4]reg.Register
		for i := range tables {
			tables[i] = YMM().AsY()
			VBROADCASTSD(Mem{Base: tablePtr, Disp: i * 8}, tables[i])
		}

		bytes := Load(Param("x").Len(), GP64())
		x := Load(Param("x").Base(), GP64())
		y := Load(Param("y").Base(), GP64())

		xLo, xHi, yLo, yHi := YMM(), YMM(), YMM(), YMM()

		Label("loop_gfni")
		VMOVDQU(Mem{Base: x, Disp: 0}, xLo)  // lo bytes of 32 elements
		VMOVDQU(Mem{Base: x, Disp: 32}, xHi) // hi bytes of 32 elements
		VMOVDQU(Mem{Base: y, Disp: 0}, yLo)  // lo bytes of 32 elements
		VMOVDQU(Mem{Base: y, Disp: 32}, yHi) // hi bytes of 32 elements

		// y = x XOR y
		VPXOR(yLo, xLo, yLo)
		VPXOR(yHi, xHi, yHi)
		VMOVDQU(yLo, Mem{Base: y, Disp: 0})
		VMOVDQU(yHi, Mem{Base: y, Disp: 32})

		// x = x + leoMul(y, table) using GFNI
		leoMulAdd256_gfni_avx2(xLo, xHi, yLo, yHi, tables, false)

		VMOVDQU(xLo, Mem{Base: x, Disp: 0})
		VMOVDQU(xHi, Mem{Base: x, Disp: 32})

		ADDQ(U8(64), x)
		ADDQ(U8(64), y)
		SUBQ(U8(64), bytes)
		JNZ(LabelRef("loop_gfni"))

		VZEROUPPER()
		RET()
	}

	// GFNI version of fftDIT2
	// Data layout is SPLIT: x[0:32] = lo bytes, x[32:64] = hi bytes of 32 elements
	{
		TEXT("fftDIT2_gfni", attr.NOSPLIT, fmt.Sprintf("func(x, y []byte, table *[4]uint64)"))
		Pragma("noescape")

		// Load 4 GFNI matrices and broadcast to YMM registers
		tablePtr := Load(Param("table"), GP64())
		var tables [4]reg.Register
		for i := range tables {
			tables[i] = YMM().AsY()
			VBROADCASTSD(Mem{Base: tablePtr, Disp: i * 8}, tables[i])
		}

		bytes := Load(Param("x").Len(), GP64())
		x := Load(Param("x").Base(), GP64())
		y := Load(Param("y").Base(), GP64())

		xLo, xHi, yLo, yHi := YMM(), YMM(), YMM(), YMM()

		Label("loop_fft_gfni")
		VMOVDQU(Mem{Base: x, Disp: 0}, xLo)  // lo bytes of 32 elements
		VMOVDQU(Mem{Base: x, Disp: 32}, xHi) // hi bytes of 32 elements
		VMOVDQU(Mem{Base: y, Disp: 0}, yLo)  // lo bytes of 32 elements
		VMOVDQU(Mem{Base: y, Disp: 32}, yHi) // hi bytes of 32 elements

		// x = x + leoMul(y, table) using GFNI
		leoMulAdd256_gfni_avx2(xLo, xHi, yLo, yHi, tables, false)

		VMOVDQU(xLo, Mem{Base: x, Disp: 0})
		VMOVDQU(xHi, Mem{Base: x, Disp: 32})

		// y = x XOR y (after x is updated)
		VPXOR(yLo, xLo, yLo)
		VPXOR(yHi, xHi, yHi)
		VMOVDQU(yLo, Mem{Base: y, Disp: 0})
		VMOVDQU(yHi, Mem{Base: y, Disp: 32})

		ADDQ(U8(64), x)
		ADDQ(U8(64), y)
		SUBQ(U8(64), bytes)
		JNZ(LabelRef("loop_fft_gfni"))

		VZEROUPPER()
		RET()
	}

	// GFNI version of mulgf16 (AVX2+GFNI)
	// x = y * table
	// Data layout is SPLIT: [0:32] = lo bytes, [32:64] = hi bytes of 32 elements
	{
		TEXT("mulgf16_gfni", attr.NOSPLIT, fmt.Sprintf("func(x, y []byte, table *[4]uint64)"))
		Pragma("noescape")

		tablePtr := Load(Param("table"), GP64())
		var tables [4]reg.VecVirtual
		for i := range tables {
			tables[i] = YMM()
			VBROADCASTSD(Mem{Base: tablePtr, Disp: i * 8}, tables[i])
		}

		bytes := Load(Param("x").Len(), GP64())
		x := Load(Param("x").Base(), GP64())
		y := Load(Param("y").Base(), GP64())

		yLo, yHi := YMM(), YMM()

		Label("loop_mulgf16_gfni")
		VMOVDQU(Mem{Base: y, Disp: 0}, yLo)
		VMOVDQU(Mem{Base: y, Disp: 32}, yHi)

		// prodLo = A*yLo XOR B*yHi, prodHi = C*yLo XOR D*yHi
		tmpA, tmpB, tmpC, tmpD := YMM(), YMM(), YMM(), YMM()
		VGF2P8AFFINEQB(U8(0), tables[0], yLo, tmpA)
		VGF2P8AFFINEQB(U8(0), tables[1], yHi, tmpB)
		VGF2P8AFFINEQB(U8(0), tables[2], yLo, tmpC)
		VGF2P8AFFINEQB(U8(0), tables[3], yHi, tmpD)
		VPXOR(tmpA, tmpB, tmpA)
		VPXOR(tmpC, tmpD, tmpC)

		VMOVDQU(tmpA, Mem{Base: x, Disp: 0})
		VMOVDQU(tmpC, Mem{Base: x, Disp: 32})

		ADDQ(U8(64), x)
		ADDQ(U8(64), y)
		SUBQ(U8(64), bytes)
		JNZ(LabelRef("loop_mulgf16_gfni"))

		VZEROUPPER()
		RET()
	}
}

// xLo, xHi updated, yLo, yHi preserved...
func leoMulAdd256(ctx gf16ctx, xLo, xHi, yLo, yHi reg.VecVirtual, table [4]table256) {
	// inlined:
	// prodLo, prodHi := leoMul256(ctx, yLo, yHi, table)
	lo := yLo
	hi := yHi
	data0, data1 := YMM(), YMM()
	VPSRLQ(U8(4), lo, data1)         // data1 = lo >> 4
	VPAND(ctx.clrMask, lo, data0)    // data0 = lo&0xf
	VPAND(ctx.clrMask, data1, data1) // data 1 = data1 &0xf
	prodLo, prodHi := YMM(), YMM()
	table[0].prepare()
	VPSHUFB(data0, table[0].Lo, prodLo)
	VPSHUFB(data0, table[0].Hi, prodHi)
	tmpLo, tmpHi := YMM(), YMM()
	table[1].prepare()
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
	table[2].prepare()
	VPSHUFB(data0, table[2].Lo, tmpLo)
	VPSHUFB(data0, table[2].Hi, tmpHi)
	VPXOR(prodLo, tmpLo, prodLo)
	VPXOR(prodHi, tmpHi, prodHi)
	table[3].prepare()
	VPSHUFB(data1, table[3].Lo, tmpLo)
	VPSHUFB(data1, table[3].Hi, tmpHi)
	if ctx.avx512 {
		VPTERNLOGD(U8(0x96), prodLo, tmpLo, xLo)
		VPTERNLOGD(U8(0x96), prodHi, tmpHi, xHi)
	} else {
		VPXOR3way(prodLo, tmpLo, xLo)
		VPXOR3way(prodHi, tmpHi, xHi)
	}
}

// leoMul256 lo, hi preserved...
func leoMul256(ctx gf16ctx, lo, hi reg.VecVirtual, table [4]table256) (prodLo, prodHi reg.VecVirtual) {
	data0, data1 := YMM(), YMM()
	VPSRLQ(U8(4), lo, data1)         // data1 = lo >> 4
	VPAND(ctx.clrMask, lo, data0)    // data0 = lo&0xf
	VPAND(ctx.clrMask, data1, data1) // data 1 = data1 &0xf
	prodLo, prodHi = YMM(), YMM()
	table[0].prepare()
	VPSHUFB(data0, table[0].Lo, prodLo)
	VPSHUFB(data0, table[0].Hi, prodHi)
	tmpLo, tmpHi := YMM(), YMM()
	table[1].prepare()
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
	table[2].prepare()
	VPSHUFB(data0, table[2].Lo, tmpLo)
	VPSHUFB(data0, table[2].Hi, tmpHi)
	VPXOR(prodLo, tmpLo, prodLo)
	VPXOR(prodHi, tmpHi, prodHi)
	table[3].prepare()
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

// leoMulAdd256_gfni_mem loads tables from memory and multiplies y, XORs into x.
// Loads tables on-demand to save YMM registers for DIT4 operations.
func leoMulAdd256_gfni_mem(xLo, xHi, yLo, yHi reg.VecVirtual, tablePtr reg.Register) {
	Comment("GFNI LEO_MULADD_256 (from memory)")

	tmpA, tmpB, tmpC, tmpD := YMM(), YMM(), YMM(), YMM()
	VPBROADCASTQ(Mem{Base: tablePtr, Disp: 0}, tmpA)
	VPBROADCASTQ(Mem{Base: tablePtr, Disp: 8}, tmpB)
	VPBROADCASTQ(Mem{Base: tablePtr, Disp: 16}, tmpC)
	VPBROADCASTQ(Mem{Base: tablePtr, Disp: 24}, tmpD)

	VGF2P8AFFINEQB(U8(0), tmpA, yLo, tmpA)
	VGF2P8AFFINEQB(U8(0), tmpB, yHi, tmpB)
	VGF2P8AFFINEQB(U8(0), tmpC, yLo, tmpC)
	VGF2P8AFFINEQB(U8(0), tmpD, yHi, tmpD)

	// XOR into x
	VPXOR3way(tmpA, tmpB, xLo)
	VPXOR3way(tmpC, tmpD, xHi)
}

// leoMulAdd256_gfni_avx2 loads tables from registers and multiplies y, XORs into x.
// Loads tables on-demand to save YMM registers for DIT4 operations.
func leoMulAdd256_gfni_avx2(xLo, xHi, yLo, yHi reg.VecVirtual, tables [4]reg.Register, tern bool) {
	Comment("GFNI LEO_MULADD_256 (from register)")

	// Apply GFNI transforms
	tmpA, tmpB, tmpC, tmpD := YMM(), YMM(), YMM(), YMM()
	VGF2P8AFFINEQB(U8(0), tables[0], yLo, tmpA) // A * yLo
	VGF2P8AFFINEQB(U8(0), tables[1], yHi, tmpB) // B * yHi
	VGF2P8AFFINEQB(U8(0), tables[2], yLo, tmpC) // C * yLo
	VGF2P8AFFINEQB(U8(0), tables[3], yHi, tmpD) // D * yHi
	// XOR into x
	if tern {
		VPTERNLOGD(U8(0x96), tmpA, tmpB, xLo)
		VPTERNLOGD(U8(0x96), tmpC, tmpD, xHi)
	} else {
		VPXOR3way(tmpA, tmpB, xLo)
		VPXOR3way(tmpC, tmpD, xHi)
	}
}

// leoMulAddZMM_gfni multiplies packed y by GFNI tables and XORs into packed x.
// x and y are in packed format: [lo_32bytes | hi_32bytes]
// tables: [0] = [A|C], [1] = [B|D] - combined matrices for efficiency
// Uses shuffle to duplicate lo/hi, then single GFNI per combined table.
func leoMulAddZMM_gfni(x, y reg.VecVirtual, tables [2]reg.VecVirtual) {
	Comment("GFNI LEO_MULADD packed ZMM (combined tables)")

	// Duplicate data halves: [lo|hi] -> [lo|lo] and [hi|hi]
	yLoLo, yHiHi := ZMM(), ZMM()
	VSHUFI64X2(U8(0x44), y, y, yLoLo) // [lo|lo]: select lanes 0,1,0,1
	VSHUFI64X2(U8(0xEE), y, y, yHiHi) // [hi|hi]: select lanes 2,3,2,3

	// Apply combined tables
	// tableAC = [A|C], tableBD = [B|D]
	// GFNI([A|C], [lo|lo]) = [A*lo | C*lo]
	// GFNI([B|D], [hi|hi]) = [B*hi | D*hi]
	tmp1, tmp2 := ZMM(), ZMM()
	VGF2P8AFFINEQB(U8(0), tables[0], yLoLo, tmp1) // [A*lo | C*lo]
	VGF2P8AFFINEQB(U8(0), tables[1], yHiHi, tmp2) // [B*hi | D*hi]

	// 3-way XOR: x ^= tmp1 ^ tmp2 using VPTERNLOG (0x96 = A XOR B XOR C)
	VPTERNLOGD(U8(0x96), tmp2, tmp1, x)
}
