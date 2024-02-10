//go:build ignore
// +build ignore

// Copyright 2015, Klaus Post, see LICENSE for details.
//
// Simple decoder example.
//
// The decoder reverses the process of "simple-encoder.go"
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
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/klauspost/reedsolomon"
)

var dataShards = flag.Int("data", 4, "Number of shards to split the data into")
var parShards = flag.Int("par", 2, "Number of parity shards")
var outFile = flag.String("out", "", "Alternative output path/file")

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  simple-decoder [-flags] basefile.ext\nDo not add the number to the filename.\n")
		fmt.Fprintf(os.Stderr, "Valid flags:\n")
		flag.PrintDefaults()
	}
}

func verifyShard(shard []byte, i int32, magic int32, numShard int32, maxShard int32, length int32, checksum [32]byte) error {
	if int32(len(shard)) != length {
		fmt.Printf("Shard length was %d, shard in file is %d, file corrupt\n", length, len(shard))
		return errors.New("input string is not of the required length of 10")
	} else {
		fmt.Println("Shard was correct length")
	}
	if magic != 2147483647 {
		fmt.Println("Shard has INCORRECT magic value")
		return errors.New("Invalid shard magic number")
	} else {
		fmt.Println("Shard has correct magic value")
	}
	h := sha256.New()
	h.Write(shard)
	if !bytes.Equal(h.Sum(nil), checksum[:]) {
		fmt.Println("Shard has INCORRECT checksum")
		return errors.New("Checksum mismatch")
	} else {
		fmt.Println("Shard has correct checksum")
	}
	if i == numShard {
		fmt.Println("Shared was in the right order")
	} else {
		fmt.Printf("Shard was in the wrong order, shard %d of %d expected, found %d\n", i, maxShard, numShard)
		return errors.New("Shared is in wrong order")
	}
	return nil
}

func main() {
	// Parse flags
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Error: No filenames given\n")
		flag.Usage()
		os.Exit(1)
	}
	fname := args[0]

	// Create matrix
	enc, err := reedsolomon.New(*dataShards, *parShards)
	checkErr(err)

	// Create shards and load the data.
	shards := make([][]byte, *dataShards+*parShards)
	for i := range shards {
		infn := fmt.Sprintf("%s.%d", fname, i)
		fmt.Println("Opening", infn)
		file, err := os.Open(infn)
		if err != nil {
			fmt.Println("Error opening file", err)
			os.Exit(1)
		}

		var header [4]int32

		for i := 0; i < 4; i++ {
			err = binary.Read(file, binary.LittleEndian, &header[i])
		}
		if err != nil {
			log.Fatal(err)
		}
		var checksum [256 / 8]byte

		err = binary.Read(file, binary.LittleEndian, &checksum)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("checksum=%x\n", checksum)
		shards[i], err = ioutil.ReadAll(file)
		if err != nil {
			fmt.Println("Error reading file", err)
			shards[i] = nil
		}

		err = verifyShard(shards[i], int32(i), header[0], header[1], header[2], header[3], checksum)
		if err != nil {
			fmt.Printf("Shard FAILED, removing from reconstruction\n")
			shards[i] = nil
		}
	}

	// Verify the shards
	ok, err := enc.Verify(shards)
	if ok {
		fmt.Println("No reconstruction needed")
	} else {
		fmt.Println("Verification failed. Reconstructing data")
		err = enc.Reconstruct(shards)
		if err != nil {
			fmt.Println("Reconstruct failed -", err)
			os.Exit(1)
		}
		ok, err = enc.Verify(shards)
		if !ok {
			fmt.Println("Verification failed after reconstruction, data likely corrupted.")
			os.Exit(1)
		}
		checkErr(err)
	}

	// Join the shards and write them
	outfn := *outFile
	if outfn == "" {
		outfn = fname
	}

	fmt.Println("Writing data to", outfn)
	f, err := os.Create(outfn)
	checkErr(err)

	// We don't know the exact filesize.
	err = enc.Join(f, shards, len(shards[0])**dataShards)
	checkErr(err)
}

func checkErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(2)
	}
}
