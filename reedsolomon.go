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

// Encoder is an interface to encode Reed-Salomon parity sets for your data.
type Encoder interface {
	// Encodes parity for a set of data shards.
	// An array 'shards' containing data shards followed by parity shards.
	// The number of shards must match the number given to New.
	// Each shard is a byte array, and they must all be the same size.
	// The parity shards will always be overwritten and the data shards
	// will remain the same.
	Encode(shards [][]byte) error

	// Verify returns true if the parity shards contain the right data.
	// The data is the same format as Encode. No data is modified.
	Verify(shards [][]byte) (bool, error)

	// Reconstruct will recreate the missing shards, if possible.
	//
	// Given a list of shards, some of which contain data, fills in the
	// ones that don't have data.
	//
	// The length of the array must be equal to Shards.
	// You indicate that a shard is missing by setting it to nil.
	//
	// If there are too few shards to reconstruct the missing
	// ones, ErrTooFewShards will be returned.
	//
	// The reconstructed shard set is complete, but integrity is not verified.
	// Use the Verify function to check if data set is ok.
	Reconstruct(shards [][]byte) error
}

// reedSolomon contains a matrix for a specific
// distribution of datashards and parity shards.
// Construct if using New()
type reedSolomon struct {
	DataShards   int // Number of data shards, should not be modified.
	ParityShards int // Number of parity shards, should not be modified.
	Shards       int // Total number of shards. Calculated, and should not be modified.
	m            matrix
	parity       [][]byte
}

// ErrInvShardNum will be returned by New, if you attempt to create
// an Encoder where either data or parity shards is zero or less.
var ErrInvShardNum = errors.New("cannot create Encoder with zero or less data/parity shards")

// New creates a new encoder and initializes it to
// the number of data shards and parity shards that
// you want to use. You can reuse this encoder.
func New(dataShards, parityShards int) (Encoder, error) {
	r := reedSolomon{
		DataShards:   dataShards,
		ParityShards: parityShards,
		Shards:       dataShards + parityShards,
	}

	if dataShards <= 0 || parityShards <= 0 {
		return nil, ErrInvShardNum
	}

	// Start with a Vandermonde matrix.  This matrix would work,
	// in theory, but doesn't have the property that the data
	// shards are unchanged after encoding.
	vm, err := vandermonde(r.Shards, dataShards)
	if err != nil {
		return nil, err
	}

	// Multiply by the inverse of the top square of the matrix.
	// This will make the top square be the identity matrix, but
	// preserve the property that any square subset of rows  is
	// invertible.
	top, _ := vm.SubMatrix(0, 0, dataShards, dataShards)
	top, _ = top.Invert()
	r.m, err = vm.Multiply(top)

	r.parity = make([][]byte, parityShards)
	for i := range r.parity {
		r.parity[i] = r.m[dataShards+i]
	}

	return &r, err
}

// ErrTooFewShards is returned if too few shards where given to
// Encode/Verify/Reconstruct. It will also be returned from Reconstruct
// if there were too few shards to reconstruct the missing data.
var ErrTooFewShards = errors.New("too few shards given")

// Encodes parity for a set of data shards.
// An array 'shards' containing data shards followed by parity shards.
// The number of shards must match the number given to New.
// Each shard is a byte array, and they must all be the same size.
// The parity shards will always be overwritten and the data shards
// will remain the same.
func (r reedSolomon) Encode(shards [][]byte) error {
	if len(shards) != r.Shards {
		return ErrTooFewShards
	}

	err := checkShards(shards, false)
	if err != nil {
		return err
	}

	// Get the slice of output buffers.
	output := shards[r.DataShards:]

	// Do the coding.
	r.codeSomeShards(r.parity, shards[0:r.DataShards], output, r.ParityShards, len(shards[0]))
	return nil
}

