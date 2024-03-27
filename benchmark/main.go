// Copyright 2023+, Klaus Post, see LICENSE for details.

package main

import (
	"errors"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/klauspost/cpuid/v2"
	"github.com/klauspost/reedsolomon"
)

var (
	blockSize  = flag.String("size", "10MiB", "Size of each input block.")
	blocks     = flag.Int("blocks", 1, "Total number of blocks")
	kShards    = flag.Int("k", 12, "Data shards")
	mShards    = flag.Int("m", 4, "Parity shards")
	codec      = flag.String("codec", "vandermonde", "Encoder Algorithm")
	codecs     = flag.Bool("codecs", false, "Display codecs and exit")
	invCache   = flag.Bool("cache", true, "Enable inversion cache")
	corrupt    = flag.Int("corrupt", 0, "Corrupt 1 to n shards. 0 means up to m shards.")
	duration   = flag.Int("duration", 10, "Minimum number of seconds to run.")
	progress   = flag.Bool("progress", true, "Display progress while running")
	concurrent = flag.Bool("concurrent", false, "Run blocks in parallel")
	cpu        = flag.Int("cpu", 16, "Set maximum number of cores to use")
	csv        = flag.Bool("csv", false, "Output as CSV")

	sSE2     = flag.Bool("sse2", cpuid.CPU.Has(cpuid.SSE2), "Use SSE2")
	sSSE3    = flag.Bool("ssse3", cpuid.CPU.Has(cpuid.SSSE3), "Use SSSE3")
	aVX2     = flag.Bool("avx2", cpuid.CPU.Has(cpuid.AVX2), "Use AVX2")
	aVX512   = flag.Bool("avx512", cpuid.CPU.Supports(cpuid.AVX512F, cpuid.AVX512BW, cpuid.AVX512VL), "Use AVX512")
	gNFI     = flag.Bool("gfni", cpuid.CPU.Supports(cpuid.AVX512F, cpuid.GFNI, cpuid.AVX512DQ), "Use AVX512+GFNI")
	avx2GNFI = flag.Bool("avx-gfni", cpuid.CPU.Supports(cpuid.AVX2, cpuid.GFNI), "Use AVX+GFNI")
)

var codecDefinitions = map[string]struct {
	Description string
	MaxKM       int
	MaxM        int
	Opts        []reedsolomon.Option
}{
	"vandermonde": {Description: "Vandermonde style matrix", MaxKM: 256},
	"cauchy":      {Description: "Cauchy style matrix", MaxKM: 256, Opts: []reedsolomon.Option{reedsolomon.WithCauchyMatrix()}},
	"jerasure":    {Description: "Uses Vandermonde matrix in the same way as done by the Jerasure library", MaxKM: 256, Opts: []reedsolomon.Option{reedsolomon.WithJerasureMatrix()}},
	"xor":         {Description: "XOR - supporting only one parity shard", MaxKM: 256, MaxM: 1, Opts: []reedsolomon.Option{reedsolomon.WithFastOneParityMatrix()}},
	"par1":        {Description: "PAR1 style matrix (not reliable)", MaxKM: 256, MaxM: 1, Opts: []reedsolomon.Option{reedsolomon.WithPAR1Matrix()}},
	"leopard":     {Description: "Progressive Leopard-RS encoding, automatic choose 8 or 16 bits", MaxKM: 65536, Opts: []reedsolomon.Option{reedsolomon.WithLeopardGF(true)}},
	"leopard8":    {Description: "Progressive Leopard-RS encoding, 8 bits", MaxKM: 256, Opts: []reedsolomon.Option{reedsolomon.WithLeopardGF(true)}},
	"leopard16":   {Description: "Progressive Leopard-RS encoding, 16 bits", MaxKM: 65536, Opts: []reedsolomon.Option{reedsolomon.WithLeopardGF16(true)}},
}

