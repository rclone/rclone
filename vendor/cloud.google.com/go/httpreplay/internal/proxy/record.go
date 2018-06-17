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

// The proxy package provides a record/replay HTTP proxy. It is designed to support
// both an in-memory API (cloud.google.com/go/httpreplay) and a standalone server
// (cloud.google.com/go/httpreplay/cmd/httpr).
package proxy

// See github.com/google/martian/cmd/proxy/main.go for the origin of much of this.

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/martian"
	"github.com/google/martian/fifo"
	"github.com/google/martian/har"
	"github.com/google/martian/httpspec"
	"github.com/google/martian/martianlog"
	"github.com/google/martian/mitm"
)

// A Proxy is an HTTP proxy that supports recording or replaying requests.
type Proxy struct {
	// The certificate that the proxy uses to participate in TLS.
	CACert *x509.Certificate

	// The URL of the proxy.
	URL *url.URL

	// Initial state of the client. Must be serializable with json.Marshal.
	Initial interface{}

	mproxy   *martian.Proxy
	filename string      // for log
	logger   *har.Logger // for recording only
}

// ForRecording returns a Proxy configured to record.
func ForRecording(filename string, port int) (*Proxy, error) {
	p, err := newProxy(filename)
	if err != nil {
		return nil, err
	}
	// Configure the transport for the proxy's outgoing traffic.
	p.mproxy.SetRoundTripper(&http.Transport{
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	})

	// Construct a group that performs the standard proxy stack of request/response
	// modifications.
	stack, _ := httpspec.NewStack("httpr") // second arg is an internal group that we don't need
	p.mproxy.SetRequestModifier(stack)
	p.mproxy.SetResponseModifier(stack)

	// Make a group for logging requests and responses.
	logGroup := fifo.NewGroup()
	skipAuth := skipLoggingByHost("accounts.google.com")
	logGroup.AddRequestModifier(skipAuth)
	logGroup.AddResponseModifier(skipAuth)
	p.logger = har.NewLogger()
	logGroup.AddRequestModifier(martian.RequestModifierFunc(
		func(req *http.Request) error { return withRedactedHeaders(req, p.logger) }))
	logGroup.AddResponseModifier(p.logger)

	stack.AddRequestModifier(logGroup)
	stack.AddResponseModifier(logGroup)

	// Ordinary debug logging.
	logger := martianlog.NewLogger()
	logger.SetDecode(true)
	stack.AddRequestModifier(logger)
	stack.AddResponseModifier(logger)

	if err := p.start(port); err != nil {
		return nil, err
	}
	return p, nil
}

func newProxy(filename string) (*Proxy, error) {
	mproxy := martian.NewProxy()
	// Set up a man-in-the-middle configuration with a CA certificate so the proxy can
	// participate in TLS.
	x509c, priv, err := mitm.NewAuthority("cloud.google.com/go/httpreplay", "HTTPReplay Authority", time.Hour)
	if err != nil {
		return nil, err
	}
	mc, err := mitm.NewConfig(x509c, priv)
	if err != nil {
		return nil, err
	}
	mc.SetValidity(time.Hour)
	mc.SetOrganization("cloud.google.com/go/httpreplay")
	mc.SkipTLSVerify(false)
	if err != nil {
		return nil, err
	}
	mproxy.SetMITM(mc)
	return &Proxy{
		mproxy:   mproxy,
		CACert:   x509c,
		filename: filename,
	}, nil
}

func (p *Proxy) start(port int) error {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	p.URL = &url.URL{Scheme: "http", Host: l.Addr().String()}
	go p.mproxy.Serve(l)
	return nil
}

// Transport returns an http.Transport for clients who want to talk to the proxy.
func (p *Proxy) Transport() *http.Transport {
	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(p.CACert)
	return &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: caCertPool},
		Proxy:           func(*http.Request) (*url.URL, error) { return p.URL, nil },
	}
}

// Close closes the proxy. If the proxy is recording, it also writes the log.
func (p *Proxy) Close() error {
	p.mproxy.Close()
	if p.logger != nil {
		return p.writeLog()
	}
	return nil
}

type httprFile struct {
	Initial interface{}
	HAR     *har.HAR
}

func (p *Proxy) writeLog() error {
	f := httprFile{
		Initial: p.Initial,
		HAR:     p.logger.ExportAndReset(),
	}
	bytes, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(p.filename, bytes, 0600) // only accessible by owner
}

// Headers that may contain sensitive data (auth tokens, keys).
var sensitiveHeaders = []string{
	"Authorization",
	"X-Goog-Encryption-Key",             // used by Cloud Storage for customer-supplied encryption
	"X-Goog-Copy-Source-Encryption-Key", // ditto
}

// withRedactedHeaders removes sensitive header contents before calling mod.
func withRedactedHeaders(req *http.Request, mod martian.RequestModifier) error {
	// We have to change the headers, then log, then restore them.
	replaced := map[string]string{}
	for _, h := range sensitiveHeaders {
		if v := req.Header.Get(h); v != "" {
			replaced[h] = v
			req.Header.Set(h, "REDACTED")
		}
	}
	err := mod.ModifyRequest(req)
	for h, v := range replaced {
		req.Header.Set(h, v)
	}
	return err
}

// skipLoggingByHost disables logging for traffic to a particular host.
type skipLoggingByHost string

func (s skipLoggingByHost) ModifyRequest(req *http.Request) error {
	if strings.HasPrefix(req.Host, string(s)) {
		martian.NewContext(req).SkipLogging()
	}
	return nil
}

func (s skipLoggingByHost) ModifyResponse(res *http.Response) error {
	return s.ModifyRequest(res.Request)
}
