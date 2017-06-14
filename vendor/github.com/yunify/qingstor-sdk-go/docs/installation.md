# Installation Guide

## Requirement

This SDK requires Go 1.6 and higher vendor feature, the dependencies this project uses are included in the `vendor` directory. We use [glide](https://glide.sh) to manage project dependences.

___Notice:___ _You can also use Go 1.5 with the `GO15VENDOREXPERIMENT=1`._

## Install from source code

Use `go get` to download this SDK from GitHub:

``` bash
$ go get -u github.com/yunify/qingstor-sdk-go
```

You can also download a specified version of zipped source code in the repository [releases page](https://github.com/yunify/qingstor-sdk-go/releases). The zipped source code only contains golang source code without unit test files.

___Examples:___

- *[qingstor-sdk-go-source-v0.7.1.zip](https://github.com/yunify/qingstor-sdk-go/releases/download/v0.7.1/qingstor-sdk-go-source-v0.7.1.zip)*
- *[qingstor-sdk-go-source-with-vendor-v0.7.1.zip](https://github.com/yunify/qingstor-sdk-go/releases/download/v0.7.1/qingstor-sdk-go-source-with-vendor-v0.7.1.zip)*

## Install from binary release (deprecated)

After Go 1.7, there's a new feature called Binary-Only Package. It allows distributing packages in binary form without including the source code used for compiling the package. For more information about Binary-Only Package, please read [_GoLang Package Build_](https://golang.org/pkg/go/build/) to know how to use that.

We provide Linux, macOS and Windows binary packages along with a header files. A header file only contains three lines of content, "//go:binary-only-package" is the first line, the second line is blank, and the second is the package name. There's one header file named "binary.go" for each golang package.

You can download a specified version of zipped binary release in the repository [releases page](https://github.com/yunify/qingstor-sdk-go/releases).

___Notice:___ _We didn't provide 386 version binary packages, since there's almost no one using a 386 machine._

___Examples:___

- *[qingstor-sdk-go-header-v0.7.1-go-1.7.zip](https://github.com/yunify/qingstor-sdk-go/releases/download/v0.7.1/qingstor-sdk-go-header-v0.7.1-go-1.7.zip)*
- *[qingstor-sdk-go-binary-v0.7.1-linux_amd64-go-1.7.zip](https://github.com/yunify/qingstor-sdk-go/releases/download/v0.7.1/qingstor-sdk-go-binary-v0.7.1-linux_amd64-go-1.7.zip)*
- *[qingstor-sdk-go-binary-v0.7.1-darwin_amd64-go-1.7.zip](https://github.com/yunify/qingstor-sdk-go/releases/download/v0.7.1/qingstor-sdk-go-binary-v0.7.1-darwin_amd64-go-1.7.zip)*
- *[qingstor-sdk-go-binary-v0.7.1-windows_amd64-go-1.7.zip](https://github.com/yunify/qingstor-sdk-go/releases/download/v0.7.1/qingstor-sdk-go-binary-v0.7.1-windows_amd64-go-1.7.zip)*
