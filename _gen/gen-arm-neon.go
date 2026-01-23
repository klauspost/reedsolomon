// Copyright 2024, Klaus Post/Minio Inc. See LICENSE for details.

package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func convert2Neon(asmBuf *bytes.Buffer, lines []string) {

	asmF := func(format string, args ...interface{}) {
		(*asmBuf).WriteString(fmt.Sprintf(format, args...))
	}

	reAddrMode := regexp.MustCompile(`\[(.*?)\]`) // regexp to match content between square brackets

	getZregister := func(reg string) int {
		if reg[0] == 'z' {
			reg = strings.NewReplacer(",", "", ".d", "", ".b", "", "[0]", "").Replace(reg[1:])
			num, err := strconv.Atoi(reg)
			if err != nil {
				panic(err)
			}
			return num
		}
		return -1
	}

	getXregister := func(reg string) int {
		if reg[0] == 'x' {
			reg = strings.ReplaceAll(reg, ",", "")
			num, err := strconv.Atoi(reg[1:])
			if err != nil {
				panic(err)
			}
			return num
		}
		return -1
	}

	getHashImm := func(imm string) int {
		if imm[0] == '#' {
			num, err := strconv.Atoi(imm[1:])
			if err != nil {
				panic(err)
			}
			return num
		} else {
			panic("bad immediate")
		}
	}

	parseAddrModeMulVl := func(addrMode string) string {
		addrMode = strings.NewReplacer("[", "", "]", "").Replace(addrMode)
		f := strings.Fields(addrMode)
		xn, offset := getXregister(f[0]), ""
		if len(f) > 1 {
			if len(f) == 4 && f[2] == "MUL" && f[3] == "VL" {
				num, err := strconv.Atoi(strings.NewReplacer("#", "", ",", "").Replace(f[1]))
				if err != nil {
					panic(err)
				}
				offset = fmt.Sprintf("%d", num*32)
			} else {
				panic("bad addressing mode")
			}
		}

		return fmt.Sprintf("%s(R%d)", offset, xn)
	}

	parseAddrModeIndexed := func(addrMode string) (string, string) {
		addrMode = strings.NewReplacer("[", "", "]", "").Replace(addrMode)
		f := strings.Fields(addrMode)
		regbase := getXregister(f[0])
		regshifted := getXregister(f[1])
		shift := getHashImm(f[3])
		return fmt.Sprintf("(R%d)", regbase), fmt.Sprintf("R%d<<%d", regshifted, shift)
	}

	for _, line := range lines {
		if strings.Contains(line, " // ldr z") {
			if matches := reAddrMode.FindAllStringSubmatch(line, -1); len(matches) == 1 {
				line = strings.ReplaceAll(line, matches[0][0], "ADDRMODE")
				f := strings.Fields(line)
				zd := getZregister(f[len(f)-2])
				var am string
				{ // HACK: ignore offset since we're fixating it at 32
					baseReg := strings.Split(matches[0][0], ",")[0]
					am = parseAddrModeMulVl(baseReg)
				}
				asmF("    VLD1.P 32%s, [V%d.B16, V%d.B16]\n", am, zd*2, zd*2+1)
			} else {
				panic("bad 'ldr' instrunction")
			}
		} else if strings.Contains(line, " // str z") {
			if matches := reAddrMode.FindAllStringSubmatch(line, -1); len(matches) == 1 {
				line = strings.ReplaceAll(line, matches[0][0], "ADDRMODE")
				f := strings.Fields(line)
				zd := getZregister(f[len(f)-2])
				var am string
				{ // HACK: ignore offset since we're fixating it at 32
					baseReg := strings.Split(matches[0][0], ",")[0]
					am = parseAddrModeMulVl(baseReg)
				}
				asmF("    VST1.P [V%d.D2, V%d.D2], 32%s\n", zd*2, zd*2+1, am)
			} else {
				panic("bad 'str' instrunction")
			}
		} else if strings.Contains(line, " // ld1d { z") {
			if matches := reAddrMode.FindAllStringSubmatch(line, -1); len(matches) == 1 {
				line = strings.ReplaceAll(line, matches[0][0], "ADDRMODE")
				f := strings.Fields(line)
				zd := getZregister(f[5])
				base, shifted := parseAddrModeIndexed(matches[0][0])
				asmF("    ADD    %s, %s\n", shifted, strings.NewReplacer("(", "", ")", "").Replace(base))
				asmF("    VLD1   %s, [V%d.B16, V%d.B16]\n", base, zd*2, zd*2+1)
			} else {
				panic("bad 'ld1d' instrunction")
			}
		} else if strings.Contains(line, " // st1d { z") {
			if matches := reAddrMode.FindAllStringSubmatch(line, -1); len(matches) == 1 {
				line = strings.ReplaceAll(line, matches[0][0], "ADDRMODE")
				f := strings.Fields(line)
				zd := getZregister(f[5])
				base, shifted := parseAddrModeIndexed(matches[0][0])
				asmF("    ADD    %s, %s\n", shifted, strings.NewReplacer("(", "", ")", "").Replace(base))
				asmF("    VST1   [V%d.D2, V%d.D2], %s\n", zd*2, zd*2+1, base)
			} else {
				panic("bad 'st1d' instrunction")
			}
		} else if strings.Contains(line, " // lsr z") {
			f := strings.Fields(line)
			zd := getZregister(f[len(f)-3])
			zn := getZregister(f[len(f)-2])
			imm := getHashImm(f[len(f)-1])
			asmF("    VUSHR  $%d, V%d.B16, V%d.B16\n", imm, zn*2, zd*2)
			asmF("    VUSHR  $%d, V%d.B16, V%d.B16\n", imm, zn*2+1, zd*2+1)
		} else if strings.Contains(line, " // and z") {
			f := strings.Fields(line)
			zd := getZregister(f[len(f)-3])
			zn := getZregister(f[len(f)-2])
			zn2 := getZregister(f[len(f)-1])
			asmF("    VAND   V%d.B16, V%d.B16, V%d.B16\n", zn2*2, zn*2, zd*2)
			asmF("    VAND   V%d.B16, V%d.B16, V%d.B16\n", zn2*2 /*+1*/, zn*2+1, zd*2+1)
		} else if strings.Contains(line, " // tbl z") {
			f := strings.Fields(line)
			zd := getZregister(f[len(f)-3])
			zn := getZregister(f[len(f)-2])
			zn2 := getZregister(f[len(f)-1])
			asmF("    VTBL   V%d.B16, [V%d.B16], V%d.B16\n", zn2*2, zn*2, zd*2)
			asmF("    VTBL   V%d.B16, [V%d.B16], V%d.B16\n", zn2*2+1, zn*2+1, zd*2+1)
		} else if strings.Contains(line, " // eor z") {
			f := strings.Fields(line)
			zd := getZregister(f[len(f)-3])
			zn := getZregister(f[len(f)-2])
			zn2 := getZregister(f[len(f)-1])
			asmF("    VEOR   V%d.B16, V%d.B16, V%d.B16\n", zn2*2, zn*2, zd*2)
			asmF("    VEOR   V%d.B16, V%d.B16, V%d.B16\n", zn2*2+1, zn*2+1, zd*2+1)
		} else if strings.Contains(line, " // mov z") {
			f := strings.Fields(line)
			zd := getZregister(f[len(f)-2])
			xn := getXregister(f[len(f)-1])
			asmF("    VMOV   R%d, V%d.B[0]\n", xn, zd*2)
		} else if strings.Contains(line, " // dup z") {
			f := strings.Fields(line)
			zd := getZregister(f[len(f)-2])
			zn := getZregister(f[len(f)-1])
			asmF("    VDUP   V%d.B[0], V%d.B16\n", zn*2, zd*2)
		} else if strings.Contains(line, " // add x") {
			f := strings.Fields(line)
			xd := getXregister(f[len(f)-3])
			if xd != getXregister(f[len(f)-2]) {
				panic("registers don't match")
			}
			if f[len(f)-1][0] == '#' {
				imm := getHashImm(f[len(f)-1])
				asmF("    ADD    $%d, R%d\n", imm, xd)
			} else {
				xn := getXregister(f[len(f)-1])
				asmF("    ADD    R%d, R%d\n", xn, xd)
			}
		} else if strings.Contains(line, " // subs x") {
			f := strings.Fields(line)
			xd := getXregister(f[len(f)-3])
			if xd != getXregister(f[len(f)-2]) {
				panic("registers don't match")
			}
			imm := getHashImm(f[len(f)-1])
			asmF("    SUBS $%d, R%d\n", imm, xd)
		} else if strings.Contains(line, " // lsr x") {
			f := strings.Fields(line)
			xd := getXregister(f[len(f)-3])
			if xd != getXregister(f[len(f)-2]) {
				panic("registers don't match")
			}
			imm := getHashImm(f[len(f)-1])
			asmF("    LSR  $%d, R%d\n", imm, xd)
		} else if strings.Contains(line, " // tst x") {
			f := strings.Fields(line)
			xd := getXregister(f[len(f)-2])
			xn := getXregister(f[len(f)-1])
			asmF("    TST  R%d, R%d\n", xn, xd)
		} else if strings.Contains(line, " // mov x") {
			f := strings.Fields(line)
			xd := getXregister(f[len(f)-2])
			imm := getHashImm(f[len(f)-1])
			asmF("    MOVD   $%d, R%d\n", imm, xd)
		} else if strings.HasSuffix(line, ":") ||
			strings.HasPrefix(line, "    BEQ") ||
			strings.HasPrefix(line, "    BNE") ||
			strings.HasPrefix(line, "TEXT ·mulSve") ||
			strings.HasPrefix(line, "// func mulSve") {
			line = strings.ReplaceAll(line, "Sve", "Neon")
			asmF("%s\n", line)
		} else if strings.Contains(line, "Requires: SVE") {
			line = strings.ReplaceAll(line, "SVE", "NEON")
			asmF("%s\n", line)
		} else if strings.Contains(line, " // ptrue p") {
			// intentionally drop line
		} else if strings.HasPrefix(line, "    // ") ||
			strings.HasPrefix(line, "    MOVD ") ||
			strings.HasPrefix(line, "    CMP ") ||
			strings.HasPrefix(line, "    RET") ||
			len(line) == 0 {
			asmF("%s\n", line)
		} else {
			panic(fmt.Sprintf("convert2Neon unsupported: `%s`", line))
		}
	}
}

