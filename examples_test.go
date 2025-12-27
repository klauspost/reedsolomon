package reedsolomon_test

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math/rand"

	"github.com/klauspost/reedsolomon"
)

func fillRandom(p []byte) {
	for i := 0; i < len(p); i += 7 {
		val := rand.Int63()
		for j := 0; i+j < len(p) && j < 7; j++ {
			p[i+j] = byte(val)
			val >>= 8
		}
	}
}

// Simple example of how to use all functions of the Encoder.
// Note that all error checks have been removed to keep it short.
func ExampleEncoder() {
	// Create some sample data
	var data = make([]byte, 250000)
	fillRandom(data)

	// Create an encoder with 17 data and 3 parity slices.
	enc, _ := reedsolomon.New(17, 3)

	// Split the data into shards
	shards, _ := enc.Split(data)

	// Encode the parity set
	_ = enc.Encode(shards)

	// Verify the parity set
	ok, _ := enc.Verify(shards)
	if ok {
		fmt.Println("ok")
	}

	// Delete two shards
	shards[10], shards[11] = nil, nil

	// Reconstruct the shards
	_ = enc.Reconstruct(shards)

	// Verify the data set
	ok, _ = enc.Verify(shards)
	if ok {
		fmt.Println("ok")
	}
	// Output: ok
	// ok
}

// Simple example of how to use all functions of the EncoderIdx.
// Note that all error checks have been removed to keep it short.
func ExampleEncoder_EncodeIdx() {
	const dataShards = 7
	const erasureShards = 3

	// Create some sample data
	var data = make([]byte, 250000)
	fillRandom(data)

	// Create an encoder with 7 data and 3 parity slices.
	enc, _ := reedsolomon.New(dataShards, erasureShards)

	// Split the data into shards
	shards, _ := enc.Split(data)

	// Zero erasure shards.
	for i := range erasureShards {
		clear := shards[dataShards+i]
		for j := range clear {
			clear[j] = 0
		}
	}

	for i := range dataShards {
		// Encode one shard at the time.
		// Note how this gives linear access.
		// There is however no requirement on shards being delivered in order.
		// All parity shards will be updated on each run.
		_ = enc.EncodeIdx(shards[i], i, shards[dataShards:])
	}

	// Verify the parity set
	ok, err := enc.Verify(shards)
	if ok {
		fmt.Println("ok")
	} else {
		fmt.Println(err)
	}

	// Delete two shards
	shards[dataShards-2], shards[dataShards-2] = nil, nil

	// Reconstruct the shards
	_ = enc.Reconstruct(shards)

	// Verify the data set
	ok, err = enc.Verify(shards)
	if ok {
		fmt.Println("ok")
	} else {
		fmt.Println(err)
	}
	// Output: ok
	// ok
}

// This demonstrates that shards can be arbitrary sliced and
// merged and still remain valid.
func ExampleEncoder_slicing() {
	// Create some sample data
	var data = make([]byte, 250000)
	fillRandom(data)

	// Create 5 data slices of 50000 elements each
	enc, _ := reedsolomon.New(5, 3)
	shards, _ := enc.Split(data)
	err := enc.Encode(shards)
	if err != nil {
		panic(err)
	}

	// Check that it verifies
	ok, err := enc.Verify(shards)
	if ok && err == nil {
		fmt.Println("encode ok")
	}

	// Split the data set of 50000 elements into two of 25000
	splitA := make([][]byte, 8)
	splitB := make([][]byte, 8)

	// Merge into a 100000 element set
	merged := make([][]byte, 8)

	// Split/merge the shards
	for i := range shards {
		splitA[i] = shards[i][:25000]
		splitB[i] = shards[i][25000:]

		// Concencate it to itself
		merged[i] = append(make([]byte, 0, len(shards[i])*2), shards[i]...)
		merged[i] = append(merged[i], shards[i]...)
	}

	// Each part should still verify as ok.
	ok, err = enc.Verify(shards)
	if ok && err == nil {
		fmt.Println("splitA ok")
	}

	ok, err = enc.Verify(splitB)
	if ok && err == nil {
		fmt.Println("splitB ok")
	}

	ok, err = enc.Verify(merged)
	if ok && err == nil {
		fmt.Println("merge ok")
	}
	// Output: encode ok
	// splitA ok
	// splitB ok
	// merge ok
}

