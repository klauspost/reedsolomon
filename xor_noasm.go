//go:build noasm || gccgo || appengine || (ppc64le && nopshufb)

package reedsolomon

func sliceXor(in, out []byte, o *options) {
	sliceXorGo(in, out, o)
}
