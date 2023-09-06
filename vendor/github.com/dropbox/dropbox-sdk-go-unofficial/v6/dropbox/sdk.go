// Copyright (c) Dropbox, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package dropbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
)

const (
	apiVersion    = 2
	defaultDomain = ".dropboxapi.com"
	hostAPI       = "api"
	hostContent   = "content"
	hostNotify    = "notify"
	sdkVersion    = "6.0.5"
	specVersion   = "c36ba27"
)

// Version returns the current SDK version and API Spec version
func Version() (string, string) {
	return sdkVersion, specVersion
}

// Tagged is used for tagged unions.
type Tagged struct {
	Tag string `json:".tag"`
}

// APIError is the base type for endpoint-specific errors.
type APIError struct {
	ErrorSummary string `json:"error_summary"`
}

func (e APIError) Error() string {
	return e.ErrorSummary
}

type SDKInternalError struct {
	StatusCode int
	Content    string
}

func (e SDKInternalError) Error() string {
	return fmt.Sprintf("Unexpected error: %v (code: %v)", e.Content, e.StatusCode)
}

// Config contains parameters for configuring the SDK.
type Config struct {
	// OAuth2 access token
	Token string
	// Logging level for SDK generated logs
	LogLevel LogLevel
	// Logging target for verbose SDK logging
	Logger *log.Logger
	// Used with APIs that support operations as another user
	AsMemberID string
	// Used with APIs that support operations as an admin
	AsAdminID string
	// Path relative to which action should be taken
	PathRoot string
	// No need to set -- for testing only
	Domain string
	// No need to set -- for testing only
	Client *http.Client
	// No need to set -- for testing only
	HeaderGenerator func(hostType string, namespace string, route string) map[string]string
	// No need to set -- for testing only
	URLGenerator func(hostType string, namespace string, route string) string
}

// LogLevel defines a type that can set the desired level of logging the SDK will generate.
type LogLevel uint

const (
	// LogOff will disable all SDK logging. This is the default log level
	LogOff LogLevel = iota * (1 << 8)
	// LogDebug will enable detailed SDK debug logs. It will log requests (including arguments),
	// response and body contents.
	LogDebug
	// LogInfo will log SDK request (not including arguments) and responses.
	LogInfo
)

func (l LogLevel) shouldLog(v LogLevel) bool {
	return l > v || l&v == v
}

func (c *Config) doLog(l LogLevel, format string, v ...interface{}) {
	if !c.LogLevel.shouldLog(l) {
		return
	}

	if c.Logger != nil {
		c.Logger.Printf(format, v...)
	} else {
		log.Printf(format, v...)
	}
}

// LogDebug emits a debug level SDK log if config's log level is at least LogDebug
func (c *Config) LogDebug(format string, v ...interface{}) {
	c.doLog(LogDebug, format, v...)
}

// LogInfo emits an info level SDK log if config's log level is at least LogInfo
func (c *Config) LogInfo(format string, v ...interface{}) {
	c.doLog(LogInfo, format, v...)
}

// Ergonomic methods to set namespace relative to which action should be taken
func (c Config) WithNamespaceID(nsID string) Config {
	c.PathRoot = fmt.Sprintf(`{".tag": "namespace_id", "namespace_id": "%s"}`, nsID)
	return c
}

func (c Config) WithRoot(nsID string) Config {
	c.PathRoot = fmt.Sprintf(`{".tag": "root", "root": "%s"}`, nsID)
	return c
}

// Context is the base client context used to implement per-namespace clients.
type Context struct {
	Config          Config
	Client          *http.Client
	NoAuthClient    *http.Client
	HeaderGenerator func(hostType string, namespace string, route string) map[string]string
	URLGenerator    func(hostType string, namespace string, route string) string
}

type Request struct {
	Host      string
	Namespace string
	Route     string
	Style     string
	Auth      string

	Arg          interface{}
	ExtraHeaders map[string]string
}

