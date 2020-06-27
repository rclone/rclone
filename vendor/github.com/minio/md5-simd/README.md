
# md5-simd

This is a SIMD accelerated MD5 package, allowing up to either 8 (AVX2) or 16 (AVX512) independent MD5 sums to be calculated on a single CPU core.

It was originally based on the [md5vec](https://github.com/igneous-systems/md5vec) repository by Igneous Systems, but has been made more flexible by amongst others supporting different message sizes per lane and adding AVX512.

`md5-simd` integrates a similar mechanism as described in [minio/sha256-simd](https://github.com/minio/sha256-simd#support-for-avx512) for making it easy for clients to take advantages of the parallel nature of the MD5 calculation. This will result in reduced overall CPU load. 

It is important to understand that `md5-simd` **does not speed up** a single threaded MD5 hash sum. 
Rather it allows multiple __independent__  MD5 sums to be computed in parallel on the same CPU core, 
thereby making more efficient usage of the computing resources.

## Usage

[![Documentation](https://godoc.org/github.com/minio/md5-simd?status.svg)](https://pkg.go.dev/github.com/minio/md5-simd?tab=doc)


In order to use `md5-simd`, you must first create an `Server` which can be 
used to instantiate one or more objects for MD5 hashing. 

These objects conform to the regular [`hash.Hash`](https://pkg.go.dev/hash?tab=doc#Hash) interface 
and as such the normal Write/Reset/Sum functionality works as expected. 

As an example: 
```
    // Create server
    server := md5simd.NewServer()
    defer server.Close()

    // Create hashing object (conforming to hash.Hash)
    md5Hash := server.NewHash()
    defer md5Hash.Close()

    // Write one (or more) blocks
    md5Hash.Write(block)
    
    // Return digest
    digest := md5Hash.Sum([]byte{})
```

To keep performance both a [Server](https://pkg.go.dev/github.com/minio/md5-simd?tab=doc#Server) 
and individual [Hasher](https://pkg.go.dev/github.com/minio/md5-simd?tab=doc#Hasher) should 
be closed using the `Close()` function when no longer needed.

A Hasher can efficiently be re-used by using [`Reset()`](https://pkg.go.dev/hash?tab=doc#Hash) functionality.

In case your system does not support the instructions required it will fall back to using `crypto/md5` for hashing.

## Limitations

As explained above `md5-simd` does not speed up an individual MD5 hash sum computation,
unless some hierarchical tree construct is used but this will result in different outcomes.
Running a single hash on a server results in approximately half the throughput.

Instead, it allows running multiple MD5 calculations in parallel on a single CPU core. 
This can be beneficial in e.g. multi-threaded server applications where many go-routines 
are dealing with many requests and multiple MD5 calculations can be packed/scheduled for parallel execution on a single core.

This will result in a lower overall CPU usage as compared to using the standard `crypto/md5`
functionality where each MD5 hash computation will consume a single thread (core).

It is best to test and measure the overall CPU usage in a representative usage scenario in your application
to get an overall understanding of the benefits of `md5-simd` as compared to `crypto/md5`, ideally under heavy CPU load.

Also note that `md5-simd` is best meant to work with large objects, 
so if your application only hashes small objects of a few kilobytes 
you may be better of by using `crypto/md5`.

## Performance

For the best performance writes should be a multiple of 64 bytes, ideally a multiple of 32KB.
To help with that a [`buffered := bufio.NewWriterSize(hasher, 32<<10)`](https://golang.org/pkg/bufio/#NewWriterSize) 
can be inserted if you are unsure of the sizes of the writes. 
Remember to [flush](https://golang.org/pkg/bufio/#Writer.Flush) `buffered` before reading the hash. 

A single 'server' can process 16 streams concurrently with 1 core (AVX-512) or 2 cores (AVX2). 
In situations where it is likely that more than 16 streams are fully loaded it may be beneficial
to use multiple servers.

The following chart compares the multi-core performance between `crypto/md5` vs the AVX2 vs the AVX512 code:

![md5-performance-overview](chart/Multi-core-MD5-Aggregated-Hashing-Performance.png)

Compared to `crypto/md5`, the AVX2 version is up to 4x faster:

```
$ benchcmp crypto-md5.txt avx2.txt 
benchmark                     old MB/s     new MB/s     speedup
BenchmarkParallel/32KB-4      2229.22      7370.50      3.31x
BenchmarkParallel/64KB-4      2233.61      8248.46      3.69x
BenchmarkParallel/128KB-4     2235.43      8660.74      3.87x
BenchmarkParallel/256KB-4     2236.39      8863.87      3.96x
BenchmarkParallel/512KB-4     2238.05      8985.39      4.01x
BenchmarkParallel/1MB-4       2233.56      9042.62      4.05x
BenchmarkParallel/2MB-4       2224.11      9014.46      4.05x
BenchmarkParallel/4MB-4       2199.78      8993.61      4.09x
BenchmarkParallel/8MB-4       2182.48      8748.22      4.01x
```

Compared to `crypto/md5`, the AVX512 is up to 8x faster (for larger block sizes):

```
$ benchcmp crypto-md5.txt avx512.txt
benchmark                     old MB/s     new MB/s     speedup
BenchmarkParallel/32KB-4      2229.22      11605.78     5.21x
BenchmarkParallel/64KB-4      2233.61      14329.65     6.42x
BenchmarkParallel/128KB-4     2235.43      16166.39     7.23x
BenchmarkParallel/256KB-4     2236.39      15570.09     6.96x
BenchmarkParallel/512KB-4     2238.05      16705.83     7.46x
BenchmarkParallel/1MB-4       2233.56      16941.95     7.59x
BenchmarkParallel/2MB-4       2224.11      17136.01     7.70x
BenchmarkParallel/4MB-4       2199.78      17218.61     7.83x
BenchmarkParallel/8MB-4       2182.48      17252.88     7.91x
```

These measurements were performed on AWS EC2 instance of type `c5.xlarge` equipped with a Xeon Platinum 8124M CPU at 3.0 GHz.


## Operation

To make operation as easy as possible there is a “Server” coordinating everything. The server keeps track of individual hash states and updates them as new data comes in. This can be visualized as follows:

![server-architecture](chart/server-architecture.png)

The data is sent to the server from each hash input in blocks of up to 32KB per round. In our testing we found this to be the block size that yielded the best results.

Whenever there is data available the server will collect data for up to 16 hashes and process all 16 lanes in parallel. This means that if 16 hashes have data available all the lanes will be filled. However since that may not be the case, the server will fill less lanes and do a round anyway. Lanes can also be partially filled if less than 32KB of data is written.

![server-lanes-example](chart/server-lanes-example.png)

In this example 4 lanes are fully filled and 2 lanes are partially filled. In this case the black areas will simply be masked out from the results and ignored. This is also why calculating a single hash on a server will not result in any speedup and hash writes should be a multiple of 32KB for the best performance.

For AVX512 all 16 calculations will be done on a single core, on AVX2 on 2 cores if there is data for more than 8 lanes.
So for optimal usage there should be data available for all 16 hashes. It may be perfectly reasonable to use more than 16 concurrent hashes.


## Design & Tech

md5-simd has both an AVX2 (8-lane parallel), and an AVX512 (16-lane parallel version) algorithm to accelerate the computation with the following function definitions:
```
//go:noescape
func block8(state *uint32, base uintptr, bufs *int32, cache *byte, n int)

//go:noescape
func block16(state *uint32, ptrs *int64, mask uint64, n int)
```

The AVX2 version is based on the [md5vec](https://github.com/igneous-systems/md5vec) repository and is essentially unchanged except for minor (cosmetic) changes.

The AVX512 version is derived from the AVX2 version but adds some further optimizations and simplifications.

### Caching in upper ZMM registers

The AVX2 version passes in a `cache8` block of memory (about 0.5 KB) for temporary storage of intermediate results during `ROUND1` which are subsequently used during `ROUND2` through to `ROUND4`.

Since AVX512 has double the amount of registers (32 ZMM registers as compared to 16 YMM registers), it is possible to use the upper 16 ZMM registers for keeping the intermediate states on the CPU. As such, there is no need to pass in a corresponding `cache16` into the AVX512 block function.

### Direct loading using 64-bit pointers

The AVX2 uses the `VPGATHERDD` instruction (for YMM) to do a parallel load of 8 lanes using (8 independent) 32-bit offets. Since there is no control over how the 8 slices that are passed into the (Golang) `blockMd5` function are laid out into memory, it is not possible to derive a "base" address and corresponding offsets (all within 32-bits) for all 8 slices.

As such the AVX2 version uses an interim buffer to collect the byte slices to be hashed from all 8 inut slices and passed this buffer along with (fixed) 32-bit offsets into the assembly code.

For the AVX512 version this interim buffer is not needed since the AVX512 code uses a pair of `VPGATHERQD` instructions to directly dereference 64-bit pointers (from a base register address that is initialized to zero).

Note that two load (gather) instructions are needed because the AVX512 version processes 16-lanes in parallel, requiring 16 times 64-bit = 1024 bits in total to be loaded. A simple `VALIGND` and `VPORD` are subsequently used to merge the lower and upper halves together into a single ZMM register (that contains 16 lanes of 32-bit DWORDS).

### Masking support

Due to the fact that pointers are passed directly from the Golang slices, we need to protect against NULL pointers. 
For this a 16-bit mask is passed in the AVX512 assembly code which is used during the `VPGATHERQD` instructions to mask out lanes that could otherwise result in segment violations.

### Minor optimizations

The `roll` macro (three instructions on AVX2) is no longer needed for AVX512 and is replaced by a single `VPROLD` instruction.

Also several logical operations from the various ROUNDS of the AVX2 version could be combined into a single instruction using ternary logic (with the `VPTERMLOGD` instruction), resulting in a further simplification and speed-up.

## Low level block function performance

The benchmark below shows the (single thread) maximum performance of the `block()` function for AVX2 (having 8 lanes) and AVX512 (having 16 lanes). Also the baseline single-core performance from the standard `crypto/md5` package is shown for comparison.

```
BenchmarkCryptoMd5-4                     687.66 MB/s           0 B/op          0 allocs/op
BenchmarkBlock8-4                       4144.80 MB/s           0 B/op          0 allocs/op
BenchmarkBlock16-4                      8228.88 MB/s           0 B/op          0 allocs/op
```

## License

`md5-simd` is released under the Apache License v2.0. You can find the complete text in the file LICENSE.

## Contributing

Contributions are welcome, please send PRs for any enhancements.