/**
 * Reed-Solomon Coding over 8-bit values.
 *
 * Copyright 2015, Klaus Post
 * Copyright 2015, Backblaze, Inc.
 */

// Reed-Solomon Erasure Coding in Go
//
// For usage and examples, see https://github.com/klauspost/reedsolomon
//
package reedsolomon

import (
	"bytes"
	"errors"
	"io"
)

// StreamEncoder is an interface to encode Reed-Salomon parity sets for your data.
// It provides a fully streaming interface, and processes data in blocks of up to 4MB.
type StreamEncoder interface {
	// Encodes parity shards for a set of data shards.
	// Input is 'shards' containing readers for data shards followed by parity shards
	// io.Writer.
	// The number of shards must match the number given to New().
	// Each shard is a byte array, and they must all be the same size.
	// The parity shards will always be overwritten and the data shards
	// will remain the same, so it is safe for you to read from the
	// data shards while this is running.
	Encode(data []io.Reader, parity []io.Writer) error

	// Verify returns true if the parity shards contain correct data.
	// The data is the same format as Encode. No data is modified, so
	// you are allowed to read from data while this is running.
	Verify(shards []io.Reader) (bool, error)

	// Reconstruct will recreate the missing shards if possible.
	//
	// Given a list of valid shards (to read) and invalid shards (to write)
	//
	// The length of each slice must be equal to the total number of shards.
	// You indicate that a shard is missing by setting it to nil.
	//
	// You indicate that you would like to have a shard filled by setting
	// a non-nil writer in "fill". An index cannot contain both non-nil
	// 'valid' and 'fill' entry.
	//
	// If there are too few shards to reconstruct the missing
	// ones, ErrTooFewShards will be returned.
	//
	// The reconstructed shard set is complete, but integrity is not verified.
	// Use the Verify function to check if data set is ok.
	Reconstruct(valid []io.Reader, fill []io.Writer) error

	// Split a data slice into the number of shards given to the encoder.
	//
	// You must supply the total size of your input.
	//
	// The data will be split into equally sized shards.
	// If the data size isn't dividable by the number of shards,
	// the last shard will contain extra zeros.
	//
	// There must be at least the same number of bytes as there are data shards,
	// otherwise ErrShortData will be returned.
	Split(data io.Reader, dst []io.Writer, size int64) (err error)

	// Join the shards and write the data segment to dst.
	//
	// Only the data shards are considered.
	//
	// You must supply the exact output size you want.
	// If there are to few shards given, ErrTooFewShards will be returned.
	// If the total data size is less than outSize, ErrShortData will be returned.
	Join(dst io.Writer, shards []io.Reader, outSize int64) error
}

// reedSolomon contains a matrix for a specific
// distribution of datashards and parity shards.
// Construct if using New()
type rsStream struct {
	r  *reedSolomon
	bs int // Block size
}

// New creates a new encoder and initializes it to
// the number of data shards and parity shards that
// you want to use. You can reuse this encoder.
// Note that the maximum number of data shards is 256.
func NewStream(dataShards, parityShards int) (StreamEncoder, error) {
	enc, err := New(dataShards, parityShards)
	if err != nil {
		return nil, err
	}
	rs := enc.(*reedSolomon)
	r := rsStream{r: rs, bs: 4 << 20}
	return &r, err
}

func createSlice(n, length int) [][]byte {
	out := make([][]byte, n)
	for i := range out {
		out[i] = make([]byte, length)
	}
	return out
}

// Encodes parity for a set of data shards.
// An array 'shards' containing data shards followed by parity shards.
// The number of shards must match the number given to New.
// Each shard is a byte array, and they must all be the same size.
// The parity shards will always be overwritten and the data shards
// will remain the same.
func (r rsStream) Encode(data []io.Reader, parity []io.Writer) error {
	if len(data) != r.r.DataShards {
		return ErrTooFewShards
	}

	if len(parity) != r.r.ParityShards {
		return ErrTooFewShards
	}

	all := createSlice(r.r.Shards, r.bs)
	in := all[:r.r.DataShards]
	out := all[r.r.DataShards:]

	for {
		err := readShards(in, data)
		switch err {
		case nil:
		case io.EOF:
			return nil
		default:
			return err
		}
		out = trimShards(out, shardSize(in))
		err = r.r.Encode(all)
		if err != nil {
			return err
		}
		err = writeShards(parity, out)
		if err != nil {
			return err
		}
	}
}

// Trim the shards so they are all the same size
func trimShards(in [][]byte, size int) [][]byte {
	for i := range in {
		if in[i] != nil {
			in[i] = in[i][0:size]
		}
	}
	return in
}

