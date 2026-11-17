package reedsolomon

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
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
	for i := range 301 {
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

func TestAddMod(t *testing.T) {
	type testCase struct {
		x        ffe
		y        ffe
		expected ffe
	}
	testCases := []testCase{
		{x: ffe(1), y: ffe(2), expected: ffe(3)},
		{x: ffe(65533), y: ffe(1), expected: ffe(65534)},
		{x: ffe(65534), y: ffe(2), expected: ffe(1)},
		{x: ffe(65535), y: ffe(1), expected: ffe(1)},
		// it is expected that the following tests cases return modulus and that
		// callers of addMod will convert it to 0.
		{x: ffe(65534), y: ffe(1), expected: ffe(65535)},
		{x: ffe(65535), y: ffe(0), expected: ffe(65535)},
		{x: ffe(65535), y: ffe(65535), expected: ffe(65535)},
	}
	for _, tc := range testCases {
		got := addMod(tc.x, tc.y)
		if tc.expected != got {
			t.Errorf("expected %v, got %v", tc.expected, got)
		}
	}
}

// goldenShapes pins the leopard GF16 parity output. The digests were
// captured from the unmodified upstream kernels; any change must keep them.
var goldenShapes = []struct {
	data, parity, size int
	digest             string
}{
	{4096, 12288, 32768, "ae4e2722051b18efa3767ba5aff142319f880c4cb219f56e91d49d3e623aff6a"},
	{4096, 12288, 2048, "385c29ad99e11926b437d13d0419fa3b72cbaf3f1c7cee9ca95255997fe62577"},
	{4096, 1024, 2048, "f25581061987e1925bc3a9121af8e7f7d0acce7ec35d950bb971f81c233f19f6"},
	{1024, 1024, 2048, "824452ce4b210ec08b70b4d93b79f8371a870b8a89358f4cdce2bbe4d58173eb"},
	{100, 150, 2112, "6201ec2568fdea40e544d79cac6fce9dee8f0d483f0e03753c69eb3bc2d96f7f"},
	{64, 192, 2048, "e7707f1bc617972056faa7c3b8aa6a35cf94b7eb8930fc63bcdc28240dc01cd1"},
	{32, 96, 2048, "4df6608405af95af0d338e24d69a4d71f385ee10ca73f7365b933484273a8cfd"},
	{16, 48, 2048, "2034c78ab1b6df70d60921555a8368e6b5134ce57fde152394039e7b7cd8f712"},
	{8, 24, 2048, "67fc9f532a86b588c5fcc09ee8377ed9cc376a5f1825c3c2dc582d9940205856"},
	{3, 5, 2048, "7589338d79a56616cea69d9c9091c67088bb5b5eed8f80184bcab69f656b571a"},
	{1, 3, 2048, "f9e8561d882b6ea8a2b84d907b250feefcc8713f2c24b6f2956772c1844c22ba"},
}

// goldenVariant is a set of options the golden test runs under.
type goldenVariant struct {
	name string
	opts []Option
}

// goldenArchVariants are the variants that only exist on some platforms.
var goldenArchVariants []goldenVariant

func goldenShards(data, parity, size int, dirty bool) [][]byte {
	rng := rand.New(rand.NewSource(int64(data)*1000003 + int64(parity)*1009 + int64(size)))
	shards := AllocAligned(data+parity, size)
	for i := range shards {
		if i < data || dirty {
			rng.Read(shards[i])
		}
	}
	return shards
}

func goldenDigest(t *testing.T, shards [][]byte, data int) string {
	t.Helper()
	h := sha256.New()
	for _, s := range shards[data:] {
		h.Write(s)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func TestLeopardGF16Golden(t *testing.T) {
	variants := []goldenVariant{
		{"default", nil},
		{"noNEON", []Option{WithNEON(false)}},
	}
	variants = append(variants, goldenArchVariants...)
	for _, sh := range goldenShapes {
		if testing.Short() && (sh.data+sh.parity)*sh.size > 16<<20 {
			// The two shapes of 16384 shards take seconds a variant.
			continue
		}
		for _, v := range variants {
			for _, dirty := range []bool{false, true} {
				enc, err := New(sh.data, sh.parity, append([]Option{WithLeopardGF16(true)}, v.opts...)...)
				if err != nil {
					t.Fatal(err)
				}
				shards := goldenShards(sh.data, sh.parity, sh.size, dirty)
				if err := enc.Encode(shards); err != nil {
					t.Fatal(err)
				}
				got := goldenDigest(t, shards, sh.data)
				if got != sh.digest {
					t.Errorf("%d+%d size=%d %s dirty=%v: digest %s, want %s", sh.data, sh.parity, sh.size, v.name, dirty, got, sh.digest)
				}
			}
		}
	}
}
