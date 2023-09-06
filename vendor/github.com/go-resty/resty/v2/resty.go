// Copyright (c) 2015-2021 Jeevanandam M (jeeva@myjeeva.com), All rights reserved.
// resty source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

// Package resty provides Simple HTTP and REST client library for Go.
package resty

import (
	"net"
	"net/http"
	"net/http/cookiejar"

	"golang.org/x/net/publicsuffix"
)

// Version # of resty
const Version = "2.7.0"

// New method creates a new Resty client.
func New() *Client {
	cookieJar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	return createClient(&http.Client{
		Jar: cookieJar,
	})
}

// NewWithClient method creates a new Resty client with given `http.Client`.
func NewWithClient(hc *http.Client) *Client {
	return createClient(hc)
}

// NewWithLocalAddr method creates a new Resty client with given Local Address
// to dial from.
func NewWithLocalAddr(localAddr net.Addr) *Client {
	cookieJar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	return createClient(&http.Client{
		Jar:       cookieJar,
		Transport: createTransport(localAddr),
	})
}
