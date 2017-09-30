// Copyright 2014 Google Inc. All Rights Reserved.
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

package storage

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/internal/testutil"

	"golang.org/x/net/context"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	raw "google.golang.org/api/storage/v1"
)

func TestSignedURL(t *testing.T) {
	t.Parallel()
	expires, _ := time.Parse(time.RFC3339, "2002-10-02T10:00:00-05:00")
	url, err := SignedURL("bucket-name", "object-name", &SignedURLOptions{
		GoogleAccessID: "xxx@clientid",
		PrivateKey:     dummyKey("rsa"),
		Method:         "GET",
		MD5:            "ICy5YqxZB1uWSwcVLSNLcA==",
		Expires:        expires,
		ContentType:    "application/json",
		Headers:        []string{"x-header1", "x-header2"},
	})
	if err != nil {
		t.Error(err)
	}
	want := "https://storage.googleapis.com/bucket-name/object-name?" +
		"Expires=1033570800&GoogleAccessId=xxx%40clientid&Signature=" +
		"ZMw18bZVhySNYAMEX87RMyuZCUMtGLVi%2B2zU2ByiQ0Rxgij%2BhFZ5LsT" +
		"5ZPIH5h3QXB%2BiSb1URJnZo3aF0exVP%2FYR1hpg2e65w9HHt7yYjIqcg" +
		"%2FfAOIyxriFtgRYk3oAv%2FFLF62fI8iF%2BCp0fWSm%2FHggz22blVnQz" +
		"EtSP%2BuRhFle4172L%2B710sfMDtyQLKTz6W4TmRjC9ymTi8mVj95dZgyF" +
		"RXbibTdtw0JzndE0Ig4c6pU4xDPPiyaziUSVDMIpzZDJH1GYOGHxbFasba4" +
		"1rRoWWkdBnsMtHm2ck%2FsFD2leL6u8q0OpVAc4ZdxseucL4OpCy%2BCLhQ" +
		"JFQT5bqSljP0g%3D%3D"
	if url != want {
		t.Fatalf("Unexpected signed URL; found %v", url)
	}
}

func TestSignedURL_PEMPrivateKey(t *testing.T) {
	t.Parallel()
	expires, _ := time.Parse(time.RFC3339, "2002-10-02T10:00:00-05:00")
	url, err := SignedURL("bucket-name", "object-name", &SignedURLOptions{
		GoogleAccessID: "xxx@clientid",
		PrivateKey:     dummyKey("pem"),
		Method:         "GET",
		MD5:            "ICy5YqxZB1uWSwcVLSNLcA==",
		Expires:        expires,
		ContentType:    "application/json",
		Headers:        []string{"x-header1", "x-header2"},
	})
	if err != nil {
		t.Error(err)
	}
	want := "https://storage.googleapis.com/bucket-name/object-name?" +
		"Expires=1033570800&GoogleAccessId=xxx%40clientid&Signature=" +
		"gHlh63sOxJnNj22X%2B%2F4kwOSNMeqwXWr4udEfrzJPQcq1xzxA8ovMM5SOrOc%" +
		"2FuE%2Ftc9%2Bq7a42CDBwZff1PsvuJMBDaPbluU257h%2Bvxx8lHMnb%2Bg1wD1" +
		"99FiCE014MRH9TlIg%2FdXRkErosVWTy4GqAgZemmKHo0HwDGT6IovB9mdg%3D"
	if url != want {
		t.Fatalf("Unexpected signed URL; found %v", url)
	}
}

func TestSignedURL_SignBytes(t *testing.T) {
	t.Parallel()
	expires, _ := time.Parse(time.RFC3339, "2002-10-02T10:00:00-05:00")
	url, err := SignedURL("bucket-name", "object-name", &SignedURLOptions{
		GoogleAccessID: "xxx@clientid",
		SignBytes: func(b []byte) ([]byte, error) {
			return []byte("signed"), nil
		},
		Method:      "GET",
		MD5:         "ICy5YqxZB1uWSwcVLSNLcA==",
		Expires:     expires,
		ContentType: "application/json",
		Headers:     []string{"x-header1", "x-header2"},
	})
	if err != nil {
		t.Error(err)
	}
	want := "https://storage.googleapis.com/bucket-name/object-name?" +
		"Expires=1033570800&GoogleAccessId=xxx%40clientid&Signature=" +
		"c2lnbmVk" // base64('signed') == 'c2lnbmVk'
	if url != want {
		t.Fatalf("Unexpected signed URL\ngot:  %q\nwant: %q", url, want)
	}
}

