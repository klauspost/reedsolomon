//go:build custom
// +build custom

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
		data = bytes.Replace(data, []byte("\t// #"), []byte("#"), -1)
		data = bytes.Replace(data, []byte("\t// @"), []byte(""), -1)
		data = bytes.Replace(data, []byte("VPTERNLOGQ"), []byte("XOR3WAY("), -1)
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
