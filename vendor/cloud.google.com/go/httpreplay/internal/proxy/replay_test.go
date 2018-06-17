// Copyright 2018 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build go1.8

package proxy

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"cloud.google.com/go/internal/testutil"
	"github.com/google/go-cmp/cmp"
)

func TestRequestBody(t *testing.T) {
	req1 := &http.Request{
		Header: http.Header{"Content-Type": {"multipart/mixed; boundary=foo"}},
		Body: ioutil.NopCloser(strings.NewReader(
			"--foo\r\nFoo: one\r\n\r\nA section\r\n" +
				"--foo\r\nFoo: two\r\n\r\nAnd another\r\n" +
				"--foo--\r\n")),
	}
	rb1, err := newRequestBodyFromHTTP(req1)
	if err != nil {
		t.Fatal(err)
	}
	want := &requestBody{
		mediaType: "multipart/mixed",
		parts: [][]byte{
			[]byte("A section"),
			[]byte("And another"),
		},
	}
	if diff := testutil.Diff(rb1, want, cmp.AllowUnexported(requestBody{})); diff != "" {
		t.Error(diff)
	}

	// Same contents, different boundary.
	req2 := &http.Request{
		Header: http.Header{"Content-Type": {"multipart/mixed; boundary=bar"}},
		Body: ioutil.NopCloser(strings.NewReader(
			"--bar\r\nFoo: one\r\n\r\nA section\r\n" +
				"--bar\r\nFoo: two\r\n\r\nAnd another\r\n" +
				"--bar--\r\n")),
	}
	rb2, err := newRequestBodyFromHTTP(req2)
	if err != nil {
		t.Fatal(err)
	}
	if diff := testutil.Diff(rb1, want, cmp.AllowUnexported(requestBody{})); diff != "" {
		t.Error(diff)
	}

	if !rb1.equal(rb2) {
		t.Error("equal returned false, want true")
	}
}

func TestHeadersMatch(t *testing.T) {
	for _, test := range []struct {
		h1, h2 http.Header
		want   bool
	}{
		{
			http.Header{"A": {"x"}, "B": {"y", "z"}},
			http.Header{"A": {"x"}, "B": {"y", "z"}},
			true,
		},
		{
			http.Header{"A": {"x"}, "B": {"y", "z"}},
			http.Header{"A": {"x"}, "B": {"w"}},
			false,
		},
		{
			http.Header{"A": {"x"}, "B": {"y", "z"}, "I": {"foo"}},
			http.Header{"A": {"x"}, "B": {"y", "z"}, "I": {"bar"}},
			true,
		},
		{
			http.Header{"A": {"x"}, "B": {"y", "z"}},
			http.Header{"A": {"x"}, "B": {"y", "z"}, "I": {"bar"}},
			true,
		},
		{
			http.Header{"A": {"x"}, "B": {"y", "z"}, "I": {"foo"}},
			http.Header{"A": {"x"}, "I": {"bar"}},
			false,
		},
		{
			http.Header{"A": {"x"}, "I": {"foo"}},
			http.Header{"A": {"x"}, "B": {"y", "z"}, "I": {"bar"}},
			false,
		},
	} {
		got := headersMatch(test.h1, test.h2, map[string]bool{"I": true})
		if got != test.want {
			t.Errorf("%v, %v: got %t, want %t", test.h1, test.h2, got, test.want)
		}
	}
}
