/**
 * Reed-Solomon Coding over 8-bit values.
 *
 * Copyright 2015, Klaus Post
 * Copyright 2015, Backblaze, Inc.
 */

package reedsolomon

import (
	"errors"
	"runtime"
	"sync"
)

type ReedSolomon struct {
	DataShardCount   int
	ParityShardCount int
	TotalShardCount  int
	m                matrix
	parity           [][]byte
}

// New creates a new encoder and initializes it to
// the number of data shards and parity shards that
// you want to use. You can reuse this encoder.
func New(dataShardCount, parityShardCount int) (*ReedSolomon, error) {
	r := ReedSolomon{
		DataShardCount:   dataShardCount,
		ParityShardCount: parityShardCount,
		TotalShardCount:  dataShardCount + parityShardCount,
	}

	// Start with a Vandermonde matrix.  This matrix would work,
	// in theory, but doesn't have the property that the data
	// shards are unchanged after encoding.
	vm, err := vandermonde(r.TotalShardCount, dataShardCount)
	if err != nil {
		return nil, err
	}

	// Multiply by the inverse of the top square of the matrix.
	// This will make the top square be the identity matrix, but
	// preserve the property that any square subset of rows  is
	// invertible.
	top, _ := vm.SubMatrix(0, 0, dataShardCount, dataShardCount)
	top, _ = top.Invert()
	r.m, err = vm.Multiply(top)

	r.parity = make([][]byte, parityShardCount)
	for i := range r.parity {
		r.parity[i] = r.m[dataShardCount+i]
	}

	return &r, err
}

var ErrTooFewShards = errors.New("too few shards given to encode")

// Encodes parity for a set of data shards.
// An array 'shards' containing data shards followed by parity shards.
// The number of shards must match the number given to New.
// Each shard is a byte array, and they must all be the same size.
// The parity shards will always be overwritten and the data shards
// will remain the same.
func (r ReedSolomon) Encode(shards [][]byte) error {
	if len(shards) != r.TotalShardCount {
		return ErrTooFewShards
	}

	err := checkShards(shards, false)
	if err != nil {
		return err
	}

	// Get the slice of output buffers.
	output := shards[r.DataShardCount:]

	// Do the coding.
	r.codeSomeShards(r.parity, shards[0:r.DataShardCount], output, r.ParityShardCount, len(shards[0]))
	return nil
}

// Verify returns true if the parity shards contain the right data.
// The data is the same format as Encode. No data is modified.
func (r ReedSolomon) Verify(shards [][]byte) (bool, error) {
	if len(shards) != r.TotalShardCount {
		return false, ErrTooFewShards
	}
	err := checkShards(shards, false)
	if err != nil {
		return false, err
	}

	// Slice of buffers being checked.
	toCheck := shards[r.DataShardCount:]

	// Do the checking.
	return r.checkSomeShards(r.parity, shards[0:r.DataShardCount], toCheck, r.ParityShardCount, len(shards[0])), nil
}

// Multiplies a subset of rows from a coding matrix by a full set of
// input shards to produce some output shards.
// 'matrixRows' is The rows from the matrix to use.
// 'inputs' An array of byte arrays, each of which is one input shard.
// The number of inputs used is determined by the length of each matrix row.
// outputs Byte arrays where the computed shards are stored.
// The number of outputs computed, and the
// number of matrix rows used, is determined by
// outputCount, which is the number of outputs to compute.
func (r ReedSolomon) codeSomeShards(matrixRows, inputs, outputs [][]byte, outputCount, byteCount int) {
	if runtime.GOMAXPROCS(0) > 1 {
		r.codeSomeShardsP(matrixRows, inputs, outputs, outputCount, byteCount)
		return
	}
	for iByte := 0; iByte < byteCount; iByte++ {
		for iRow := 0; iRow < outputCount; iRow++ {
			matrixRow := matrixRows[iRow]
			var value byte
			for c := 0; c < r.DataShardCount; c++ {
				// note: manual inlining is slower
				value ^= galMultiply(matrixRow[c], inputs[c][iByte])
			}
			outputs[iRow][iByte] = value
		}
	}
}

