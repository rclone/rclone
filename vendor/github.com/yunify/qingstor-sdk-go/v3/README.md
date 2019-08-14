# QingStor SDK for Go

[![Build Status](https://travis-ci.org/yunify/qingstor-sdk-go.svg?branch=master)](https://travis-ci.org/yunify/qingstor-sdk-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/yunify/qingstor-sdk-go)](https://goreportcard.com/report/github.com/yunify/qingstor-sdk-go)
[![API Reference](http://img.shields.io/badge/api-reference-green.svg)](http://docs.qingcloud.com/qingstor/)
[![License](http://img.shields.io/badge/license-apache%20v2-blue.svg)](https://github.com/yunify/qingstor-sdk-go/blob/master/LICENSE)

The official QingStor SDK for the Go programming language.

## Getting Started

### Installation

Refer to the [Installation Guide](docs/installation.md), and have this SDK installed.

### Preparation

Before your start, please go to [QingCloud Console](https://console.qingcloud.com/access_keys/) to create a pair of QingCloud API AccessKey.

___API AccessKey Example:___

``` yaml
access_key_id: 'ACCESS_KEY_ID_EXAMPLE'
secret_access_key: 'SECRET_ACCESS_KEY_EXAMPLE'
```

### Usage

Now you are ready to code. You can read the detailed guides in the list below to have a clear understanding or just take the quick start code example.

Checkout our [releases](https://github.com/yunify/qingstor-sdk-go/releases) and [change log](https://github.com/yunify/qingstor-sdk-go/blob/master/CHANGELOG.md) for information about the latest features, bug fixes and new ideas.

- [Configuration Guide](docs/configuration.md)
- [QingStor Service Usage Guide](docs/qingstor_service_usage.md)

___Quick Start Code Example:___

``` go
package main

import (
	"fmt"

	"github.com/yunify/qingstor-sdk-go/v3/config"
	qs "github.com/yunify/qingstor-sdk-go/v3/service"
)

func main() {
	conf, _ := config.New("ACCESS_KEY_ID", "SECRET_ACCESS_KEY")

	// Initialize service object for QingStor.
	qsService, _ := qs.Init(conf)

	// List all buckets.
	qsOutput, _ := qsService.ListBuckets(&qs.ListBucketsInput{})

	// Print HTTP status code.
	fmt.Println(qs.IntValue(qsOutput.StatusCode))

	// Print the count of buckets.
	fmt.Println(qs.IntValue(qsOutput.Count))

	// Print the first bucket name.
	fmt.Println(qs.StringValue(qsOutput.Buckets[0].Name))
}
```

## Reference Documentations

- [QingStor Documentation](https://docs.qingcloud.com/qingstor/index.html)
- [QingStor Guide](https://docs.qingcloud.com/qingstor/guide/index.html)
- [QingStor APIs](https://docs.qingcloud.com/qingstor/api/index.html)

## Contributing

1. Fork it ( https://github.com/yunify/qingstor-sdk-go/fork )
2. Create your feature branch (`git checkout -b new-feature`)
3. Commit your changes (`git commit -asm 'Add some feature'`)
4. Push to the branch (`git push origin new-feature`)
5. Create a new Pull Request

## LICENSE

The Apache License (Version 2.0, January 2004).
