//+build !noasm
//+build !appengine
//+build !gccgo

// Copyright 2015, Klaus Post, see LICENSE for details.
// Copyright 2019, Minio, Inc.

package reedsolomon

import "sync"

//go:noescape
func _galMulAVX512Parallel82(in, out [][]byte, matrix *[matrixSize82]byte, addTo bool)

//go:noescape
func _galMulAVX512Parallel84(in, out [][]byte, matrix *[matrixSize84]byte, addTo bool)

const (
	dimIn        = 8                            // Number of input rows processed simultaneously
	dimOut82     = 2                            // Number of output rows processed simultaneously for x2 routine
	dimOut84     = 4                            // Number of output rows processed simultaneously for x4 routine
	matrixSize82 = (16 + 16) * dimIn * dimOut82 // Dimension of slice of matrix coefficient passed into x2 routine
	matrixSize84 = (16 + 16) * dimIn * dimOut84 // Dimension of slice of matrix coefficient passed into x4 routine
)

// Construct block of matrix coefficients for 2 outputs rows in parallel
func setupMatrix82(matrixRows [][]byte, inputOffset, outputOffset int, matrix *[matrixSize82]byte) {
	offset := 0
	for c := inputOffset; c < inputOffset+dimIn; c++ {
		for iRow := outputOffset; iRow < outputOffset+dimOut82; iRow++ {
			if c < len(matrixRows[iRow]) {
				coeff := matrixRows[iRow][c]
				copy(matrix[offset*32:], mulTableLow[coeff][:])
				copy(matrix[offset*32+16:], mulTableHigh[coeff][:])
			} else {
				// coefficients not used for this input shard (so null out)
				v := matrix[offset*32 : offset*32+32]
				for i := range v {
					v[i] = 0
				}
			}
			offset += dimIn
			if offset >= dimIn*dimOut82 {
				offset -= dimIn*dimOut82 - 1
			}
		}
	}
}

// Construct block of matrix coefficients for 4 outputs rows in parallel
func setupMatrix84(matrixRows [][]byte, inputOffset, outputOffset int, matrix *[matrixSize84]byte) {
	offset := 0
	for c := inputOffset; c < inputOffset+dimIn; c++ {
		for iRow := outputOffset; iRow < outputOffset+dimOut84; iRow++ {
			if c < len(matrixRows[iRow]) {
				coeff := matrixRows[iRow][c]
				copy(matrix[offset*32:], mulTableLow[coeff][:])
				copy(matrix[offset*32+16:], mulTableHigh[coeff][:])
			} else {
				// coefficients not used for this input shard (so null out)
				v := matrix[offset*32 : offset*32+32]
				for i := range v {
					v[i] = 0
				}
			}
			offset += dimIn
			if offset >= dimIn*dimOut84 {
				offset -= dimIn*dimOut84 - 1
			}
		}
	}
}

// Invoke AVX512 routine for 2 output rows in parallel
func galMulAVX512Parallel82(in, out [][]byte, matrixRows [][]byte, inputOffset, outputOffset int, matrix82 *[matrixSize82]byte) {
	done := len(in[0])
	if done == 0 {
		return
	}

	inputEnd := inputOffset + dimIn
	if inputEnd > len(in) {
		inputEnd = len(in)
	}
	outputEnd := outputOffset + dimOut82
	if outputEnd > len(out) {
		outputEnd = len(out)
	}

	addTo := inputOffset != 0 // Except for the first input column, add to previous results
	_galMulAVX512Parallel82(in[inputOffset:inputEnd], out[outputOffset:outputEnd], matrix82, addTo)

	done = (done >> 6) << 6
	if len(in[0])-done == 0 {
		return
	}

	for c := inputOffset; c < inputOffset+dimIn; c++ {
		for iRow := outputOffset; iRow < outputOffset+dimOut82; iRow++ {
			if c < len(matrixRows[iRow]) {
				mt := mulTable[matrixRows[iRow][c]][:256]
				for i := done; i < len(in[0]); i++ {
					if c == 0 { // only set value for first input column
						out[iRow][i] = mt[in[c][i]]
					} else { // and add for all others
						out[iRow][i] ^= mt[in[c][i]]
					}
				}
			}
		}
	}
}

// Invoke AVX512 routine for 4 output rows in parallel
func galMulAVX512Parallel84(in, out [][]byte, matrixRows [][]byte, inputOffset, outputOffset int, matrix84 *[matrixSize84]byte) {
	done := len(in[0])
	if done == 0 {
		return
	}

	inputEnd := inputOffset + dimIn
	if inputEnd > len(in) {
		inputEnd = len(in)
	}
	outputEnd := outputOffset + dimOut84
	if outputEnd > len(out) {
		outputEnd = len(out)
	}

	addTo := inputOffset != 0 // Except for the first input column, add to previous results
	_galMulAVX512Parallel84(in[inputOffset:inputEnd], out[outputOffset:outputEnd], matrix84, addTo)

	done = (done >> 6) << 6
	if len(in[0])-done == 0 {
		return
	}

	for c := inputOffset; c < inputOffset+dimIn; c++ {
		for iRow := outputOffset; iRow < outputOffset+dimOut84; iRow++ {
			if c < len(matrixRows[iRow]) {
				mt := mulTable[matrixRows[iRow][c]][:256]
				for i := done; i < len(in[0]); i++ {
					if c == 0 { // only set value for first input column
						out[iRow][i] = mt[in[c][i]]
					} else { // and add for all others
						out[iRow][i] ^= mt[in[c][i]]
					}
				}
			}
		}
	}
}

