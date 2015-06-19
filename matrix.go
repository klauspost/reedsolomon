/**
 * Matrix Algebra over an 8-bit Galois Field
 *
 * Copyright 2015, Klaus Post
 * Copyright 2015, Backblaze, Inc.
 */

package reedsolomon

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// byte[row][col]
type matrix [][]byte

// newMatrix returns a matrix of zeros.
func newMatrix(rows, cols int) (matrix, error) {
	if rows <= 0 {
		return nil, ErrInvalidRowSize
	}
	if cols <= 0 {
		return nil, ErrInvalidColSize
	}

	m := matrix(make([][]byte, rows))
	for i := range m {
		m[i] = make([]byte, cols)
	}
	return m, nil
}

// NewMatrixData initializes a matrix with the given row-major data.
// Note that data is not copied from input.
func newMatrixData(data [][]byte) (matrix, error) {
	m := matrix(data)
	err := m.Check()
	if err != nil {
		return nil, err
	}
	return m, nil
}

// IdentityMatrix returns an identity matrix of the given size.
func identityMatrix(size int) (matrix, error) {
	m, err := newMatrix(size, size)
	if err != nil {
		return nil, err
	}
	for i := range m {
		m[i][i] = 1
	}
	return m, nil
}

var ErrInvalidRowSize = errors.New("Invalid row size")
var ErrInvalidColSize = errors.New("Invalid column size")
var ErrColSizeMismatch = errors.New("Column size is not the same for all rows")

func (m matrix) Check() error {
	rows := len(m)
	if rows <= 0 {
		return ErrInvalidRowSize
	}
	cols := len(m[0])
	if cols <= 0 {
		return ErrInvalidColSize
	}

	for _, col := range m {
		if len(col) != cols {
			return ErrColSizeMismatch
		}
	}
	return nil
}

// String returns a human-readable string of the matrix contents.
//
// Example: [[1, 2], [3, 4]]
func (m matrix) String() string {
	var rowOut []string
	for _, row := range m {
		var colOut []string
		for _, col := range row {
			colOut = append(colOut, strconv.Itoa(int(col)))
		}
		rowOut = append(rowOut, "["+strings.Join(colOut, ", ")+"]")
	}
	return "[" + strings.Join(rowOut, ", ") + "]"
}

// Multiply multiplies this matrix (the one on the left) by another
// matrix (the one on the right) and returns a new matrix with the result.
func (m matrix) Multiply(right matrix) (matrix, error) {
	if len(m[0]) != len(right) {
		return nil, fmt.Errorf("Columns on left (%d) is different than rows on right (%d)", len(m[0]), len(right))
	}
	result, err := newMatrix(len(m), len(right[0]))
	if err != nil {
		return nil, err
	}
	for r, row := range result {
		for c := range row {
			var value byte
			for i := range m[0] {
				value ^= galMultiply(m[r][i], right[i][c])
			}
			result[r][c] = value
		}
	}
	return result, nil
}

// Augment returns the concatenation of this matrix and the matrix on the right.
func (m matrix) Augment(right matrix) (matrix, error) {
	if len(m) != len(right) {
		return nil, ErrMatrixSize
	}

	result, _ := newMatrix(len(m), len(m[0])+len(right[0]))
	for r, row := range m {
		for c := range row {
			result[r][c] = m[r][c]
		}
		cols := len(m[0])
		for c := range right[0] {
			result[r][cols+c] = right[r][c]
		}
	}
	return result, nil
}

var ErrMatrixSize = errors.New("matrix sizes does not match")

func (m matrix) SameSize(n matrix) error {
	if len(m) != len(n) {
		return ErrMatrixSize
	}
	for i := range m {
		if len(m[i]) != len(n[i]) {
			return ErrMatrixSize
		}
	}
	return nil
}

// Returns a part of this matrix. Data is copied.
func (m matrix) SubMatrix(rmin, cmin, rmax, cmax int) (matrix, error) {
	result, err := newMatrix(rmax-rmin, cmax-cmin)
	if err != nil {
		return nil, err
	}
	// OPTME: If used heavily, use copy function to copy slice
	for r := rmin; r < rmax; r++ {
		for c := cmin; c < cmax; c++ {
			result[r-rmin][c-cmin] = m[r][c]
		}
	}
	return result, nil
}

// SwapRows Exchanges two rows in the matrix.
func (m matrix) SwapRows(r1, r2 int) error {
	if r1 < 0 || len(m) <= r1 || r2 < 0 || len(m) <= r2 {
		return ErrInvalidRowSize
	}
	m[r2], m[r1] = m[r1], m[r2]
	return nil
}

// IsSquare will return true if the matrix is square
// and nil if the matrix is square
func (m matrix) IsSquare() bool {
	if len(m) != len(m[0]) {
		return false
	}
	return true
}

var ErrSingular = errors.New("matrix is singular")
var ErrNotSquare = errors.New("only square matrices can be inverted")

// Invert returns the inverse of this matrix.
// Returns ErrSingular when the matrix is singular and doesn't have an inverse.
// The matrix must be square, otherwise ErrNotSquare is returned.
func (m matrix) Invert() (matrix, error) {
	if !m.IsSquare() {
		return nil, ErrNotSquare
	}

	size := len(m)
	work, err := identityMatrix(size)
	if err != nil {
		return nil, err
	}
	work, err = m.Augment(work)
	if err != nil {
		return nil, err
	}

	err = work.gaussianElimination()
	if err != nil {
		return nil, err
	}

	return work.SubMatrix(0, size, size, size*2)
}

func (m matrix) gaussianElimination() error {
	rows := len(m)
	columns := len(m[0])
	// Clear out the part below the main diagonal and scale the main
	// diagonal to be 1.
	for r := 0; r < rows; r++ {
		// If the element on the diagonal is 0, find a row below
		// that has a non-zero and swap them.
		if m[r][r] == 0 {
			for rowBelow := r + 1; rowBelow < rows; rowBelow++ {
				if m[rowBelow][r] != 0 {
					m.SwapRows(r, rowBelow)
					break
				}
			}
		}
		// If we couldn't find one, the matrix is singular.
		if m[r][r] == 0 {
			return ErrSingular
		}
		// Scale to 1.
		if m[r][r] != 1 {
			scale := galDivide(1, m[r][r])
			for c := 0; c < columns; c++ {
				m[r][c] = galMultiply(m[r][c], scale)
			}
		}
		// Make everything below the 1 be a 0 by subtracting
		// a multiple of it.  (Subtraction and addition are
		// both exclusive or in the Galois field.)
		for rowBelow := r + 1; rowBelow < rows; rowBelow++ {
			if m[rowBelow][r] != 0 {
				scale := m[rowBelow][r]
				for c := 0; c < columns; c++ {
					m[rowBelow][c] ^= galMultiply(scale, m[r][c])
				}
			}
		}
	}

	// Now clear the part above the main diagonal.
	for d := 0; d < rows; d++ {
		for rowAbove := 0; rowAbove < d; rowAbove++ {
			if m[rowAbove][d] != 0 {
				scale := m[rowAbove][d]
				for c := 0; c < columns; c++ {
					m[rowAbove][c] ^= galMultiply(scale, m[d][c])
				}

			}
		}
	}
	return nil
}

// Create a Vandermonde matrix, which is guaranteed to have the
// property that any subset of rows that forms a square matrix
// is invertible.
func vandermonde(rows, cols int) (matrix, error) {
	result, err := newMatrix(rows, cols)
	if err != nil {
		return nil, err
	}
	for r, row := range result {
		for c := range row {
			result[r][c] = galExp(byte(r), c)
		}
	}
	return result, nil
}
