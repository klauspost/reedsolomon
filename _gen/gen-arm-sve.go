// Copyright 2024, Klaus Post/Minio Inc. See LICENSE for details.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	avxtwo2sve "github.com/fwessels/avxTwo2sve"
	sve_as "github.com/fwessels/sve-as"
)

func patchLabel(line string) string {
	return strings.ReplaceAll(line, "AvxTwo", "Sve")
}

func extractRoutine(filename, routine string) (lines []string, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()

	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	// Iterate over each line
	collect := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, routine) {
			collect = true
		}
		if collect {
			lines = append(lines, line)
		}
		if collect && strings.HasSuffix(line, "RET") {
			collect = false
		}
	}

	// Check for any errors that occurred during scanning
	err = scanner.Err()
	return
}

func addArmInitializations(instructions []string) (processed []string) {
	for _, instr := range instructions {
		processed = append(processed, instr)
		if strings.HasPrefix(instr, "TEXT ·") {
			sve := "ptrue p0.d"
			opcode, err := sve_as.Assemble(sve)
			if err != nil {
				processed = append(processed, fmt.Sprintf("    WORD $0x00000000 // %-44s\n", sve))
			} else {
				processed = append(processed, fmt.Sprintf("    WORD $0x%08x // %-44s\n", opcode, sve))
			}
		}
	}
	return
}

// Expand #defines
func expandHashDefines(instructions []string) (processed []string) {
	for _, instr := range instructions {
		if strings.Contains(instr, "XOR3WAY") {
			f := strings.Fields(instr)
			if len(f) >= 3 {
				dst := strings.ReplaceAll(f[len(f)-1], ")", "")
				b := strings.ReplaceAll(f[len(f)-2], ",", "")
				a := strings.ReplaceAll(f[len(f)-3], ",", "")

				processed = append(processed, fmt.Sprintf("VPXOR %s, %s, %s", a, dst, dst))
				processed = append(processed, fmt.Sprintf("VPXOR %s, %s, %s", b, dst, dst))
			} else {
				log.Fatalf("Not enough arguments for 'XOR3WAY' macro: %d", len(f))
			}
		} else if !strings.Contains(instr, "VZEROUPPER") {
			processed = append(processed, instr)
		}
	}
	return
}

func convertRoutine(asmBuf *bytes.Buffer, instructions []string) {

	asmF := func(format string, args ...interface{}) {
		(*asmBuf).WriteString(fmt.Sprintf(format, args...))
	}

	wordOpcode := regexp.MustCompile(`WORD \$0x[0-9a-f]{8}`)

	for _, instr := range instructions {
		instr = strings.TrimSpace(instr)
		if instr == "" {
			asmF("\n")
		} else if strings.HasPrefix(instr, "TEXT ") { // function header
			asmF("%s\n", patchLabel(instr))
		} else if wordOpcode.MatchString(instr) { // arm code
			asmF("    %s\n", instr)
		} else if strings.HasPrefix(instr, "//") { // comment
			asmF("    %s\n", instr)
		} else if strings.HasSuffix(instr, ":") { // label
			asmF("%s\n", patchLabel(instr))
		} else {
			sve, plan9, err := avxtwo2sve.AvxTwo2Sve(instr, patchLabel)
			if err != nil {
				panic(err)
			} else if !plan9 {
				opcode, err := sve_as.Assemble(sve)
				if err != nil {
					asmF("    WORD $0x00000000 // %-44s\n", sve)
				} else {
					asmF("    WORD $0x%08x // %-44s\n", opcode, sve)
				}
			} else {
				asmF("    %s\n", sve)
			}
		}
	}
}

