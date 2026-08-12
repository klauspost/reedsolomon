package reedsolomon

import (
	"bytes"
	"errors"
	"math"
	"testing"
)

// guarded returns a slice of size bytes backed by a larger array filled with a
// known pattern, plus a func that fails the test if anything past size changed.
func guarded(t *testing.T, size int) ([]byte, func()) {
	t.Helper()
	const slack = 1 << 16
	buf := make([]byte, size+slack)
	for i := range buf {
		buf[i] = 0x5a
	}
	clear(buf[:size])
	return buf[:size:size], func() {
		t.Helper()
		for i := size; i < len(buf); i++ {
			if buf[i] != 0x5a {
				t.Fatalf("wrote %d bytes past the end of a %d byte shard", i-size+1, size)
			}
		}
	}
}

// Update must not accept new data shards that are longer than the old ones;
// the kernels write len(newDatashards[i]) bytes into shards[i].
func TestUpdateSizeMismatch(t *testing.T) {
	const old, updated = 1024, 4096
	r, err := New(4, 2)
	if err != nil {
		t.Fatal(err)
	}
	shards := make([][]byte, 6)
	for i := range shards {
		shards[i] = make([]byte, old)
	}
	if err := r.Encode(shards); err != nil {
		t.Fatal(err)
	}
	var check func()
	shards[0], check = guarded(t, old)

	newData := make([][]byte, 4)
	newData[0] = bytes.Repeat([]byte{0xff}, updated)
	if err := r.Update(shards, newData); !errors.Is(err, ErrShardSize) {
		t.Errorf("want ErrShardSize, got %v", err)
	}
	check()
}

// Update must reject present-but-empty shards. They are not nil, so the old
// nil-only checks let them through into the kernels.
func TestUpdateEmptyShards(t *testing.T) {
	const sz = 1024
	for _, idx := range []int{0, 4} {
		r, err := New(4, 2)
		if err != nil {
			t.Fatal(err)
		}
		shards := make([][]byte, 6)
		for i := range shards {
			shards[i] = make([]byte, sz)
		}
		if err := r.Encode(shards); err != nil {
			t.Fatal(err)
		}
		full, check := guarded(t, sz)
		shards[idx] = full[:0]

		newData := make([][]byte, 4)
		newData[0] = bytes.Repeat([]byte{0xff}, sz)
		if err := r.Update(shards, newData); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("shard %d: want ErrInvalidInput, got %v", idx, err)
		}
		check()
	}
}

// An empty but non-nil new data shard means "unchanged", same as nil. Reaching
// the update loops with it panicked on the split path and silently XORed the
// old data into the parity on the serial path.
func TestUpdateEmptyNewDataShard(t *testing.T) {
	const d, p, sz = 4, 2, 64 << 10
	for _, tc := range []struct {
		name string
		opts []Option
	}{
		{"serial", []Option{WithMaxGoroutines(1)}},
		{"split", []Option{WithMaxGoroutines(4), WithMinSplitSize(1024)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := New(d, p, tc.opts...)
			if err != nil {
				t.Fatal(err)
			}
			shards := make([][]byte, d+p)
			for i := range shards {
				shards[i] = make([]byte, sz)
				fillRandom(shards[i])
			}
			if err := r.Encode(shards); err != nil {
				t.Fatal(err)
			}

			newData := make([][]byte, d)
			newData[0] = []byte{} // present, empty: shard 0 is unchanged
			newData[1] = make([]byte, sz)
			fillRandom(newData[1])

			// Reference parity: a full encode of the post-update data.
			want := make([][]byte, d+p)
			for i := range d {
				want[i] = append([]byte(nil), shards[i]...)
			}
			copy(want[1], newData[1])
			for i := d; i < d+p; i++ {
				want[i] = make([]byte, sz)
			}
			if err := r.Encode(want); err != nil {
				t.Fatal(err)
			}

			if err := r.Update(shards, newData); err != nil {
				t.Fatal(err)
			}
			for i := d; i < d+p; i++ {
				if !bytes.Equal(shards[i], want[i]) {
					t.Errorf("parity shard %d does not match a full encode", i)
				}
			}
		})
	}
}

