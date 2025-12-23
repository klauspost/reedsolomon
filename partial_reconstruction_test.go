package reedsolomon

import (
	"bytes"
	"fmt"
	"testing"
)

func TestCoefficients(t *testing.T) {
	encoder, err := New(4, 2)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("Hello, World! This is a test of partial reconstruction coefficients.")
	shards, err := encoder.Split(data)
	if err != nil {
		t.Fatal(err)
	}
	err = encoder.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	rs := encoder.(*reedSolomon)
	coeffs, err := rs.CalcPartialReconstructionCoefficients(0, []int{1, 2, 4, 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(coeffs) != 4 {
		t.Fatalf("expected 4 coefficients, got %d", len(coeffs))
	}
	fmt.Printf("Calculated coefficients: %v\n", coeffs)

	low := LowLevel{
		o: &rs.o,
	}
	out := make([]byte, len(shards[0]))
	for i, shardIndex := range []int{1, 2, 4, 5} {
		coeff := coeffs[i]
		low.GalMulSliceXor(coeff, shards[shardIndex], out)
	}
	fmt.Printf("Reconstructed shard 0: %v\n", out)
	if !bytes.Equal(out, shards[0]) {
		t.Errorf("reconstructed shard 0 does not match original")
	}
}

func TestPartialReconstruct(t *testing.T) {
	encoder, err := New(14, 10)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 12345)
	for i := range data {
		data[i] = byte(i % 256)
	}
	shards, err := encoder.Split(data)
	if err != nil {
		t.Fatal(err)
	}
	err = encoder.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	sourceIndexes := // assume az1: 0~7, az2: 8~15, az3: 16~23
		[]int{0, 1, 2, 5, 6, 7, 8, 9, 10, 16, 17, 18, 19, 20}

	rs := encoder.(*reedSolomon)
	coeffs, err := rs.CalcPartialReconstructionCoefficients(3, sourceIndexes)

	if err != nil {
		t.Fatal(err)
	}

	var az1Shards, az2Shards, az3Shards [][]byte
	var az1Coeffs, az2Coeffs, az3Coeffs []byte
	for i, shardIndex := range sourceIndexes {
		if shardIndex < 8 {
			az1Shards = append(az1Shards, shards[shardIndex])
			az1Coeffs = append(az1Coeffs, coeffs[i])
		} else if shardIndex < 16 {
			az2Shards = append(az2Shards, shards[shardIndex])
			az2Coeffs = append(az2Coeffs, coeffs[i])
		} else {
			az3Shards = append(az3Shards, shards[shardIndex])
			az3Coeffs = append(az3Coeffs, coeffs[i])
		}
	}

	az2PartialShard, err := rs.BuildPartialShard(az2Shards, az2Coeffs)
	if err != nil {
		t.Fatal(err)
	}
	az3PartialShard, err := rs.BuildPartialShard(az3Shards, az3Coeffs)
	if err != nil {
		t.Fatal(err)
	}

	rebuiltShard, err := rs.PartialReconstruct(az1Shards, az1Coeffs,
		[][]byte{az2PartialShard, az3PartialShard})
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(rebuiltShard, shards[3]) {
		t.Errorf("rebuilt shard does not match original")
	}
}
