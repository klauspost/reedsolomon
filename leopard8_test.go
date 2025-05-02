package reedsolomon

import "testing"

func TestAddMod8(t *testing.T) {
	type testCase struct {
		x        ffe8
		y        ffe8
		expected ffe8
	}
	testCases := []testCase{
		{x: ffe8(1), y: ffe8(2), expected: ffe8(3)},
		{x: ffe8(253), y: ffe8(1), expected: ffe8(254)},
		{x: ffe8(254), y: ffe8(1), expected: ffe8(0)},
		{x: ffe8(254), y: ffe8(2), expected: ffe8(1)},
		{x: ffe8(255), y: ffe8(0), expected: ffe8(0)},
		{x: ffe8(255), y: ffe8(1), expected: ffe8(1)},
		// {x: ffe8(255), y: ffe8(255), expected: ffe8(0)},
	}
	for _, tc := range testCases {
		got := addMod8(tc.x, tc.y)
		if tc.expected != got {
			t.Errorf("expected %v, got %v", tc.expected, got)
		}
	}
}
