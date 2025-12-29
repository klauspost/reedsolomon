package reedsolomon_test

import (
	"bytes"
	"fmt"
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

func ExampleNew() {
	// Create an encoder with 17 data and 3 parity slices.
	enc, _ := reedsolomon.New(17, 3)

	_ = enc
}

func ExampleEncoder_Update() {
	// Create an encoder with 7 data and 3 parity slices.
	enc, _ := reedsolomon.New(7, 3)

	// Create a slice of data slices.
	data := make([][]byte, 10)

	// Create some data. All slices must have the same length.
	length := 10000
	for i := range data {
		data[i] = make([]byte, length)
		fillRandom(data[i])
	}

	// Encode the data.
	err := enc.Encode(data)
	if err != nil {
		log.Fatal(err)
	}
	// Data is ok.
	ok, err := enc.Verify(data)
	fmt.Println("data ok:", ok, "err:", err)

	// Change some data in shard 5.
	changedData := append([]byte{}, data[5]...)
	changedData[0], changedData[1], changedData[2] = 11, 22, 33

	// Re-encode the data with Update.
	newData := make([][]byte, 7)

	// Only provide the changed data shard
	newData[5] = changedData
	err = enc.Update(data, newData)
	if err != nil {
		log.Fatal(err)
	}
	// Update the data shard.
	data[5] = changedData

	// Data is ok.
	ok, err = enc.Verify(data)
	fmt.Println("data ok:", ok, "err:", err)

	// Output: data ok: true err: <nil>
	// data ok: true err: <nil>
}

func ExampleEncoder_Reconstruct() {
	// Create an encoder with 5 data and 3 parity slices.
	enc, _ := reedsolomon.New(5, 3)

	// Create a slice of data slices.
	data := make([][]byte, 8)

	// Create 5+3 slices of 50 bytes each.
	for i := range data {
		data[i] = make([]byte, 50)
	}

	// Fill in data slices with random data, leaving parity slices as zero.
	for i := 0; i < 5; i++ {
		fillRandom(data[i])
	}

	// Encode data.
	enc.Encode(data)

	// Delete a data slices (but preserve the original)
	original := append([][]byte{}, data...)
	data[1] = nil
	data[4] = nil

	// Reconstruct the missing data slices.
	enc.Reconstruct(data)

	// Verify that reconstruction was correct.
	ok := true
	for i := range data {
		if !bytes.Equal(original[i], data[i]) {
			ok = false
		}
	}
	fmt.Println("Reconstruction ok:", ok)
	// Output: Reconstruction ok: true
}

func ExampleEncoder_Split() {
	var data = make([]byte, 250000)
	fillRandom(data)

	// Create an encoder with 17 data and 3 parity slices.
	enc, _ := reedsolomon.New(17, 3)

	// Split the data into shards.
	shards, _ := enc.Split(data)

	// This is the output size
	outputSize := len(data)

	err := enc.Encode(shards)
	if err != nil {
		log.Fatal(err)
	}
	// Duplicate a data shard
	shards[10] = nil

	// Recover the missing shard
	err = enc.Reconstruct(shards)
	if err != nil {
		log.Fatal(err)
	}
	// Join the data shards back
	buf := new(bytes.Buffer)

	// We write the joined data to buf
	_ = enc.Join(buf, shards, outputSize)

	// buf.Bytes() now contains the original data.
	fmt.Println(bytes.Equal(buf.Bytes(), data))
	// Output: true
}

func ExampleEncoder_Verify() {
	// Create an encoder with 5 data and 3 parity slices.
	enc, _ := reedsolomon.New(5, 3)

	// Create a slice of data slices.
	data := make([][]byte, 8)

	// Create 5+3 slices of 5 bytes each.
	for i := range data {
		data[i] = make([]byte, 5)
	}

	// Fill in data slices with random data, leaving parity slices as zero.
	for i := 0; i < 5; i++ {
		fillRandom(data[i])
	}

	// Encode data.
	enc.Encode(data)

	// Verify that parity slices are correct.
	ok, _ := enc.Verify(data)
	fmt.Println("parity ok:", ok)

	// Corrupt a byte in a data shard.
	data[2][1]++

	// Verify that parity slices are correct.
	ok, _ = enc.Verify(data)
	fmt.Println("parity ok:", ok)
	// Output: parity ok: true
	// parity ok: false
}

func ExampleStreamEncoder() {
	dataShards := 5
	parShards := 2

	// Create encoder (StreamEncoder is deprecated, use regular encoder)
	enc, err := reedsolomon.New(dataShards, parShards)
	if err != nil {
		log.Fatal(err)
	}

	shardSize := 50000

	// Create shards.
	shards := make([][]byte, dataShards+parShards)
	for s := range shards {
		shards[s] = make([]byte, shardSize)
		if s < dataShards {
			fillRandom(shards[s])
		}
	}

	// Encode parity.
	err = enc.Encode(shards)
	if err != nil {
		log.Fatal(err)
	}

	// Verify the parity.
	ok, err := enc.Verify(shards)
	if ok {
		fmt.Println("verified ok")
	}

	// Recover 2 lost data shards.
	shards[0] = nil
	shards[2] = nil
	err = enc.Reconstruct(shards)
	if err != nil {
		log.Fatal(err)
	}

	// Verify the data after recovering.
	ok, err = enc.Verify(shards)
	if ok {
		fmt.Println("recovered ok")
	}
}

func ExampleEncoder_EncodeIdx() {
	dataShards := 5
	parityShards := 2

	// Create encoder
	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		log.Fatal(err)
	}

	shardSize := 50000

	// Create shards.
	shards := make([][]byte, dataShards+parityShards)
	for s := range shards {
		shards[s] = make([]byte, shardSize)
	}

	// Fill data shards with some data
	shards[0] = bytes.Repeat([]byte{0}, shardSize)
	shards[1] = bytes.Repeat([]byte{1}, shardSize)
	shards[2] = bytes.Repeat([]byte{2}, shardSize)
	shards[3] = bytes.Repeat([]byte{3}, shardSize)
	shards[4] = bytes.Repeat([]byte{4}, shardSize)

	// Encode parity, one data shard at the time using EncodeIdx.
	for i := 0; i < dataShards; i++ {
		err = enc.EncodeIdx(shards[i], i, shards[dataShards:])
		if err != nil {
			log.Fatal(err)
		}
	}

	// Verify the parity.
	ok, err := enc.Verify(shards)
	if err != nil {
		log.Fatal(err)
	}
	if ok {
		fmt.Println("encode ok")
	}

	// Output: encode ok
}