func main() {
	flag.Parse()
	if *codecs {
		printCodecs(0)
	}
	sz, err := toSize(*blockSize)
	exitErr(err)
	if *kShards <= 0 {
		exitErr(errors.New("invalid k shard count"))
	}
	if sz <= 0 {
		exitErr(errors.New("invalid block size"))
	}
	runtime.GOMAXPROCS(*cpu)
	if sz > math.MaxInt || sz < 0 {
		exitErr(errors.New("block size invalid"))
	}
	if *csv {
		fmt.Printf("op\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n", "k", "m", "bsize", "blocks", "concurrency", "codec", "processed bytes", "duration (μs)", "speed ("+sizeUint+"/s)")
	}
	dataSz := int(sz)
	each := (dataSz + *kShards - 1) / *kShards

	opts := getOptions(each)
	enc, err := reedsolomon.New(*kShards, *mShards, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating encoder returned: %s\n", err.Error())
		os.Exit(1)
	}

	total := *kShards + *mShards
	data := make([][][]byte, *blocks)
	ext := enc.(reedsolomon.Extensions)
	mulOf := ext.ShardSizeMultiple()
	each = ((each + mulOf - 1) / mulOf) * mulOf
	if *csv {
		*progress = false
	} else {
		fmt.Printf("Benchmarking %d block(s) of %d data (K) and %d parity shards (M), each %d bytes using %d threads. Total %d bytes.\n\n", *blocks, *kShards, *mShards, each, *cpu, *blocks*each*total)
	}

	// Reduce GC overhead
	debug.SetGCPercent(25)
	for i := range data {
		data[i] = reedsolomon.AllocAligned(total, each)
	}
	if *concurrent {
		benchmarkEncodingConcurrent(enc, data)
		benchmarkDecodingConcurrent(enc, data)
	} else {
		benchmarkEncoding(enc, data)
		benchmarkDecoding(enc, data)
	}
}

const updateFreq = time.Second / 3

var spin = [...]byte{'|', '/', '-', '\\'}

/*
const speedDivisor = float64(1 << 30)
const speedUnit = "Gbps"
const sizeUint = "GiB"
const speedBitMul = 8
*/

const speedDivisor = float64(1 << 20)
const speedUnit = "MiB/s"
const sizeUint = "MiB"
const speedBitMul = 1

func benchmarkEncoding(enc reedsolomon.Encoder, data [][][]byte) {
	ext := enc.(reedsolomon.Extensions)
	parityShards := ext.ParityShards()
	dataShards := ext.DataShards()

	start := time.Now()
	finished := int64(0)
	lastUpdate := start
	end := start.Add(time.Second * time.Duration(*duration))
	spinIdx := 0
	for time.Now().Before(end) {
		for _, shards := range data {
			err := enc.Encode(shards)
			exitErr(err)
			finished += int64(len(shards[0]) * len(shards))
			if *progress && time.Since(lastUpdate) > updateFreq {
				encGB := float64(finished) * (1 / speedDivisor)
				speed := encGB / (float64(time.Since(start)) / float64(time.Second))
				fmt.Printf("\r %s Encoded: %.02f %s @%.02f %s.", string(spin[spinIdx]), encGB, sizeUint, speed*speedBitMul, speedUnit)
				spinIdx = (spinIdx + 1) % len(spin)
				lastUpdate = time.Now()
			}
		}
	}
	encGB := float64(finished) * (1 / speedDivisor)
	speed := encGB / (float64(time.Since(start)) / float64(time.Second))
	if *csv {
		fmt.Printf("encode\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n", *kShards, *mShards, *blockSize, *blocks, *cpu, *codec, finished, time.Since(start).Microseconds(), speed*speedBitMul)
	} else {
		fmt.Printf("\r * Encoded %.00f %s in %v. Speed: %.02f %s (%d+%d:%d)\n", encGB, sizeUint, time.Since(start).Round(time.Millisecond), speedBitMul*speed, speedUnit, dataShards, parityShards, len(data[0][0]))
	}
}

func benchmarkEncodingConcurrent(enc reedsolomon.Encoder, data [][][]byte) {
	ext := enc.(reedsolomon.Extensions)
	parityShards := ext.ParityShards()
	dataShards := ext.DataShards()

	start := time.Now()
	finished := int64(0)
	end := start.Add(time.Second * time.Duration(*duration))
	spinIdx := 0
	var wg sync.WaitGroup
	var exit = make(chan struct{})
	wg.Add(len(data))
	for _, shards := range data {
		go func(shards [][]byte) {
			defer wg.Done()
			for {
				select {
				case <-exit:
					return
				default:
				}
				err := enc.Encode(shards)
				exitErr(err)
				atomic.AddInt64(&finished, int64(len(shards[0])*len(shards)))
			}
		}(shards)
	}

	t := time.NewTicker(updateFreq)
	defer t.Stop()

	for range t.C {
		if time.Now().After(end) {
			break
		}
		if *progress {
			encGB := float64(atomic.LoadInt64(&finished)) * (1 / speedDivisor)
			speed := encGB / (float64(time.Since(start)) / float64(time.Second))
			fmt.Printf("\r %s Encoded: %.02f %s @%.02f %s.", string(spin[spinIdx]), encGB, sizeUint, speed*speedBitMul, speedUnit)
			spinIdx = (spinIdx + 1) % len(spin)
		}
	}
	close(exit)
	wg.Wait()
	encGB := float64(finished) * (1 / speedDivisor)
	speed := encGB / (float64(time.Since(start)) / float64(time.Second))
	if *csv {
		fmt.Printf("encode conc\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n", *kShards, *mShards, *blockSize, *blocks, *cpu, *codec, finished, time.Since(start).Microseconds(), speed*speedBitMul)
	} else {
		fmt.Printf("\r * Encoded concurrent %.00f %s in %v. Speed: %.02f %s (%d+%d:%d/%d)\n", encGB, sizeUint, time.Since(start).Round(time.Millisecond), speedBitMul*speed, speedUnit, dataShards, parityShards, len(data[0][0]), len(data))
	}
}

