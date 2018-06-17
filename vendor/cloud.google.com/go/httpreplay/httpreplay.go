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

// Package httpreplay provides an API for recording and replaying traffic
// from HTTP-based Google API clients.
//
// To record:
// 1.  Call NewRecorder to get a Recorder.
// 2.  Use its Client method to obtain an HTTP client to use when making API calls.
// 3.  Close the Recorder when you're done. That will save the
//     log of interactions to the file you provided to NewRecorder.
//
// To replay:
// 1.  Call NewReplayer with the same filename you used to record to get a Replayer.
// 2.  Call its Client method and use the client to make the same API calls.
//     You will get back the recorded responses.
// 3.  Close the Replayer when you're done.
//
// This package is EXPERIMENTAL and is subject to change or removal without notice.
// It requires Go version 1.8 or higher.
package httpreplay

// TODO(jba): add examples.

import (
	"fmt"
	"net/http"

	"cloud.google.com/go/httpreplay/internal/proxy"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	htransport "google.golang.org/api/transport/http"
)

// A Recorder records HTTP interactions.
type Recorder struct {
	filename string
	proxy    *proxy.Proxy
}

// NewRecorder creates a recorder that writes to filename. The file will
// also store initial state that can be retrieved to configure replay. The "initial"
// argument must work with json.Marshal.
//
// You must call Close on the Recorder to ensure that all data is written.
func NewRecorder(filename string, initial interface{}) (*Recorder, error) {
	p, err := proxy.ForRecording(filename, 0)
	if err != nil {
		return nil, err
	}
	p.Initial = initial
	return &Recorder{proxy: p}, nil
}

// Client returns an http.Client to be used for recording. Provide authentication options
// like option.WithTokenSource as you normally would, or omit them to use Application Default
// Credentials.
func (r *Recorder) Client(ctx context.Context, opts ...option.ClientOption) (*http.Client, error) {
	hc, _, err := htransport.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	// The http.Client returned by htransport.NewClient contains an
	// http.RoundTripper. We want to somehow plug in a Transport that calls the proxy
	// (returned by r.proxy.Transport).
	//
	// htransport.NewClient constructs its RoundTripper via the decorator pattern, by
	// nesting several implementations of RoundTripper inside each other, ending with
	// http.DefaultTransport. For example, one of the decorators is oauth2.Transport,
	// which inserts an Authorization header and then calls the next RoundTripper in
	// the sequence (stored in a field called Base).
	//
	// The problem is that we need to insert the proxy Transport at the end of this
	// sequence, where http.DefaultTransport currently lives. But we can't traverse
	// that sequence of RoundTrippers in general, because we don't know their types.
	//
	// For now, we only handle the special (but common) case where the first
	// RoundTripper in the sequence is an oauth2.Transport. We can replace its Base
	// field with the proxy transport. This causes us to lose the other RoundTrippers
	// in the sequence, but those aren't essential for testing.
	//
	// A better solution would be to add option.WithBaseTransport, which would allow
	// us to replace the http.DefaultTransport at the end of the sequence with the
	// transport of our choice.
	otrans, ok := hc.Transport.(*oauth2.Transport)
	if !ok {
		return nil, fmt.Errorf("can't handle Transport of type %T", hc.Transport)
	}
	otrans.Base = r.proxy.Transport()
	return hc, nil
}

// Close closes the Recorder and saves the log file.
func (r *Recorder) Close() error {
	return r.proxy.Close()
}

// A Replayer replays previously recorded HTTP interactions.
type Replayer struct {
	proxy *proxy.Proxy
}

// NewReplayer creates a replayer that reads from filename.
func NewReplayer(filename string) (*Replayer, error) {
	p, err := proxy.ForReplaying(filename, 0)
	if err != nil {
		return nil, err
	}
	return &Replayer{proxy: p}, nil
}

// Client returns an HTTP client for replaying. The client does not need to be
// configured with credentials for authenticating to a server, since it never
// contacts a real backend.
func (r *Replayer) Client(ctx context.Context) (*http.Client, error) {
	return &http.Client{Transport: r.proxy.Transport()}, nil
}

// Initial returns the initial state saved by the Recorder.
func (r *Replayer) Initial() interface{} {
	return r.proxy.Initial
}

// Close closes the replayer.
func (r *Replayer) Close() error {
	return r.proxy.Close()
}

// DebugHeaders helps to determine whether a header should be ignored.
// When true, if requests have the same method, URL and body but differ
// in a header, the first mismatched header is logged.
func DebugHeaders() {
	proxy.DebugHeaders = true
}
