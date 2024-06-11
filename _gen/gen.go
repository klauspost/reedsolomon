//go:build generate

// Copyright 2022+, Klaus Post. See LICENSE for details.

//go:generate go run -tags=generate,nopshufb . -out ../galois_gen_nopshufb_amd64.s -stubs ../galois_gen_nopshufb_amd64.go -pkg=reedsolomon
//go:generate go fmt ../galois_gen_switch_nopshufb_amd64.go
//go:generate go fmt ../galois_gen_nopshufb_amd64.go
//go:generate go run cleanup.go ../galois_gen_nopshufb_amd64.s

//go:generate go run -tags=generate . -out ../galois_gen_amd64.s -stubs ../galois_gen_amd64.go -pkg=reedsolomon
//go:generate go fmt ../galois_gen_switch_amd64.go
//go:generate go fmt ../galois_gen_amd64.go
//go:generate go run cleanup.go ../galois_gen_amd64.s

package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/mmcloughlin/avo/attr"
	. "github.com/mmcloughlin/avo/build"
	"github.com/mmcloughlin/avo/buildtags"
	. "github.com/mmcloughlin/avo/operand"
	"github.com/mmcloughlin/avo/reg"
)

// Technically we can do slightly bigger, but we stay reasonable.
const inputMax = 10
const outputMax = 10

var switchDefs [inputMax][outputMax]string
var switchDefsX [inputMax][outputMax]string

var switchDefs512 [inputMax][outputMax]string
var switchDefsX512 [inputMax][outputMax]string

var switchDefsAvxGFNI [inputMax][outputMax]string
var switchDefsXAvxGFNI [inputMax][outputMax]string

// Prefetch offsets, set to 0 to disable.
// Disabled since they appear to be consistently slower.
const prefetchSrc = 0
const prefetchDst = 0

func main() {
	Constraint(buildtags.Not("appengine").ToConstraint())
	Constraint(buildtags.Not("noasm").ToConstraint())
	Constraint(buildtags.Not("nogen").ToConstraint())
	if pshufb {
		Constraint(buildtags.Not("nopshufb").ToConstraint())
	} else {
		Constraint(buildtags.Opt("nopshufb").ToConstraint())
	}
	Constraint(buildtags.Term("gc").ToConstraint())

	TEXT("_dummy_", 0, "func()")
	Comment("#ifdef GOAMD64_v4")
	Comment("#define XOR3WAY(ignore, a, b, dst)\\")
	Comment("@\tVPTERNLOGD $0x96, a, b, dst")
	Comment("#else")
	Comment("#define XOR3WAY(ignore, a, b, dst)\\")
	Comment("@\tVPXOR a, dst, dst\\")
	Comment("@\tVPXOR b, dst, dst")
	Comment("#endif")
	RET()

	genXor()

	for i := 1; i <= inputMax; i++ {
		for j := 1; j <= outputMax; j++ {
			if pshufb {
				genMulAvx2(fmt.Sprintf("mulAvxTwo_%dx%d", i, j), i, j, false)
				genMulAvx2Sixty64(fmt.Sprintf("mulAvxTwo_%dx%d_64", i, j), i, j, false)
			}
			genMulAvx512GFNI(fmt.Sprintf("mulGFNI_%dx%d_64", i, j), i, j, false)
			genMulAvxGFNI(fmt.Sprintf("mulAvxGFNI_%dx%d", i, j), i, j, false)
			genMulAvx512GFNI(fmt.Sprintf("mulGFNI_%dx%d_64Xor", i, j), i, j, true)
			genMulAvxGFNI(fmt.Sprintf("mulAvxGFNI_%dx%dXor", i, j), i, j, true)

			if pshufb {
				genMulAvx2(fmt.Sprintf("mulAvxTwo_%dx%dXor", i, j), i, j, true)
				genMulAvx2Sixty64(fmt.Sprintf("mulAvxTwo_%dx%d_64Xor", i, j), i, j, true)
			}
		}
	}

	genSwitch()
	genGF16()
	genGF8()

	if pshufb {
		genArmSve()
		genArmNeon()
	}
	Generate()
}

