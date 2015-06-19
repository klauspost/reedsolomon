/**
 * Unit tests for Matrix
 *
 * Copyright 2015, Klaus Post
 * Copyright 2015, Backblaze, Inc.  All rights reserved.
 */

package reedsolomon

import (
	"testing"
)

func TestMatrixIdentity(t *testing.T) {
	m, err := identityMatrix(3)
	if err != nil {
		t.Fatal(err)
	}
	str := m.String()
	expect := "[[1, 0, 0], [0, 1, 0], [0, 0, 1]]"
	if str != expect {
		t.Fatal(str, "!=", expect)
	}
}

func TestMatricMultiply(t *testing.T) {
	m1, err := newMatrixData(
		[][]byte{
			[]byte{1, 2},
			[]byte{3, 4},
		})
	if err != nil {
		t.Fatal(err)
	}

	m2, err := newMatrixData(
		[][]byte{
			[]byte{5, 6},
			[]byte{7, 8},
		})
	if err != nil {
		t.Fatal(err)
	}
	actual, err := m1.Multiply(m2)
	if err != nil {
		t.Fatal(err)
	}
	str := actual.String()
	expect := "[[11, 22], [19, 42]]"
	if str != expect {
		t.Fatal(str, "!=", expect)
	}
}

func TestMatrixInverse(t *testing.T) {
	m, err := newMatrixData([][]byte{
		[]byte{56, 23, 98},
		[]byte{3, 100, 200},
		[]byte{45, 201, 123},
	})
	if err != nil {
		t.Fatal(err)
	}
	m, err = m.Invert()
	if err != nil {
		t.Fatal(err)
	}
	str := m.String()
	expect := "[[175, 133, 33], [130, 13, 245], [112, 35, 126]]"
	if str != expect {
		t.Fatal(str, "!=", expect)
	}
}

func TestMatrixInverse2(t *testing.T) {
	m, err := newMatrixData([][]byte{
		[]byte{1, 0, 0, 0, 0},
		[]byte{0, 1, 0, 0, 0},
		[]byte{0, 0, 0, 1, 0},
		[]byte{0, 0, 0, 0, 1},
		[]byte{7, 7, 6, 6, 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	m, err = m.Invert()
	if err != nil {
		t.Fatal(err)
	}
	str := m.String()
	expect := "[[1, 0, 0, 0, 0]," +
		" [0, 1, 0, 0, 0]," +
		" [123, 123, 1, 122, 122]," +
		" [0, 0, 1, 0, 0]," +
		" [0, 0, 0, 1, 0]]"
	if str != expect {
		t.Fatal(str, "!=", expect)
	}
}
