package reedsolomon

import "testing"

// Enabling an instruction set must never exceed what the CPU actually supports,
// or the generated assembly will fault. AVX512 and GFNI in particular are
// separate CPU features; CPUs with AVX512 but no GFNI exist (e.g. Skylake-SP).
func TestOptionsCannotForceUnsupported(t *testing.T) {
	var o options
	for _, opt := range []Option{
		WithSSE2(true), WithSSSE3(true), WithAVX2(true),
		WithAVX512(true), WithGFNI(true), WithAVXGFNI(true),
		WithNEON(true), WithSVE(true),
	} {
		opt(&o)
	}
	for _, tt := range []struct {
		name          string
		enabled, have bool
	}{
		{"useSSE2", o.useSSE2, defaultOptions.useSSE2},
		{"useSSSE3", o.useSSSE3, defaultOptions.useSSSE3},
		{"useAVX2", o.useAVX2, defaultOptions.useAVX2},
		{"useAVX512", o.useAVX512, defaultOptions.useAVX512},
		{"useAvx512GFNI", o.useAvx512GFNI, defaultOptions.useAvx512GFNI},
		{"useAvxGNFI", o.useAvxGNFI, defaultOptions.useAvxGNFI},
		{"useNEON", o.useNEON, defaultOptions.useNEON},
		{"useSVE", o.useSVE, defaultOptions.useSVE},
	} {
		if tt.enabled && !tt.have {
			t.Errorf("%s was enabled, but CPU does not support it", tt.name)
		}
	}
}

// The NEON kernels read fixed 32 byte tables, while SVE uses the hardware vector
// length - 16 bytes on a 128 bit core, as set by the galois_arm64.go init. Turning
// SVE off must restore the table size, or NEON silently reads the wrong layout.
func TestSVEVectorLength(t *testing.T) {
	for _, tt := range []struct {
		name string
		opt  Option
	}{
		{"WithSVE(false)", WithSVE(false)},
		{"WithNEON(false)", WithNEON(false)},
	} {
		o := defaultOptions
		o.useSVE, o.vectorLength = true, 16
		tt.opt(&o)
		if o.useSVE {
			t.Errorf("%s: SVE still enabled", tt.name)
		}
		if o.vectorLength != 32 {
			t.Errorf("%s: vectorLength = %d, want 32", tt.name, o.vectorLength)
		}
	}
}