func genSwitch() {
	name := "../galois_gen_switch_amd64.go"
	tag := "// +build !nopshufb\n"
	if !pshufb {
		name = "../galois_gen_switch_nopshufb_amd64.go"
		tag = "// +build nopshufb\n"
	}
	f, err := os.Create(name)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	w.WriteString(`// Code generated by command: go generate ` + os.Getenv("GOFILE") + `. DO NOT EDIT.

// +build !appengine
// +build !noasm
// +build gc
// +build !nogen
` + tag + `

package reedsolomon

import (
	"fmt"
)

`)
	avx2funcs := string(`	fAvx2       = galMulSlicesAvx2
	fAvx2Xor    = galMulSlicesAvx2Xor
`)

	hasCodeGenImpl := string(`	return &fAvx2, &fAvx2Xor, codeGen && pshufb && r.o.useAVX2 &&
	byteCount >= codeGenMinSize && inputs+outputs >= codeGenMinShards &&
	inputs <= codeGenMaxInputs && outputs <= codeGenMaxOutputs`)

	if !pshufb {
		avx2funcs = ""
		hasCodeGenImpl = `	return nil, nil, false // no code generation for generic case (only GFNI cases)	`
	}

	w.WriteString(fmt.Sprintf(`const (
codeGen              = true
codeGenMaxGoroutines = 8
codeGenMaxInputs     = %d
codeGenMaxOutputs    = %d
minCodeGenSize       = 64
)

var (
%s	fGFNI       = galMulSlicesGFNI
	fGFNIXor    = galMulSlicesGFNIXor
	fAvxGFNI    = galMulSlicesAvxGFNI
	fAvxGFNIXor = galMulSlicesAvxGFNIXor
)

func (r *reedSolomon) hasCodeGen(byteCount int, inputs, outputs int) (_, _ *func(matrix []byte, in, out [][]byte, start, stop int) int, ok bool) {
%s
}

func (r *reedSolomon) canGFNI(byteCount int, inputs, outputs int) (_, _ *func(matrix []uint64, in, out [][]byte, start, stop int) int, ok bool) {
	if r.o.useAvx512GFNI {
		return &fGFNI, &fGFNIXor, codeGen &&
			byteCount >= codeGenMinSize && inputs+outputs >= codeGenMinShards &&
			inputs <= codeGenMaxInputs && outputs <= codeGenMaxOutputs
	}
	return &fAvxGFNI, &fAvxGFNIXor, codeGen && r.o.useAvxGNFI &&
		byteCount >= codeGenMinSize && inputs+outputs >= codeGenMinShards &&
		inputs <= codeGenMaxInputs && outputs <= codeGenMaxOutputs
}`, inputMax, outputMax, avx2funcs, hasCodeGenImpl))

	if !pshufb {
		w.WriteString("\n\nfunc galMulSlicesAvx2(matrix []byte, in, out [][]byte, start, stop int) int { panic(`no pshufb`)}\n")
		w.WriteString("func galMulSlicesAvx2Xor(matrix []byte, in, out [][]byte, start, stop int) int { panic(`no pshufb`)}\n")
	}

	if pshufb {
		w.WriteString(`

func galMulSlicesAvx2(matrix []byte, in, out [][]byte, start, stop int) int {
	 n := stop-start

`)

		w.WriteString(`switch len(in) {
`)
		for in, defs := range switchDefs[:] {
			w.WriteString(fmt.Sprintf("		case %d:\n			switch len(out) {\n", in+1))
			for out, def := range defs[:] {
				w.WriteString(fmt.Sprintf("				case %d:\n", out+1))
				w.WriteString(def)
			}
			w.WriteString("}\n")
		}
		w.WriteString(`}
	panic(fmt.Sprintf("unhandled size: %dx%d", len(in), len(out)))
}

func galMulSlicesAvx2Xor(matrix []byte, in, out [][]byte, start, stop int) int {
	n := (stop-start)

`)

		w.WriteString(`switch len(in) {
`)
		for in, defs := range switchDefsX[:] {
			w.WriteString(fmt.Sprintf("		case %d:\n			switch len(out) {\n", in+1))
			for out, def := range defs[:] {
				w.WriteString(fmt.Sprintf("				case %d:\n", out+1))
				w.WriteString(def)
			}
			w.WriteString("}\n")
		}
		w.WriteString(`}
	panic(fmt.Sprintf("unhandled size: %dx%d", len(in), len(out)))
}
`)
	}

	w.WriteString(`

func galMulSlicesGFNI(matrix []uint64, in, out [][]byte, start, stop int) int {
	n := (stop-start) & (maxInt - (64 - 1))

`)

	w.WriteString(`switch len(in) {
`)
	for in, defs := range switchDefs512[:] {
		w.WriteString(fmt.Sprintf("		case %d:\n			switch len(out) {\n", in+1))
		for out, def := range defs[:] {
			w.WriteString(fmt.Sprintf("				case %d:\n", out+1))
			w.WriteString(def)
		}
		w.WriteString("}\n")
	}
	w.WriteString(`}
	panic(fmt.Sprintf("unhandled size: %dx%d", len(in), len(out)))
}

func galMulSlicesGFNIXor(matrix []uint64, in, out [][]byte, start, stop int) int {
	n := (stop-start) & (maxInt - (64 - 1))

`)

	w.WriteString(`switch len(in) {
`)
	for in, defs := range switchDefsX512[:] {
		w.WriteString(fmt.Sprintf("		case %d:\n			switch len(out) {\n", in+1))
		for out, def := range defs[:] {
			w.WriteString(fmt.Sprintf("				case %d:\n", out+1))
			w.WriteString(def)
		}
		w.WriteString("}\n")
	}
	w.WriteString(`}
	panic(fmt.Sprintf("unhandled size: %dx%d", len(in), len(out)))
}
`)

	w.WriteString(`

func galMulSlicesAvxGFNI(matrix []uint64, in, out [][]byte, start, stop int) int {
	n := (stop-start) & (maxInt - (32 - 1))

`)

	w.WriteString(`switch len(in) {
`)
	for in, defs := range switchDefsAvxGFNI[:] {
		w.WriteString(fmt.Sprintf("		case %d:\n			switch len(out) {\n", in+1))
		for out, def := range defs[:] {
			w.WriteString(fmt.Sprintf("				case %d:\n", out+1))
			w.WriteString(def)
		}
		w.WriteString("}\n")
	}
	w.WriteString(`}
	panic(fmt.Sprintf("unhandled size: %dx%d", len(in), len(out)))
}

func galMulSlicesAvxGFNIXor(matrix []uint64, in, out [][]byte, start, stop int) int {
	n := (stop-start) & (maxInt - (32 - 1))

`)

	w.WriteString(`switch len(in) {
`)
	for in, defs := range switchDefsXAvxGFNI[:] {
		w.WriteString(fmt.Sprintf("		case %d:\n			switch len(out) {\n", in+1))
		for out, def := range defs[:] {
			w.WriteString(fmt.Sprintf("				case %d:\n", out+1))
			w.WriteString(def)
		}
		w.WriteString("}\n")
	}
	w.WriteString(`}
	panic(fmt.Sprintf("unhandled size: %dx%d", len(in), len(out)))
}
`)
}

// VPXOR3way will 3-way xor a and b and dst.
func VPXOR3way(a, b, dst reg.VecVirtual) {
	// VPTERNLOGQ is replaced by XOR3WAY - we just use an equivalent operation
	VPTERNLOGQ(U8(0), a, b, dst)
}

