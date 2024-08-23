//go:build !noasm && !appengine && !gccgo && !nopshufb

// Copyright 2015, Klaus Post, see LICENSE for details.

package reedsolomon

import (
	"testing"
)

func TestGenGalois(t *testing.T) {
	if defaultOptions.useAVX2 {
		testGenGaloisUpto10x10(t, galMulSlicesAvx2, galMulSlicesAvx2Xor, 32)
	}
}
