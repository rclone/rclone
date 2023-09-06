# [![DRPC](logo.png)](https://storj.github.io/drpc/)

A drop-in, lightweight gRPC replacement.

[![Go Report Card](https://goreportcard.com/badge/storj.io/drpc)](https://goreportcard.com/report/storj.io/drpc)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](https://pkg.go.dev/storj.io/drpc)
![Beta](https://img.shields.io/badge/version-beta-green.svg)
[![Zulip Chat](https://img.shields.io/badge/zulip-join_chat-brightgreen.svg)](https://drpc.zulipchat.com)

## Links

 * [DRPC website](https://storj.github.io/drpc/)
 * [Examples](https://github.com/storj/drpc/tree/main/examples)
 * [Quickstart documentation](https://storj.github.io/drpc/docs.html)
 * [Launch blog post](https://www.storj.io/blog/introducing-drpc-our-replacement-for-grpc)

## Highlights

* Simple, at just a few thousand [lines of code](#lines-of-code).
* [Small dependencies](./blob/main/go.mod). Only 3 requirements in go.mod, and 9 lines of `go mod graph`!
* Compatible. Works for many gRPC use-cases as-is!
* [Fast](#benchmarks). DRPC has a lightning quick [wire format](https://github.com/storj/drpc/wiki/Docs:-Wire-protocol).
* [Extensible](#external-packages). DRPC is transport agnostic, supports middleware, and is designed around interfaces.
* Battle Tested. Already used in production for years across tens of thousands of servers.

## External Packages

 * [go.bryk.io/pkg/net/drpc](https://pkg.go.dev/go.bryk.io/pkg/net/drpc)
    - Simplified TLS setup (for client and server)
    - Server middleware, including basic components for logging, token-based auth, rate limit, panic recovery, etc
    - Client middleware, including basic components for logging, custom metadata, panic recovery, etc
    - Bi-directional streaming support over upgraded HTTP(S) connections using WebSockets
    - Concurrent RPCs via connection pool

* [go.arsenm.dev/drpc](https://pkg.go.dev/go.arsenm.dev/drpc)
    - Concurrent RPCs based on [yamux](https://pkg.go.dev/github.com/hashicorp/yamux)
    - Simple drop-in replacements for `drpcserver` and `drpcconn`

 * Open an issue or join the [Zulip chat](https://drpc.zulipchat.com) if you'd like to be featured here.

 ## Examples

  * [A basic drpc client and server](../../tree/main/examples/drpc)
  * [A basic drpc client and server that also serves a Twirp/grpc-web compatible http server on the same port](../../tree/main/examples/drpc)
  * [Serving gRPC and DRPC on the same port](../../tree/main/examples/grpc_and_drpc)

## Other Languages

DRPC can be made compatible with RPC clients generated from other languages. For example, [Twirp](https://github.com/twitchtv/twirp) clients and [grpc-web](https://github.com/grpc/grpc-web/) clients can be used against the [drpchttp](https://pkg.go.dev/storj.io/drpc/drpchttp) package.

Native implementations can have some advantages, and so some support for other languages are in progress, all in various states of completeness. Join the [Zulip chat](https://drpc.zulipchat.com) if you want more information or to help out with any!

| Language | Repository                          | Status     |
|----------|-------------------------------------|------------|
| C++      | https://github.com/storj/drpc-cpp   | Incomplete |
| Rust     | https://github.com/zeebo/drpc-rs    | Incomplete |
| Node     | https://github.com/mjpitz/drpc-node | Incomplete |

## Licensing

DRPC is licensed under the MIT/expat license. See the LICENSE file for more.

---

## Benchmarks

These microbenchmarks attempt to provide a comparison and come with some caveats. First, it does not send data over a network connection which is expected to be the bottleneck almost all of the time. Second, no attempt was made to do the benchmarks in a controlled environment (CPU scaling disabled, noiseless, etc.). Third, no tuning was done to ensure they're both performing optimally, so there is an inherent advantage for DRPC because the author is familiar with how it works.

<table>
    <tr>
        <td rowspan=2>Measure</td>
        <td rowspan=2>Benchmark</td><td rowspan=2></td>
        <td colspan=3>Small</td><td rowspan=2></td>
        <td colspan=3>Medium</td><td rowspan=2></td>
        <td colspan=3>Large</td>
    </tr>
    <tr>
        <td>gRPC</td><td>DRPC</td><td>delta</td>
        <td>gRPC</td><td>DRPC</td><td>delta</td>
        <td>gRPC</td><td>DRPC</td><td>delta</td>
    </tr>
    <tr><td colspan=14></td></tr>
    <tr>
        <td rowspan=4>time/op</td>
        <td>Unitary</td><td rowspan=4></td>
        <td>24.5µs</td><td>6.1µs</td><td>-74.87%</td><td rowspan=4></td>
        <td>32.4µs</td><td>8.8µs</td><td>-72.89%</td><td rowspan=4></td>
        <td>1.43ms</td><td>0.58ms</td><td>-59.47%</td>
    </tr>
    <tr>
        <td>Input Stream</td>
        <td>745ns</td><td>528ns</td><td>-29.13%</td>
        <td>2.63µs</td><td>1.46µs</td><td>-44.66%</td>
        <td>512µs</td><td>236µs</td><td>-53.89%</td>
    </tr>
    <tr>
        <td>Output Stream</td>
        <td>711ns</td><td>532ns</td><td>-25.11%</td>
        <td>2.63µs</td><td>1.51µs</td><td>-42.59%</td>
        <td>515µs</td><td>210µs</td><td>-59.26%</td>
    </tr>
    <tr>
        <td>Bidir Stream</td>
        <td>7.29µs</td><td>2.52µs</td><td>-65.46%</td>
        <td>12.3µs</td><td>3.9µs</td><td>-68.68%</td>
        <td>1.44ms</td><td>0.44ms</td><td>-69.05%</td>
    </tr>
    <tr><td colspan=14></td></tr>
    <tr>
        <td rowspan=4>speed</td>
        <td>Unitary</td><td rowspan=4></td>
        <td>80.0kB/s</td><td>325.0kB/s</td><td>+306.25%</td><td rowspan=4></td>
        <td>63.4MB/s</td><td>234.3MB/s</td><td>+269.56%</td><td rowspan=4></td>
        <td>734MB/s</td><td>1812MB/s</td><td>+146.99%</td>
    </tr>
    <tr>
        <td>Input Stream</td>
        <td>2.69MB/s</td><td>3.79MB/s</td><td>+41.00%</td>
        <td>780MB/s</td><td>1409MB/s</td><td>+80.67%</td>
        <td>2.05GB/s</td><td>4.45GB/s</td><td>+117.12%</td>
    </tr>
    <tr>
        <td>Output Stream</td>
        <td>2.81MB/s</td><td>3.76MB/s</td><td>+33.52%</td>
        <td>780MB/s</td><td>1360MB/s</td><td>+74.23%</td>
        <td>2.04GB/s</td><td>5.01GB/s</td><td>+145.53%</td>
    </tr>
    <tr>
        <td>Bidir Stream</td>
        <td>274kB/s</td><td>794kB/s</td><td>+189.95%</td>
        <td>166MB/s</td><td>533MB/s</td><td>+220.19%</td>
        <td>730MB/s</td><td>2360MB/s</td><td>+223.10%</td>
    </tr>
    <tr><td colspan=14></td></tr>
    <tr>
        <td rowspan=4>mem/op</td>
        <td>Unitary</td><td rowspan=4></td>
        <td>8.66kB</td><td>1.42kB</td><td>-83.62%</td><td rowspan=4></td>
        <td>22.2kB</td><td>7.8kB</td><td>-64.83%</td><td rowspan=4></td>
        <td>6.61MB</td><td>3.16MB</td><td>-52.21%</td>
    </tr>
    <tr>
        <td>Input Stream</td>
        <td>381B</td><td>80B</td><td>-79.01%</td>
        <td>7.08kB</td><td>2.13kB</td><td>-69.95%</td>
        <td>3.20MB</td><td>1.05MB</td><td>-67.17%</td>
    </tr>
    <tr>
        <td>Output Stream</td>
        <td>305B</td><td>80B</td><td>-73.80%</td>
        <td>7.00kB</td><td>2.13kB</td><td>-69.62%</td>
        <td>3.20MB</td><td>1.05MB</td><td>-67.19%</td>
    </tr>
    <tr>
        <td>Bidir Stream</td>
        <td>1.00kB</td><td>0.24kB</td><td>-75.90%</td>
        <td>14.5kB</td><td>4.3kB</td><td>-70.10%</td>
        <td>6.61MB</td><td>2.10MB</td><td>-68.20%</td>
    </tr>
    <tr><td colspan=14></td></tr>
    <tr>
        <td rowspan=4>allocs/op</td>
        <td>Unitary</td><td rowspan=4></td>
        <td>168</td><td>7</td><td>-95.83%</td><td rowspan=4></td>
        <td>170</td><td>9</td><td>-94.71%</td><td rowspan=4></td>
        <td>400</td><td>9</td><td>-97.75%</td>
    </tr>
    <tr>
        <td>Input Stream</td>
        <td>9</td><td>1</td><td>-88.89%</td>
        <td>10</td><td>2</td><td>-80.00%</td>
        <td>118</td><td>2</td><td>-98.31%</td>
    </tr>
    <tr>
        <td>Output Stream</td>
        <td>9</td><td>1</td><td>-88.89%</td>
        <td>10</td><td>2</td><td>-80.00%</td>
        <td>120</td><td>2</td><td>-98.33%</td>
    </tr>
    <tr>
        <td>Bidir Stream</td>
        <td>39</td><td>3</td><td>-92.31%</td>
        <td>42</td><td>5</td><td>-88.10%</td>
        <td>277</td><td>5</td><td>-98.20%</td>
    </tr>
</table>

## Lines of code

DRPC is proud to get as much done in as few lines of code as possible. It's the author's belief that this is only possible by having a clean, strong architecture and that it reduces the chances for bugs to exist (most studies show a linear corellation with number of bugs and lines of code). This table helps keep the library honest, and it would be nice if more libraries considered this.

| Package                              | Lines    |
| ---                                  | ---      |
| storj.io/drpc/drpchttp               | 478      |
| storj.io/drpc/drpcstream             | 435      |
| storj.io/drpc/cmd/protoc-gen-go-drpc | 424      |
| storj.io/drpc/drpcmanager            | 338      |
| storj.io/drpc/drpcwire               | 336      |
| storj.io/drpc/drpcmigrate            | 239      |
| storj.io/drpc/drpcpool               | 233      |
| storj.io/drpc/drpcsignal             | 133      |
| storj.io/drpc/drpcserver             | 133      |
| storj.io/drpc/drpcconn               | 116      |
| storj.io/drpc/drpcmetadata           | 115      |
| storj.io/drpc/drpcmux                | 95       |
| storj.io/drpc/drpccache              | 54       |
| storj.io/drpc                        | 47       |
| storj.io/drpc/drpctest               | 45       |
| storj.io/drpc/drpcerr                | 42       |
| storj.io/drpc/drpcctx                | 41       |
| storj.io/drpc/drpcdebug              | 22       |
| storj.io/drpc/drpcenc                | 15       |
| storj.io/drpc/internal/drpcopts      | 14       |
| **Total**                            | **3355** |