func (c *Context) Execute(req Request, body io.Reader) ([]byte, io.ReadCloser, error) {
	url := c.URLGenerator(req.Host, req.Namespace, req.Route)
	httpReq, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, nil, err
	}

	for k, v := range req.ExtraHeaders {
		httpReq.Header.Add(k, v)
	}

	for k, v := range c.HeaderGenerator(req.Host, req.Namespace, req.Route) {
		httpReq.Header.Add(k, v)
	}

	if httpReq.Header.Get("Host") != "" {
		httpReq.Host = httpReq.Header.Get("Host")
	}

	if req.Auth == "noauth" {
		httpReq.Header.Del("Authorization")
	}
	if req.Auth != "team" && c.Config.AsMemberID != "" {
		httpReq.Header.Add("Dropbox-API-Select-User", c.Config.AsMemberID)
	}
	if req.Auth != "team" && c.Config.AsAdminID != "" {
		httpReq.Header.Add("Dropbox-API-Select-Admin", c.Config.AsAdminID)
	}
	if c.Config.PathRoot != "" {
		httpReq.Header.Add("Dropbox-API-Path-Root", c.Config.PathRoot)
	}

	if req.Arg != nil {
		serializedArg, err := json.Marshal(req.Arg)
		if err != nil {
			return nil, nil, err
		}

		switch req.Style {
		case "rpc":
			if body != nil {
				return nil, nil, errors.New("RPC style requests can not have body")
			}

			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Body = ioutil.NopCloser(bytes.NewReader(serializedArg))
			httpReq.ContentLength = int64(len(serializedArg))
		case "upload", "download":
			httpReq.Header.Set("Dropbox-API-Arg", string(serializedArg))
			httpReq.Header.Set("Content-Type", "application/octet-stream")
		}
	}

	client := c.Client
	if req.Auth == "noauth" {
		client = c.NoAuthClient
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
		switch req.Style {
		case "rpc", "upload":
			if resp.Body == nil {
				return nil, nil, errors.New("Expected body in RPC response, got nil")
			}

			b, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return nil, nil, err
			}

			return b, nil, nil
		case "download":
			b := []byte(resp.Header.Get("Dropbox-API-Result"))
			return b, resp.Body, nil
		}
	}

	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, nil, err
	}

	return nil, nil, SDKInternalError{
		StatusCode: resp.StatusCode,
		Content:    string(b),
	}
}

// NewContext returns a new Context with the given Config.
func NewContext(c Config) Context {
	domain := c.Domain
	if domain == "" {
		domain = defaultDomain
	}

	client := c.Client
	if client == nil {
		var conf = &oauth2.Config{Endpoint: OAuthEndpoint(domain)}
		tok := &oauth2.Token{AccessToken: c.Token}
		client = conf.Client(context.Background(), tok)
	}

	noAuthClient := c.Client
	if noAuthClient == nil {
		noAuthClient = &http.Client{}
	}

	headerGenerator := c.HeaderGenerator
	if headerGenerator == nil {
		headerGenerator = func(hostType string, namespace string, route string) map[string]string {
			return map[string]string{}
		}
	}

	urlGenerator := c.URLGenerator
	if urlGenerator == nil {
		hostMap := map[string]string{
			hostAPI:     hostAPI + domain,
			hostContent: hostContent + domain,
			hostNotify:  hostNotify + domain,
		}
		urlGenerator = func(hostType string, namespace string, route string) string {
			fqHost := hostMap[hostType]
			return fmt.Sprintf("https://%s/%d/%s/%s", fqHost, apiVersion, namespace, route)
		}
	}

	return Context{c, client, noAuthClient, headerGenerator, urlGenerator}
}

// OAuthEndpoint constructs an `oauth2.Endpoint` for the given domain
func OAuthEndpoint(domain string) oauth2.Endpoint {
	if domain == "" {
		domain = defaultDomain
	}
	authURL := fmt.Sprintf("https://meta%s/1/oauth2/authorize", domain)
	tokenURL := fmt.Sprintf("https://api%s/1/oauth2/token", domain)
	if domain == defaultDomain {
		authURL = "https://www.dropbox.com/1/oauth2/authorize"
	}
	return oauth2.Endpoint{AuthURL: authURL, TokenURL: tokenURL}
}

// HTTPHeaderSafeJSON encode the JSON passed in b []byte passed in in
// a way that is suitable for HTTP headers.
//
// See: https://www.dropbox.com/developers/reference/json-encoding
func HTTPHeaderSafeJSON(b []byte) string {
	var s strings.Builder
	s.Grow(len(b))
	for _, r := range string(b) {
		if r >= 0x007f {
			fmt.Fprintf(&s, "\\u%04x", r)
		} else {
			s.WriteRune(r)
		}
	}
	return s.String()
}