// This shows how to use DecodeIdx to progressively reconstruct shards,
// including both data and parity shards. The new signature allows
// reconstructing multiple shards simultaneously and merging partial decodings.
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
	ext := enc.(reedsolomon.Extensions)

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

	// Save originals for verification
	originals := make([][]byte, totalShards)
	for i := range shards {
		originals[i] = make([]byte, len(shards[i]))
		copy(originals[i], shards[i])
	}

	// Example 1: Reconstruct multiple shards at once
	// Simulate losing shards 0, 3 (data) and 7 (parity)
	shards[0] = nil
	shards[3] = nil
	shards[7] = nil

	// Set up dst with the shards we want to reconstruct
	dst := make([][]byte, totalShards)
	dst[0] = make([]byte, shardSize)
	dst[3] = make([]byte, shardSize)
	dst[7] = make([]byte, shardSize)

	// Mark which shards are available as input
	expectInput := make([]bool, totalShards)
	expectInput[1] = true
	expectInput[2] = true
	expectInput[4] = true
	expectInput[5] = true
	expectInput[6] = true

	// Provide the available shards
	input := make([][]byte, totalShards)
	input[1] = shards[1]
	input[2] = shards[2]
	input[4] = shards[4]

	// Reconstruct all missing shards in two calls
	err = ext.DecodeIdx(dst, expectInput, input)
	if err != nil {
		log.Fatal(err)
	}

	input = make([][]byte, totalShards)
	input[5] = shards[5]
	input[6] = shards[6]
	err = ext.DecodeIdx(dst, expectInput, input)
	if err != nil {
		log.Fatal(err)
	}

	// Verify reconstruction
	if bytes.Equal(dst[0], originals[0]) &&
		bytes.Equal(dst[3], originals[3]) &&
		bytes.Equal(dst[7], originals[7]) {
		fmt.Println("Multiple shards reconstructed successfully")
	}

	// Example 2: Progressive reconstruction with merging
	// Reset for progressive example
	dst2 := make([][]byte, totalShards)
	dst2[0] = make([]byte, shardSize)

	// First partial decode using shards 1-2
	input1 := make([][]byte, totalShards)
	input1[1] = shards[1]
	input1[2] = shards[2]

	err = ext.DecodeIdx(dst2, expectInput, input1)
	if err != nil {
		log.Fatal(err)
	}

	// Second partial decode using shards 4-6
	dst3 := make([][]byte, totalShards)
	dst3[0] = make([]byte, shardSize)

	input2 := make([][]byte, totalShards)
	input2[4] = shards[4]
	input2[5] = shards[5]
	input2[6] = shards[6]

	err = ext.DecodeIdx(dst3, expectInput, input2)
	if err != nil {
		log.Fatal(err)
	}

	// Merge the two partial decodings using nil expectInput
	err = ext.DecodeIdx(dst2, nil, dst3)
	if err != nil {
		log.Fatal(err)
	}

	if bytes.Equal(dst2[0], originals[0]) {
		fmt.Println("Progressive reconstruction with merge successful")
	}

	// Output: Multiple shards reconstructed successfully
	// Progressive reconstruction with merge successful
}

func ExampleNew_maxSize() {
	// Create an encoder with 17 data and 3 parity slices.
	// Use a bigger max cache size
	enc, err := reedsolomon.New(17, 3, reedsolomon.WithMaxGoroutines(64),
		reedsolomon.WithInversionCache(true), reedsolomon.WithLeopardGF(true))
	if err != nil {
		log.Fatal(err)
	}

	// Encode some data.
	data := make([][]byte, 17+3)
	for i := range data {
		data[i] = make([]byte, 16384)
		fillRandom(data[i])
	}

	err = enc.Encode(data)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("ok")
	// OUTPUT: ok
}
