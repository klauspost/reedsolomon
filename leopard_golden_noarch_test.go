//go:build !arm64 || noasm || appengine || gccgo || nopshufb

package reedsolomon

func goldenArchVariants() []struct {
	name string
	opts []Option
	fast bool
} {
	return nil
}