// The platform mulAdd8 wrappers run a SIMD prefix (64/32/16 byte granularity)
// and hand the remainder to refMulAdd8, so it must finish sub-block tails
// rather than dropping them. Leopard only ever passes multiples of 64 today.
func TestRefMulAdd8Tail(t *testing.T) {
	initConstants8()
	for _, n := range []int{0, 1, 15, 16, 31, 63, 64, 65, 100, 127, 128, 191} {
		x := make([]byte, n)
		y := make([]byte, n)
		want := make([]byte, n)
		for i := range y {
			x[i] = byte(i * 3)
			y[i] = byte(i*7 + 1)
			want[i] = x[i] ^ byte(mul8LUTs[3].Value[y[i]])
		}
		refMulAdd8(x, y, 3)
		if !bytes.Equal(x, want) {
			t.Errorf("len %d: refMulAdd8 did not process the whole slice", n)
		}
	}
}

// leopardFits must not reach ceilPow2 (which returns 0 for non-positive input)
// or overflow its rounding, and must decide identically for in-range counts.
func TestLeopardFitsBounds(t *testing.T) {
	for _, tc := range [][2]int{
		{10, 0}, {10, -1}, {0, 10}, {-1, 10},
		{math.MaxInt, 10}, {10, math.MaxInt}, {math.MaxInt, math.MaxInt},
		{order + 1, 10}, {10, order + 1},
	} {
		for _, fieldOrder := range []int{order8, order} {
			if leopardFits(tc[0], tc[1], fieldOrder) {
				t.Errorf("leopardFits(%d, %d, %d) = true, want false", tc[0], tc[1], fieldOrder)
			}
		}
	}

	// In-range counts must match the original rounding expression.
	check := func(fieldOrder, step int) {
		for d := 1; d <= fieldOrder; d += step {
			for p := 1; p <= fieldOrder; p += step {
				m := ceilPow2(p)
				want := ((d+m-1)/m)*m <= fieldOrder-m
				if got := leopardFits(d, p, fieldOrder); got != want {
					t.Fatalf("leopardFits(%d, %d, %d) = %v, want %v", d, p, fieldOrder, got, want)
				}
			}
		}
	}
	check(order8, 1)
	check(order, 97)
}

// New must not hand back an encoder, or panic, for shard counts that are
// invalid or that overflow when summed. The overflowed total slipped past the
// shard count limits and reached make([][]byte, negative) with a custom matrix.
func TestNewOverflowingShardCounts(t *testing.T) {
	customMatrix := [][]byte{{1, 2, 3}, {4, 5, 6}}
	for _, tc := range [][2]int{
		{math.MaxInt, 1}, {math.MaxInt - 5, 10}, {math.MaxInt, math.MaxInt},
		{math.MaxInt/2 + 1, math.MaxInt/2 + 1}, // sum overflows to negative
		{-1, 5}, {5, -1}, {-1, -1}, {0, 0},
	} {
		for _, opts := range [][]Option{
			nil,
			{WithLeopardGF16(true)},
			{WithLeopardGF(true)},
			{WithCustomMatrix(customMatrix)},
			{WithJerasureMatrix()},
			{WithFastOneParityMatrix()},
		} {
			enc, err := New(tc[0], tc[1], opts...)
			if err == nil {
				t.Errorf("New(%d, %d, %d opts) returned encoder %T, want error", tc[0], tc[1], len(opts), enc)
			}
		}
	}
}

