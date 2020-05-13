//+build generate

//go:generate go run gen.go -out galois_gen_amd64.s -stubs galois_gen_amd64.go

package main

import (
	"fmt"

	. "github.com/mmcloughlin/avo/build"
	"github.com/mmcloughlin/avo/buildtags"
	. "github.com/mmcloughlin/avo/operand"
	"github.com/mmcloughlin/avo/reg"
)

func main() {
	Constraint(buildtags.Not("appengine").ToConstraint())
	Constraint(buildtags.Not("noasm").ToConstraint())
	Constraint(buildtags.Term("gc").ToConstraint())

	for i := 1; i <= 11; i++ {
		for j := 1; j <= 11; j++ {
			genMulAvx2(fmt.Sprintf("mulAvxTwoXor_%dx%d", i, j), i, j, true)
			genMulAvx2(fmt.Sprintf("mulAvxTwo_%dx%d", i, j), i, j, false)
		}
	}

	Generate()
}

// [6][16]byte, high [6][16]byte, in [3][]byte, out [2][]byte -> 15

func genMulAvx2(name string, inputs int, outputs int, xor bool) {
	total := inputs * outputs
	const perLoopBits = 5
	const perLoop = 1 << perLoopBits

	doc := []string{
		fmt.Sprintf("%s takes %d inputs and produces %d outputs.", name, inputs, outputs),
	}
	if !xor {
		doc = append(doc, "The output is initialized to 0.")
	}

	// Keep shuffle masks packed in registers
	var loadHalf bool
	// Load shuffle masks on every use.
	var loadNone bool
	// Use registers for destination registers.
	var regDst = true

	// lo, hi, 1 in, 1 out, 2 tmp, 1 mask
	est := total*2 + outputs + 5
	if outputs == 1 {
		// We don't need to keep a copy of the input if only 1 output.
		est -= 2
	}

	if est > 16 {
		return
		loadHalf = true
		// Only half the inputs
		est := total + outputs + 5
		doc = append(doc, "Half registers used.")

		//Commentf("Half registers estimated %d YMM used", est)
		if est > 16 {
			return
			loadNone = true
			loadHalf = false
			// We run out of GPS registers first, now.
			Comment("Loading no tables to registers")
			if inputs+outputs > 13 {
				regDst = false
			}
		} else {
			//Comment("Loading half tables to registers")
		}
	} else {
	}

	TEXT(name, 0, fmt.Sprintf("func(low, high [%d][16]byte, in [%d][]byte, out [%d][]byte)", total, inputs, outputs))

	Doc(doc...)
	Pragma("noescape")

	Commentf("Full registers estimated %d YMM used", est)
	Comment("Load all tables to registers")

	length := Load(Param("in").Index(0).Len(), GP64())
	SHRQ(U8(perLoopBits), length)
	TESTQ(length, length)
	JZ(LabelRef(name + "_end"))

	dst := make([]reg.VecVirtual, outputs)
	dstPtr := make([]reg.GPVirtual, outputs)
	for i := range dst {
		dst[i] = YMM()
		if !regDst {
			continue
		}
		ptr := GP64()
		p, err := Param("out").Index(i).Base().Resolve()
		if err != nil {
			panic(err)
		}
		MOVQ(p.Addr, ptr)
		dstPtr[i] = ptr
	}

	inLo := make([]reg.VecVirtual, total)
	inHi := make([]reg.VecVirtual, total)

	for i := range inLo {
		if loadNone {
			break
		}
		tableLo := YMM()
		MOVOU(Param("low").Index(i).MustAddr(), tableLo.AsX())
		if loadHalf {
			VINSERTI128(U8(1), Param("high").Index(i).MustAddr(), tableLo, tableLo)
			inLo[i] = tableLo
		} else {
			tableHi := YMM()
			MOVOU(Param("high").Index(i).MustAddr(), tableHi.AsX())
			VINSERTI128(U8(1), tableLo.AsX(), tableLo, tableLo)
			VINSERTI128(U8(1), tableHi.AsX(), tableHi, tableHi)
			inLo[i] = tableLo
			inHi[i] = tableHi
		}
	}

	inPtr := make([]reg.GPVirtual, inputs)
	for i := range inPtr {
		ptr := GP64()
		p, err := Param("in").Index(i).Base().Resolve()
		if err != nil {
			panic(err)
		}
		MOVQ(p.Addr, ptr)
		inPtr[i] = ptr
	}

	tmpMask := GP64()
	MOVQ(U32(15), tmpMask)
	lowMask := YMM()
	MOVQ(tmpMask, lowMask.AsX())
	VPBROADCASTB(lowMask.AsX(), lowMask)

	offset := GP64()
	XORQ(offset, offset)
	Label(name + "_loop")
	if xor {
		Commentf("Load %d outputs", outputs)
	} else {
		Commentf("Clear %d outputs", outputs)
	}
	for i := range dst {
		if xor {
			if regDst {
				VMOVDQU(Mem{Base: dstPtr[i], Index: offset, Scale: 1}, dst[i])
				continue
			}
			ptr := GP64()
			p, err := Param("out").Index(i).Base().Resolve()
			if err != nil {
				panic(err)
			}
			MOVQ(p.Addr, ptr)
			VMOVDQU(Mem{Base: ptr, Index: offset, Scale: 1}, dst[i])
		} else {
			VPXOR(dst[i], dst[i], dst[i])
		}
	}

	lookLow, lookHigh := YMM(), YMM()
	inLow, inHigh := YMM(), YMM()
	for i := range inPtr {
		Commentf("Load and process 32 bytes from input %d to %d outputs", i, outputs)
		VMOVDQU(Mem{Base: inPtr[i], Index: offset, Scale: 1}, inLow)
		VPSRLQ(U8(4), inLow, inHigh)
		VPAND(lowMask, inLow, inLow)
		VPAND(lowMask, inHigh, inHigh)
		for j := range dst {
			if loadNone {
				MOVOU(Param("low").Index(i*outputs+j).MustAddr(), lookLow.AsX())
				MOVOU(Param("high").Index(i*outputs+j).MustAddr(), lookHigh.AsX())
				VINSERTI128(U8(1), lookLow.AsX(), lookLow, lookLow)
				VINSERTI128(U8(1), lookHigh.AsX(), lookHigh, lookHigh)
				VPSHUFB(inLow, lookLow, lookLow)
				VPSHUFB(inHigh, lookHigh, lookHigh)
			} else if loadHalf {
				VPERM2I128(U8(1+(1<<4)), inLo[i*outputs+j], inLo[i*outputs+j], lookHigh)
				VINSERTI128(U8(1), inLo[i*outputs+j].AsX(), inLo[i*outputs+j], lookLow)
				VPSHUFB(inLow, lookLow, lookLow)
				VPSHUFB(inHigh, lookHigh, lookHigh)
			} else {
				VPSHUFB(inLow, inLo[i*outputs+j], lookLow)
				VPSHUFB(inHigh, inHi[i*outputs+j], lookHigh)
			}
			VPXOR(lookLow, lookHigh, lookLow)
			VPXOR(lookLow, dst[j], dst[j])
			if loadHalf && i != len(inPtr)-1 {
				// Break dependency
				VMOVDQA(lowMask, lookLow)
				VMOVDQA(lowMask, lookHigh)
			}
		}
	}
	Commentf("Store %d outputs", outputs)
	for i := range dst {
		if regDst {
			VMOVDQU(dst[i], Mem{Base: dstPtr[i], Index: offset, Scale: 1})
			continue
		}
		ptr := GP64()
		p, err := Param("out").Index(i).Base().Resolve()
		if err != nil {
			panic(err)
		}
		MOVQ(p.Addr, ptr)
		VMOVDQU(dst[i], Mem{Base: ptr, Index: offset, Scale: 1})
	}
	Comment("Prepare for next loop")
	ADDQ(U8(perLoop), offset)
	DECQ(length)
	JNZ(LabelRef(name + "_loop"))
	VZEROUPPER()

	Label(name + "_end")
	RET()
}