// How many bytes per goroutine.
const splitSize = 256

// Perform the same as codeSomeShards, but split the workload into
// several goroutines.
func (r ReedSolomon) codeSomeShardsP(matrixRows, inputs, outputs [][]byte, outputCount, byteCount int) {
	var wg sync.WaitGroup
	left := byteCount
	start := 0
	for {
		do := left
		if do > splitSize {
			do = splitSize
		}
		if do == 0 {
			break
		}
		left -= do
		wg.Add(1)
		go func(start, stop int) {
			for iByte := start; iByte < stop; iByte++ {
				for iRow := 0; iRow < outputCount; iRow++ {
					matrixRow := matrixRows[iRow]
					var value byte
					for c := 0; c < r.DataShardCount; c++ {
						// note: manual inlining is slower
						value ^= galMultiply(matrixRow[c], inputs[c][iByte])
					}
					outputs[iRow][iByte] = value
				}
			}
			wg.Done()
		}(start, start+do)
		start += do
	}
	wg.Wait()
}

// checkSomeShards is nostly the same as codeSomeShards,
// except this will check values and return
// as soon as a difference is found.
func (r ReedSolomon) checkSomeShards(matrixRows, inputs, toCheck [][]byte, outputCount, byteCount int) bool {
	if runtime.GOMAXPROCS(0) > 1 {
		return r.checkSomeShardsP(matrixRows, inputs, toCheck, outputCount, byteCount)
	}
	for iByte := 0; iByte < byteCount; iByte++ {
		for iRow := 0; iRow < outputCount; iRow++ {
			matrixRow := matrixRows[iRow]
			var value byte
			for c := 0; c < r.DataShardCount; c++ {
				// note: manual inlining is slower
				value ^= galMultiply(matrixRow[c], inputs[c][iByte])
			}
			if toCheck[iRow][iByte] != value {
				return false
			}
		}
	}
	return true
}

// Parallel version of checkSomeShards
func (r ReedSolomon) checkSomeShardsP(matrixRows, inputs, toCheck [][]byte, outputCount, byteCount int) bool {
	var wg sync.WaitGroup
	left := byteCount
	start := 0
	failed := make(chan bool)
	for {
		do := left
		if do > splitSize {
			do = splitSize
		}
		if do == 0 {
			break
		}
		left -= do
		wg.Add(1)
		go func(start, stop int) {
			defer wg.Done()
			for iByte := start; iByte < stop; iByte++ {
				for iRow := 0; iRow < outputCount; iRow++ {
					matrixRow := matrixRows[iRow]
					var value byte
					for c := 0; c < r.DataShardCount; c++ {
						// note: manual inlining is slower
						value ^= galMultiply(matrixRow[c], inputs[c][iByte])
					}
					if toCheck[iRow][iByte] != value {
						close(failed)
						return
					}
				}
			}
		}(start, start+do)
		start += do
	}
	wg.Wait()
	select {
	case <-failed:
		return false
	default:
	}
	return true
}

var ErrShardNoData = errors.New("no shard data")
var ErrShardSize = errors.New("shard sizes does not match")

// checkShards will check if shards are the same size
// or 0, if allowed. An error is returned if this fails.
// An error is also returned if all shards are size 0.
func checkShards(shards [][]byte, nilok bool) error {
	if len(shards) == 0 {
		return ErrShardNoData
	}
	size := shardSize(shards)
	if size == 0 {
		return ErrShardNoData
	}
	for _, shard := range shards {
		if len(shard) != size {
			if len(shard) != 0 || !nilok {
				return ErrShardSize
			}
		}
	}
	return nil
}

// shardSize return the size of a single shard.
// The first non-zero size is returned,
// or 0 if all shards are size 0.
func shardSize(shards [][]byte) int {
	for _, shard := range shards {
		if len(shard) != 0 {
			return len(shard)
		}
	}
	return 0
}

