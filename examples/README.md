# Examples

This folder contains usage examples of the Reed-Solomon encoder.

# Simple Encoder/Decoder

Shows basic use of the encoder, and will encode a single file into a number of
data and parity shards. This is meant as an example and is not meant for production use
since there is a number of shortcomings noted below.

To build an executable use:

```bash 
go build simple-decoder.go
go build simple-encoder.go
```

# Streaming API examples

There are streaming examples of the same functionality, which streams data instead of keeping it in memory.

To build the executables use:

```bash 
go build stream-decoder.go
go build stream-encoder.go
```

# Example usage

On Windows, the following command will generate six files `README.md.0` to `README.md.5`

```bash
.\simple-encoder.exe .\README.md
```

Rename `README.md` to `README.md.org`, and delete two of the six generated files. The following
command will reconstruct `README.md` from the four generated files.

```bash
.\simple-decoder.exe .\README.md
```

Appreciate that the reconstructed file mau have nul padding as explained in the following shortcomings.


## Shortcomings
* If the file size of the input isn't dividable by the number of data shards
  the output will contain extra zeroes
* If the shard numbers isn't the same for the decoder as in the
  encoder, invalid output will be generated.
* If values have changed in a shard, it cannot be reconstructed.
* If two shards have been swapped, reconstruction will always fail.
  You need to supply the shards in the same order as they were given to you.

The solution for this is to save a metadata file containing:

* File size.
* The number of data/parity shards.
* HASH of each shard.
* Order of the shards.

If you save these properties, you should abe able to detect file corruption in a shard and be able to reconstruct your data if you have the needed number of shards left.