func TestSignedURL_URLUnsafeObjectName(t *testing.T) {
	t.Parallel()
	expires, _ := time.Parse(time.RFC3339, "2002-10-02T10:00:00-05:00")
	url, err := SignedURL("bucket-name", "object name界", &SignedURLOptions{
		GoogleAccessID: "xxx@clientid",
		PrivateKey:     dummyKey("pem"),
		Method:         "GET",
		MD5:            "ICy5YqxZB1uWSwcVLSNLcA==",
		Expires:        expires,
		ContentType:    "application/json",
		Headers:        []string{"x-header1", "x-header2"},
	})
	if err != nil {
		t.Error(err)
	}
	want := "https://storage.googleapis.com/bucket-name/object%20name%E7%95%8C?" +
		"Expires=1033570800&GoogleAccessId=xxx%40clientid&Signature=" +
		"LSxs1YwXNKOa7mQv1ZAI2ao0Fuv6yXLLU7%2BQ97z2B7hYZ57OiFwQ72EdGXSiIM" +
		"JwLisEKkwoSlYCMm3uuTdgJtXXVi7SYXMfdeKaonyQwMv531KETCBTSewt8CW%2B" +
		"FaUJ5SEYG44SeJCiqeIr3GF7t90UNWs6TdFXDaKShpQzBGg%3D"
	if url != want {
		t.Fatalf("Unexpected signed URL; found %v", url)
	}
}

func TestSignedURL_MissingOptions(t *testing.T) {
	t.Parallel()
	pk := dummyKey("rsa")
	expires, _ := time.Parse(time.RFC3339, "2002-10-02T10:00:00-05:00")
	var tests = []struct {
		opts   *SignedURLOptions
		errMsg string
	}{
		{
			&SignedURLOptions{},
			"missing required GoogleAccessID",
		},
		{
			&SignedURLOptions{GoogleAccessID: "access_id"},
			"exactly one of PrivateKey or SignedBytes must be set",
		},
		{
			&SignedURLOptions{
				GoogleAccessID: "access_id",
				SignBytes:      func(b []byte) ([]byte, error) { return b, nil },
				PrivateKey:     pk,
			},
			"exactly one of PrivateKey or SignedBytes must be set",
		},
		{
			&SignedURLOptions{
				GoogleAccessID: "access_id",
				PrivateKey:     pk,
			},
			"missing required method",
		},
		{
			&SignedURLOptions{
				GoogleAccessID: "access_id",
				SignBytes:      func(b []byte) ([]byte, error) { return b, nil },
			},
			"missing required method",
		},
		{
			&SignedURLOptions{
				GoogleAccessID: "access_id",
				PrivateKey:     pk,
				Method:         "PUT",
			},
			"missing required expires",
		},
		{
			&SignedURLOptions{
				GoogleAccessID: "access_id",
				PrivateKey:     pk,
				Method:         "PUT",
				Expires:        expires,
				MD5:            "invalid",
			},
			"invalid MD5 checksum",
		},
	}
	for _, test := range tests {
		_, err := SignedURL("bucket", "name", test.opts)
		if !strings.Contains(err.Error(), test.errMsg) {
			t.Errorf("expected err: %v, found: %v", test.errMsg, err)
		}
	}
}

func dummyKey(kind string) []byte {
	slurp, err := ioutil.ReadFile(fmt.Sprintf("./testdata/dummy_%s", kind))
	if err != nil {
		log.Fatal(err)
	}
	return slurp
}

