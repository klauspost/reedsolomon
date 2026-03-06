package reedsolomon

import "testing"

func TestGF16Init(t *testing.T) {
	GF16Init()
	if logLUT == nil || expLUT == nil {
		t.Fatal("GF16Init did not initialize lookup tables")
	}
}

func TestGF16MulZero(t *testing.T) {
	GF16Init()
	// Anything multiplied by zero is zero.
	for _, v := range []uint16{0, 1, 2, 100, 65535} {
		if got := GF16Mul(v, 0); got != 0 {
			t.Errorf("GF16Mul(%d, 0) = %d, want 0", v, got)
		}
		if got := GF16Mul(0, v); got != 0 {
			t.Errorf("GF16Mul(0, %d) = %d, want 0", v, got)
		}
	}
}

func TestGF16MulOne(t *testing.T) {
	GF16Init()
	// Multiplying by 1 (the multiplicative identity) is a no-op.
	// The identity element in GF(2^16) is 1 (expLUT[0]).
	one := uint16(expLUT[0])
	for _, v := range []uint16{1, 2, 7, 255, 1000, 65535} {
		if got := GF16Mul(v, one); got != v {
			t.Errorf("GF16Mul(%d, 1) = %d, want %d", v, got, v)
		}
		if got := GF16Mul(one, v); got != v {
			t.Errorf("GF16Mul(1, %d) = %d, want %d", v, got, v)
		}
	}
}

func TestGF16MulCommutativity(t *testing.T) {
	GF16Init()
	cases := [][2]uint16{
		{1, 2}, {3, 5}, {255, 256}, {1000, 2000}, {65534, 65535},
	}
	for _, c := range cases {
		a, b := c[0], c[1]
		ab := GF16Mul(a, b)
		ba := GF16Mul(b, a)
		if ab != ba {
			t.Errorf("GF16Mul(%d, %d) = %d != GF16Mul(%d, %d) = %d", a, b, ab, b, a, ba)
		}
	}
}

func TestGF16MulAssociativity(t *testing.T) {
	GF16Init()
	a, b, c := uint16(3), uint16(5), uint16(7)
	// (a*b)*c == a*(b*c)
	lhs := GF16Mul(GF16Mul(a, b), c)
	rhs := GF16Mul(a, GF16Mul(b, c))
	if lhs != rhs {
		t.Errorf("associativity failed: (%d*%d)*%d=%d, %d*(%d*%d)=%d", a, b, c, lhs, a, b, c, rhs)
	}
}

func TestGF16MulDistributivity(t *testing.T) {
	GF16Init()
	// Addition in GF(2^16) is XOR. Distributivity: a*(b^c) == a*b ^ a*c.
	cases := [][3]uint16{
		{3, 5, 7}, {255, 256, 1000}, {1, 2, 3}, {65534, 65535, 1},
	}
	for _, c := range cases {
		a, b, cc := c[0], c[1], c[2]
		lhs := GF16Mul(a, b^cc)
		rhs := GF16Mul(a, b) ^ GF16Mul(a, cc)
		if lhs != rhs {
			t.Errorf("distributivity failed: %d*(%d^%d)=%d, %d*%d^%d*%d=%d", a, b, cc, lhs, a, b, a, cc, rhs)
		}
	}
}

func TestGF16MulConsistentWithInternal(t *testing.T) {
	GF16Init()
	// GF16Mul(a, b) should agree with the internal mulLog when b != 0.
	cases := [][2]uint16{
		{1, 2}, {3, 5}, {255, 256}, {1000, 2000},
	}
	for _, c := range cases {
		a, b := c[0], c[1]
		want := uint16(mulLog(ffe(a), logLUT[ffe(b)]))
		got := GF16Mul(a, b)
		if got != want {
			t.Errorf("GF16Mul(%d, %d) = %d, internal mulLog gives %d", a, b, got, want)
		}
	}
}