// ReconstructSome accepts a "required" of either DataShards or TotalShards
// entries. With the short form, missing parity shards must not be indexed.
func TestReconstructSomeShortRequired(t *testing.T) {
	r, err := New(4, 2)
	if err != nil {
		t.Fatal(err)
	}
	shards := make([][]byte, 6)
	for i := range shards {
		shards[i] = make([]byte, 64)
		fillRandom(shards[i])
	}
	if err := r.Encode(shards); err != nil {
		t.Fatal(err)
	}
	want := append([]byte(nil), shards[0]...)
	shards[0] = nil
	shards[5] = nil

	required := make([]bool, 4)
	required[0] = true
	if err := r.ReconstructSome(shards, required); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(shards[0], want) {
		t.Error("data shard not reconstructed")
	}
	if shards[5] != nil {
		t.Error("parity shard reconstructed, should have been skipped")
	}
}

// A present-but-empty input must not pass the size check by masquerading as
// "size not yet known"; the kernels would then read shardSize bytes from it.
func TestDecodeIdxEmptyInput(t *testing.T) {
	const sz = 1024
	r, err := New(4, 2)
	if err != nil {
		t.Fatal(err)
	}
	e := r.(Extensions)
	dst := make([][]byte, 6)
	input := make([][]byte, 6)
	expect := make([]bool, 6)
	for i := range 4 {
		expect[i] = true
		input[i] = make([]byte, sz)
	}
	input[0] = input[0][:0]
	dst[4] = make([]byte, sz)
	dst[5] = make([]byte, sz)

	if err := e.DecodeIdx(dst, expect, input); !errors.Is(err, ErrInvalidShardSize) {
		t.Errorf("want ErrInvalidShardSize, got %v", err)
	}
}

// DecodeIdx may be given more inputs than there are data shards. The temporary
// matrix buffer is only sized for dataShards x parityShards.
func TestDecodeIdxExcessInputs(t *testing.T) {
	const d, p, sz = 5, 20, 1 << 20
	const inputs = 13
	r, err := New(d, p, WithGFNI(false), WithAVXGFNI(false))
	if err != nil {
		t.Fatal(err)
	}
	e := r.(Extensions)

	shards := AllocAligned(d+p, sz)
	for i := range d {
		fillRandom(shards[i])
	}
	if err := r.Encode(shards); err != nil {
		t.Fatal(err)
	}

	dst := make([][]byte, d+p)
	input := make([][]byte, d+p)
	expect := make([]bool, d+p)
	for i := range d + p {
		if i < inputs {
			expect[i] = true
			input[i] = shards[i]
		} else {
			dst[i] = make([]byte, sz)
		}
	}
	if err := e.DecodeIdx(dst, expect, input); err != nil {
		t.Fatal(err)
	}
	for i := inputs; i < d+p; i++ {
		if !bytes.Equal(dst[i], shards[i]) {
			t.Fatalf("shard %d mismatch", i)
		}
	}
}

// Leopard walks the FFT skew table in steps of ceilPow2(parityShards); shard
// counts that walk past the end of the table must be rejected by New.
func TestLeopardShardCombo(t *testing.T) {
	for _, tc := range []struct {
		d, p int
		opt  Option
		ok   bool
	}{
		// GF(2^8), order 256.
		{200, 32, WithLeopardGF(true), true},
		{200, 33, WithLeopardGF(true), false},
		{100, 156, WithLeopardGF(true), false},
		{128, 128, WithLeopardGF(true), true},
		{127, 129, WithLeopardGF(true), false},
		// GF(2^16), order 65536.
		{65024, 300, nil, true},
		{65100, 300, nil, false},
		{32768, 32768, nil, true},
		{32767, 32769, nil, false},
		{300, 33, nil, true},
	} {
		var err error
		if tc.opt == nil {
			_, err = New(tc.d, tc.p)
		} else {
			_, err = New(tc.d, tc.p, tc.opt)
		}
		switch {
		case tc.ok && err != nil:
			t.Errorf("New(%d, %d): unexpected error %v", tc.d, tc.p, err)
		case !tc.ok && !errors.Is(err, ErrInvShardCombo):
			t.Errorf("New(%d, %d): want ErrInvShardCombo, got %v", tc.d, tc.p, err)
		}
	}
}