func readShards(dst [][]byte, in []io.Reader) error {
	if len(in) != len(dst) {
		panic("internal error: in and dst size does not match")
	}
	size := -1
	for i := range in {
		if in[i] == nil {
			dst[i] = nil
			continue
		}
		n, err := io.ReadFull(in[i], dst[i])
		// The error is EOF only if no bytes were read.
		// If an EOF happens after reading some but not all the bytes,
		// ReadFull returns ErrUnexpectedEOF.
		switch err {
		case io.ErrUnexpectedEOF, io.EOF:
			if size < 0 {
				size = n
			} else if n != size {
				// Shard sizes must match.
				return ErrShardSize
			}
			dst[i] = dst[i][0:n]
		case nil:
		default:
			return err
		}
	}
	if size == 0 {
		return io.EOF
	}
	return nil
}

func writeShards(out []io.Writer, in [][]byte) error {
	if len(out) != len(in) {
		panic("internal error: in and out size does not match")
	}
	for i := range in {
		if out[i] == nil {
			continue
		}
		n, err := out[i].Write(in[i])
		if err != nil {
			return err
		}
		//
		if n != len(in[i]) {
			return io.ErrShortWrite
		}
	}
	return nil
}

// Verify returns true if the parity shards contain the right data.
// The data is the same format as Encode. No data is modified.
func (r rsStream) Verify(shards []io.Reader) (bool, error) {
	if len(shards) != r.r.Shards {
		return false, ErrTooFewShards
	}

	all := createSlice(r.r.Shards, r.bs)
	for {
		err := readShards(all, shards)
		if err == io.EOF {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		ok, err := r.r.Verify(all)
		if !ok || err != nil {
			return ok, err
		}
	}

	return false, nil
}

// This error is returned by the StreamEncoder, if you supply "valid" and "fill" streams
// on the same index.
// Therefore it is impossible to see if you consider the shard valid
// or would like to have it reconstructed.
var ErrReconstructMismatch = errors.New("valid shards and fill shards are mutully exclusive")

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
func (r rsStream) Reconstruct(valid []io.Reader, fill []io.Writer) error {
	if len(valid) != r.r.Shards {
		return ErrTooFewShards
	}
	if len(fill) != r.r.Shards {
		return ErrTooFewShards
	}

	all := createSlice(r.r.Shards, r.bs)
	for i := range valid {
		if valid[i] != nil && fill[i] != nil {
			return ErrReconstructMismatch
		}
	}

	for {
		err := readShards(all, valid)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		all = trimShards(all, shardSize(all))

		err = r.r.Reconstruct(all)
		if err != nil {
			return err
		}
		err = writeShards(fill, all)
		if err != nil {
			return err
		}
	}
}

// Join the shards and write the data segment to dst.
//
// Only the data shards are considered.
// You must supply the exact output size you want.
// If there are to few shards given, ErrTooFewShards will be returned.
// If the total data size is less than outSize, ErrShortData will be returned.
func (r rsStream) Join(dst io.Writer, shards []io.Reader, outSize int64) error {
	// Do we have enough shards?
	if len(shards) < r.r.DataShards {
		return ErrTooFewShards
	}

	// Trim off parity shards if any
	shards = shards[:r.r.DataShards]
	for i := range shards {
		if shards[i] == nil {
			return ErrShardNoData
		}
	}
	// Join all shards
	src := io.MultiReader(shards...)

	// Copy data to dst
	n, err := io.CopyN(dst, src, outSize)
	if err == io.EOF {
		return ErrShortData
	}
	if err != nil {
		return err
	}
	if n != outSize {
		return ErrShortData
	}
	return nil
}

// Split a data slice into the number of shards given to the encoder,
// and create empty parity shards.
//
// The data will be split into equally sized shards.
// If the data size isn't divisible by the number of shards,
// the last shard will contain extra zeros.
//
// There must be at least the same number of bytes as there are data shards,
// otherwise ErrShortData will be returned.
func (r rsStream) Split(data io.Reader, dst []io.Writer, size int64) error {
	if size < int64(r.r.DataShards) {
		return ErrShortData
	}

	if len(dst) != r.r.DataShards {
		return ErrInvShardNum
	}

	for i := range dst {
		if dst[i] == nil {
			return ErrShardNoData
		}
	}

	// Calculate number of bytes per shard.
	perShard := (size + int64(r.r.DataShards) - 1) / int64(r.r.DataShards)

	// Pad data to r.Shards*perShard.
	padding := make([]byte, (int64(r.r.Shards)*perShard)-size)
	data = io.MultiReader(data, bytes.NewBuffer(padding))

	// Split into equal-length shards and copy.
	for i := range dst {
		n, err := io.CopyN(dst[i], data, perShard)
		if err != io.EOF && err != nil {
			return err
		}
		if n != perShard {
			return ErrShortData
		}
	}

	return nil
}
