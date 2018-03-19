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
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"cloud.google.com/go/internal/testutil"

	"golang.org/x/net/context"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

type fakeTransport struct {
	gotReq  *http.Request
	gotBody []byte
	results []transportResult
}

type transportResult struct {
	res *http.Response
	err error
}

func (t *fakeTransport) addResult(res *http.Response, err error) {
	t.results = append(t.results, transportResult{res, err})
}

func (t *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.gotReq = req
	t.gotBody = nil
	if req.Body != nil {
		bytes, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		t.gotBody = bytes
	}
	if len(t.results) == 0 {
		return nil, fmt.Errorf("error handling request")
	}
	result := t.results[0]
	t.results = t.results[1:]
	return result.res, result.err
}

func TestErrorOnObjectsInsertCall(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const contents = "hello world"

	doWrite := func(hc *http.Client) *Writer {
		client, err := NewClient(ctx, option.WithHTTPClient(hc))
		if err != nil {
			t.Fatalf("error when creating client: %v", err)
		}
		wc := client.Bucket("bucketname").Object("filename1").NewWriter(ctx)
		wc.ContentType = "text/plain"

		// We can't check that the Write fails, since it depends on the write to the
		// underling fakeTransport failing which is racy.
		wc.Write([]byte(contents))
		return wc
	}

	wc := doWrite(&http.Client{Transport: &fakeTransport{}})
	// Close must always return an error though since it waits for the transport to
	// have closed.
	if err := wc.Close(); err == nil {
		t.Errorf("expected error on close, got nil")
	}

	// Retry on 5xx
	ft := &fakeTransport{}
	ft.addResult(&http.Response{
		StatusCode: 503,
		Body:       ioutil.NopCloser(&bytes.Buffer{}),
	}, nil)
	ft.addResult(&http.Response{
		StatusCode: 200,
		Body:       ioutil.NopCloser(strings.NewReader("{}")),
	}, nil)
	wc = doWrite(&http.Client{Transport: ft})
	if err := wc.Close(); err != nil {
		t.Errorf("got %v, want nil", err)
	}
	got := string(ft.gotBody)
	if !strings.Contains(got, contents) {
		t.Errorf("got body %q, which does not contain %q", got, contents)
	}
}

func TestEncryption(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ft := &fakeTransport{}
	hc := &http.Client{Transport: ft}
	client, err := NewClient(ctx, option.WithHTTPClient(hc))
	if err != nil {
		t.Fatalf("error when creating client: %v", err)
	}
	obj := client.Bucket("bucketname").Object("filename1")
	key := []byte("secret-key-that-is-32-bytes-long")
	wc := obj.Key(key).NewWriter(ctx)
	// TODO(jba): use something other than fakeTransport, which always returns error.
	wc.Write([]byte("hello world"))
	wc.Close()
	if got, want := ft.gotReq.Header.Get("x-goog-encryption-algorithm"), "AES256"; got != want {
		t.Errorf("algorithm: got %q, want %q", got, want)
	}
	gotKey, err := base64.StdEncoding.DecodeString(ft.gotReq.Header.Get("x-goog-encryption-key"))
	if err != nil {
		t.Fatalf("decoding key: %v", err)
	}
	if !testutil.Equal(gotKey, key) {
		t.Errorf("key: got %v, want %v", gotKey, key)
	}
	wantHash := sha256.Sum256(key)
	gotHash, err := base64.StdEncoding.DecodeString(ft.gotReq.Header.Get("x-goog-encryption-key-sha256"))
	if err != nil {
		t.Fatalf("decoding hash: %v", err)
	}
	if !testutil.Equal(gotHash, wantHash[:]) { // wantHash is an array
		t.Errorf("hash: got\n%v, want\n%v", gotHash, wantHash)
	}
}

// This test demonstrates the data race on Writer.err that can happen when the
// Writer's context is cancelled. To see the race, comment out the w.mu.Lock/Unlock
// lines in writer.go and run this test with -race.
func TestRaceOnCancel(t *testing.T) {
	ctx := context.Background()
	ft := &fakeTransport{}
	hc := &http.Client{Transport: ft}
	client, err := NewClient(ctx, option.WithHTTPClient(hc))
	if err != nil {
		t.Fatalf("error when creating client: %v", err)
	}

	cctx, cancel := context.WithCancel(ctx)
	w := client.Bucket("b").Object("o").NewWriter(cctx)
	w.ChunkSize = googleapi.MinUploadChunkSize
	buf := make([]byte, w.ChunkSize)
	// This Write starts the goroutine in Writer.open. That reads the first chunk in its entirety
	// before sending the request (see google.golang.org/api/gensupport.PrepareUpload),
	// so to exhibit the race we must provide ChunkSize bytes.  The goroutine then makes the RPC (L137).
	w.Write(buf)
	// Canceling the context causes the call to return context.Canceled, which makes the open goroutine
	// write to w.err (L151).
	cancel()
	// This call to Write concurrently reads w.err (L169).
	w.Write([]byte(nil))
}