func TestCopyToMissingFields(t *testing.T) {
	t.Parallel()
	var tests = []struct {
		srcBucket, srcName, destBucket, destName string
		errMsg                                   string
	}{
		{
			"mybucket", "", "mybucket", "destname",
			"name is empty",
		},
		{
			"mybucket", "srcname", "mybucket", "",
			"name is empty",
		},
		{
			"", "srcfile", "mybucket", "destname",
			"name is empty",
		},
		{
			"mybucket", "srcfile", "", "destname",
			"name is empty",
		},
	}
	ctx := context.Background()
	client, err := NewClient(ctx, option.WithHTTPClient(&http.Client{Transport: &fakeTransport{}}))
	if err != nil {
		panic(err)
	}
	for i, test := range tests {
		src := client.Bucket(test.srcBucket).Object(test.srcName)
		dst := client.Bucket(test.destBucket).Object(test.destName)
		_, err := dst.CopierFrom(src).Run(ctx)
		if !strings.Contains(err.Error(), test.errMsg) {
			t.Errorf("CopyTo test #%v:\ngot err  %q\nwant err %q", i, err, test.errMsg)
		}
	}
}

func TestObjectNames(t *testing.T) {
	t.Parallel()
	// Naming requirements: https://cloud.google.com/storage/docs/bucket-naming
	const maxLegalLength = 1024

	type testT struct {
		name, want string
	}
	tests := []testT{
		// Embedded characters important in URLs.
		{"foo % bar", "foo%20%25%20bar"},
		{"foo ? bar", "foo%20%3F%20bar"},
		{"foo / bar", "foo%20/%20bar"},
		{"foo %?/ bar", "foo%20%25%3F/%20bar"},

		// Non-Roman scripts
		{"타코", "%ED%83%80%EC%BD%94"},
		{"世界", "%E4%B8%96%E7%95%8C"},

		// Longest legal name
		{strings.Repeat("a", maxLegalLength), strings.Repeat("a", maxLegalLength)},

		// Line terminators besides CR and LF: https://en.wikipedia.org/wiki/Newline#Unicode
		{"foo \u000b bar", "foo%20%0B%20bar"},
		{"foo \u000c bar", "foo%20%0C%20bar"},
		{"foo \u0085 bar", "foo%20%C2%85%20bar"},
		{"foo \u2028 bar", "foo%20%E2%80%A8%20bar"},
		{"foo \u2029 bar", "foo%20%E2%80%A9%20bar"},

		// Null byte.
		{"foo \u0000 bar", "foo%20%00%20bar"},

		// Non-control characters that are discouraged, but not forbidden, according to the documentation.
		{"foo # bar", "foo%20%23%20bar"},
		{"foo []*? bar", "foo%20%5B%5D%2A%3F%20bar"},

		// Angstrom symbol singleton and normalized forms: http://unicode.org/reports/tr15/
		{"foo \u212b bar", "foo%20%E2%84%AB%20bar"},
		{"foo \u0041\u030a bar", "foo%20A%CC%8A%20bar"},
		{"foo \u00c5 bar", "foo%20%C3%85%20bar"},

		// Hangul separating jamo: http://www.unicode.org/versions/Unicode7.0.0/ch18.pdf (Table 18-10)
		{"foo \u3131\u314f bar", "foo%20%E3%84%B1%E3%85%8F%20bar"},
		{"foo \u1100\u1161 bar", "foo%20%E1%84%80%E1%85%A1%20bar"},
		{"foo \uac00 bar", "foo%20%EA%B0%80%20bar"},
	}

	// C0 control characters not forbidden by the docs.
	var runes []rune
	for r := rune(0x01); r <= rune(0x1f); r++ {
		if r != '\u000a' && r != '\u000d' {
			runes = append(runes, r)
		}
	}
	tests = append(tests, testT{fmt.Sprintf("foo %s bar", string(runes)), "foo%20%01%02%03%04%05%06%07%08%09%0B%0C%0E%0F%10%11%12%13%14%15%16%17%18%19%1A%1B%1C%1D%1E%1F%20bar"})

	// C1 control characters, plus DEL.
	runes = nil
	for r := rune(0x7f); r <= rune(0x9f); r++ {
		runes = append(runes, r)
	}
	tests = append(tests, testT{fmt.Sprintf("foo %s bar", string(runes)), "foo%20%7F%C2%80%C2%81%C2%82%C2%83%C2%84%C2%85%C2%86%C2%87%C2%88%C2%89%C2%8A%C2%8B%C2%8C%C2%8D%C2%8E%C2%8F%C2%90%C2%91%C2%92%C2%93%C2%94%C2%95%C2%96%C2%97%C2%98%C2%99%C2%9A%C2%9B%C2%9C%C2%9D%C2%9E%C2%9F%20bar"})

	opts := &SignedURLOptions{
		GoogleAccessID: "xxx@clientid",
		PrivateKey:     dummyKey("rsa"),
		Method:         "GET",
		MD5:            "ICy5YqxZB1uWSwcVLSNLcA==",
		Expires:        time.Date(2002, time.October, 2, 10, 0, 0, 0, time.UTC),
		ContentType:    "application/json",
		Headers:        []string{"x-header1", "x-header2"},
	}

	for _, test := range tests {
		g, err := SignedURL("bucket-name", test.name, opts)
		if err != nil {
			t.Errorf("SignedURL(%q) err=%v, want nil", test.name, err)
		}
		if w := "/bucket-name/" + test.want; !strings.Contains(g, w) {
			t.Errorf("SignedURL(%q)=%q, want substring %q", test.name, g, w)
		}
	}
}

