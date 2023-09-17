# goftp #

[![Units tests](https://github.com/jlaffaye/ftp/actions/workflows/unit_tests.yaml/badge.svg)](https://github.com/jlaffaye/ftp/actions/workflows/unit_tests.yaml)
[![Coverage Status](https://coveralls.io/repos/jlaffaye/ftp/badge.svg?branch=master&service=github)](https://coveralls.io/github/jlaffaye/ftp?branch=master)
[![golangci-lint](https://github.com/jlaffaye/ftp/actions/workflows/golangci-lint.yaml/badge.svg)](https://github.com/jlaffaye/ftp/actions/workflows/golangci-lint.yaml)
[![CodeQL](https://github.com/jlaffaye/ftp/actions/workflows/codeql-analysis.yml/badge.svg)](https://github.com/jlaffaye/ftp/actions/workflows/codeql-analysis.yml)
[![Go ReportCard](https://goreportcard.com/badge/jlaffaye/ftp)](http://goreportcard.com/report/jlaffaye/ftp)
[![Go Reference](https://pkg.go.dev/badge/github.com/jlaffaye/ftp.svg)](https://pkg.go.dev/github.com/jlaffaye/ftp)

A FTP client package for Go

## Install ##

```
go get -u github.com/jlaffaye/ftp
```

## Documentation ##

https://pkg.go.dev/github.com/jlaffaye/ftp

## Example ##

```go
c, err := ftp.Dial("ftp.example.org:21", ftp.DialWithTimeout(5*time.Second))
if err != nil {
    log.Fatal(err)
}

err = c.Login("anonymous", "anonymous")
if err != nil {
    log.Fatal(err)
}

// Do something with the FTP conn

if err := c.Quit(); err != nil {
    log.Fatal(err)
}
```

## Store a file example ##

```go
data := bytes.NewBufferString("Hello World")
err = c.Stor("test-file.txt", data)
if err != nil {
	panic(err)
}
```

## Read a file example ##

```go
r, err := c.Retr("test-file.txt")
if err != nil {
	panic(err)
}
defer r.Close()

buf, err := ioutil.ReadAll(r)
println(string(buf))
```
