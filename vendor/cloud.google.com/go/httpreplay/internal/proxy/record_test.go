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
	"net/http"
	"testing"

	"cloud.google.com/go/internal/testutil"
	"github.com/google/martian"
)

func TestWithRedactedHeaders(t *testing.T) {
	clone := func(h http.Header) http.Header {
		h2 := http.Header{}
		for k, v := range h {
			h2[k] = v
		}
		return h2
	}

	orig := http.Header{
		"Content-Type":                      {"text/plain"},
		"Authorization":                     {"oauth2-token"},
		"X-Goog-Encryption-Key":             {"a-secret-key"},
		"X-Goog-Copy-Source-Encryption-Key": {"another-secret-key"},
	}
	req := &http.Request{Header: clone(orig)}
	var got http.Header
	mod := martian.RequestModifierFunc(func(req *http.Request) error {
		got = clone(req.Header)
		return nil
	})
	if err := withRedactedHeaders(req, mod); err != nil {
		t.Fatal(err)
	}
	// Logged headers should be redacted.
	want := http.Header{
		"Content-Type":                      {"text/plain"},
		"Authorization":                     {"REDACTED"},
		"X-Goog-Encryption-Key":             {"REDACTED"},
		"X-Goog-Copy-Source-Encryption-Key": {"REDACTED"},
	}
	if !testutil.Equal(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
	// The request's headers should be the same.
	if got, want := req.Header, orig; !testutil.Equal(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}
