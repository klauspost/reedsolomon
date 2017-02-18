package reedsolomon

import (
	"runtime"

	"github.com/klauspost/cpuid"
	"errors"
	"sync"
)

// Options allows to override processing parameters.
// Options should be based on DefaultOptions and not created from scratch.
type Options struct {
	maxGoroutines int
	minSplitSize  int
	useAVX2, useSSSE3 bool

	// Unsetable
	valid *struct{}
}

var defaultOptions = Options {
	maxGoroutines: 50,
	minSplitSize:  512,
}
var defaultOptionsMu sync.RWMutex

// ErrInvalidOptions will be returned, if your options are not based on
// DefaultOptions.
var ErrInvalidOptions = errors.New("invalid option set")

// DefaultOptions returns the default options.
func DefaultOptions() Options {
	defaultOptionsMu.RLock()
	o := defaultOptions
	defaultOptionsMu.RUnlock()
	return o
}

// SetDefaultOptions will override the default options.
func SetDefaultOptions(o Options) error {
	if o.valid == nil {
		return ErrInvalidOptions
	}
	defaultOptionsMu.Lock()
	defaultOptions = o
	defaultOptionsMu.Unlock()
	return nil
}

func init() {
	if runtime.GOMAXPROCS(0) <= 1 {
		defaultOptions.maxGoroutines = 1
	}
	// Detect CPU capabilities.
	defaultOptions.useSSSE3 = cpuid.CPU.SSSE3()
	defaultOptions.useAVX2 = cpuid.CPU.AVX2()
	defaultOptions.valid = &struct{}{}
}

// MaxGoroutines is the maximum number of goroutines number for encoding & decoding.
// Jobs will be split into this many parts, unless each goroutine would have to process
// less than minSplitSize bytes (set below).
// For the best speed, keep this well above the GOMAXPROCS number for more fine grained
// scheduling.
// If n < 0, default options will be used.
func (o Options) MaxGoroutines(n int) Options {
	if n <= 0 {
		n = defaultOptions.maxGoroutines
	}
	o.maxGoroutines = n
	return o
}

// MinSplitSize Is the minimum encoding size in bytes per goroutine.
// See MaxGoroutines on how jobs are split.
// If n < 0, default options will be used.
func (o Options) MinSplitSize(n int) Options {
	if n <= 0 {
		n = defaultOptions.maxGoroutines
	}
	o.minSplitSize = n
	return o
}