// Perform the same as codeSomeShards, but taking advantage of
// AVX512 parallelism for up to 4x faster execution as compared to AVX2
func (r reedSolomon) codeSomeShardsAvx512(matrixRows, inputs, outputs [][]byte, outputCount, byteCount int) {
	outputRow := 0

	// First process (multiple) batches of 4 output rows in parallel
	if outputRow+dimOut84 <= outputCount {
		matrix84 := [matrixSize84]byte{}
		for ; outputRow+dimOut84 <= outputCount; outputRow += dimOut84 {
			for inputRow := 0; inputRow < len(inputs); inputRow += dimIn {
				setupMatrix84(matrixRows, inputRow, outputRow, &matrix84)
				galMulAVX512Parallel84(inputs, outputs, matrixRows, inputRow, outputRow, &matrix84)
			}
		}
	}
	// Then process a (single) batch of 2 output rows in parallel
	if outputRow+dimOut82 <= outputCount {
		matrix82 := [matrixSize82]byte{}
		for inputRow := 0; inputRow < len(inputs); inputRow += dimIn {
			setupMatrix82(matrixRows, inputRow, outputRow, &matrix82)
			galMulAVX512Parallel82(inputs, outputs, matrixRows, inputRow, outputRow, &matrix82)
		}
		outputRow += dimOut82
	}
	// Lastly, we may have a single output row left (for uneven parity)
	if outputRow < outputCount {
		for c := 0; c < r.DataShards; c++ {
			if c == 0 {
				galMulSlice(matrixRows[outputRow][c], inputs[c], outputs[outputRow], &r.o)
			} else {
				galMulSliceXor(matrixRows[outputRow][c], inputs[c], outputs[outputRow], &r.o)
			}
		}
	}
}

// Perform the same as codeSomeShards, but taking advantage of
// AVX512 parallelism for up to 4x faster execution as compared to AVX2
func (r reedSolomon) codeSomeShardsAvx512P(matrixRows, inputs, outputs [][]byte, outputCount, byteCount int) {
	var wg sync.WaitGroup
	do := byteCount / r.o.maxGoroutines
	if do < r.o.minSplitSize {
		do = r.o.minSplitSize
	}
	// Make sizes divisible by 32
	do = (do + 63) & (^63)
	start := 0
	for start < byteCount {
		if start+do > byteCount {
			do = byteCount - start
		}
		wg.Add(1)
		go func(start, stop int) {
			outputRow := 0
			// First process (multiple) batches of 4 output rows in parallel
			if outputRow+dimOut84 <= outputCount {
				// 1K matrix buffer
				matrix84 := [matrixSize84]byte{}
				for ; outputRow+dimOut84 <= outputCount; outputRow += dimOut84 {
					for inputRow := 0; inputRow < len(inputs); inputRow += dimIn {
						setupMatrix84(matrixRows, inputRow, outputRow, &matrix84)
						// FIXME: Only process from start to stop
						galMulAVX512Parallel84(inputs, outputs, matrixRows, inputRow, outputRow, &matrix84)
					}
				}
			}
			// Then process a (single) batch of 2 output rows in parallel
			if outputRow+dimOut82 <= outputCount {
				// 512B matrix buffer
				matrix82 := [matrixSize82]byte{}
				for inputRow := 0; inputRow < len(inputs); inputRow += dimIn {
					setupMatrix82(matrixRows, inputRow, outputRow, &matrix82)
					// FIXME: Only process from start to stop
					galMulAVX512Parallel82(inputs, outputs, matrixRows, inputRow, outputRow, &matrix82)
				}
				outputRow += dimOut82
			}
			// Lastly, we may have a single output row left (for uneven parity)
			if outputRow < outputCount {
				for c := 0; c < r.DataShards; c++ {
					in := inputs[c][start:stop]
					for iRow := 0; iRow < outputCount; iRow++ {
						if c == 0 {
							galMulSlice(matrixRows[iRow][c], in, outputs[iRow][start:stop], &r.o)
						} else {
							galMulSliceXor(matrixRows[iRow][c], in, outputs[iRow][start:stop], &r.o)
						}
					}
				}
			}
			wg.Done()
		}(start, start+do)
		start += do
	}
	wg.Wait()
}
