<h1 align="center">
  mimetype
</h1>

<h4 align="center">
  A package for detecting MIME types and extensions based on magic numbers
</h4>
<h6 align="center">
  No bindings, all written in pure go
</h6>

<p align="center">
  <a href="https://travis-ci.org/gabriel-vasile/mimetype">
    <img alt="Build Status" src="https://travis-ci.org/gabriel-vasile/mimetype.svg?branch=master">
  </a>
  <a href="https://godoc.org/github.com/gabriel-vasile/mimetype">
    <img alt="Documentation" src="https://godoc.org/github.com/gabriel-vasile/mimetype?status.svg">
  </a>
  <a href="https://goreportcard.com/report/github.com/gabriel-vasile/mimetype">
    <img alt="Go report card" src="https://goreportcard.com/badge/github.com/gabriel-vasile/mimetype">
  </a>
  <a href="https://coveralls.io/github/gabriel-vasile/mimetype?branch=master">
    <img alt="Go report card" src="https://coveralls.io/repos/github/gabriel-vasile/mimetype/badge.svg?branch=master">
  </a>
  <a href="LICENSE">
    <img alt="License" src="https://img.shields.io/badge/License-MIT-green.svg">
  </a>
</p>

## Install
```bash
go get github.com/gabriel-vasile/mimetype
```

## Use
The library exposes three functions you can use in order to detect a file type.
See [Godoc](https://godoc.org/github.com/gabriel-vasile/mimetype) for full reference.
```go
func Detect(in []byte) (mime, extension string) {...}
func DetectReader(r io.Reader) (mime, extension string, err error) {...}
func DetectFile(file string) (mime, extension string, err error) {...}
```
When detecting from a `ReadSeeker` interface, such as `os.File`, make sure
to reset the offset of the reader to the beginning if needed:
```go
_, err = file.Seek(0, io.SeekStart)
```

## Supported MIME types
See [supported mimes](supported_mimes.md) for the list of detected MIME types.
If support is needed for a specific file format, please open an [issue](https://github.com/gabriel-vasile/mimetype/issues/new/choose).

## Structure
**mimetype** uses an hierarchical structure to keep the matching functions.
This reduces the number of calls needed for detecting the file type. The reason
behind this choice is that there are file formats used as containers for other
file formats. For example, Microsoft office files are just zip archives,
containing specific metadata files.
<div align="center">
  <img alt="structure" src="mimetype.gif" width="88%">
</div>

## Contributing
See [CONTRIBUTING.md](CONTRIBUTING.md).