func TestCondition(t *testing.T) {
	t.Parallel()
	gotReq := make(chan *http.Request, 1)
	hc, close := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(ioutil.Discard, r.Body)
		gotReq <- r
		w.WriteHeader(200)
	})
	defer close()
	ctx := context.Background()
	c, err := NewClient(ctx, option.WithHTTPClient(hc))
	if err != nil {
		t.Fatal(err)
	}

	obj := c.Bucket("buck").Object("obj")
	dst := c.Bucket("dstbuck").Object("dst")
	tests := []struct {
		fn   func()
		want string
	}{
		{
			func() { obj.Generation(1234).NewReader(ctx) },
			"GET /buck/obj?generation=1234",
		},
		{
			func() { obj.If(Conditions{GenerationMatch: 1234}).NewReader(ctx) },
			"GET /buck/obj?ifGenerationMatch=1234",
		},
		{
			func() { obj.If(Conditions{GenerationNotMatch: 1234}).NewReader(ctx) },
			"GET /buck/obj?ifGenerationNotMatch=1234",
		},
		{
			func() { obj.If(Conditions{MetagenerationMatch: 1234}).NewReader(ctx) },
			"GET /buck/obj?ifMetagenerationMatch=1234",
		},
		{
			func() { obj.If(Conditions{MetagenerationNotMatch: 1234}).NewReader(ctx) },
			"GET /buck/obj?ifMetagenerationNotMatch=1234",
		},
		{
			func() { obj.If(Conditions{MetagenerationNotMatch: 1234}).Attrs(ctx) },
			"GET /storage/v1/b/buck/o/obj?alt=json&ifMetagenerationNotMatch=1234&projection=full",
		},

		{
			func() { obj.If(Conditions{MetagenerationMatch: 1234}).Update(ctx, ObjectAttrsToUpdate{}) },
			"PATCH /storage/v1/b/buck/o/obj?alt=json&ifMetagenerationMatch=1234&projection=full",
		},
		{
			func() { obj.Generation(1234).Delete(ctx) },
			"DELETE /storage/v1/b/buck/o/obj?alt=json&generation=1234",
		},
		{
			func() {
				w := obj.If(Conditions{GenerationMatch: 1234}).NewWriter(ctx)
				w.ContentType = "text/plain"
				w.Close()
			},
			"POST /upload/storage/v1/b/buck/o?alt=json&ifGenerationMatch=1234&projection=full&uploadType=multipart",
		},
		{
			func() {
				w := obj.If(Conditions{DoesNotExist: true}).NewWriter(ctx)
				w.ContentType = "text/plain"
				w.Close()
			},
			"POST /upload/storage/v1/b/buck/o?alt=json&ifGenerationMatch=0&projection=full&uploadType=multipart",
		},
		{
			func() {
				dst.If(Conditions{MetagenerationMatch: 5678}).CopierFrom(obj.If(Conditions{GenerationMatch: 1234})).Run(ctx)
			},
			"POST /storage/v1/b/buck/o/obj/rewriteTo/b/dstbuck/o/dst?alt=json&ifMetagenerationMatch=5678&ifSourceGenerationMatch=1234&projection=full",
		},
	}

	for i, tt := range tests {
		tt.fn()
		select {
		case r := <-gotReq:
			got := r.Method + " " + r.RequestURI
			if got != tt.want {
				t.Errorf("%d. RequestURI = %q; want %q", i, got, tt.want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%d. timeout", i)
		}
		if err != nil {
			t.Fatal(err)
		}
	}

	// Test an error, too:
	err = obj.Generation(1234).NewWriter(ctx).Close()
	if err == nil || !strings.Contains(err.Error(), "NewWriter: generation not supported") {
		t.Errorf("want error about unsupported generation; got %v", err)
	}
}

func TestConditionErrors(t *testing.T) {
	t.Parallel()
	for _, conds := range []Conditions{
		{GenerationMatch: 0},
		{DoesNotExist: false}, // same as above, actually
		{GenerationMatch: 1, GenerationNotMatch: 2},
		{GenerationNotMatch: 2, DoesNotExist: true},
		{MetagenerationMatch: 1, MetagenerationNotMatch: 2},
	} {
		if err := conds.validate(""); err == nil {
			t.Errorf("%+v: got nil, want error", conds)
		}
	}
}

// Test object compose.
func TestObjectCompose(t *testing.T) {
	t.Parallel()
	gotURL := make(chan string, 1)
	gotBody := make(chan []byte, 1)
	hc, close := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		body, _ := ioutil.ReadAll(r.Body)
		gotURL <- r.URL.String()
		gotBody <- body
		w.Write([]byte("{}"))
	})
	defer close()
	ctx := context.Background()
	c, err := NewClient(ctx, option.WithHTTPClient(hc))
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		desc    string
		dst     *ObjectHandle
		srcs    []*ObjectHandle
		attrs   *ObjectAttrs
		wantReq raw.ComposeRequest
		wantURL string
		wantErr bool
	}{
		{
			desc: "basic case",
			dst:  c.Bucket("foo").Object("bar"),
			srcs: []*ObjectHandle{
				c.Bucket("foo").Object("baz"),
				c.Bucket("foo").Object("quux"),
			},
			wantURL: "/storage/v1/b/foo/o/bar/compose?alt=json",
			wantReq: raw.ComposeRequest{
				Destination: &raw.Object{Bucket: "foo"},
				SourceObjects: []*raw.ComposeRequestSourceObjects{
					{Name: "baz"},
					{Name: "quux"},
				},
			},
		},
		{
			desc: "with object attrs",
			dst:  c.Bucket("foo").Object("bar"),
			srcs: []*ObjectHandle{
				c.Bucket("foo").Object("baz"),
				c.Bucket("foo").Object("quux"),
			},
			attrs: &ObjectAttrs{
				Name:        "not-bar",
				ContentType: "application/json",
			},
			wantURL: "/storage/v1/b/foo/o/bar/compose?alt=json",
			wantReq: raw.ComposeRequest{
				Destination: &raw.Object{
					Bucket:      "foo",
					Name:        "not-bar",
					ContentType: "application/json",
				},
				SourceObjects: []*raw.ComposeRequestSourceObjects{
					{Name: "baz"},
					{Name: "quux"},
				},
			},
		},
		{
			desc: "with conditions",
			dst: c.Bucket("foo").Object("bar").If(Conditions{
				GenerationMatch:     12,
				MetagenerationMatch: 34,
			}),
			srcs: []*ObjectHandle{
				c.Bucket("foo").Object("baz").Generation(56),
				c.Bucket("foo").Object("quux").If(Conditions{GenerationMatch: 78}),
			},
			wantURL: "/storage/v1/b/foo/o/bar/compose?alt=json&ifGenerationMatch=12&ifMetagenerationMatch=34",
			wantReq: raw.ComposeRequest{
				Destination: &raw.Object{Bucket: "foo"},
				SourceObjects: []*raw.ComposeRequestSourceObjects{
					{
						Name:       "baz",
						Generation: 56,
					},
					{
						Name: "quux",
						ObjectPreconditions: &raw.ComposeRequestSourceObjectsObjectPreconditions{
							IfGenerationMatch: 78,
						},
					},
				},
			},
		},
		{
			desc:    "no sources",
			dst:     c.Bucket("foo").Object("bar"),
			wantErr: true,
		},
		{
			desc: "destination, no bucket",
			dst:  c.Bucket("").Object("bar"),
			srcs: []*ObjectHandle{
				c.Bucket("foo").Object("baz"),
			},
			wantErr: true,
		},
		{
			desc: "destination, no object",
			dst:  c.Bucket("foo").Object(""),
			srcs: []*ObjectHandle{
				c.Bucket("foo").Object("baz"),
			},
			wantErr: true,
		},
		{
			desc: "source, different bucket",
			dst:  c.Bucket("foo").Object("bar"),
			srcs: []*ObjectHandle{
				c.Bucket("otherbucket").Object("baz"),
			},
			wantErr: true,
		},
		{
			desc: "source, no object",
			dst:  c.Bucket("foo").Object("bar"),
			srcs: []*ObjectHandle{
				c.Bucket("foo").Object(""),
			},
			wantErr: true,
		},
		{
			desc: "destination, bad condition",
			dst:  c.Bucket("foo").Object("bar").Generation(12),
			srcs: []*ObjectHandle{
				c.Bucket("foo").Object("baz"),
			},
			wantErr: true,
		},
		{
			desc: "source, bad condition",
			dst:  c.Bucket("foo").Object("bar"),
			srcs: []*ObjectHandle{
				c.Bucket("foo").Object("baz").If(Conditions{MetagenerationMatch: 12}),
			},
			wantErr: true,
		},
	}

	for _, tt := range testCases {
		composer := tt.dst.ComposerFrom(tt.srcs...)
		if tt.attrs != nil {
			composer.ObjectAttrs = *tt.attrs
		}
		_, err := composer.Run(ctx)
		if gotErr := err != nil; gotErr != tt.wantErr {
			t.Errorf("%s: got error %v; want err %t", tt.desc, err, tt.wantErr)
			continue
		}
		if tt.wantErr {
			continue
		}
		url, body := <-gotURL, <-gotBody
		if url != tt.wantURL {
			t.Errorf("%s: request URL\ngot  %q\nwant %q", tt.desc, url, tt.wantURL)
		}
		var req raw.ComposeRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("%s: json.Unmarshal %v (body %s)", tt.desc, err, body)
		}
		if !testutil.Equal(req, tt.wantReq) {
			// Print to JSON.
			wantReq, _ := json.Marshal(tt.wantReq)
			t.Errorf("%s: request body\ngot  %s\nwant %s", tt.desc, body, wantReq)
		}
	}
}

