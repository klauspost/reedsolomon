package reedsolomon

import (
	"bytes"
	"testing"
)

func TestEncoderReconstructLeo(t *testing.T) {
	testEncoderReconstructLeo(t)
}

func testEncoderReconstructLeo(t *testing.T, o ...Option) {
	// Create some sample data
	var data = make([]byte, 2<<20)
	fillRandom(data)

	// Create 5 data slices of 50000 elements each
	enc, err := New(500, 300, testOptions(o...)...)
	if err != nil {
		t.Fatal(err)
	}
	shards, err := enc.Split(data)
	if err != nil {
		t.Fatal(err)
	}
	err = enc.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Check that it verifies
	ok, err := enc.Verify(shards)
	if !ok || err != nil {
		t.Fatal("not ok:", ok, "err:", err)
	}

	// Delete a shard
	shards[0] = nil

	// Should reconstruct
	err = enc.Reconstruct(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Check that it verifies
	ok, err = enc.Verify(shards)
	if !ok || err != nil {
		t.Fatal("not ok:", ok, "err:", err)
	}

	// Recover original bytes
	buf := new(bytes.Buffer)
	err = enc.Join(buf, shards, len(data))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), data) {
		t.Fatal("recovered bytes do not match")
	}

	// Corrupt a shard
	shards[0] = nil
	shards[1][0], shards[1][500] = 75, 75

	// Should reconstruct (but with corrupted data)
	err = enc.Reconstruct(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Check that it verifies
	ok, err = enc.Verify(shards)
	if ok || err != nil {
		t.Fatal("error or ok:", ok, "err:", err)
	}

	// Recovered data should not match original
	buf.Reset()
	err = enc.Join(buf, shards, len(data))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(buf.Bytes(), data) {
		t.Fatal("corrupted data matches original")
	}
}

func TestEncoderReconstructFailLeo(t *testing.T) {
	// Create some sample data
	var data = make([]byte, 2<<20)
	fillRandom(data)

	// Create 5 data slices of 50000 elements each
	enc, err := New(500, 300, testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	shards, err := enc.Split(data)
	if err != nil {
		t.Fatal(err)
	}
	err = enc.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// Check that it verifies
	ok, err := enc.Verify(shards)
	if !ok || err != nil {
		t.Fatal("not ok:", ok, "err:", err)
	}

	// Delete more than parity shards
	for i := 0; i < 301; i++ {
		shards[i] = nil
	}

	// Should not reconstruct
	err = enc.Reconstruct(shards)
	if err != ErrTooFewShards {
		t.Fatal("want ErrTooFewShards, got:", err)
	}
}

func TestSplitJoinLeo(t *testing.T) {
	var data = make([]byte, (250<<10)-1)
	fillRandom(data)

	enc, _ := New(500, 300, testOptions()...)
	shards, err := enc.Split(data)
	if err != nil {
		t.Fatal(err)
	}

	_, err = enc.Split([]byte{})
	if err != ErrShortData {
		t.Errorf("expected %v, got %v", ErrShortData, err)
	}

	buf := new(bytes.Buffer)
	err = enc.Join(buf, shards, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), data[:5000]) {
		t.Fatal("recovered data does match original")
	}

	err = enc.Join(buf, [][]byte{}, 0)
	if err != ErrTooFewShards {
		t.Errorf("expected %v, got %v", ErrTooFewShards, err)
	}

	err = enc.Join(buf, shards, len(data)+500*64)
	if err != ErrShortData {
		t.Errorf("expected %v, got %v", ErrShortData, err)
	}

	shards[0] = nil
	err = enc.Join(buf, shards, len(data))
	if err != ErrReconstructRequired {
		t.Errorf("expected %v, got %v", ErrReconstructRequired, err)
	}
}