// This demonstrates that shards can xor'ed and
// still remain a valid set.
//
// The xor value must be the same for element 'n' in each shard,
// except if you xor with a similar sized encoded shard set.
func ExampleEncoder_xor() {
	// Create some sample data
	var data = make([]byte, 25000)
	fillRandom(data)

	// Create 5 data slices of 5000 elements each
	enc, _ := reedsolomon.New(5, 3)
	shards, _ := enc.Split(data)
	err := enc.Encode(shards)
	if err != nil {
		panic(err)
	}

	// Check that it verifies
	ok, err := enc.Verify(shards)
	if !ok || err != nil {
		fmt.Println("falied initial verify", err)
	}

	// Create an xor'ed set
	xored := make([][]byte, 8)

	// We xor by the index, so you can see that the xor can change,
	// It should however be constant vertically through your slices.
	for i := range shards {
		xored[i] = make([]byte, len(shards[i]))
		for j := range xored[i] {
			xored[i][j] = shards[i][j] ^ byte(j&0xff)
		}
	}

	// Each part should still verify as ok.
	ok, err = enc.Verify(xored)
	if ok && err == nil {
		fmt.Println("verified ok after xor")
	}
	// Output: verified ok after xor
}

// This will show a simple stream encoder where we encode from
// a []io.Reader which contain a reader for each shard.
//
// Input and output can be exchanged with files, network streams
// or what may suit your needs.
func ExampleStreamEncoder() {
	dataShards := 5
	parityShards := 2

	// Create a StreamEncoder with the number of data and
	// parity shards.
	rs, err := reedsolomon.NewStream(dataShards, parityShards)
	if err != nil {
		log.Fatal(err)
	}

	shardSize := 50000

	// Create input data shards.
	input := make([][]byte, dataShards)
	for s := range input {
		input[s] = make([]byte, shardSize)
		fillRandom(input[s])
	}

	// Convert our buffers to io.Readers
	readers := make([]io.Reader, dataShards)
	for i := range readers {
		readers[i] = io.Reader(bytes.NewBuffer(input[i]))
	}

	// Create our output io.Writers
	out := make([]io.Writer, parityShards)
	for i := range out {
		out[i] = io.Discard
	}

	// Encode from input to output.
	err = rs.Encode(readers, out)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("ok")
	// OUTPUT: ok
}

// This shows how to use DecodeIdx to progressively reconstruct any shard,
// including parity shards. DecodeIdx allows you to accumulate reconstruction
// data from multiple input shards, which is useful for streaming scenarios
// or when inputs arrive at different times.
func ExampleExtensions_DecodeIdx() {
	const dataShards = 5
	const parityShards = 3
	const totalShards = dataShards + parityShards
	const shardSize = 50000

	// Create encoder
	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		log.Fatal(err)
	}

	// DecodeIdx is available through the Extensions interface
	r := enc.(reedsolomon.Extensions)

	// Create some sample data
	var data = make([]byte, dataShards*shardSize)
	fillRandom(data)

	// Split into shards and encode
	shards, err := enc.Split(data)
	if err != nil {
		log.Fatal(err)
	}
	err = enc.Encode(shards)
	if err != nil {
		log.Fatal(err)
	}

	// Save original parity for verification
	originalParity := make([]byte, shardSize)
	copy(originalParity, shards[dataShards]) // Copy first parity shard

	// Simulate losing the first parity shard and some data shards
	lostShards := []int{1, 3, dataShards} // data shards 1,3 and parity shard 0
	for _, idx := range lostShards {
		shards[idx] = nil
	}

	// Reconstruct the first parity shard progressively using DecodeIdx
	// We'll use the remaining available shards: 0, 2, 4 (data) + 6, 7 (parity)
	availableShards := []int{0, 2, 4, dataShards + 1, dataShards + 2}

	// Set up expectInput to indicate which shards we plan to use
	expectInput := make([]bool, totalShards)
	for _, idx := range availableShards {
		expectInput[idx] = true
	}

	// Progressive reconstruction of parity shard 0
	targetShard := dataShards // First parity shard
	dst := make([]byte, shardSize)

	// Add each available shard progressively
	for _, inputIdx := range availableShards {
		err = r.DecodeIdx(dst, targetShard, expectInput, shards[inputIdx], inputIdx)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Verify reconstruction
	if bytes.Equal(dst, originalParity) {
		fmt.Println("Parity shard reconstructed successfully")
	}

	// DecodeIdx can also reconstruct data shards the same way
	// Let's reconstruct data shard 1
	dst2 := make([]byte, shardSize)
	targetShard2 := 1 // Data shard 1

	for _, inputIdx := range availableShards {
		err = r.DecodeIdx(dst2, targetShard2, expectInput, shards[inputIdx], inputIdx)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Split original data to check
	originalShards, _ := enc.Split(data)
	if bytes.Equal(dst2, originalShards[1]) {
		fmt.Println("Data shard reconstructed successfully")
	}

	fmt.Println("Both reconstructions completed")
	// Output: Parity shard reconstructed successfully
	// Data shard reconstructed successfully
	// Both reconstructions completed
}
