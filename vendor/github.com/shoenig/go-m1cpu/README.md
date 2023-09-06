# m1cpu

[![Go Reference](https://pkg.go.dev/badge/github.com/shoenig/go-m1cpu.svg)](https://pkg.go.dev/github.com/shoenig/go-m1cpu)
[![MPL License](https://img.shields.io/github/license/shoenig/go-m1cpu?color=g&style=flat-square)](https://github.com/shoenig/go-m1cpu/blob/main/LICENSE)
[![Run CI Tests](https://github.com/shoenig/go-m1cpu/actions/workflows/ci.yaml/badge.svg)](https://github.com/shoenig/go-m1cpu/actions/workflows/ci.yaml)

The `go-m1cpu` module is a library for inspecting Apple Silicon CPUs in Go.

Use the `m1cpu` Go package for looking up the CPU frequency for Apple M1 and M2 CPUs.

# Install

```shell
go get github.com/shoenig/go-m1cpu@latest
```

# CGO

This package requires the use of [CGO](https://go.dev/blog/cgo).

Extracting the CPU properties is done via Apple's [IOKit](https://developer.apple.com/documentation/iokit?language=objc)
framework, which is accessible only through system C libraries.

# Example

Simple Go program to print Apple Silicon M1/M2 CPU speeds.

```go
package main

import (
	"fmt"

	"github.com/shoenig/go-m1cpu"
)

func main() {
	fmt.Println("Apple Silicon", m1cpu.IsAppleSilicon())

	fmt.Println("pCore GHz", m1cpu.PCoreGHz())
	fmt.Println("eCore GHz", m1cpu.ECoreGHz())

	fmt.Println("pCore Hz", m1cpu.PCoreHz())
	fmt.Println("eCore Hz", m1cpu.ECoreHz())
}
```

Using `go test` to print out available information.

```
➜ go test -v -run Show
=== RUN   Test_Show
    cpu_test.go:42: pCore Hz 3504000000
    cpu_test.go:43: eCore Hz 2424000000
    cpu_test.go:44: pCore GHz 3.504
    cpu_test.go:45: eCore GHz 2.424
    cpu_test.go:46: pCore count 8
    cpu_test.go:47: eCoreCount 4
    cpu_test.go:50: pCore Caches 196608 131072 16777216
    cpu_test.go:53: eCore Caches 131072 65536 4194304
--- PASS: Test_Show (0.00s)
```

# License

Open source under the [MPL](LICENSE)
