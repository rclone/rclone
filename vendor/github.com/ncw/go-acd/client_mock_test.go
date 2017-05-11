// Copyright (c) 2015 Serge Gebhardt. All rights reserved.
//
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

package acd

import (
	"bytes"
	"io/ioutil"
	"net/http"
)

// MockResponse is a static HTTP response.
type MockResponse struct {
	Code int
	Body []byte
}

// NewMockResponseOkString creates a new MockResponse with Code 200 (OK)
// and Body built from string argument
func NewMockResponseOkString(response string) *MockResponse {
	return &MockResponse{
		Code: 200,
		Body: []byte(response),
	}
}

// mockTransport is a mocked Transport that always returns the same MockResponse.
type mockTransport struct {
	resp MockResponse
}

// Satisfies the RoundTripper interface.
func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := http.Response{
		StatusCode: t.resp.Code,
		Proto:      "HTTP/1.0",
		ProtoMajor: 1,
		ProtoMinor: 0,
	}

	if len(t.resp.Body) > 0 {
		buf := bytes.NewBuffer(t.resp.Body)
		r.Body = ioutil.NopCloser(buf)
	}

	return &r, nil
}

// MockClient is a mocked Client that is used for tests.
func NewMockClient(response MockResponse) *Client {
	t := &mockTransport{resp: response}
	c := &http.Client{Transport: t}
	return NewClient(c)
}
