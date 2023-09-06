# BLAKE3

<p>
  <a href="https://pkg.go.dev/github.com/zeebo/blake3"><img src="https://img.shields.io/badge/doc-reference-007d9b?logo=go&style=flat-square" alt="go.dev" /></a>
  <a href="https://goreportcard.com/report/github.com/zeebo/blake3"><img src="https://goreportcard.com/badge/github.com/zeebo/blake3?style=flat-square" alt="Go Report Card" /></a>
  <a href="https://sourcegraph.com/github.com/zeebo/blake3?badge"><img src="https://sourcegraph.com/github.com/zeebo/blake3/-/badge.svg?style=flat-square" alt="SourceGraph" /></a>
</p>

Pure Go implementation of [BLAKE3](https://blake3.io) with AVX2 and SSE4.1 acceleration.

Special thanks to the excellent [avo](https://github.com/mmcloughlin/avo) making writing vectorized version much easier.

# Benchmarks

## Caveats

This library makes some different design decisions than the upstream Rust crate around internal buffering. Specifically, because it does not target the embedded system space, nor does it support multithreading, it elects to do its own internal buffering. This means that a user does not have to worry about providing large enough buffers to get the best possible performance, but it does worse on smaller input sizes. So some notes:

- The Rust benchmarks below are all single-threaded to match this Go implementation.
- I make no attempt to get precise measurements (cpu throttling, noisy environment, etc.) so please benchmark on your own systems.
- These benchmarks are run on an i7-6700K which does not support AVX-512, so Rust is limited to use AVX2 at sizes above 8 kib.
- I tried my best to make them benchmark the same thing, but who knows? :smile:

## Charts

In this case, both libraries are able to avoid a lot of data copying and will use vectorized instructions to hash as fast as possible, and perform similarly.

![Large Full Buffer](/assets/large-full-buffer.svg)

For incremental writes, you must provide the Rust version large enough buffers so that it can use vectorized instructions. This Go library performs consistently regardless of the size being sent into the update function.

![Incremental](/assets/incremental.svg)

The downside of internal buffering is most apparent with small sizes as most time is spent initializing the hasher state. In terms of hashing rate, the difference is 3-4x, but in an absolute sense it's ~100ns (see tables below). If you wish to hash a large number of very small strings and you care about those nanoseconds, be sure to use the Reset method to avoid re-initializing the state.

![Small Full Buffer](/assets/small-full-buffer.svg)

## Timing Tables

### Small

| Size   | Full Buffer |  Reset     | | Full Buffer Rate | Reset Rate   |
|--------|-------------|------------|-|------------------|--------------|
| 64 b   |  `205ns`    |  `86.5ns`  | |  `312MB/s`       |   `740MB/s`  |
| 256 b  |  `364ns`    |   `250ns`  | |  `703MB/s`       |  `1.03GB/s`  |
| 512 b  |  `575ns`    |   `468ns`  | |  `892MB/s`       |  `1.10GB/s`  |
| 768 b  |  `795ns`    |   `682ns`  | |  `967MB/s`       |  `1.13GB/s`  |

### Large

| Size     | Incremental | Full Buffer | Reset      | | Incremental Rate | Full Buffer Rate | Reset Rate   |
|----------|-------------|-------------|------------|-|------------------|------------------|--------------|
| 1 kib    |  `1.02µs`   |  `1.01µs`   |   `891ns`  | |  `1.00GB/s`      |  `1.01GB/s`      |  `1.15GB/s`  |
| 2 kib    |  `2.11µs`   |  `2.07µs`   |  `1.95µs`  | |   `968MB/s`      |   `990MB/s`      |  `1.05GB/s`  |
| 4 kib    |  `2.28µs`   |  `2.15µs`   |  `2.05µs`  | |  `1.80GB/s`      |  `1.90GB/s`      |  `2.00GB/s`  |
| 8 kib    |  `2.64µs`   |  `2.52µs`   |  `2.44µs`  | |  `3.11GB/s`      |  `3.25GB/s`      |  `3.36GB/s`  |
| 16 kib   |  `4.93µs`   |  `4.54µs`   |  `4.48µs`  | |  `3.33GB/s`      |  `3.61GB/s`      |  `3.66GB/s`  |
| 32 kib   |  `9.41µs`   |  `8.62µs`   |  `8.54µs`  | |  `3.48GB/s`      |  `3.80GB/s`      |  `3.84GB/s`  |
| 64 kib   |  `18.2µs`   |  `16.7µs`   |  `16.6µs`  | |  `3.59GB/s`      |  `3.91GB/s`      |  `3.94GB/s`  |
| 128 kib  |  `36.3µs`   |  `32.9µs`   |  `33.1µs`  | |  `3.61GB/s`      |  `3.99GB/s`      |  `3.96GB/s`  |
| 256 kib  |  `72.5µs`   |  `65.7µs`   |  `66.0µs`  | |  `3.62GB/s`      |  `3.99GB/s`      |  `3.97GB/s`  |
| 512 kib  |   `145µs`   |   `131µs`   |   `132µs`  | |  `3.60GB/s`      |  `4.00GB/s`      |  `3.97GB/s`  |
| 1024 kib |   `290µs`   |   `262µs`   |   `262µs`  | |  `3.62GB/s`      |  `4.00GB/s`      |  `4.00GB/s`  |

### No ASM

| Size     | Incremental | Full Buffer | Reset      | | Incremental Rate | Full Buffer Rate | Reset Rate  |
|----------|-------------|-------------|------------|-|------------------|------------------|-------------|
| 64 b     |   `253ns`   |   `254ns`   |   `134ns`  | |  `253MB/s`       |  `252MB/s`       |  `478MB/s`  |
| 256 b    |   `553ns`   |   `557ns`   |   `441ns`  | |  `463MB/s`       |  `459MB/s`       |  `580MB/s`  |
| 512 b    |   `948ns`   |   `953ns`   |   `841ns`  | |  `540MB/s`       |  `538MB/s`       |  `609MB/s`  |
| 768 b    |  `1.38µs`   |  `1.40µs`   |  `1.35µs`  | |  `558MB/s`       |  `547MB/s`       |  `570MB/s`  |
| 1 kib    |  `1.77µs`   |  `1.77µs`   |  `1.70µs`  | |  `577MB/s`       |  `580MB/s`       |  `602MB/s`  |
|          |             |             |            | |                  |                  |             |
| 1024 kib |   `880µs`   |   `883µs`   |   `878µs`  | |  `596MB/s`       |  `595MB/s`       |  `598MB/s`  |

The speed caps out at around 1 kib, so most rows have been elided from the presentation.