func genMulAvx2(name string, inputs int, outputs int, xor bool) {
	if outputs < 4 {
		// Covered by 64-byte version.
		return
	}
	const perLoopBits = 5
	const perLoop = 1 << perLoopBits

	total := inputs * outputs
	doc := []string{
		fmt.Sprintf("%s takes %d inputs and produces %d outputs.", name, inputs, outputs),
	}
	if !xor {
		doc = append(doc, "The output is initialized to 0.")
	}

	// Load shuffle masks on every use.
	var loadNone bool
	// Use registers for destination registers.
	var regDst = true
	var reloadLength = false

	// lo, hi, 1 in, 1 out, 2 tmp, 1 mask
	est := total*2 + outputs + 5
	if outputs == 1 {
		// We don't need to keep a copy of the input if only 1 output.
		est -= 2
	}

	if est > 16 {
		loadNone = true
		// We run out of GP registers first, now.
		if inputs+outputs > 13 {
			regDst = false
		}
		// Save one register by reloading length.
		if inputs+outputs > 12 && regDst {
			reloadLength = true
		}
	}

	x := ""
	if xor {
		x = "Xor"
	}

	TEXT(name, attr.NOSPLIT, fmt.Sprintf("func(matrix []byte, in [][]byte, out [][]byte, start, n int)"))

	// SWITCH DEFINITION:
	s := fmt.Sprintf("			mulAvxTwo_%dx%d%s(matrix, in, out, start, n)\n", inputs, outputs, x)
	s += fmt.Sprintf("\t\t\t\treturn n & (maxInt - %d)\n", perLoop-1)
	if xor {
		switchDefsX[inputs-1][outputs-1] = s
	} else {
		switchDefs[inputs-1][outputs-1] = s
	}

	if loadNone {
		Comment("Loading no tables to registers")
	} else {
		// loadNone == false
		Comment("Loading all tables to registers")
	}
	if regDst {
		Comment("Destination kept in GP registers")
	} else {
		Comment("Destination kept on stack")
	}

	Doc(doc...)
	Pragma("noescape")
	Commentf("Full registers estimated %d YMM used", est)

	length := Load(Param("n"), GP64())
	matrixBase := GP64()
	addr, err := Param("matrix").Base().Resolve()
	if err != nil {
		panic(err)
	}
	MOVQ(addr.Addr, matrixBase)
	SHRQ(U8(perLoopBits), length)
	TESTQ(length, length)
	JZ(LabelRef(name + "_end"))

	inLo := make([]reg.VecVirtual, total)
	inHi := make([]reg.VecVirtual, total)

	for i := range inLo {
		if loadNone {
			break
		}
		tableLo := YMM()
		tableHi := YMM()
		VMOVDQU(Mem{Base: matrixBase, Disp: i * 64}, tableLo)
		VMOVDQU(Mem{Base: matrixBase, Disp: i*64 + 32}, tableHi)
		inLo[i] = tableLo
		inHi[i] = tableHi
	}

	inPtrs := make([]reg.GPVirtual, inputs)
	inSlicePtr := GP64()
	addr, err = Param("in").Base().Resolve()
	if err != nil {
		panic(err)
	}
	MOVQ(addr.Addr, inSlicePtr)
	for i := range inPtrs {
		ptr := GP64()
		MOVQ(Mem{Base: inSlicePtr, Disp: i * 24}, ptr)
		inPtrs[i] = ptr
	}
	// Destination
	dst := make([]reg.VecVirtual, outputs)
	dstPtr := make([]reg.GPVirtual, outputs)
	addr, err = Param("out").Base().Resolve()
	if err != nil {
		panic(err)
	}
	outSlicePtr := GP64()
	MOVQ(addr.Addr, outSlicePtr)
	for i := range dst {
		dst[i] = YMM()
		if !regDst {
			continue
		}
		ptr := GP64()
		MOVQ(Mem{Base: outSlicePtr, Disp: i * 24}, ptr)
		dstPtr[i] = ptr
	}

	offset := GP64()
	addr, err = Param("start").Resolve()
	if err != nil {
		panic(err)
	}

	MOVQ(addr.Addr, offset)
	if regDst {
		Comment("Add start offset to output")
		for _, ptr := range dstPtr {
			ADDQ(offset, ptr)
		}
	}

	Comment("Add start offset to input")
	for _, ptr := range inPtrs {
		ADDQ(offset, ptr)
	}
	// Offset no longer needed unless not regDst
	if !regDst {
		SHRQ(U8(3), offset) // divide by 8 since we'll be scaling it up when loading or storing
	}

	tmpMask := GP64()
	MOVQ(U32(15), tmpMask)
	lowMask := YMM()
	MOVQ(tmpMask, lowMask.AsX())
	VPBROADCASTB(lowMask.AsX(), lowMask)

	if reloadLength {
		length = Load(Param("n"), GP64())
		SHRQ(U8(perLoopBits), length)
	}
	Label(name + "_loop")

	// Load data before loop or during first iteration?
	// No clear winner.
	preloadInput := xor && false
	if preloadInput {
		Commentf("Load %d outputs", outputs)
		for i := range dst {
			if regDst {
				VMOVDQU(Mem{Base: dstPtr[i]}, dst[i])
				if prefetchDst > 0 {
					PREFETCHT0(Mem{Base: dstPtr[i], Disp: prefetchDst})
				}
				continue
			}
			ptr := GP64()
			MOVQ(Mem{Base: outSlicePtr, Disp: i * 24}, ptr)
			VMOVDQU(Mem{Base: ptr, Index: offset, Scale: 8}, dst[i])
			if prefetchDst > 0 {
				PREFETCHT0(Mem{Base: ptr, Disp: prefetchDst, Index: offset, Scale: 8})
			}
		}
	}

	lookLow, lookHigh := YMM(), YMM()
	inLow, inHigh := YMM(), YMM()
	for i := range inPtrs {
		Commentf("Load and process 32 bytes from input %d to %d outputs", i, outputs)
		VMOVDQU(Mem{Base: inPtrs[i]}, inLow)
		if prefetchSrc > 0 {
			PREFETCHT0(Mem{Base: inPtrs[i], Disp: prefetchSrc})
		}
		ADDQ(U8(perLoop), inPtrs[i])
		VPSRLQ(U8(4), inLow, inHigh)
		VPAND(lowMask, inLow, inLow)
		VPAND(lowMask, inHigh, inHigh)
		for j := range dst {
			//Commentf(" xor:%v i: %v", xor, i)
			if !preloadInput && xor && i == 0 {
				if regDst {
					VMOVDQU(Mem{Base: dstPtr[j]}, dst[j])
					if prefetchDst > 0 {
						PREFETCHT0(Mem{Base: dstPtr[j], Disp: prefetchDst})
					}
				} else {
					ptr := GP64()
					MOVQ(Mem{Base: outSlicePtr, Disp: j * 24}, ptr)
					VMOVDQU(Mem{Base: ptr, Index: offset, Scale: 8}, dst[j])
					if prefetchDst > 0 {
						PREFETCHT0(Mem{Base: ptr, Disp: prefetchDst, Index: offset, Scale: 8})
					}
				}
			}
			if loadNone {
				VMOVDQU(Mem{Base: matrixBase, Disp: 64 * (i*outputs + j)}, lookLow)
				VMOVDQU(Mem{Base: matrixBase, Disp: 32 + 64*(i*outputs+j)}, lookHigh)
				VPSHUFB(inLow, lookLow, lookLow)
				VPSHUFB(inHigh, lookHigh, lookHigh)
			} else {
				VPSHUFB(inLow, inLo[i*outputs+j], lookLow)
				VPSHUFB(inHigh, inHi[i*outputs+j], lookHigh)
			}
			if i == 0 && !xor {
				// We don't have any existing data, write directly.
				VPXOR(lookLow, lookHigh, dst[j])
			} else {
				VPXOR3way(lookLow, lookHigh, dst[j])
			}
		}
	}
	Commentf("Store %d outputs", outputs)
	for i := range dst {
		if regDst {
			VMOVDQU(dst[i], Mem{Base: dstPtr[i]})
			if prefetchDst > 0 && !xor {
				PREFETCHT0(Mem{Base: dstPtr[i], Disp: prefetchDst})
			}
			ADDQ(U8(perLoop), dstPtr[i])
			continue
		}
		ptr := GP64()
		MOVQ(Mem{Base: outSlicePtr, Disp: i * 24}, ptr)
		VMOVDQU(dst[i], Mem{Base: ptr, Index: offset, Scale: 8})
		if prefetchDst > 0 && !xor {
			PREFETCHT0(Mem{Base: ptr, Disp: prefetchDst, Index: offset, Scale: 8})
		}
	}
	Comment("Prepare for next loop")
	if !regDst {
		ADDQ(U8(perLoop>>3), offset)
	}
	DECQ(length)
	JNZ(LabelRef(name + "_loop"))
	VZEROUPPER()

	Label(name + "_end")
	RET()
}

