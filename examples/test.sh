#!/bin/bash
mkdir test
dd if=/dev/random of=test/1 bs=16384 count=16384
go run ./simple-encoder.go ./test/1
cp test/1.0 test/1.0.orig # keep a good copy
echo
echo "testing wrong length"
echo " " >> test/1.0
go run ./simple-decoder.go ./test/1 | head -4
echo

cp -f test/1.0.orig test/1.0
echo "testing broken magic file"
printf '\x31\xc0\xc3' | dd of=test/1.0 bs=1 seek=0 count=3 conv=notrunc 2> /dev/null
go run ./simple-decoder.go ./test/1 | head -5
echo

cp -f test/1.0.orig test/1.0
echo "testing broken sha256 checksum"
printf '\x31\xc0\xc3' | dd of=test/1.0 bs=1 seek=100 count=3 conv=notrunc 2> /dev/null
go run ./simple-decoder.go ./test/1 | head -6
echo

cp -f test/1.5 test/1.0
echo "testing broken shards in wrong order"
go run ./simple-decoder.go ./test/1 | head -7


