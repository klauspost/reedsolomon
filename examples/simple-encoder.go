//go:build ignore
// +build ignore

// Copyright 2015, Klaus Post, see LICENSE for details.
//
// Simple encoder example
//
// The encoder encodes a simple file into a number of shards
// To reverse the process see "simpledecoder.go"
//
// To build an executable use:
//
// go build simple-decoder.go
//
// Simple Encoder/Decoder Shortcomings:
// * If the file size of the input isn't divisible by the number of data shards
//   the output will contain extra zeroes
//
// * If the shard numbers isn't the same for the decoder as in the
//   encoder, invalid output will be generated.
//
// * If values have changed in a shard, it cannot be reconstructed.
//
// * If two shards have been swapped, reconstruction will always fail.
//   You need to supply the shards in the same order as they were given to you.
//
// The solution for this is to save a metadata file containing:
//
// * File size.
// * The number of data/parity shards.
// * HASH of each shard.
// * Order of the shards.
//
// If you save these properties, you should abe able to detect file corruption
// in a shard and be able to reconstruct your data if you have the needed number of shards left.

package main

import (
	"bufio"
	sha
	"encoding/binary"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/klauspost/reedsolomon"
)

var dataShards = flag.Int("data", 4, "Number of shards to split the data into, must be below 257.")
var parShards = flag.Int("par", 2, "Number of parity shards")
var outDir = flag.String("out", "", "Alternative output directory")

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  simple-encoder [-flags] filename.ext\n\n")
		fmt.Fprintf(os.Stderr, "Valid flags:\n")
		flag.PrintDefaults()
	}
}

func main() {
	// Parse command line parameters.
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Error: No input filename given\n")
		flag.Usage()
		os.Exit(1)
	}
	if (*dataShards + *parShards) > 256 {
		fmt.Fprintf(os.Stderr, "Error: sum of data and parity shards cannot exceed 256\n")
		os.Exit(1)
	}
	fname := args[0]

	// Create encoding matrix.
	enc, err := reedsolomon.New(*dataShards, *parShards)
	checkErr(err)

	fmt.Println("Opening", fname)
	b, err := ioutil.ReadFile(fname)
	checkErr(err)

	// Split the file into equally sized shards.
	shards, err := enc.Split(b)
	checkErr(err)
	fmt.Printf("File split into %d data+parity shards with %d bytes/shard.\n", len(shards), len(shards[0]))

	// Encode parity
	err = enc.Encode(shards)
	checkErr(err)

	// Write out the resulting files.
	dir, file := filepath.Split(fname)
	if *outDir != "" {
		dir = *outDir
	}
	for i, shard := range shards {
		outfn := fmt.Sprintf("%s.%d", file, i)

		fmt.Println("Writing to: ", outfn)
		//		err = os.WriteFile(filepath.Join(dir, outfn), shard, 0644)
		err = WriteShard(int32(i), int32(len(shards)), filepath.Join(dir, outfn), shard)
		checkErr(err)
	}
}

func WriteShard(i int32, numShards int32, numShardsfilename string, data []byte) error {

	h := sha256.New()
	h.Write(data)
	fmt.Printf("sha256 of file %s (shared %d of %d) %x\n", numShardsfilename, i+1, numShards, h.Sum(nil))
	file, err := os.Create(numShardsfilename)
	if err != nil {
		log.Fatal(err)
	}
	// Use a magic number so we can recognize files we wrote
	header := []int32{2147483647, i, numShards, int32(len(data))}

	writer := bufio.NewWriter(file)

	for _, value := range header {
		err := binary.Write(writer, binary.LittleEndian, value)
		if err != nil {
			fmt.Println("Error writing int32:", err)
			return err
		}
	}
	err = binary.Write(writer, binary.LittleEndian, h.Sum(nil))
	if err != nil {
		fmt.Println("Error writing checksum:", err)
		return err
	}

	if _, err = writer.Write(data); err != nil {
		fmt.Printf("Write failed for %s with %s\n", numShardsfilename, err)
	}

	file.Close()
	return nil
}

func checkErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(2)
	}
}
