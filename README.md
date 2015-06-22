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

This section assumes you know the basics of Reed-Solomon encoding. A good start is this [Backblaze blog post](https://www.backblaze.com/blog/reed-solomon/).

This package only performs the calculation of the parity sets. The usage is therefore really simple.

First of all, you need to choose your distribution of data and parity shards. A 'good' distribution is very subjective, and will depend a lot on your usage scenario. A good starting point is above 5 and below 50 data shards, and the number of parity shards to be 2 or above, and below the number of data shards.

To create an encoder with 10 data shards and 3 parity shards:
```Go
    encoder, err := reedsolomon.New(10, 3)
```
This encoder will work for all parity sets with this distribution of data and parity shards. The error will only be set if you specify 0 or negative values in any of the parameters.

The you send and receive data  is a simple slice of byte slices; `[][]byte`. In the example above, the top slice must have a length of 13.
```Go
    input := make([][]byte, 13)
```
You should then fill the 10 first slices with *equally sized* data.

```Go
    // Create all shards, size them at 50000 each
    for i := range input {
      input[i] := make([]byte, 50000)
    }
    
    
  // Fill some data into the data shards
    for i, in := range input[:10] {
      for j:= range in {
         data[j] = byte((i+j)&0xff)
      }
    }
```

# Streaming/Merging


# Performance
Performance depends mainly on the number of parity shards. In rough terms, doubling the number of parity shards will double the encoding time.

Here are the throughput numbers with some different selections of data and parity shards. For reference each shard is 1MB random data, and 2 CPU cores are used for encoding.

| Data | Parity | Parity | MB/s   | SSE3 MB/s  | SSE3 Speed | Rel. Speed |
|------|--------|--------|--------|------------|------------|------------|
| 5    | 2      | 40%    | 576,11 | 2599,2     | 451%       | 100,00%    |
| 10   | 2      | 20%    | 587,73 | 3100,28    | 528%       | 102,02%    |
| 10   | 4      | 40%    | 298,38 | 2470,97    | 828%       | 51,79%     |
| 50   | 20     | 40%    | 59,81  | 713,28     | 1193%      | 10,38%     |

If `runtime.GOMAXPROCS()` is set to a value higher than 1, the encoder will use multiple goroutines to perform the calculations in `Verify`, `Encode` and `Reconstruct`.

Example of performance scaling on Intel(R) Core(TM) i7-2600 CPU @ 3.40GHz - 4 physical cores, 8 logical cores. The example uses 10 blocks with 16MB data each and 4 parity blocks.

| Threads | MB/s    | Speed |
|---------|---------|-------|
| 1       | 1355,11 | 100%  |
| 2       | 2339,78 | 172%  |
| 4       | 3179,33 | 235%  |
| 8       | 4346,18 | 321%  |

# License

This code, as the original [JavaReedSolomon](https://github.com/Backblaze/JavaReedSolomon) is published under an MIT license. See LICENSE file for more information.