func fixPostIncrementNeon(asmBuf *bytes.Buffer, lines []string) {

	asmF := func(format string, args ...interface{}) {
		(*asmBuf).WriteString(fmt.Sprintf(format, args...))
	}

	const MATRIX_BASE = "matrix_base"

	skipResetMatrixBase := false
	{
		routine := strings.Join(lines, "\n")
		reFramePtr := regexp.MustCompile(`MOVD\s*` + MATRIX_BASE + `\+\d*(\(FP\),\s*R\d*)`)

		if matches := reFramePtr.FindAllStringSubmatch(routine, -1); len(matches) == 1 {
			framePtrToDest := matches[0][1]

			// check if we're loading into register
			// more than once from the stack frame
			// (meaning we overwrite the 'matrix_base' value)
			escaped := strings.NewReplacer("(", `\(`, ")", `\)`).Replace(framePtrToDest)
			reSameDest := regexp.MustCompile(`MOVD\s*\w*\+\d*` + escaped)
			if m := reSameDest.FindAllStringSubmatch(routine, -1); len(m) == 2 {
				skipResetMatrixBase = true
			}
		}
	}

	isXor := false
	{
		routine := strings.Join(lines, "\n")
		isXor = strings.Count(routine, "Xor(SB)") > 0
	}

	resetMatrixBaseAtStartOfLoop := ""
	for i := 0; i < len(lines); i++ {

		if !skipResetMatrixBase {
			//
			// Since we are loading with post-increment,
			// reset register holding matrix array at
			// start of each loop
			//
			if strings.Contains(lines[i], MATRIX_BASE) {
				resetMatrixBaseAtStartOfLoop = lines[i]
				continue
			} else if strings.HasSuffix(lines[i], "_loop:") {
				asmF("%s\n", lines[i])
				asmF("%s\n", resetMatrixBaseAtStartOfLoop)
				resetMatrixBaseAtStartOfLoop = ""
				continue
			}
		}

		//
		// Remove the explicit ADDition of the
		// pointer to the shard (since we are already
		// using post-increments for the loads/stores)
		//
		if i < len(lines)-1 &&
			strings.Contains(lines[i], "32(R") &&
			strings.Contains(lines[i+1], "ADD") && strings.Contains(lines[i+1], "$32, R") {

			storing := strings.Contains(lines[i], "VST1.P")
			if storing && isXor {
				// move post-increment into a "pre-decrement" to offset
				// post-increment for loading of existing content in case of Xor-case
				asmF("%s\n", strings.ReplaceAll(lines[i+1], "ADD", "SUB"))
				asmF("%s\n", lines[i])
			} else {
				asmF("%s\n", lines[i])
				// intentionally skip line with ADD
			}
			i += 1
			continue
		}
		if i < len(lines)-2 &&
			strings.Contains(lines[i], "32(R") &&
			strings.Contains(lines[i+1], "32(R") &&
			strings.Contains(lines[i+2], "ADD") && strings.Contains(lines[i+2], "$64, R") {

			storing := strings.Contains(lines[i], "VST1.P") && strings.Contains(lines[i+1], "VST1.P")
			if storing && isXor {
				// move post-increment into a "pre-decrement" to offset
				// post-increment for loading of existing content in case of Xor-case
				asmF("%s\n", strings.ReplaceAll(lines[i+2], "ADD", "SUB"))
				asmF("%s\n", lines[i])
				asmF("%s\n", lines[i+1])
			} else {
				asmF("%s\n", lines[i])
				asmF("%s\n", lines[i+1])
				// intentionally skip line with ADD
			}
			i += 2
			continue
		}

		asmF("%s\n", lines[i])
	}
}