// Test that ObjectIterator's Next and NextPage methods correctly terminate
// if there is nothing to iterate over.
func TestEmptyObjectIterator(t *testing.T) {
	t.Parallel()
	hClient, close := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(ioutil.Discard, r.Body)
		fmt.Fprintf(w, "{}")
	})
	defer close()
	ctx := context.Background()
	client, err := NewClient(ctx, option.WithHTTPClient(hClient))
	if err != nil {
		t.Fatal(err)
	}
	it := client.Bucket("b").Objects(ctx, nil)
	_, err = it.Next()
	if err != iterator.Done {
		t.Errorf("got %v, want Done", err)
	}
}

// Test that BucketIterator's Next method correctly terminates if there is
// nothing to iterate over.
func TestEmptyBucketIterator(t *testing.T) {
	t.Parallel()
	hClient, close := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(ioutil.Discard, r.Body)
		fmt.Fprintf(w, "{}")
	})
	defer close()
	ctx := context.Background()
	client, err := NewClient(ctx, option.WithHTTPClient(hClient))
	if err != nil {
		t.Fatal(err)
	}
	it := client.Buckets(ctx, "project")
	_, err = it.Next()
	if err != iterator.Done {
		t.Errorf("got %v, want Done", err)
	}

}

func TestCodecUint32(t *testing.T) {
	t.Parallel()
	for _, u := range []uint32{0, 1, 256, 0xFFFFFFFF} {
		s := encodeUint32(u)
		d, err := decodeUint32(s)
		if err != nil {
			t.Fatal(err)
		}
		if d != u {
			t.Errorf("got %d, want input %d", d, u)
		}
	}
}