// Every leopard GF(2^8) shard combination that New accepts must survive an
// encode and a reconstruct.
func TestLeopard8ShardComboExhaustive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping exhaustive test in short mode")
	}
	for d := 1; d <= 255; d++ {
		for p := 1; d+p <= 256; p++ {
			r, err := New(d, p, WithLeopardGF(true))
			if errors.Is(err, ErrInvShardCombo) {
				continue
			}
			if err != nil {
				t.Fatalf("d=%d p=%d: %v", d, p, err)
			}
			shards := AllocAligned(d+p, 64)
			for i := range d {
				fillRandom(shards[i])
			}
			if err := r.Encode(shards); err != nil {
				t.Fatalf("d=%d p=%d encode: %v", d, p, err)
			}
			want := append([]byte(nil), shards[0]...)
			shards[0] = nil
			if p > 1 {
				shards[d] = nil
			}
			if err := r.Reconstruct(shards); err != nil {
				t.Fatalf("d=%d p=%d reconstruct: %v", d, p, err)
			}
			if !bytes.Equal(shards[0], want) {
				t.Fatalf("d=%d p=%d: reconstruct mismatch", d, p)
			}
		}
	}
}

// WithCustomMatrix documents "at least ParityShards rows"; surplus rows are
// unused and must not run past the end of the coding matrix.
func TestCustomMatrixExtraRows(t *testing.T) {
	const d, p = 5, 2
	cm := make([][]byte, p+2)
	for i := range cm {
		cm[i] = make([]byte, d)
		for j := range cm[i] {
			cm[i][j] = byte(i + j + 1)
		}
	}
	r, err := New(d, p, WithCustomMatrix(cm))
	if err != nil {
		t.Fatal(err)
	}
	shards := AllocAligned(d+p, 64)
	for i := range d {
		fillRandom(shards[i])
	}
	if err := r.Encode(shards); err != nil {
		t.Fatal(err)
	}
	ok, err := r.Verify(shards)
	if err != nil || !ok {
		t.Fatalf("verify: %v %v", ok, err)
	}
}

// NewStream only supports the classic encoder; leopard options must not make
// it fail the type assertion on the returned Encoder.
func TestNewStreamLeopard(t *testing.T) {
	for _, opt := range []Option{WithLeopardGF16(true), WithLeopardGF(true)} {
		if _, err := NewStream(10, 3, opt); !errors.Is(err, ErrNotSupported) {
			t.Errorf("want ErrNotSupported, got %v", err)
		}
	}
}

func TestLowLevelShortOutput(t *testing.T) {
	var l LowLevel
	in := make([]byte, 256)
	out, check := guarded(t, 64)
	for _, fn := range []func(){
		func() { l.GalMulSlice(3, in, out) },
		func() { l.GalMulSliceXor(3, in, out) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("no panic on short output")
				}
			}()
			fn()
		}()
		check()
	}
}

// shortAllocator hands back buffers that are one byte too small.
type shortAllocator struct{}

func (shortAllocator) Get(n, size int) [][]byte { return AllocAligned(n, size-1) }
func (shortAllocator) Put([][]byte)             {}

func TestWorkAllocatorContract(t *testing.T) {
	for _, opt := range []Option{WithLeopardGF(true), WithLeopardGF16(true)} {
		r, err := New(10, 4, opt, WithWorkAllocator(shortAllocator{}))
		if err != nil {
			t.Fatal(err)
		}
		shards := AllocAligned(14, 64)
		if err := r.Encode(shards); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("encode: want ErrInvalidInput, got %v", err)
		}
		shards[0] = nil
		if err := r.Reconstruct(shards); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("reconstruct: want ErrInvalidInput, got %v", err)
		}
	}
}
