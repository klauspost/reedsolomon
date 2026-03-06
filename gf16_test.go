package reedsolomon

import "testing"

var gf16 LowLevel

func TestGF16MulZero(t *testing.T) {
	// Anything multiplied by zero is zero.
	for _, v := range []uint16{0, 1, 2, 100, 65535} {
		if got := gf16.GF16Mul(v, 0); got != 0 {
			t.Errorf("GF16Mul(%d, 0) = %d, want 0", v, got)
		}
		if got := gf16.GF16Mul(0, v); got != 0 {
			t.Errorf("GF16Mul(0, %d) = %d, want 0", v, got)
		}
	}
}

func TestGF16MulOne(t *testing.T) {
	initConstants()
	// Multiplying by 1 (the multiplicative identity) is a no-op.
	// The identity element in GF(2^16) is 1 (expLUT[0]).
	one := uint16(expLUT[0])
	for _, v := range []uint16{1, 2, 7, 255, 1000, 65535} {
		if got := gf16.GF16Mul(v, one); got != v {
			t.Errorf("GF16Mul(%d, 1) = %d, want %d", v, got, v)
		}
		if got := gf16.GF16Mul(one, v); got != v {
			t.Errorf("GF16Mul(1, %d) = %d, want %d", v, got, v)
		}
	}
}

func TestGF16MulCommutativity(t *testing.T) {
	cases := [][2]uint16{
		{1, 2}, {3, 5}, {255, 256}, {1000, 2000}, {65534, 65535},
	}
	for _, c := range cases {
		a, b := c[0], c[1]
		ab := gf16.GF16Mul(a, b)
		ba := gf16.GF16Mul(b, a)
		if ab != ba {
			t.Errorf("GF16Mul(%d, %d) = %d != GF16Mul(%d, %d) = %d", a, b, ab, b, a, ba)
		}
	}
}

func TestGF16MulAssociativity(t *testing.T) {
	a, b, c := uint16(3), uint16(5), uint16(7)
	// (a*b)*c == a*(b*c)
	lhs := gf16.GF16Mul(gf16.GF16Mul(a, b), c)
	rhs := gf16.GF16Mul(a, gf16.GF16Mul(b, c))
	if lhs != rhs {
		t.Errorf("associativity failed: (%d*%d)*%d=%d, %d*(%d*%d)=%d", a, b, c, lhs, a, b, c, rhs)
	}
}

func TestGF16MulDistributivity(t *testing.T) {
	// Addition in GF(2^16) is XOR. Distributivity: a*(b^c) == a*b ^ a*c.
	cases := [][3]uint16{
		{3, 5, 7}, {255, 256, 1000}, {1, 2, 3}, {65534, 65535, 1},
	}
	for _, c := range cases {
		a, b, cc := c[0], c[1], c[2]
		lhs := gf16.GF16Mul(a, b^cc)
		rhs := gf16.GF16Mul(a, b) ^ gf16.GF16Mul(a, cc)
		if lhs != rhs {
			t.Errorf("distributivity failed: %d*(%d^%d)=%d, %d*%d^%d*%d=%d", a, b, cc, lhs, a, b, a, cc, rhs)
		}
	}
}

func TestGF16MulConsistentWithInternal(t *testing.T) {
	initConstants()
	// GF16Mul(a, b) should agree with the internal mulLog when b != 0.
	cases := [][2]uint16{
		{1, 2}, {3, 5}, {255, 256}, {1000, 2000},
	}
	for _, c := range cases {
		a, b := c[0], c[1]
		want := uint16(mulLog(ffe(a), logLUT[ffe(b)]))
		got := gf16.GF16Mul(a, b)
		if got != want {
			t.Errorf("GF16Mul(%d, %d) = %d, internal mulLog gives %d", a, b, got, want)
		}
	}
}