// Verify returns true if the parity shards contain the right data.
// The data is the same format as Encode. No data is modified.
func (r reedSolomon) Verify(shards [][]byte) (bool, error) {
	if len(shards) != r.Shards {
		return false, ErrTooFewShards
	}
	err := checkShards(shards, false)
	if err != nil {
		return false, err
	}

	// Slice of buffers being checked.
	toCheck := shards[r.DataShards:]

	// Do the checking.
	return r.checkSomeShards(r.parity, shards[0:r.DataShards], toCheck, r.ParityShards, len(shards[0])), nil
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
func (r reedSolomon) codeSomeShards(matrixRows, inputs, outputs [][]byte, outputCount, byteCount int) {
	if runtime.GOMAXPROCS(0) > 1 {
		r.codeSomeShardsP(matrixRows, inputs, outputs, outputCount, byteCount)
		return
	}
	for iByte := 0; iByte < byteCount; iByte++ {
		for iRow := 0; iRow < outputCount; iRow++ {
			matrixRow := matrixRows[iRow]
			var value byte
			for c := 0; c < r.DataShards; c++ {
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
func (r reedSolomon) codeSomeShardsP(matrixRows, inputs, outputs [][]byte, outputCount, byteCount int) {
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
					for c := 0; c < r.DataShards; c++ {
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
func (r reedSolomon) checkSomeShards(matrixRows, inputs, toCheck [][]byte, outputCount, byteCount int) bool {
	if runtime.GOMAXPROCS(0) > 1 {
		return r.checkSomeShardsP(matrixRows, inputs, toCheck, outputCount, byteCount)
	}
	for iByte := 0; iByte < byteCount; iByte++ {
		for iRow := 0; iRow < outputCount; iRow++ {
			matrixRow := matrixRows[iRow]
			var value byte
			for c := 0; c < r.DataShards; c++ {
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
func (r reedSolomon) checkSomeShardsP(matrixRows, inputs, toCheck [][]byte, outputCount, byteCount int) bool {
	var wg sync.WaitGroup
	left := byteCount
	start := 0

	same := true
	var mu sync.RWMutex // For above

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
					for c := 0; c < r.DataShards; c++ {
						// note: manual inlining is slower
						value ^= galMultiply(matrixRow[c], inputs[c][iByte])
					}
					if toCheck[iRow][iByte] != value {
						mu.Lock()
						same = false
						mu.Unlock()
						return
					}
				}
				// At regular intervals check if others have failed and return if so
				if iByte&15 == 15 {
					mu.RLock()
					if !same {
						mu.RUnlock()
						return
					}
					mu.RUnlock()
				}
			}
		}(start, start+do)
		start += do
	}
	wg.Wait()
	return same
}

// ErrShardNoData will be returned if there are no shards,
// or if the length of all shards is zero.
var ErrShardNoData = errors.New("no shard data")

// ErrShardSize is returned if shard length isn't the same for all
// shards.
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
// The length of the array must be equal to Shards.
// You indicate that a shard is missing by setting it to nil.
//
// If there are too few shards to reconstruct the missing
// ones, ErrTooFewShards will be returned.
//
// The reconstructed shard set is complete, but integrity is not verified.
// Use the Verify function to check if data set is ok.
func (r reedSolomon) Reconstruct(shards [][]byte) error {
	if len(shards) != r.Shards {
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
	for i := 0; i < r.Shards; i++ {
		if len(shards[i]) != 0 {
			numberPresent++
		}
	}
	if numberPresent == r.Shards {
		// Cool.  All of the shards data data.  We don't
		// need to do anything.
		return nil
	}

	// More complete sanity check
	if numberPresent < r.DataShards {
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
	subMatrix, err := newMatrix(r.DataShards, r.DataShards)
	if err != nil {
		return err
	}
	subShards := make([][]byte, r.DataShards)
	subMatrixRow := 0
	for matrixRow := 0; matrixRow < r.Shards && subMatrixRow < r.DataShards; matrixRow++ {
		if len(shards[matrixRow]) != 0 {
			for c := 0; c < r.DataShards; c++ {
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

	outputs := make([][]byte, r.ParityShards)
	matrixRows := make([][]byte, r.ParityShards)
	outputCount := 0

	for iShard := 0; iShard < r.DataShards; iShard++ {
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
	for iShard := r.DataShards; iShard < r.Shards; iShard++ {
		if len(shards[iShard]) == 0 {
			shards[iShard] = make([]byte, shardSize)
			outputs[outputCount] = shards[iShard]
			matrixRows[outputCount] = r.parity[iShard-r.DataShards]
			outputCount++
		}
	}
	r.codeSomeShards(matrixRows, shards[:r.DataShards], outputs[:outputCount], outputCount, shardSize)
	return nil
}