func genMulAvx2Sixty64(name string, inputs int, outputs int, xor bool) {
	if outputs >= 4 {
		return
	}
	const perLoopBits = 6
	const perLoop = 1 << perLoopBits

	total := inputs * outputs

	doc := []string{
		fmt.Sprintf("%s takes %d inputs and produces %d outputs.", name, inputs, outputs),
	}
	if !xor {
		doc = append(doc, "The output is initialized to 0.")
	}

	// Load shuffle masks on every use.
	var loadNone bool
	// Use registers for destination registers.
	var regDst = true
	var reloadLength = false

	// lo, hi, 1 in, 1 out, 2 tmp, 1 mask
	est := total*4 + outputs + 7
	if outputs == 1 {
		// We don't need to keep a copy of the input if only 1 output.
		est -= 2
	}

	if est > 16 {
		loadNone = true
		// We run out of GP registers first, now.
		if inputs+outputs > 13 {
			regDst = false
		}
		// Save one register by reloading length.
		if inputs+outputs > 12 && regDst {
			reloadLength = true
		}
	}

	TEXT(name, 0, fmt.Sprintf("func(matrix []byte, in [][]byte, out [][]byte, start, n int)"))
	x := ""
	if xor {
		x = "Xor"
	}
	// SWITCH DEFINITION:
	//s := fmt.Sprintf("n = (n>>%d)<<%d\n", perLoopBits, perLoopBits)
	s := fmt.Sprintf("			mulAvxTwo_%dx%d_64%s(matrix, in, out, start, n)\n", inputs, outputs, x)
	s += fmt.Sprintf("\t\t\t\treturn n & (maxInt - %d)\n", perLoop-1)
	if xor {
		switchDefsX[inputs-1][outputs-1] = s
	} else {
		switchDefs[inputs-1][outputs-1] = s
	}

	if loadNone {
		Comment("Loading no tables to registers")
	} else {
		// loadNone == false
		Comment("Loading all tables to registers")
	}
	if regDst {
		Comment("Destination kept in GP registers")
	} else {
		Comment("Destination kept on stack")
	}

	Doc(doc...)
	Pragma("noescape")
	Commentf("Full registers estimated %d YMM used", est)

	length := Load(Param("n"), GP64())
	matrixBase := GP64()
	addr, err := Param("matrix").Base().Resolve()
	if err != nil {
		panic(err)
	}
	MOVQ(addr.Addr, matrixBase)
	SHRQ(U8(perLoopBits), length)
	TESTQ(length, length)
	JZ(LabelRef(name + "_end"))

	inLo := make([]reg.VecVirtual, total)
	inHi := make([]reg.VecVirtual, total)

	for i := range inLo {
		if loadNone {
			break
		}
		tableLo := YMM()
		tableHi := YMM()
		VMOVDQU(Mem{Base: matrixBase, Disp: i * 64}, tableLo)
		VMOVDQU(Mem{Base: matrixBase, Disp: i*64 + 32}, tableHi)
		inLo[i] = tableLo
		inHi[i] = tableHi
	}

	inPtrs := make([]reg.GPVirtual, inputs)
	inSlicePtr := GP64()
	addr, err = Param("in").Base().Resolve()
	if err != nil {
		panic(err)
	}
	MOVQ(addr.Addr, inSlicePtr)
	for i := range inPtrs {
		ptr := GP64()
		MOVQ(Mem{Base: inSlicePtr, Disp: i * 24}, ptr)
		inPtrs[i] = ptr
	}
	// Destination
	dst := make([]reg.VecVirtual, outputs)
	dst2 := make([]reg.VecVirtual, outputs)
	dstPtr := make([]reg.GPVirtual, outputs)
	addr, err = Param("out").Base().Resolve()
	if err != nil {
		panic(err)
	}
	outBase := addr.Addr
	outSlicePtr := GP64()
	MOVQ(outBase, outSlicePtr)
	for i := range dst {
		dst[i] = YMM()
		dst2[i] = YMM()
		if !regDst {
			continue
		}
		ptr := GP64()
		MOVQ(Mem{Base: outSlicePtr, Disp: i * 24}, ptr)
		dstPtr[i] = ptr
	}

	offset := GP64()
	addr, err = Param("start").Resolve()
	if err != nil {
		panic(err)
	}

	MOVQ(addr.Addr, offset)
	if regDst {
		Comment("Add start offset to output")
		for _, ptr := range dstPtr {
			ADDQ(offset, ptr)
		}
	}

	Comment("Add start offset to input")
	for _, ptr := range inPtrs {
		ADDQ(offset, ptr)
	}
	// Offset no longer needed unless not regdst

	tmpMask := GP64()
	MOVQ(U32(15), tmpMask)
	lowMask := YMM()
	MOVQ(tmpMask, lowMask.AsX())
	VPBROADCASTB(lowMask.AsX(), lowMask)

	if reloadLength {
		Commentf("Reload length to save a register")
		length = Load(Param("n"), GP64())
		SHRQ(U8(perLoopBits), length)
	}
	Label(name + "_loop")

	if xor {
		Commentf("Load %d outputs", outputs)
		for i := range dst {
			if regDst {
				VMOVDQU(Mem{Base: dstPtr[i]}, dst[i])
				VMOVDQU(Mem{Base: dstPtr[i], Disp: 32}, dst2[i])
				if prefetchDst > 0 {
					PREFETCHT0(Mem{Base: dstPtr[i], Disp: prefetchDst})
				}
				continue
			}
			ptr := GP64()
			MOVQ(Mem{Base: outSlicePtr, Disp: i * 24}, ptr)
			VMOVDQU(Mem{Base: ptr, Index: offset, Scale: 1}, dst[i])
			VMOVDQU(Mem{Base: ptr, Index: offset, Scale: 1, Disp: 32}, dst2[i])

			if prefetchDst > 0 {
				PREFETCHT0(Mem{Base: ptr, Disp: prefetchDst, Index: offset, Scale: 1})
			}
		}
	}

	lookLow, lookHigh := YMM(), YMM()
	lookLow2, lookHigh2 := YMM(), YMM()
	inLow, inHigh := YMM(), YMM()
	in2Low, in2High := YMM(), YMM()
	for i := range inPtrs {
		Commentf("Load and process 64 bytes from input %d to %d outputs", i, outputs)
		VMOVDQU(Mem{Base: inPtrs[i]}, inLow)
		VMOVDQU(Mem{Base: inPtrs[i], Disp: 32}, in2Low)
		if prefetchSrc > 0 {
			PREFETCHT0(Mem{Base: inPtrs[i], Disp: prefetchSrc})
		}
		ADDQ(U8(perLoop), inPtrs[i])
		VPSRLQ(U8(4), inLow, inHigh)
		VPSRLQ(U8(4), in2Low, in2High)
		VPAND(lowMask, inLow, inLow)
		VPAND(lowMask, in2Low, in2Low)
		VPAND(lowMask, inHigh, inHigh)
		VPAND(lowMask, in2High, in2High)
		for j := range dst {
			if loadNone {
				VMOVDQU(Mem{Base: matrixBase, Disp: 64 * (i*outputs + j)}, lookLow)
				VMOVDQU(Mem{Base: matrixBase, Disp: 32 + 64*(i*outputs+j)}, lookHigh)
				VPSHUFB(in2Low, lookLow, lookLow2)
				VPSHUFB(inLow, lookLow, lookLow) // Reuse lookLow to save a reg
				VPSHUFB(in2High, lookHigh, lookHigh2)
				VPSHUFB(inHigh, lookHigh, lookHigh) // Reuse lookHigh to save a reg
			} else {
				VPSHUFB(inLow, inLo[i*outputs+j], lookLow)
				VPSHUFB(in2Low, inLo[i*outputs+j], lookLow2)
				VPSHUFB(inHigh, inHi[i*outputs+j], lookHigh)
				VPSHUFB(in2High, inHi[i*outputs+j], lookHigh2)
			}
			if i == 0 && !xor {
				// We don't have any existing data, write directly.
				VPXOR(lookLow, lookHigh, dst[j])
				VPXOR(lookLow2, lookHigh2, dst2[j])
			} else {
				VPXOR3way(lookLow, lookHigh, dst[j])
				VPXOR3way(lookLow2, lookHigh2, dst2[j])
			}
		}
	}
	Commentf("Store %d outputs", outputs)
	for i := range dst {
		if regDst {
			VMOVDQU(dst[i], Mem{Base: dstPtr[i]})
			VMOVDQU(dst2[i], Mem{Base: dstPtr[i], Disp: 32})
			if prefetchDst > 0 && !xor {
				PREFETCHT0(Mem{Base: dstPtr[i], Disp: prefetchDst})
			}
			ADDQ(U8(perLoop), dstPtr[i])
			continue
		}
		ptr := GP64()
		MOVQ(Mem{Base: outSlicePtr, Disp: i * 24}, ptr)
		VMOVDQU(dst[i], Mem{Base: ptr, Index: offset, Scale: 1})
		VMOVDQU(dst2[i], Mem{Base: ptr, Index: offset, Scale: 1, Disp: 32})
		if prefetchDst > 0 && !xor {
			PREFETCHT0(Mem{Base: ptr, Disp: prefetchDst, Index: offset, Scale: 1})
		}
	}
	Comment("Prepare for next loop")
	if !regDst {
		ADDQ(U8(perLoop), offset)
	}
	DECQ(length)
	JNZ(LabelRef(name + "_loop"))
	VZEROUPPER()

	Label(name + "_end")
	RET()
}

