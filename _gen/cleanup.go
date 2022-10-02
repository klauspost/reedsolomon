//go:build custom
// +build custom

// Copyright 2022+, Klaus Post. See LICENSE for details.

package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/klauspost/asmfmt"
)

func main() {
	flag.Parse()
	args := flag.Args()
	for _, file := range args {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			log.Fatalln(err)
		}
		data = bytes.ReplaceAll(data, []byte("\t// #"), []byte("#"))
		data = bytes.ReplaceAll(data, []byte("\t// @"), []byte(""))
		data = bytes.ReplaceAll(data, []byte("VPTERNLOGQ"), []byte("XOR3WAY("))
		split := bytes.Split(data, []byte("\n"))
		// Add closing ')'
		want := []byte("\tXOR3WAY(")
		for i, b := range split {
			if bytes.Contains(b, want) {
				b = []byte(string(b) + ")")
				split[i] = b
			}
		}
		data = bytes.Join(split, []byte("\n"))
		data, err = asmfmt.Format(bytes.NewBuffer(data))
		if err != nil {
			log.Fatalln(err)
		}
		err = ioutil.WriteFile(file, data, os.ModePerm)
		if err != nil {
			log.Fatalln(err)
		}
	}
}