func benchmarkDecoding(enc reedsolomon.Encoder, data [][][]byte) {
	// Prepare
	for _, shards := range data {
		err := enc.Encode(shards)
		exitErr(err)
	}
	ext := enc.(reedsolomon.Extensions)
	parityShards := ext.ParityShards()
	dataShards := ext.DataShards()
	rng := rand.New(rand.NewSource(0))

	start := time.Now()
	finished := int64(0)
	lastUpdate := start
	end := start.Add(time.Second * time.Duration(*duration))
	spinIdx := 0
	for time.Now().Before(end) {
		for _, shards := range data {
			// Corrupt random number of shards up to what we can allow
			cor := *corrupt
			if cor == 0 {
				cor = 1 + rng.Intn(parityShards)
			}
			for cor > 0 {
				idx := rng.Intn(len(shards))
				if len(shards[idx]) > 0 {
					shards[idx] = shards[idx][:0]
					cor--
				}
			}
			err := enc.Reconstruct(shards)
			exitErr(err)
			finished += int64(len(shards[0]) * len(shards))
			if *progress && time.Since(lastUpdate) > updateFreq {
				encGB := float64(finished) * (1 / speedDivisor)
				speed := encGB / (float64(time.Since(start)) / float64(time.Second))
				fmt.Printf("\r %s Repaired: %.02f %s @%.02f %s.", string(spin[spinIdx]), encGB, sizeUint, speed*speedBitMul, speedUnit)
				spinIdx = (spinIdx + 1) % len(spin)
				lastUpdate = time.Now()
			}
		}
	}
	encGB := float64(finished) * (1 / speedDivisor)
	speed := encGB / (float64(time.Since(start)) / float64(time.Second))
	if *csv {
		fmt.Printf("decode\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n", *kShards, *mShards, *blockSize, *blocks, *cpu, *codec, finished, time.Since(start).Microseconds(), speed)
	} else {
		fmt.Printf("\r * Repaired %.00f %s in %v. Speed: %.02f %s (%d+%d:%d)\n", encGB, sizeUint, time.Since(start).Round(time.Millisecond), speedBitMul*speed, speedUnit, dataShards, parityShards, len(data[0][0]))
	}
}

func benchmarkDecodingConcurrent(enc reedsolomon.Encoder, data [][][]byte) {
	// Prepare
	for _, shards := range data {
		err := enc.Encode(shards)
		exitErr(err)
	}
	ext := enc.(reedsolomon.Extensions)
	parityShards := ext.ParityShards()
	dataShards := ext.DataShards()

	start := time.Now()
	finished := int64(0)
	end := start.Add(time.Second * time.Duration(*duration))
	spinIdx := 0
	var wg sync.WaitGroup
	var exit = make(chan struct{})
	wg.Add(len(data))
	for _, shards := range data {
		go func(shards [][]byte) {
			rng := rand.New(rand.NewSource(0))
			defer wg.Done()
			for {
				select {
				case <-exit:
					return
				default:
				}
				// Corrupt random number of shards up to what we can allow
				cor := *corrupt
				if cor == 0 {
					cor = 1 + rng.Intn(parityShards)
				}
				for cor > 0 {
					idx := rng.Intn(len(shards))
					if len(shards[idx]) > 0 {
						shards[idx] = shards[idx][:0]
						cor--
					}
				}
				err := enc.Reconstruct(shards)
				exitErr(err)
				atomic.AddInt64(&finished, int64(len(shards[0])*len(shards)))
			}
		}(shards)
	}
	t := time.NewTicker(updateFreq)
	defer t.Stop()
	for range t.C {
		if time.Now().After(end) {
			break
		}
		if *progress {
			encGB := float64(finished) * (1 / speedDivisor)
			speed := encGB / (float64(time.Since(start)) / float64(time.Second))
			fmt.Printf("\r %s Repaired: %.02f %s @%.02f %s.", string(spin[spinIdx]), encGB, sizeUint, speed*speedBitMul, speedUnit)
			spinIdx = (spinIdx + 1) % len(spin)
		}
	}
	encGB := float64(finished) * (1 / speedDivisor)
	speed := encGB / (float64(time.Since(start)) / float64(time.Second))
	if *csv {
		fmt.Printf("decode conc\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n", *kShards, *mShards, *blockSize, *blocks, *cpu, *codec, finished, time.Since(start).Microseconds(), speed)
	} else {
		fmt.Printf("\r * Repaired concurrent %.00f %s in %v. Speed: %.02f %s (%d+%d:%d/%d)\n", encGB, sizeUint, time.Since(start).Round(time.Millisecond), speedBitMul*speed, speedUnit, dataShards, parityShards, len(data[0][0]), len(data))
	}
}

