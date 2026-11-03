package reedsolomon

import (
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"testing"
)

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

func goldenShards(data, parity, size int, dirty bool) [][]byte {
	rng := rand.New(rand.NewSource(int64(data*1000003 + parity*1009 + size)))
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
	variants := []struct {
		name string
		opts []Option
		fast bool
	}{
		{"default", nil, true},
		{"noNEON", []Option{WithNEON(false)}, false},
	}
	variants = append(variants, goldenArchVariants()...)
	for _, sh := range goldenShapes {
		for _, v := range variants {
			if testing.Short() && sh.size > 4096 && !v.fast {
				continue
			}
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