func genMulAvx512GFNI(name string, inputs int, outputs int, xor bool) {
	const perLoopBits = 6
	const perLoop = 1 << perLoopBits

	total := inputs * outputs

	doc := []string{
		fmt.Sprintf("%s takes %d inputs and produces %d outputs.", name, inputs, outputs),
	}
	if !xor {
		doc = append(doc, "The output is initialized to 0.")
	}

	// Load shuffle masks on every use.
	var loadNone bool
	// Use registers for destination registers.
	var regDst = true
	var reloadLength = false

	est := total + outputs + 2
	// When we can't hold all, keep this many in registers.
	inReg := 0
	if est > 32 {
		loadNone = true
		inReg = 32 - outputs - 2
		// We run out of GP registers first, now.
		if inputs+outputs > 13 {
			regDst = false
		}
		// Save one register by reloading length.
		if inputs+outputs > 12 && regDst {
			reloadLength = true
		}
	}

	TEXT(name, 0, fmt.Sprintf("func(matrix []uint64, in [][]byte, out [][]byte, start, n int)"))
	x := ""
	if xor {
		x = "Xor"
	}
	// SWITCH DEFINITION:
	//s := fmt.Sprintf("n = (n>>%d)<<%d\n", perLoopBits, perLoopBits)
	s := fmt.Sprintf("			mulGFNI_%dx%d_64%s(matrix, in, out, start, n)\n", inputs, outputs, x)
	s += fmt.Sprintf("\t\t\t\treturn n\n")
	if xor {
		switchDefsX512[inputs-1][outputs-1] = s
	} else {
		switchDefs512[inputs-1][outputs-1] = s
	}

	if loadNone {
		Commentf("Loading %d of %d tables to registers", inReg, inputs*outputs)
	} else {
		// loadNone == false
		Comment("Loading all tables to registers")
	}
	if regDst {
		Comment("Destination kept in GP registers")
	} else {
		Comment("Destination kept on stack")
	}

	Doc(doc...)
	Pragma("noescape")
	Commentf("Full registers estimated %d YMM used", est)

	length := Load(Param("n"), GP64())
	matrixBase := GP64()
	addr, err := Param("matrix").Base().Resolve()
	if err != nil {
		panic(err)
	}
	MOVQ(addr.Addr, matrixBase)
	SHRQ(U8(perLoopBits), length)
	TESTQ(length, length)
	JZ(LabelRef(name + "_end"))

	matrix := make([]reg.VecVirtual, total)

	for i := range matrix {
		if loadNone && i >= inReg {
			break
		}
		table := ZMM()
		VBROADCASTF32X2(Mem{Base: matrixBase, Disp: i * 8}, table)
		matrix[i] = table
	}

	inPtrs := make([]reg.GPVirtual, inputs)
	inSlicePtr := GP64()
	addr, err = Param("in").Base().Resolve()
	if err != nil {
		panic(err)
	}
	MOVQ(addr.Addr, inSlicePtr)
	for i := range inPtrs {
		ptr := GP64()
		MOVQ(Mem{Base: inSlicePtr, Disp: i * 24}, ptr)
		inPtrs[i] = ptr
	}
	// Destination
	dst := make([]reg.VecVirtual, outputs)
	dstPtr := make([]reg.GPVirtual, outputs)
	addr, err = Param("out").Base().Resolve()
	if err != nil {
		panic(err)
	}
	outBase := addr.Addr
	outSlicePtr := GP64()
	MOVQ(addr.Addr, outSlicePtr)
	MOVQ(outBase, outSlicePtr)
	for i := range dst {
		dst[i] = ZMM()
		if !regDst {
			continue
		}
		ptr := GP64()
		MOVQ(Mem{Base: outSlicePtr, Disp: i * 24}, ptr)
		dstPtr[i] = ptr
	}

	offset := GP64()
	addr, err = Param("start").Resolve()
	if err != nil {
		panic(err)
	}

	MOVQ(addr.Addr, offset)
	if regDst {
		Comment("Add start offset to output")
		for _, ptr := range dstPtr {
			ADDQ(offset, ptr)
		}
	}

	Comment("Add start offset to input")
	for _, ptr := range inPtrs {
		ADDQ(offset, ptr)
	}
	// Offset no longer needed unless not regdst

	if reloadLength {
		Commentf("Reload length to save a register")
		length = Load(Param("n"), GP64())
		SHRQ(U8(perLoopBits), length)
	}
	Label(name + "_loop")

	if xor {
		Commentf("Load %d outputs", outputs)
		for i := range dst {
			if regDst {
				VMOVDQU64(Mem{Base: dstPtr[i]}, dst[i])
				if prefetchDst > 0 {
					PREFETCHT0(Mem{Base: dstPtr[i], Disp: prefetchDst})
				}
				continue
			}
			ptr := GP64()
			MOVQ(Mem{Base: outSlicePtr, Disp: i * 24}, ptr)
			VMOVDQU64(Mem{Base: ptr, Index: offset, Scale: 1}, dst[i])

			if prefetchDst > 0 {
				PREFETCHT0(Mem{Base: ptr, Disp: prefetchDst, Index: offset, Scale: 1})
			}
		}
	}

	in := ZMM()
	look := ZMM()
	for i := range inPtrs {
		Commentf("Load and process 64 bytes from input %d to %d outputs", i, outputs)
		VMOVDQU64(Mem{Base: inPtrs[i]}, in)
		if prefetchSrc > 0 {
			PREFETCHT0(Mem{Base: inPtrs[i], Disp: prefetchSrc})
		}
		ADDQ(U8(perLoop), inPtrs[i])

		for j := range dst {
			idx := i*outputs + j
			if loadNone && idx >= inReg {
				if i == 0 && !xor {
					VGF2P8AFFINEQB_BCST(U8(0), Mem{Base: matrixBase, Disp: 8 * idx}, in, dst[j])
				} else {
					VGF2P8AFFINEQB_BCST(U8(0), Mem{Base: matrixBase, Disp: 8 * idx}, in, look)
					VXORPD(dst[j], look, dst[j])
				}
			} else {
				if i == 0 && !xor {
					VGF2P8AFFINEQB(U8(0), matrix[i*outputs+j], in, dst[j])
				} else {
					VGF2P8AFFINEQB(U8(0), matrix[i*outputs+j], in, look)
					VXORPD(dst[j], look, dst[j])
				}
			}
		}
	}
	Commentf("Store %d outputs", outputs)
	for i := range dst {
		if regDst {
			VMOVDQU64(dst[i], Mem{Base: dstPtr[i]})
			if prefetchDst > 0 && !xor {
				PREFETCHT0(Mem{Base: dstPtr[i], Disp: prefetchDst})
			}
			ADDQ(U8(perLoop), dstPtr[i])
			continue
		}
		ptr := GP64()
		MOVQ(Mem{Base: outSlicePtr, Disp: i * 24}, ptr)
		VMOVDQU64(dst[i], Mem{Base: ptr, Index: offset, Scale: 1})
		if prefetchDst > 0 && !xor {
			PREFETCHT0(Mem{Base: ptr, Disp: prefetchDst, Index: offset, Scale: 1})
		}
	}
	Comment("Prepare for next loop")
	if !regDst {
		ADDQ(U8(perLoop), offset)
	}
	DECQ(length)
	JNZ(LabelRef(name + "_loop"))
	VZEROUPPER()

	Label(name + "_end")
	RET()
}

