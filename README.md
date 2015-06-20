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
The library is not heavily optimized at the moment. In the future it is very likely 

Performance depends mainly on the number of data and parity shards. In rough terms, doubling the number of data and parity shards will double the encoding time.

Here are the throughput numbers with some different selections of data and parity shards. For reference each shard is 1MB random data, and 2 CPU cores are used for encoding.

| Data | Parity | MB/s   | Speed   |
|------|--------|--------|---------|
| 5    | 2      | 209.87 | 100.00% |
| 10   | 2      | 151.96 | 72.41%  |
| 10   | 4      | 106.72 | 50.85%  |
| 50   | 20     | 13.59  | 6.48%   |


If `runtime.GOMAXPROCS()` is set to a value higher than 1, the encoder will use multiple goroutines to perform the calculations in `Verify`, `Encode` and `Reconstruct`.

Example of performance scaling on Intel(R) Core(TM) i7-2600 CPU @ 3.40GHz - 4 physical cores, 8 logical cores. The example uses 10 blocks with 16MB data each and 4 parity blocks.

| Threads | MB/s  | Speed |
|---------|-------|-------|
| 1       | 24.34 | 100%  |
| 2       | 48.47 | 199%  |
| 4       | 78.28 | 322%  |
| 8       | 97.88 | 402%  |

# License

This code, as the original [JavaReedSolomon](https://github.com/Backblaze/JavaReedSolomon) is published under an MIT license. See LICENSE file for more information.