func TestBucketAttrs(t *testing.T) {
	for _, c := range []struct {
		attrs BucketAttrs
		raw   raw.Bucket
	}{{
		attrs: BucketAttrs{
			Lifecycle: Lifecycle{
				Rules: []LifecycleRule{{
					Action: LifecycleAction{
						Type:         SetStorageClassAction,
						StorageClass: "NEARLINE",
					},
					Condition: LifecycleCondition{
						AgeInDays:             10,
						Liveness:              Live,
						CreatedBefore:         time.Date(2017, 1, 2, 3, 4, 5, 6, time.UTC),
						MatchesStorageClasses: []string{"MULTI_REGIONAL", "REGIONAL", "STANDARD"},
						NumNewerVersions:      3,
					},
				}, {
					Action: LifecycleAction{
						Type: DeleteAction,
					},
					Condition: LifecycleCondition{
						AgeInDays:             30,
						Liveness:              Live,
						CreatedBefore:         time.Date(2017, 1, 2, 3, 4, 5, 6, time.UTC),
						MatchesStorageClasses: []string{"NEARLINE"},
						NumNewerVersions:      10,
					},
				}, {
					Action: LifecycleAction{
						Type: DeleteAction,
					},
					Condition: LifecycleCondition{
						Liveness: Archived,
					},
				}},
			},
		},
		raw: raw.Bucket{
			Lifecycle: &raw.BucketLifecycle{
				Rule: []*raw.BucketLifecycleRule{{
					Action: &raw.BucketLifecycleRuleAction{
						Type:         SetStorageClassAction,
						StorageClass: "NEARLINE",
					},
					Condition: &raw.BucketLifecycleRuleCondition{
						Age:                 10,
						IsLive:              googleapi.Bool(true),
						CreatedBefore:       "2017-01-02",
						MatchesStorageClass: []string{"MULTI_REGIONAL", "REGIONAL", "STANDARD"},
						NumNewerVersions:    3,
					},
				}, {
					Action: &raw.BucketLifecycleRuleAction{
						Type: DeleteAction,
					},
					Condition: &raw.BucketLifecycleRuleCondition{
						Age:                 30,
						IsLive:              googleapi.Bool(true),
						CreatedBefore:       "2017-01-02",
						MatchesStorageClass: []string{"NEARLINE"},
						NumNewerVersions:    10,
					},
				}, {
					Action: &raw.BucketLifecycleRuleAction{
						Type: DeleteAction,
					},
					Condition: &raw.BucketLifecycleRuleCondition{
						IsLive: googleapi.Bool(false),
					},
				}},
			},
		},
	}} {
		if got := c.attrs.toRawBucket(); !testutil.Equal(*got, c.raw) {
			t.Errorf("toRawBucket: got %v, want %v", *got, c.raw)
		}
	}
}

func newTestServer(handler func(w http.ResponseWriter, r *http.Request)) (*http.Client, func()) {
	ts := httptest.NewTLSServer(http.HandlerFunc(handler))
	tlsConf := &tls.Config{InsecureSkipVerify: true}
	tr := &http.Transport{
		TLSClientConfig: tlsConf,
		DialTLS: func(netw, addr string) (net.Conn, error) {
			return tls.Dial("tcp", ts.Listener.Addr().String(), tlsConf)
		},
	}
	return &http.Client{Transport: tr}, func() {
		tr.CloseIdleConnections()
		ts.Close()
	}
}
