# Reed-Solomon
[![GoDoc][1]][2] [![Build Status][3]][4]

[1]: https://godoc.org/github.com/klauspost/reedsolomon?status.svg
[2]: https://godoc.org/github.com/klauspost/reedsolomon
[3]: https://travis-ci.org/klauspost/reedsolomon.svg
[4]: https://travis-ci.org/klauspost/reedsolomon

Reed Solomon Erasure Coding in Go

This is a golang port of the [JavaReedSolomon](https://github.com/Backblaze/JavaReedSolomon) library released by [Backblaze](backblaze.com).

For an introduction on erasure coding, see the post on the [Backblaze blog](https://www.backblaze.com/blog/reed-solomon/).

Package home: https://github.com/klauspost/reedsolomon
Godoc: https://godoc.org/github.com/klauspost/reedsolomon

# Installation
To get the package use the standard:
```bash
go get github.com/klauspost/reedsolomon
```
# Usage

# Performance
Performance depends mainly on the number of parity shards. In rough terms, doubling the number of parity shards will double the encoding time.

Here are the throughput numbers with some different selections of data and parity shards. For reference each shard is 1MB random data, and 2 CPU cores are used for encoding.

| Data | Parity | MB/s   | Parity | Speed   |
|------|--------|--------|--------|---------|
| 5    | 2      | 427,62 | 40%    | 100,00% |
| 10   | 2      | 525,84 | 20%    | 122,97% |
| 10   | 4      | 265,85 | 40%    | 62,17%  |
| 50   | 20     | 52,98  | 40%    | 12,39%  |


If `runtime.GOMAXPROCS()` is set to a value higher than 1, the encoder will use multiple goroutines to perform the calculations in `Verify`, `Encode` and `Reconstruct`.

Example of performance scaling on Intel(R) Core(TM) i7-2600 CPU @ 3.40GHz - 4 physical cores, 8 logical cores. The example uses 10 blocks with 16MB data each and 4 parity blocks.

| Threads | MB/s   | Speed   |
|---------|--------|---------|
| 1       | 156,49 | 100,00% |
| 2       | 287,86 | 199,00% |
| 4       | 491,83 | 322,00% |
| 8       | 575,32 | 402,00% |

# License

This code, as the original [JavaReedSolomon](https://github.com/Backblaze/JavaReedSolomon) is published under an MIT license. See LICENSE file for more information.