// convert (R..*1) memory accesses into (R..*8) offsets
func patchScaledLoads(code string, outputs int, isXor bool) (patched []string) {

	scaledMemOps := strings.Count(code, "*1)")
	if scaledMemOps == 0 {
		// in case of no scaled loads, exit out early
		return strings.Split(code, "\n")
	}

	sanityCheck := outputs
	if isXor {
		sanityCheck *= 2 // need to load all values as well as store them
	}
	if scaledMemOps != sanityCheck {
		panic("Couldn't find expected number of scaled memory ops")
	}

	scaledReg := ""
	re := regexp.MustCompile(`R(\d+)\*1`)
	if match := re.FindStringSubmatch(code); len(match) > 1 {
		scaledReg = fmt.Sprintf("R%s", match[1])
	} else {
		panic("Failed to find register used for scaled memory ops")
	}

	const inputs = 10

	scaledRegUses := strings.Count(code, scaledReg)
	sanityCheck += inputs // needed to add start offset to input
	sanityCheck += 1      // needed to load offset from stack
	sanityCheck += 1      // needed to increment offset

	if scaledRegUses != sanityCheck {
		panic("Did not find expected number of uses of scaled register")
	}

	// Adjust all scaled loads
	code = strings.ReplaceAll(code, fmt.Sprintf("(%s*1)", scaledReg), fmt.Sprintf("(%s*8)", scaledReg))

	// Adjust increment at end of loop
	reAdd := regexp.MustCompile(`ADDQ\s*\$(0x[0-9a-f]+),\s*` + scaledReg)
	if match := reAdd.FindStringSubmatch(code); len(match) > 1 && match[1][:2] == "0x" {
		if increment, err := strconv.ParseInt(match[1][2:], 16, 64); err == nil {
			code = strings.ReplaceAll(code, fmt.Sprintf("0x%x, %s", increment, scaledReg), fmt.Sprintf("0x%02x, %s", increment>>3, scaledReg))
		} else {
			panic(err)
		}
	} else {
		panic("Failed to find increment of offset")
	}

	// Add shift instruction during initialization after inputs have been adjusted
	reShift := regexp.MustCompilePOSIX(fmt.Sprintf(`^[[:blank:]]+ADDQ[[:blank:]]+%s.*$`, scaledReg))
	if matches := reShift.FindAllStringIndex(code, -1); len(matches) == inputs {
		lastInpIncr := code[matches[inputs-1][0]:matches[inputs-1][1]]
		shiftCorrection := strings.ReplaceAll(strings.Split(lastInpIncr, scaledReg)[0], "ADDQ", "SHRQ")
		shiftCorrection += "$0x03, " + scaledReg
		code = strings.ReplaceAll(code, lastInpIncr, lastInpIncr+"\n"+shiftCorrection)
	} else {
		fmt.Println(matches)
		panic("Did not find expected number start offset corrections")
	}

	return strings.Split(code, "\n")
}