func printCodecs(exitCode int) {
	var keys []string
	maxLen := 0
	for k := range codecDefinitions {
		keys = append(keys, k)
		if len(k) > maxLen {
			maxLen = len(k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		def := codecDefinitions[k]
		k = k + strings.Repeat(" ", maxLen-len(k))
		fmt.Printf("%s %s. Max K+M: %d.", k, def.Description, def.MaxKM)
		if def.MaxM > 0 {
			fmt.Printf(" Max M: %d.", def.MaxM)
		}
		fmt.Print("\n")
	}
	// Exit
	if exitCode >= 0 {
		os.Exit(exitCode)
	}
}

func getOptions(shardSize int) []reedsolomon.Option {
	var o []reedsolomon.Option
	c, ok := codecDefinitions[*codec]
	if !ok {
		fmt.Fprintf(os.Stderr, "ERR: unknown codec: %q\n", *codec)
		printCodecs(1)
	}
	total := *kShards + *mShards
	if total > c.MaxKM {
		fmt.Fprintf(os.Stderr, "ERR: maximum shards (k+m) %d exceeds maximum %d for codex %q\n", total, c.MaxKM, *codec)
		os.Exit(1)
	}
	if c.MaxM > 0 && *mShards > c.MaxM {
		fmt.Fprintf(os.Stderr, "ERR: maximum parity shards (m) %d exceeds maximum %d for codex %q\n", *mShards, c.MaxM, *codec)
		os.Exit(1)
	}
	o = append(o, c.Opts...)
	if !*sSSE3 {
		o = append(o, reedsolomon.WithSSSE3(false))
	}
	if !*sSE2 {
		o = append(o, reedsolomon.WithSSE2(false))
	}
	if !*aVX2 {
		o = append(o, reedsolomon.WithAVX2(false))
	}
	if !*aVX512 {
		o = append(o, reedsolomon.WithAVX512(false))
	}
	if !*gNFI {
		o = append(o, reedsolomon.WithGFNI(false))
	}
	if !*avx2GNFI {
		o = append(o, reedsolomon.WithAVXGFNI(false))
	}
	if !*invCache {
		o = append(o, reedsolomon.WithInversionCache(false))
	}
	o = append(o, reedsolomon.WithAutoGoroutines(shardSize))
	return o
}

func exitErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERR: %s\n", err.Error())
		os.Exit(1)
	}
}

// toSize converts a size indication to bytes.
func toSize(size string) (uint64, error) {
	size = strings.ToUpper(strings.TrimSpace(size))
	firstLetter := strings.IndexFunc(size, unicode.IsLetter)
	if firstLetter == -1 {
		firstLetter = len(size)
	}

	bytesString, multiple := size[:firstLetter], size[firstLetter:]
	bytes, err := strconv.ParseUint(bytesString, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unable to parse size: %v", err)
	}

	switch multiple {
	case "G", "GIB":
		return bytes * 1 << 30, nil
	case "GB":
		return bytes * 1e9, nil
	case "M", "MIB":
		return bytes * 1 << 20, nil
	case "MB":
		return bytes * 1e6, nil
	case "K", "KIB":
		return bytes * 1 << 10, nil
	case "KB":
		return bytes * 1e3, nil
	case "B", "":
		return bytes, nil
	default:
		return 0, fmt.Errorf("unknown size suffix: %v", multiple)
	}
}