func genMulAvxGFNI(name string, inputs int, outputs int, xor bool) {
	const perLoopBits = 5
	const perLoop = 1 << perLoopBits

	total := inputs * outputs

	doc := []string{
		fmt.Sprintf("%s takes %d inputs and produces %d outputs.", name, inputs, outputs),
	}
	if !xor {
		doc = append(doc, "The output is initialized to 0.")
	}

	// Load shuffle masks on every use.
	var loadNone bool
	// Use registers for destination registers.
	var regDst = true
	var reloadLength = false

	est := total + outputs + 2
	// When we can't hold all, keep this many in registers.
	inReg := 0
	if est > 16 {
		loadNone = true
		inReg = 16 - outputs - 2
		// We run out of GP registers first, now.
		if inputs+outputs > 13 {
			regDst = false
		}
		// Save one register by reloading length.
		if inputs+outputs > 12 && regDst {
			reloadLength = true
		}
	}

	TEXT(name, 0, fmt.Sprintf("func(matrix []uint64, in [][]byte, out [][]byte, start, n int)"))
	x := ""
	if xor {
		x = "Xor"
	}
	// SWITCH DEFINITION:
	//s := fmt.Sprintf("n = (n>>%d)<<%d\n", perLoopBits, perLoopBits)
	s := fmt.Sprintf("			mulAvxGFNI_%dx%d%s(matrix, in, out, start, n)\n", inputs, outputs, x)
	s += fmt.Sprintf("\t\t\t\treturn n\n")
	if xor {
		switchDefsXAvxGFNI[inputs-1][outputs-1] = s
	} else {
		switchDefsAvxGFNI[inputs-1][outputs-1] = s
	}

	if loadNone {
		Commentf("Loading %d of %d tables to registers", inReg, inputs*outputs)
	} else {
		// loadNone == false
		Comment("Loading all tables to registers")
	}
	if regDst {
		Comment("Destination kept in GP registers")
	} else {
		Comment("Destination kept on stack")
	}

	Doc(doc...)
	Pragma("noescape")
	Commentf("Full registers estimated %d YMM used", est)

	length := Load(Param("n"), GP64())
	matrixBase := GP64()
	addr, err := Param("matrix").Base().Resolve()
	if err != nil {
		panic(err)
	}
	MOVQ(addr.Addr, matrixBase)
	SHRQ(U8(perLoopBits), length)
	TESTQ(length, length)
	JZ(LabelRef(name + "_end"))

	matrix := make([]reg.VecVirtual, total)

	for i := range matrix {
		if loadNone && i >= inReg {
			break
		}
		table := YMM()
		VBROADCASTSD(Mem{Base: matrixBase, Disp: i * 8}, table)
		matrix[i] = table
	}

	inPtrs := make([]reg.GPVirtual, inputs)
	inSlicePtr := GP64()
	addr, err = Param("in").Base().Resolve()
	if err != nil {
		panic(err)
	}
	MOVQ(addr.Addr, inSlicePtr)
	for i := range inPtrs {
		ptr := GP64()
		MOVQ(Mem{Base: inSlicePtr, Disp: i * 24}, ptr)
		inPtrs[i] = ptr
	}
	// Destination
	dst := make([]reg.VecVirtual, outputs)
	dstPtr := make([]reg.GPVirtual, outputs)
	addr, err = Param("out").Base().Resolve()
	if err != nil {
		panic(err)
	}
	outBase := addr.Addr
	outSlicePtr := GP64()
	MOVQ(addr.Addr, outSlicePtr)
	MOVQ(outBase, outSlicePtr)
	for i := range dst {
		dst[i] = YMM()
		if !regDst {
			continue
		}
		ptr := GP64()
		MOVQ(Mem{Base: outSlicePtr, Disp: i * 24}, ptr)
		dstPtr[i] = ptr
	}

	offset := GP64()
	addr, err = Param("start").Resolve()
	if err != nil {
		panic(err)
	}

	MOVQ(addr.Addr, offset)
	if regDst {
		Comment("Add start offset to output")
		for _, ptr := range dstPtr {
			ADDQ(offset, ptr)
		}
	}

	Comment("Add start offset to input")
	for _, ptr := range inPtrs {
		ADDQ(offset, ptr)
	}
	// Offset no longer needed unless not regdst

	if reloadLength {
		Commentf("Reload length to save a register")
		length = Load(Param("n"), GP64())
		SHRQ(U8(perLoopBits), length)
	}
	Label(name + "_loop")

	if xor {
		Commentf("Load %d outputs", outputs)
		for i := range dst {
			if regDst {
				VMOVDQU(Mem{Base: dstPtr[i]}, dst[i])
				if prefetchDst > 0 {
					PREFETCHT0(Mem{Base: dstPtr[i], Disp: prefetchDst})
				}
				continue
			}
			ptr := GP64()
			MOVQ(Mem{Base: outSlicePtr, Disp: i * 24}, ptr)
			VMOVDQU(Mem{Base: ptr, Index: offset, Scale: 1}, dst[i])

			if prefetchDst > 0 {
				PREFETCHT0(Mem{Base: ptr, Disp: prefetchDst, Index: offset, Scale: 1})
			}
		}
	}

	in := YMM()
	look := YMM()
	for i := range inPtrs {
		Commentf("Load and process 32 bytes from input %d to %d outputs", i, outputs)
		VMOVDQU(Mem{Base: inPtrs[i]}, in)
		if prefetchSrc > 0 {
			PREFETCHT0(Mem{Base: inPtrs[i], Disp: prefetchSrc})
		}
		ADDQ(U8(perLoop), inPtrs[i])

		for j := range dst {
			idx := i*outputs + j
			if loadNone && idx >= inReg {
				tmp := YMM()
				if i == 0 && !xor {
					VBROADCASTSD(Mem{Base: matrixBase, Disp: idx * 8}, tmp)
					VGF2P8AFFINEQB(U8(0), tmp, in, dst[j])
				} else {
					VBROADCASTSD(Mem{Base: matrixBase, Disp: idx * 8}, tmp)
					VGF2P8AFFINEQB(U8(0), tmp, in, look)
					VXORPD(dst[j], look, dst[j])
				}
			} else {
				if i == 0 && !xor {
					VGF2P8AFFINEQB(U8(0), matrix[i*outputs+j], in, dst[j])
				} else {
					VGF2P8AFFINEQB(U8(0), matrix[i*outputs+j], in, look)
					VXORPD(dst[j], look, dst[j])
				}
			}
		}
	}
	Commentf("Store %d outputs", outputs)
	for i := range dst {
		if regDst {
			VMOVDQU(dst[i], Mem{Base: dstPtr[i]})
			if prefetchDst > 0 && !xor {
				PREFETCHT0(Mem{Base: dstPtr[i], Disp: prefetchDst})
			}
			ADDQ(U8(perLoop), dstPtr[i])
			continue
		}
		ptr := GP64()
		MOVQ(Mem{Base: outSlicePtr, Disp: i * 24}, ptr)
		VMOVDQU(dst[i], Mem{Base: ptr, Index: offset, Scale: 1})
		if prefetchDst > 0 && !xor {
			PREFETCHT0(Mem{Base: ptr, Disp: prefetchDst, Index: offset, Scale: 1})
		}
	}
	Comment("Prepare for next loop")
	if !regDst {
		ADDQ(U8(perLoop), offset)
	}
	DECQ(length)
	JNZ(LabelRef(name + "_loop"))
	VZEROUPPER()

	Label(name + "_end")
	RET()
}

