# go-textwrapper

[![GoDoc](https://godoc.org/github.com/emersion/go-textwrapper?status.svg)](https://godoc.org/github.com/emersion/go-textwrapper)
[![Build Status](https://travis-ci.org/emersion/go-textwrapper.svg?branch=master)](https://travis-ci.org/emersion/go-textwrapper)

A writer that wraps long text lines to a specified length

## Usage

```go
import (
	"os"

	"github.com/emersion/go-textwrapper"
)

func main() {
	w := textwrapper.New(os.Stdout, "/", 5)

	w.Write([]byte("helloworldhelloworldhelloworld"))
	// Output: hello/world/hello/world/hello/world
}
```

## License

MIT