func genArmNeon() {
	const SVE_CODE = "../galois_gen_arm64.s"

	asmOut, goOut := &bytes.Buffer{}, &bytes.Buffer{}

	if asmSve, err := os.ReadFile(SVE_CODE); err != nil {
		log.Fatalf("Failed to read %s: %v", SVE_CODE, err)
	} else {
		// start with SVE code
		asmOut.WriteString(string(asmSve))
	}
	if goSve, err := os.ReadFile(strings.ReplaceAll(SVE_CODE, ".s", ".go")); err != nil {
		log.Fatalf("Failed to read %s: %v", SVE_CODE, err)
	} else {
		goOut.WriteString(string(goSve))
	}

	const input = 10

	// Processing 64 bytes variants
	for output := 1; output <= 3; output++ {
		for op := ""; len(op) <= 3; op += "Xor" {
			templName := fmt.Sprintf("mulSve_%dx%d_64%s", input, output, op)
			funcDef := fmt.Sprintf("func %s(matrix []byte, in [][]byte, out [][]byte, start int, n int)", strings.ReplaceAll(templName, "Sve", "Neon"))

			lines, err := extractRoutine(SVE_CODE, fmt.Sprintf("TEXT ·%s(SB)", templName))
			if err != nil {
				log.Fatal(err)
			}

			// prepend output with commented out function definition and comment
			asmOut.WriteString(fmt.Sprintf("// %s\n", funcDef))
			asmOut.WriteString("// Requires: NEON\n")

			{
				asmTemp := &bytes.Buffer{}
				convert2Neon(asmTemp, lines)
				fixPostIncrementNeon(asmOut, strings.Split(string(asmTemp.Bytes()), "\n"))
			}

			// golang declaration
			goOut.WriteString(fmt.Sprintf("//go:noescape\n%s\n\n", funcDef))
		}
	}

	// Processing 32 bytes variants
	for output := 4; output <= 10; output++ {
		for op := ""; len(op) <= 3; op += "Xor" {
			templName := fmt.Sprintf("mulSve_%dx%d%s", input, output, op)
			funcDef := fmt.Sprintf("func %s(matrix []byte, in [][]byte, out [][]byte, start int, n int)", strings.ReplaceAll(templName, "Sve", "Neon"))

			lines, err := extractRoutine(SVE_CODE, fmt.Sprintf("TEXT ·%s(SB)", templName))
			if err != nil {
				log.Fatal(err)
			}

			// prepend output with commented out function definition and comment
			asmOut.WriteString(fmt.Sprintf("// %s\n", funcDef))
			asmOut.WriteString("// Requires: NEON\n")

			{
				asmTemp := &bytes.Buffer{}
				convert2Neon(asmTemp, lines)
				fixPostIncrementNeon(asmOut, strings.Split(string(asmTemp.Bytes()), "\n"))
			}

			// golang declaration
			goOut.WriteString(fmt.Sprintf("//go:noescape\n%s\n", funcDef))

			if !(output == 10 && op == "Xor") {
				goOut.WriteString("\n")
			}
		}
	}
	if err := os.WriteFile("../galois_gen_arm64.s", asmOut.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("../galois_gen_arm64.go", goOut.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}
}
