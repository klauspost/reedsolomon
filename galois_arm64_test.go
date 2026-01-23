//go:build !noasm && !appengine && !gccgo && !nopshufb

// Copyright 2015, Klaus Post, see LICENSE for details.
// Copyright 2024, Minio, Inc.

package reedsolomon

import (
	"testing"
)

func TestGenGalois(t *testing.T) {
	if defaultOptions.useSVE {
		testGenGaloisUpto10x10(t, galMulSlicesSve, galMulSlicesSveXor, defaultOptions.vectorLength)
	}
	if defaultOptions.useNEON {
		testGenGaloisUpto10x10(t, galMulSlicesNeon, galMulSlicesNeonXor, 32)
	}
}