// Reconstruct will recreate the missing shards, if possible.
//
// Given a list of shards, some of which contain data, fills in the
// ones that don't have data.
//
// The length of the array must be equal to TotalShardCount.
// You indicate that a shard is missing by setting it to nil.
//
// If there are too few shards to reconstruct the missing
// ones, ErrTooFewShards will be returned.
//
// The reconstructed shard set is complete, but integrity is not verified.
// Use the Verify function to check if data set is ok.
func (r ReedSolomon) Reconstruct(shards [][]byte) error {
	if len(shards) != r.TotalShardCount {
		return ErrTooFewShards
	}
	// Check arguments.
	err := checkShards(shards, true)
	if err != nil {
		return err
	}

	shardSize := shardSize(shards)

	// Quick check: are all of the shards present?  If so, there's
	// nothing to do.
	numberPresent := 0
	for i := 0; i < r.TotalShardCount; i++ {
		if len(shards[i]) != 0 {
			numberPresent += 1
		}
	}
	if numberPresent == r.TotalShardCount {
		// Cool.  All of the shards data data.  We don't
		// need to do anything.
		return nil
	}

	// More complete sanity check
	if numberPresent < r.DataShardCount {
		return ErrTooFewShards
	}

	// Pull out the rows of the matrix that correspond to the
	// shards that we have and build a square matrix.  This
	// matrix could be used to generate the shards that we have
	// from the original data.
	//
	// Also, pull out an array holding just the shards that
	// correspond to the rows of the submatrix.  These shards
	// will be the input to the decoding process that re-creates
	// the missing data shards.
	subMatrix, err := newMatrix(r.DataShardCount, r.DataShardCount)
	if err != nil {
		return err
	}
	subShards := make([][]byte, r.DataShardCount)
	subMatrixRow := 0
	for matrixRow := 0; matrixRow < r.TotalShardCount && subMatrixRow < r.DataShardCount; matrixRow++ {
		if len(shards[matrixRow]) != 0 {
			for c := 0; c < r.DataShardCount; c++ {
				subMatrix[subMatrixRow][c] = r.m[matrixRow][c]
			}
			subShards[subMatrixRow] = shards[matrixRow]
			subMatrixRow++
		}
	}

	// Invert the matrix, so we can go from the encoded shards
	// back to the original data.  Then pull out the row that
	// generates the shard that we want to decode.  Note that
	// since this matrix maps back to the orginal data, it can
	// be used to create a data shard, but not a parity shard.
	dataDecodeMatrix, err := subMatrix.Invert()
	if err != nil {
		return err
	}

	// Re-create any data shards that were missing.
	//
	// The input to the coding is all of the shards we actually
	// have, and the output is the missing data shards.  The computation
	// is done using the special decode matrix we just built.

	outputs := make([][]byte, r.ParityShardCount)
	matrixRows := make([][]byte, r.ParityShardCount)
	outputCount := 0

	for iShard := 0; iShard < r.DataShardCount; iShard++ {
		if len(shards[iShard]) == 0 {
			shards[iShard] = make([]byte, shardSize)
			outputs[outputCount] = shards[iShard]
			matrixRows[outputCount] = dataDecodeMatrix[iShard]
			outputCount++
		}
	}
	r.codeSomeShards(matrixRows, subShards, outputs[:outputCount], outputCount, shardSize)

	// Now that we have all of the data shards intact, we can
	// compute any of the parity that is missing.
	//
	// The input to the coding is ALL of the data shards, including
	// any that we just calculated.  The output is whichever of the
	// data shards were missing.
	outputCount = 0
	for iShard := r.DataShardCount; iShard < r.TotalShardCount; iShard++ {
		if len(shards[iShard]) == 0 {
			shards[iShard] = make([]byte, shardSize)
			outputs[outputCount] = shards[iShard]
			matrixRows[outputCount] = r.parity[iShard-r.DataShardCount]
			outputCount++
		}
	}
	r.codeSomeShards(matrixRows, shards[:r.DataShardCount], outputs[:outputCount], outputCount, shardSize)
	return nil
}