func genXor() {
	// SSE 2
	{
		Comment("sSE2XorSlice will XOR in with out and store in out.")
		Comment("Processes 16 bytes/loop.")
		TEXT("sSE2XorSlice", 0, fmt.Sprintf("func(in, out []byte)"))
		Pragma("noescape")
		src := Load(Param("in").Base(), GP64())
		dst := Load(Param("out").Base(), GP64())
		length := Load(Param("in").Len(), GP64())
		SHRQ(U8(4), length)
		srcX, dstX := XMM(), XMM()
		JZ(LabelRef("end"))
		Label("loop")
		MOVOU(Mem{Base: src}, srcX)
		MOVOU(Mem{Base: dst}, dstX)
		PXOR(srcX, dstX)
		MOVOU(dstX, Mem{Base: dst})
		ADDQ(U8(16), src)
		ADDQ(U8(16), dst)
		DECQ(length)
		JNZ(LabelRef("loop"))
		Label("end")
		RET()
	}

	// SSE2 64 bytes
	{
		Comment("sSE2XorSlice_64 will XOR in with out and store in out.")
		Comment("Processes 64 bytes/loop.")
		TEXT("sSE2XorSlice_64", 0, fmt.Sprintf("func(in, out []byte)"))
		Pragma("noescape")
		src := Load(Param("in").Base(), GP64())
		dst := Load(Param("out").Base(), GP64())
		length := Load(Param("in").Len(), GP64())
		SHRQ(U8(6), length)
		var srcX, dstX [4]reg.VecVirtual
		for i := range srcX {
			srcX[i], dstX[i] = XMM(), XMM()
		}
		JZ(LabelRef("end"))
		Label("loop")
		for i := range srcX {
			MOVOU(Mem{Base: src, Disp: 16 * i}, srcX[i])
		}
		for i := range srcX {
			MOVOU(Mem{Base: dst, Disp: 16 * i}, dstX[i])
		}
		for i := range srcX {
			PXOR(srcX[i], dstX[i])
		}
		for i := range srcX {
			MOVOU(dstX[i], Mem{Base: dst, Disp: 16 * i})
		}
		ADDQ(U8(64), src)
		ADDQ(U8(64), dst)
		DECQ(length)
		JNZ(LabelRef("loop"))
		Label("end")
		RET()
	}
	//AVX 2
	{
		Comment("avx2XorSlice_64 will XOR in with out and store in out.")
		Comment("Processes 64 bytes/loop.")
		TEXT("avx2XorSlice_64", 0, fmt.Sprintf("func(in, out []byte)"))
		Pragma("noescape")
		src := Load(Param("in").Base(), GP64())
		dst := Load(Param("out").Base(), GP64())
		length := Load(Param("in").Len(), GP64())
		SHRQ(U8(6), length)
		srcX, dstX := YMM(), YMM()
		srcX2, dstX2 := YMM(), YMM()
		JZ(LabelRef("end"))

		Label("loop")
		VMOVDQU(Mem{Base: src}, srcX)
		VMOVDQU(Mem{Base: src, Disp: 32}, srcX2)
		VMOVDQU(Mem{Base: dst}, dstX)
		VMOVDQU(Mem{Base: dst, Disp: 32}, dstX2)
		VPXOR(srcX, dstX, dstX)
		VPXOR(srcX2, dstX2, dstX2)
		VMOVDQU(dstX, Mem{Base: dst})
		VMOVDQU(dstX2, Mem{Base: dst, Disp: 32})
		ADDQ(U8(64), src)
		ADDQ(U8(64), dst)
		DECQ(length)
		JNZ(LabelRef("loop"))

		Label("end")
		VZEROUPPER()
		RET()
	}
}
