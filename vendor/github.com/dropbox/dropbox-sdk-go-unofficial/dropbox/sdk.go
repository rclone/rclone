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
	"fmt"
	"io"
	"log"
	"net/http"

	"golang.org/x/oauth2"
)

const (
	apiVersion    = 2
	defaultDomain = ".dropboxapi.com"
	hostAPI       = "api"
	hostContent   = "content"
	hostNotify    = "notify"
	sdkVersion    = "1.0.0-beta"
	specVersion   = "6194bea"
)

// Version returns the current SDK version and API Spec version
func Version() (string, string) {
	return sdkVersion, specVersion
}

// Config contains parameters for configuring the SDK.
type Config struct {
	// OAuth2 access token
	Token string
	// Enable verbose logging in SDK
	Verbose bool
	// Logging target for verbose SDK logging
	Logger *log.Logger
	// Used with APIs that support operations as another user
	AsMemberID string
	// No need to set -- for testing only
	Domain string
	// No need to set -- for testing only
	Client *http.Client
	// No need to set -- for testing only
	HeaderGenerator func(hostType string, style string, namespace string, route string) map[string]string
	// No need to set -- for testing only
	URLGenerator func(hostType string, style string, namespace string, route string) string
}

// TryLog will, if Verbose is set, log to the config's logger
// or the default log (stderr) if Config.Logger is nil.
func (c *Config) TryLog(format string, v ...interface{}) {
	if !c.Verbose {
		return
	}
	if c.Logger != nil {
		c.Logger.Printf(format, v...)
	} else {
		log.Printf(format, v...)
	}
}

// Context is the base client context used to implement per-namespace clients.
type Context struct {
	Config          Config
	Client          *http.Client
	HeaderGenerator func(hostType string, style string, namespace string, route string) map[string]string
	URLGenerator    func(hostType string, style string, namespace string, route string) string
}

// NewRequest returns an appropriate Request object for the given namespace/route.
func (c *Context) NewRequest(
	hostType string,
	style string,
	authed bool,
	namespace string,
	route string,
	headers map[string]string,
	body io.Reader,
) (*http.Request, error) {
	url := c.URLGenerator(hostType, style, namespace, route)
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Add(k, v)
	}
	for k, v := range c.HeaderGenerator(hostType, style, namespace, route) {
		req.Header.Add(k, v)
	}
	if req.Header.Get("Host") != "" {
		req.Host = req.Header.Get("Host")
	}
	if !authed {
		req.Header.Del("Authorization")
	}
	return req, nil
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
		client = conf.Client(oauth2.NoContext, tok)
	}

	headerGenerator := c.HeaderGenerator
	if headerGenerator == nil {
		headerGenerator = func(hostType string, style string, namespace string, route string) map[string]string {
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
		urlGenerator = func(hostType string, style string, namespace string, route string) string {
			fqHost := hostMap[hostType]
			return fmt.Sprintf("https://%s/%d/%s/%s", fqHost, apiVersion, namespace, route)
		}
	}

	return Context{c, client, headerGenerator, urlGenerator}
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

func init() {
	// These are not registered in the oauth library by default
	oauth2.RegisterBrokenAuthHeaderProvider("https://api.dropboxapi.com")
	oauth2.RegisterBrokenAuthHeaderProvider("https://api-dbdev.dev.corp.dropbox.com")
}
