//go:build ignore
// +build ignore

package main

import (
	"flag"
	"log"
	"strconv"

	"github.com/klauspost/cpuid/v2"
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		log.Fatalln("Supply CPU level 1-4 to test as argument")
	}
	l, err := strconv.Atoi(args[0])
	if err != nil {
		log.Fatalln("Unable to parse level:", err)
	}
	if l < 1 || l > 4 {
		log.Fatalln("Supply CPU level 1-4 to test as argument")
	}
	if cpuid.CPU.X64Level() < l {
		// Does os.Exit(1)
		log.Fatalln("CPU level not supported")
	}
}
