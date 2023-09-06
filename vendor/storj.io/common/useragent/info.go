// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package useragent implements parts of
// https://tools.ietf.org/html/rfc7231#section-5.5 and
// https://tools.ietf.org/html/rfc2616#section-14.43
package useragent

import (
	"strings"

	"github.com/zeebo/errs"
)

// Error is the default error class.
var Error = errs.Class("useragent")

// Info contains parsed user agent.
type Info struct {
	Product Product

	Full string
}

// Product is an user agent product.
type Product struct {
	Name    string
	Version string
}

// Parse parses user agent string to information.
func Parse(s string) (Info, error) {
	if s == "" {
		return Info{}, nil
	}
	s = strings.TrimSpace(s)

	info := Info{}
	info.Full = s

	parts := strings.SplitN(s, " ", 2)
	productTokens := strings.SplitN(parts[0], "/", 2)

	if len(productTokens) >= 1 {
		info.Product.Name = productTokens[0]
	}
	if len(productTokens) >= 2 {
		info.Product.Version = productTokens[1]
	}

	return info, nil
}
