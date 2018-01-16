tree [![Build status][travis-image]][travis-url] [![License][license-image]][license-url]
---
> An implementation of the [`tree`](http://mama.indstate.edu/users/ice/tree/) command written in Go, that can be used  programmatically.

<img src="https://raw.githubusercontent.com/a8m/tree/assets/assets/tree.png" height="300" alt="tree command">

#### Installation:
```sh
$ go get github.com/a8m/tree/cmd/tree
```

#### How to use `tree` programmatically ?
You can take a look on [`cmd/tree`](https://github.com/a8m/tree/blob/master/cmd/tree/tree.go), and [s3tree](http://github.com/a8m/s3tree) or see the example below.
```go
import (
    "github.com/a8m/tree"
)

func main() {
    opts := &tree.Options{
        // Fs, and OutFile are required fields.
        // fs should implement the tree file-system interface(see: tree.Fs),
        // and OutFile should be type io.Writer
        Fs: fs,
        OutFile: os.Stdout,
        // ...
    }
    inf.New("root-dir")
    // Visit all nodes recursively
    inf.Visit(opts)
    // Print nodes 
    inf.Print(opts)
}
```

### License
MIT


[travis-image]: https://img.shields.io/travis/a8m/tree.svg?style=flat-square
[travis-url]: https://travis-ci.org/a8m/tree
[license-image]: http://img.shields.io/npm/l/deep-keys.svg?style=flat-square
[license-url]: LICENSE