func fromAvx2ToSve() {
	asmOut, goOut := &bytes.Buffer{}, &bytes.Buffer{}

	goOut.WriteString(`// Code generated by command: go generate ` + os.Getenv("GOFILE") + `. DO NOT EDIT.` + "\n\n")
	goOut.WriteString("//go:build !noasm && !appengine && !gccgo && !nopshufb\n\n")
	goOut.WriteString("package reedsolomon\n\n")

	const input = 10
	const AVX2_CODE = "../galois_gen_amd64.s"

	// Processing 64 bytes variants
	for output := 1; output <= 3; output++ {
		for op := ""; len(op) <= 3; op += "Xor" {
			templName := fmt.Sprintf("mulAvxTwo_%dx%d_64%s", input, output, op)
			funcDef := fmt.Sprintf("func %s(matrix []byte, in [][]byte, out [][]byte, start int, n int)", strings.ReplaceAll(templName, "AvxTwo", "Sve"))

			// asm first
			lines, err := extractRoutine(AVX2_CODE, fmt.Sprintf("TEXT ·%s(SB)", templName))
			if err != nil {
				log.Fatal(err)
			}
			lines = patchScaledLoads(strings.Join(lines, "\n"), output, strings.HasSuffix(templName, "Xor"))
			lines = expandHashDefines(lines)

			convertRoutine(asmOut, lines)

			// add newline after RET
			asmOut.WriteString("\n")

			// golang declaration
			goOut.WriteString(fmt.Sprintf("//go:noescape\n%s\n\n", funcDef))
		}
	}

	// Processing 32 bytes variants
	for output := 4; output <= 10; output++ {
		for op := ""; len(op) <= 3; op += "Xor" {
			templName := fmt.Sprintf("mulAvxTwo_%dx%d%s", input, output, op)
			funcDef := fmt.Sprintf("func %s(matrix []byte, in [][]byte, out [][]byte, start int, n int)", strings.ReplaceAll(templName, "AvxTwo", "Sve"))

			// asm first
			lines, err := extractRoutine(AVX2_CODE, fmt.Sprintf("TEXT ·%s(SB)", templName))
			if err != nil {
				log.Fatal(err)
			}
			lines = patchScaledLoads(strings.Join(lines, "\n"), output, strings.HasSuffix(templName, "Xor"))
			lines = expandHashDefines(lines)

			// add additional initialization for SVE
			// (for predicated loads and stores in
			//  case of register shortage)
			lines = addArmInitializations(lines)

			convertRoutine(asmOut, lines)

			// add newline after RET
			asmOut.WriteString("\n")

			// golang declaration
			goOut.WriteString(fmt.Sprintf("//go:noescape\n%s\n\n", funcDef))
		}
	}

	if err := os.WriteFile("../galois_gen_arm64.s", asmOut.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("../galois_gen_arm64.go", goOut.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}
}

func insertEarlyExit(lines []string, funcName string, outputs int) (processed []string) {

	const reg = "R16"
	label := funcName + "_store"

	reComment := regexp.MustCompile(fmt.Sprintf(`// Load and process \d* bytes from input (\d*) to %d outputs`, outputs))
	reLoop := regexp.MustCompile(`^` + strings.ReplaceAll(label, "store", "loop") + `:`)
	reStore := regexp.MustCompile(fmt.Sprintf(`// Store %d outputs`, outputs))

	for _, line := range lines {
		if matches := reLoop.FindAllStringSubmatch(line, -1); len(matches) == 1 {
			lastline := processed[len(processed)-1]
			processed = processed[:len(processed)-1]
			processed = append(processed, "")
			processed = append(processed, fmt.Sprintf("    // Load number of input shards"))
			processed = append(processed, fmt.Sprintf("    MOVD   in_len+32(FP), %s", reg))
			processed = append(processed, lastline)
		}

		if matches := reComment.FindAllStringSubmatch(line, -1); len(matches) == 1 {
			if inputs, err := strconv.Atoi(matches[0][1]); err != nil {
				panic(err)
			} else {
				if inputs > 0 && inputs < 10 {
					lastline := processed[len(processed)-1]
					processed = processed[:len(processed)-1]
					processed = append(processed, fmt.Sprintf("    // Check for early termination"))
					processed = append(processed, fmt.Sprintf("    CMP    $%d, %s", inputs, reg))
					processed = append(processed, fmt.Sprintf("    BEQ    %s", label))
					processed = append(processed, lastline)
				}
			}
		}

		if matches := reStore.FindAllStringSubmatch(line, -1); len(matches) == 1 {
			processed = append(processed, fmt.Sprintf("%s:", label))
		}

		processed = append(processed, line)
	}
	return
}

func addEarlyExit(arch string) {
	const filename = "../galois_gen_arm64.s"
	asmOut := &bytes.Buffer{}

	asmOut.WriteString(`// Code generated by command: go generate ` + os.Getenv("GOFILE") + `. DO NOT EDIT.` + "\n\n")
	asmOut.WriteString("//go:build !appengine && !noasm && !nogen && !nopshufb && gc\n\n")
	asmOut.WriteString(`#include "textflag.h"` + "\n\n")

	input := 10
	for outputs := 1; outputs <= 3; outputs++ {
		for op := ""; len(op) <= 3; op += "Xor" {
			funcName := fmt.Sprintf("mul%s_%dx%d_64%s", arch, input, outputs, op)
			funcDef := fmt.Sprintf("func %s(matrix []byte, in [][]byte, out [][]byte, start int, n int)", funcName)

			lines, _ := extractRoutine(filename, fmt.Sprintf("TEXT ·%s(SB)", funcName))

			// prepend output with commented out function definition and comment
			asmOut.WriteString(fmt.Sprintf("// %s\n", funcDef))
			asmOut.WriteString("// Requires: SVE\n")

			lines = insertEarlyExit(lines, funcName, outputs)

			asmOut.WriteString(strings.Join(lines, "\n"))
			asmOut.WriteString("\n\n")
		}
	}

	for outputs := 4; outputs <= 10; outputs++ {
		for op := ""; len(op) <= 3; op += "Xor" {
			funcName := fmt.Sprintf("mul%s_%dx%d%s", arch, input, outputs, op)
			funcDef := fmt.Sprintf("func %s(matrix []byte, in [][]byte, out [][]byte, start int, n int)", funcName)

			lines, _ := extractRoutine(filename, fmt.Sprintf("TEXT ·%s(SB)", funcName))

			// prepend output with commented out function definition and comment
			asmOut.WriteString(fmt.Sprintf("// %s\n", funcDef))
			asmOut.WriteString("// Requires: SVE\n")

			lines = insertEarlyExit(lines, funcName, outputs)
			asmOut.WriteString(strings.Join(lines, "\n"))
			asmOut.WriteString("\n\n")
		}
	}

	if err := os.WriteFile("../galois_gen_arm64.s", asmOut.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}
}

func genArmSve() {
	fromAvx2ToSve()
	addEarlyExit("Sve")
}

func assemble(sve string) string {
	opcode, err := sve_as.Assemble(sve)
	if err != nil {
		return fmt.Sprintf("    WORD $0x00000000 // %s", sve)
	} else {
		return fmt.Sprintf("    WORD $0x%08x // %s", opcode, sve)
	}
}

func addArmSveVectorLength() (addInits []string) {
	const filename = "../galois_gen_arm64.s"
	asmOut := &bytes.Buffer{}

	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()

	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	routine := ""
	addInits = make([]string, 0)

	// Iterate over each line
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "TEXT ·") {
			routine = line
		}

		correctShift := func(shift, vl string) {
			if strings.Contains(line, " // lsr ") && strings.HasSuffix(strings.TrimSpace(line), ", "+shift) {
				instr := strings.Split(strings.TrimSpace(line), "// lsr ")[1]
				args := strings.Split(instr, ", ")
				if len(args) == 3 && args[0] == args[1] {
					// keep the original right shift, but reverse the effect (so effectively
					// clearing out the lower bits so we cannot do eg. "half loops" )
					line += "\n"
					line += assemble(fmt.Sprintf("lsl %s, %s, %s", args[0], args[1], shift)) + "\n"
					line += assemble(fmt.Sprintf("rdvl x16, %s", vl)) + "\n"
					line += assemble(fmt.Sprintf("udiv %s, %s, x16", args[0], args[1]))
				}
			}
		}

		correctShift("#6", "#2")
		correctShift("#5", "#1")

		if strings.Contains(line, " // add ") && strings.HasSuffix(strings.TrimSpace(line), "#64") {
			instr := strings.Split(strings.TrimSpace(line), "// add ")[1]
			args := strings.Split(instr, ", ")
			if len(args) == 3 && args[0] == args[1] {
				line = assemble(fmt.Sprintf("addvl %s, %s, #2", args[0], args[1]))
			}
		}

		if strings.Contains(line, " // add ") && strings.HasSuffix(strings.TrimSpace(line), "#32") {
			instr := strings.Split(strings.TrimSpace(line), "// add ")[1]
			args := strings.Split(instr, ", ")
			if len(args) == 3 && args[0] == args[1] {
				line = assemble(fmt.Sprintf("addvl %s, %s, #1", args[0], args[1]))
			}
		}

		if strings.Contains(line, " // add ") && strings.HasSuffix(strings.TrimSpace(line), "#4") {
			// mark routine as needing initialization of register 17
			addInits = append(addInits, routine)
			line = assemble("add x15, x15, x17")
		}

		asmOut.WriteString(line + "\n")
	}

	// Check for any errors that occurred during scanning
	if err = scanner.Err(); err != nil {
		log.Fatal(err)
	} else if err = os.WriteFile("../galois_gen_arm64.s", asmOut.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}

	return
}

func addArmSveInitializations(addInits []string) {

	const filename = "../galois_gen_arm64.s"
	asmOut := &bytes.Buffer{}

	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()

	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)
	routine := ""
	checkNextLine := false

	// Iterate over each line
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "TEXT ·") {
			routine = line
		}

		if strings.Contains(line, "// Load number of input shards") {
			checkNextLine = true
		} else {
			if checkNextLine {
				idx := slices.IndexFunc(addInits, func(s string) bool { return s == routine })
				if idx != -1 {
					line += "\n"
					line += assemble("rdvl x17, #1") + "\n"
					line += assemble("lsr  x17, x17, #3")
				}
				checkNextLine = false
			}
		}

		asmOut.WriteString(line + "\n")
	}

	// Check for any errors that occurred during scanning
	if err = scanner.Err(); err != nil {
		log.Fatal(err)
	} else if err = os.WriteFile("../galois_gen_arm64.s", asmOut.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}
}

func genArmSveAllVl() {
	addInits := addArmSveVectorLength()
	addArmSveInitializations(addInits)
}
