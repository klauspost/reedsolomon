module github.com/klauspost/reedsolomon

go 1.22

require github.com/klauspost/cpuid/v2 v2.2.10

require golang.org/x/sys v0.30.0 // indirect

retract (
	v1.12.2 // https://github.com/klauspost/reedsolomon/pull/283
	v1.11.6 // https://github.com/klauspost/reedsolomon/issues/240
	[v1.11.3, v1.11.5] // https://github.com/klauspost/reedsolomon/pull/238
	v1.11.2 // https://github.com/klauspost/reedsolomon/pull/229
)
