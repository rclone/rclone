// Copyright (c) 2015-2021 Jeevanandam M (jeeva@myjeeva.com), All rights reserved.
// resty source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package resty

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// MethodGet HTTP method
	MethodGet = "GET"

	// MethodPost HTTP method
	MethodPost = "POST"

	// MethodPut HTTP method
	MethodPut = "PUT"

	// MethodDelete HTTP method
	MethodDelete = "DELETE"

	// MethodPatch HTTP method
	MethodPatch = "PATCH"

	// MethodHead HTTP method
	MethodHead = "HEAD"

	// MethodOptions HTTP method
	MethodOptions = "OPTIONS"
)

var (
	hdrUserAgentKey       = http.CanonicalHeaderKey("User-Agent")
	hdrAcceptKey          = http.CanonicalHeaderKey("Accept")
	hdrContentTypeKey     = http.CanonicalHeaderKey("Content-Type")
	hdrContentLengthKey   = http.CanonicalHeaderKey("Content-Length")
	hdrContentEncodingKey = http.CanonicalHeaderKey("Content-Encoding")
	hdrLocationKey        = http.CanonicalHeaderKey("Location")

	plainTextType   = "text/plain; charset=utf-8"
	jsonContentType = "application/json"
	formContentType = "application/x-www-form-urlencoded"

	jsonCheck = regexp.MustCompile(`(?i:(application|text)/(json|.*\+json|json\-.*)(;|$))`)
	xmlCheck  = regexp.MustCompile(`(?i:(application|text)/(xml|.*\+xml)(;|$))`)

	hdrUserAgentValue = "go-resty/" + Version + " (https://github.com/go-resty/resty)"
	bufPool           = &sync.Pool{New: func() interface{} { return &bytes.Buffer{} }}
)

type (
	// RequestMiddleware type is for request middleware, called before a request is sent
	RequestMiddleware func(*Client, *Request) error

	// ResponseMiddleware type is for response middleware, called after a response has been received
	ResponseMiddleware func(*Client, *Response) error

	// PreRequestHook type is for the request hook, called right before the request is sent
	PreRequestHook func(*Client, *http.Request) error

	// RequestLogCallback type is for request logs, called before the request is logged
	RequestLogCallback func(*RequestLog) error

	// ResponseLogCallback type is for response logs, called before the response is logged
	ResponseLogCallback func(*ResponseLog) error

	// ErrorHook type is for reacting to request errors, called after all retries were attempted
	ErrorHook func(*Request, error)
)

// Client struct is used to create Resty client with client level settings,
// these settings are applicable to all the request raised from the client.
//
// Resty also provides an options to override most of the client settings
// at request level.
type Client struct {
	BaseURL               string
	HostURL               string // Deprecated: use BaseURL instead. To be removed in v3.0.0 release.
	QueryParam            url.Values
	FormData              url.Values
	PathParams            map[string]string
	Header                http.Header
	UserInfo              *User
	Token                 string
	AuthScheme            string
	Cookies               []*http.Cookie
	Error                 reflect.Type
	Debug                 bool
	DisableWarn           bool
	AllowGetMethodPayload bool
	RetryCount            int
	RetryWaitTime         time.Duration
	RetryMaxWaitTime      time.Duration
	RetryConditions       []RetryConditionFunc
	RetryHooks            []OnRetryFunc
	RetryAfter            RetryAfterFunc
	JSONMarshal           func(v interface{}) ([]byte, error)
	JSONUnmarshal         func(data []byte, v interface{}) error
	XMLMarshal            func(v interface{}) ([]byte, error)
	XMLUnmarshal          func(data []byte, v interface{}) error

	// HeaderAuthorizationKey is used to set/access Request Authorization header
	// value when `SetAuthToken` option is used.
	HeaderAuthorizationKey string

	jsonEscapeHTML     bool
	setContentLength   bool
	closeConnection    bool
	notParseResponse   bool
	trace              bool
	debugBodySizeLimit int64
	outputDirectory    string
	scheme             string
	log                Logger
	httpClient         *http.Client
	proxyURL           *url.URL
	beforeRequest      []RequestMiddleware
	udBeforeRequest    []RequestMiddleware
	preReqHook         PreRequestHook
	afterResponse      []ResponseMiddleware
	requestLog         RequestLogCallback
	responseLog        ResponseLogCallback
	errorHooks         []ErrorHook
}

// User type is to hold an username and password information
type User struct {
	Username, Password string
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Client methods
//___________________________________

// SetHostURL method is to set Host URL in the client instance. It will be used with request
// raised from this client with relative URL
//		// Setting HTTP address
//		client.SetHostURL("http://myjeeva.com")
//
//		// Setting HTTPS address
//		client.SetHostURL("https://myjeeva.com")
//
// Deprecated: use SetBaseURL instead. To be removed in v3.0.0 release.
func (c *Client) SetHostURL(url string) *Client {
	c.SetBaseURL(url)
	return c
}

// SetBaseURL method is to set Base URL in the client instance. It will be used with request
// raised from this client with relative URL
//		// Setting HTTP address
//		client.SetBaseURL("http://myjeeva.com")
//
//		// Setting HTTPS address
//		client.SetBaseURL("https://myjeeva.com")
//
// Since v2.7.0
func (c *Client) SetBaseURL(url string) *Client {
	c.BaseURL = strings.TrimRight(url, "/")
	c.HostURL = c.BaseURL
	return c
}

// SetHeader method sets a single header field and its value in the client instance.
// These headers will be applied to all requests raised from this client instance.
// Also it can be overridden at request level header options.
//
// See `Request.SetHeader` or `Request.SetHeaders`.
//
// For Example: To set `Content-Type` and `Accept` as `application/json`
//
// 		client.
// 			SetHeader("Content-Type", "application/json").
// 			SetHeader("Accept", "application/json")
func (c *Client) SetHeader(header, value string) *Client {
	c.Header.Set(header, value)
	return c
}

// SetHeaders method sets multiple headers field and its values at one go in the client instance.
// These headers will be applied to all requests raised from this client instance. Also it can be
// overridden at request level headers options.
//
// See `Request.SetHeaders` or `Request.SetHeader`.
//
// For Example: To set `Content-Type` and `Accept` as `application/json`
//
// 		client.SetHeaders(map[string]string{
//				"Content-Type": "application/json",
//				"Accept": "application/json",
//			})
func (c *Client) SetHeaders(headers map[string]string) *Client {
	for h, v := range headers {
		c.Header.Set(h, v)
	}
	return c
}

// SetHeaderVerbatim method is to set a single header field and its value verbatim in the current request.
//
// For Example: To set `all_lowercase` and `UPPERCASE` as `available`.
// 		client.R().
//			SetHeaderVerbatim("all_lowercase", "available").
//			SetHeaderVerbatim("UPPERCASE", "available")
//
// Also you can override header value, which was set at client instance level.
//
// Since v2.6.0
func (c *Client) SetHeaderVerbatim(header, value string) *Client {
	c.Header[header] = []string{value}
	return c
}

// SetCookieJar method sets custom http.CookieJar in the resty client. Its way to override default.
//
// For Example: sometimes we don't want to save cookies in api contacting, we can remove the default
// CookieJar in resty client.
//
//		client.SetCookieJar(nil)
func (c *Client) SetCookieJar(jar http.CookieJar) *Client {
	c.httpClient.Jar = jar
	return c
}

// SetCookie method appends a single cookie in the client instance.
// These cookies will be added to all the request raised from this client instance.
// 		client.SetCookie(&http.Cookie{
// 					Name:"go-resty",
//					Value:"This is cookie value",
// 				})
func (c *Client) SetCookie(hc *http.Cookie) *Client {
	c.Cookies = append(c.Cookies, hc)
	return c
}

// SetCookies method sets an array of cookies in the client instance.
// These cookies will be added to all the request raised from this client instance.
// 		cookies := []*http.Cookie{
// 			&http.Cookie{
// 				Name:"go-resty-1",
// 				Value:"This is cookie 1 value",
// 			},
// 			&http.Cookie{
// 				Name:"go-resty-2",
// 				Value:"This is cookie 2 value",
// 			},
// 		}
//
//		// Setting a cookies into resty
// 		client.SetCookies(cookies)
func (c *Client) SetCookies(cs []*http.Cookie) *Client {
	c.Cookies = append(c.Cookies, cs...)
	return c
}

// SetQueryParam method sets single parameter and its value in the client instance.
// It will be formed as query string for the request.
//
// For Example: `search=kitchen%20papers&size=large`
// in the URL after `?` mark. These query params will be added to all the request raised from
// this client instance. Also it can be overridden at request level Query Param options.
//
// See `Request.SetQueryParam` or `Request.SetQueryParams`.
// 		client.
//			SetQueryParam("search", "kitchen papers").
//			SetQueryParam("size", "large")
func (c *Client) SetQueryParam(param, value string) *Client {
	c.QueryParam.Set(param, value)
	return c
}

// SetQueryParams method sets multiple parameters and their values at one go in the client instance.
// It will be formed as query string for the request.
//
// For Example: `search=kitchen%20papers&size=large`
// in the URL after `?` mark. These query params will be added to all the request raised from this
// client instance. Also it can be overridden at request level Query Param options.
//
// See `Request.SetQueryParams` or `Request.SetQueryParam`.
// 		client.SetQueryParams(map[string]string{
//				"search": "kitchen papers",
//				"size": "large",
//			})
func (c *Client) SetQueryParams(params map[string]string) *Client {
	for p, v := range params {
		c.SetQueryParam(p, v)
	}
	return c
}

// SetFormData method sets Form parameters and their values in the client instance.
// It's applicable only HTTP method `POST` and `PUT` and requets content type would be set as
// `application/x-www-form-urlencoded`. These form data will be added to all the request raised from
// this client instance. Also it can be overridden at request level form data.
//
// See `Request.SetFormData`.
// 		client.SetFormData(map[string]string{
//				"access_token": "BC594900-518B-4F7E-AC75-BD37F019E08F",
//				"user_id": "3455454545",
//			})
func (c *Client) SetFormData(data map[string]string) *Client {
	for k, v := range data {
		c.FormData.Set(k, v)
	}
	return c
}

// SetBasicAuth method sets the basic authentication header in the HTTP request. For Example:
//		Authorization: Basic <base64-encoded-value>
//
// For Example: To set the header for username "go-resty" and password "welcome"
// 		client.SetBasicAuth("go-resty", "welcome")
//
// This basic auth information gets added to all the request rasied from this client instance.
// Also it can be overridden or set one at the request level is supported.
//
// See `Request.SetBasicAuth`.
func (c *Client) SetBasicAuth(username, password string) *Client {
	c.UserInfo = &User{Username: username, Password: password}
	return c
}

// SetAuthToken method sets the auth token of the `Authorization` header for all HTTP requests.
// The default auth scheme is `Bearer`, it can be customized with the method `SetAuthScheme`. For Example:
// 		Authorization: <auth-scheme> <auth-token-value>
//
// For Example: To set auth token BC594900518B4F7EAC75BD37F019E08FBC594900518B4F7EAC75BD37F019E08F
//
// 		client.SetAuthToken("BC594900518B4F7EAC75BD37F019E08FBC594900518B4F7EAC75BD37F019E08F")
//
// This auth token gets added to all the requests rasied from this client instance.
// Also it can be overridden or set one at the request level is supported.
//
// See `Request.SetAuthToken`.
func (c *Client) SetAuthToken(token string) *Client {
	c.Token = token
	return c
}

// SetAuthScheme method sets the auth scheme type in the HTTP request. For Example:
//      Authorization: <auth-scheme-value> <auth-token-value>
//
// For Example: To set the scheme to use OAuth
//
// 		client.SetAuthScheme("OAuth")
//
// This auth scheme gets added to all the requests rasied from this client instance.
// Also it can be overridden or set one at the request level is supported.
//
// Information about auth schemes can be found in RFC7235 which is linked to below
// along with the page containing the currently defined official authentication schemes:
//     https://tools.ietf.org/html/rfc7235
//     https://www.iana.org/assignments/http-authschemes/http-authschemes.xhtml#authschemes
//
// See `Request.SetAuthToken`.
func (c *Client) SetAuthScheme(scheme string) *Client {
	c.AuthScheme = scheme
	return c
}

// R method creates a new request instance, its used for Get, Post, Put, Delete, Patch, Head, Options, etc.
func (c *Client) R() *Request {
	r := &Request{
		QueryParam: url.Values{},
		FormData:   url.Values{},
		Header:     http.Header{},
		Cookies:    make([]*http.Cookie, 0),

		client:          c,
		multipartFiles:  []*File{},
		multipartFields: []*MultipartField{},
		PathParams:      map[string]string{},
		jsonEscapeHTML:  true,
	}
	return r
}

// NewRequest is an alias for method `R()`. Creates a new request instance, its used for
// Get, Post, Put, Delete, Patch, Head, Options, etc.
func (c *Client) NewRequest() *Request {
	return c.R()
}

// OnBeforeRequest method appends request middleware into the before request chain.
// Its gets applied after default Resty request middlewares and before request
// been sent from Resty to host server.
// 		client.OnBeforeRequest(func(c *resty.Client, r *resty.Request) error {
//				// Now you have access to Client and Request instance
//				// manipulate it as per your need
//
//				return nil 	// if its success otherwise return error
//			})
func (c *Client) OnBeforeRequest(m RequestMiddleware) *Client {
	c.udBeforeRequest = append(c.udBeforeRequest, m)
	return c
}

// OnAfterResponse method appends response middleware into the after response chain.
// Once we receive response from host server, default Resty response middleware
// gets applied and then user assigened response middlewares applied.
// 		client.OnAfterResponse(func(c *resty.Client, r *resty.Response) error {
//				// Now you have access to Client and Response instance
//				// manipulate it as per your need
//
//				return nil 	// if its success otherwise return error
//			})
func (c *Client) OnAfterResponse(m ResponseMiddleware) *Client {
	c.afterResponse = append(c.afterResponse, m)
	return c
}

// OnError method adds a callback that will be run whenever a request execution fails.
// This is called after all retries have been attempted (if any).
// If there was a response from the server, the error will be wrapped in *ResponseError
// which has the last response received from the server.
//
//		client.OnError(func(req *resty.Request, err error) {
//			if v, ok := err.(*resty.ResponseError); ok {
//				// Do something with v.Response
//			}
//			// Log the error, increment a metric, etc...
//		})
func (c *Client) OnError(h ErrorHook) *Client {
	c.errorHooks = append(c.errorHooks, h)
	return c
}

// SetPreRequestHook method sets the given pre-request function into resty client.
// It is called right before the request is fired.
//
// Note: Only one pre-request hook can be registered. Use `client.OnBeforeRequest` for mutilple.
func (c *Client) SetPreRequestHook(h PreRequestHook) *Client {
	if c.preReqHook != nil {
		c.log.Warnf("Overwriting an existing pre-request hook: %s", functionName(h))
	}
	c.preReqHook = h
	return c
}

// SetDebug method enables the debug mode on Resty client. Client logs details of every request and response.
// For `Request` it logs information such as HTTP verb, Relative URL path, Host, Headers, Body if it has one.
// For `Response` it logs information such as Status, Response Time, Headers, Body if it has one.
//		client.SetDebug(true)
func (c *Client) SetDebug(d bool) *Client {
	c.Debug = d
	return c
}

// SetDebugBodyLimit sets the maximum size for which the response and request body will be logged in debug mode.
//		client.SetDebugBodyLimit(1000000)
func (c *Client) SetDebugBodyLimit(sl int64) *Client {
	c.debugBodySizeLimit = sl
	return c
}

// OnRequestLog method used to set request log callback into Resty. Registered callback gets
// called before the resty actually logs the information.
func (c *Client) OnRequestLog(rl RequestLogCallback) *Client {
	if c.requestLog != nil {
		c.log.Warnf("Overwriting an existing on-request-log callback from=%s to=%s",
			functionName(c.requestLog), functionName(rl))
	}
	c.requestLog = rl
	return c
}

// OnResponseLog method used to set response log callback into Resty. Registered callback gets
// called before the resty actually logs the information.
func (c *Client) OnResponseLog(rl ResponseLogCallback) *Client {
	if c.responseLog != nil {
		c.log.Warnf("Overwriting an existing on-response-log callback from=%s to=%s",
			functionName(c.responseLog), functionName(rl))
	}
	c.responseLog = rl
	return c
}

// SetDisableWarn method disables the warning message on Resty client.
//
// For Example: Resty warns the user when BasicAuth used on non-TLS mode.
//		client.SetDisableWarn(true)
func (c *Client) SetDisableWarn(d bool) *Client {
	c.DisableWarn = d
	return c
}

// SetAllowGetMethodPayload method allows the GET method with payload on Resty client.
//
// For Example: Resty allows the user sends request with a payload on HTTP GET method.
//		client.SetAllowGetMethodPayload(true)
func (c *Client) SetAllowGetMethodPayload(a bool) *Client {
	c.AllowGetMethodPayload = a
	return c
}

// SetLogger method sets given writer for logging Resty request and response details.
//
// Compliant to interface `resty.Logger`.
func (c *Client) SetLogger(l Logger) *Client {
	c.log = l
	return c
}

// SetContentLength method enables the HTTP header `Content-Length` value for every request.
// By default Resty won't set `Content-Length`.
// 		client.SetContentLength(true)
//
// Also you have an option to enable for particular request. See `Request.SetContentLength`
func (c *Client) SetContentLength(l bool) *Client {
	c.setContentLength = l
	return c
}

// SetTimeout method sets timeout for request raised from client.
//		client.SetTimeout(time.Duration(1 * time.Minute))
func (c *Client) SetTimeout(timeout time.Duration) *Client {
	c.httpClient.Timeout = timeout
	return c
}

// SetError method is to register the global or client common `Error` object into Resty.
// It is used for automatic unmarshalling if response status code is greater than 399 and
// content type either JSON or XML. Can be pointer or non-pointer.
// 		client.SetError(&Error{})
//		// OR
//		client.SetError(Error{})
func (c *Client) SetError(err interface{}) *Client {
	c.Error = typeOf(err)
	return c
}

// SetRedirectPolicy method sets the client redirect poilicy. Resty provides ready to use
// redirect policies. Wanna create one for yourself refer to `redirect.go`.
//
//		client.SetRedirectPolicy(FlexibleRedirectPolicy(20))
//
// 		// Need multiple redirect policies together
//		client.SetRedirectPolicy(FlexibleRedirectPolicy(20), DomainCheckRedirectPolicy("host1.com", "host2.net"))
func (c *Client) SetRedirectPolicy(policies ...interface{}) *Client {
	for _, p := range policies {
		if _, ok := p.(RedirectPolicy); !ok {
			c.log.Errorf("%v does not implement resty.RedirectPolicy (missing Apply method)",
				functionName(p))
		}
	}

	c.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		for _, p := range policies {
			if err := p.(RedirectPolicy).Apply(req, via); err != nil {
				return err
			}
		}
		return nil // looks good, go ahead
	}

	return c
}

// SetRetryCount method enables retry on Resty client and allows you
// to set no. of retry count. Resty uses a Backoff mechanism.
func (c *Client) SetRetryCount(count int) *Client {
	c.RetryCount = count
	return c
}

// SetRetryWaitTime method sets default wait time to sleep before retrying
// request.
//
// Default is 100 milliseconds.
func (c *Client) SetRetryWaitTime(waitTime time.Duration) *Client {
	c.RetryWaitTime = waitTime
	return c
}

// SetRetryMaxWaitTime method sets max wait time to sleep before retrying
// request.
//
// Default is 2 seconds.
func (c *Client) SetRetryMaxWaitTime(maxWaitTime time.Duration) *Client {
	c.RetryMaxWaitTime = maxWaitTime
	return c
}

// SetRetryAfter sets callback to calculate wait time between retries.
// Default (nil) implies exponential backoff with jitter
func (c *Client) SetRetryAfter(callback RetryAfterFunc) *Client {
	c.RetryAfter = callback
	return c
}

// AddRetryCondition method adds a retry condition function to array of functions
// that are checked to determine if the request is retried. The request will
// retry if any of the functions return true and error is nil.
//
// Note: These retry conditions are applied on all Request made using this Client.
// For Request specific retry conditions check *Request.AddRetryCondition
func (c *Client) AddRetryCondition(condition RetryConditionFunc) *Client {
	c.RetryConditions = append(c.RetryConditions, condition)
	return c
}

// AddRetryAfterErrorCondition adds the basic condition of retrying after encountering
// an error from the http response
//
// Since v2.6.0
func (c *Client) AddRetryAfterErrorCondition() *Client {
	c.AddRetryCondition(func(response *Response, err error) bool {
		return response.IsError()
	})
	return c
}

// AddRetryHook adds a side-effecting retry hook to an array of hooks
// that will be executed on each retry.
//
// Since v2.6.0
func (c *Client) AddRetryHook(hook OnRetryFunc) *Client {
	c.RetryHooks = append(c.RetryHooks, hook)
	return c
}

// SetTLSClientConfig method sets TLSClientConfig for underling client Transport.
//
// For Example:
// 		// One can set custom root-certificate. Refer: http://golang.org/pkg/crypto/tls/#example_Dial
//		client.SetTLSClientConfig(&tls.Config{ RootCAs: roots })
//
// 		// or One can disable security check (https)
//		client.SetTLSClientConfig(&tls.Config{ InsecureSkipVerify: true })
//
// Note: This method overwrites existing `TLSClientConfig`.
func (c *Client) SetTLSClientConfig(config *tls.Config) *Client {
	transport, err := c.transport()
	if err != nil {
		c.log.Errorf("%v", err)
		return c
	}
	transport.TLSClientConfig = config
	return c
}

// SetProxy method sets the Proxy URL and Port for Resty client.
//		client.SetProxy("http://proxyserver:8888")
//
// OR Without this `SetProxy` method, you could also set Proxy via environment variable.
//
// Refer to godoc `http.ProxyFromEnvironment`.
func (c *Client) SetProxy(proxyURL string) *Client {
	transport, err := c.transport()
	if err != nil {
		c.log.Errorf("%v", err)
		return c
	}

	pURL, err := url.Parse(proxyURL)
	if err != nil {
		c.log.Errorf("%v", err)
		return c
	}

	c.proxyURL = pURL
	transport.Proxy = http.ProxyURL(c.proxyURL)
	return c
}

// RemoveProxy method removes the proxy configuration from Resty client
//		client.RemoveProxy()
func (c *Client) RemoveProxy() *Client {
	transport, err := c.transport()
	if err != nil {
		c.log.Errorf("%v", err)
		return c
	}
	c.proxyURL = nil
	transport.Proxy = nil
	return c
}

// SetCertificates method helps to set client certificates into Resty conveniently.
func (c *Client) SetCertificates(certs ...tls.Certificate) *Client {
	config, err := c.tlsConfig()
	if err != nil {
		c.log.Errorf("%v", err)
		return c
	}
	config.Certificates = append(config.Certificates, certs...)
	return c
}

// SetRootCertificate method helps to add one or more root certificates into Resty client
// 		client.SetRootCertificate("/path/to/root/pemFile.pem")
func (c *Client) SetRootCertificate(pemFilePath string) *Client {
	rootPemData, err := ioutil.ReadFile(pemFilePath)
	if err != nil {
		c.log.Errorf("%v", err)
		return c
	}

	config, err := c.tlsConfig()
	if err != nil {
		c.log.Errorf("%v", err)
		return c
	}
	if config.RootCAs == nil {
		config.RootCAs = x509.NewCertPool()
	}

	config.RootCAs.AppendCertsFromPEM(rootPemData)
	return c
}

// SetRootCertificateFromString method helps to add one or more root certificates into Resty client
// 		client.SetRootCertificateFromString("pem file content")
func (c *Client) SetRootCertificateFromString(pemContent string) *Client {
	config, err := c.tlsConfig()
	if err != nil {
		c.log.Errorf("%v", err)
		return c
	}
	if config.RootCAs == nil {
		config.RootCAs = x509.NewCertPool()
	}

	config.RootCAs.AppendCertsFromPEM([]byte(pemContent))
	return c
}

// SetOutputDirectory method sets output directory for saving HTTP response into file.
// If the output directory not exists then resty creates one. This setting is optional one,
// if you're planning using absolute path in `Request.SetOutput` and can used together.
// 		client.SetOutputDirectory("/save/http/response/here")
func (c *Client) SetOutputDirectory(dirPath string) *Client {
	c.outputDirectory = dirPath
	return c
}

// SetTransport method sets custom `*http.Transport` or any `http.RoundTripper`
// compatible interface implementation in the resty client.
//
// Note:
//
// - If transport is not type of `*http.Transport` then you may not be able to
// take advantage of some of the Resty client settings.
//
// - It overwrites the Resty client transport instance and it's configurations.
//
//		transport := &http.Transport{
//			// somthing like Proxying to httptest.Server, etc...
//			Proxy: func(req *http.Request) (*url.URL, error) {
//				return url.Parse(server.URL)
//			},
//		}
//
//		client.SetTransport(transport)
func (c *Client) SetTransport(transport http.RoundTripper) *Client {
	if transport != nil {
		c.httpClient.Transport = transport
	}
	return c
}

// SetScheme method sets custom scheme in the Resty client. It's way to override default.
// 		client.SetScheme("http")
func (c *Client) SetScheme(scheme string) *Client {
	if !IsStringEmpty(scheme) {
		c.scheme = strings.TrimSpace(scheme)
	}
	return c
}

// SetCloseConnection method sets variable `Close` in http request struct with the given
// value. More info: https://golang.org/src/net/http/request.go
func (c *Client) SetCloseConnection(close bool) *Client {
	c.closeConnection = close
	return c
}

// SetDoNotParseResponse method instructs `Resty` not to parse the response body automatically.
// Resty exposes the raw response body as `io.ReadCloser`. Also do not forget to close the body,
// otherwise you might get into connection leaks, no connection reuse.
//
// Note: Response middlewares are not applicable, if you use this option. Basically you have
// taken over the control of response parsing from `Resty`.
func (c *Client) SetDoNotParseResponse(parse bool) *Client {
	c.notParseResponse = parse
	return c
}

// SetPathParam method sets single URL path key-value pair in the
// Resty client instance.
// 		client.SetPathParam("userId", "sample@sample.com")
//
// 		Result:
// 		   URL - /v1/users/{userId}/details
// 		   Composed URL - /v1/users/sample@sample.com/details
// It replaces the value of the key while composing the request URL.
//
// Also it can be overridden at request level Path Params options,
// see `Request.SetPathParam` or `Request.SetPathParams`.
func (c *Client) SetPathParam(param, value string) *Client {
	c.PathParams[param] = value
	return c
}

// SetPathParams method sets multiple URL path key-value pairs at one go in the
// Resty client instance.
// 		client.SetPathParams(map[string]string{
// 		   "userId": "sample@sample.com",
// 		   "subAccountId": "100002",
// 		})
//
// 		Result:
// 		   URL - /v1/users/{userId}/{subAccountId}/details
// 		   Composed URL - /v1/users/sample@sample.com/100002/details
// It replaces the value of the key while composing the request URL.
//
// Also it can be overridden at request level Path Params options,
// see `Request.SetPathParam` or `Request.SetPathParams`.
func (c *Client) SetPathParams(params map[string]string) *Client {
	for p, v := range params {
		c.SetPathParam(p, v)
	}
	return c
}

// SetJSONEscapeHTML method is to enable/disable the HTML escape on JSON marshal.
//
// Note: This option only applicable to standard JSON Marshaller.
func (c *Client) SetJSONEscapeHTML(b bool) *Client {
	c.jsonEscapeHTML = b
	return c
}

// EnableTrace method enables the Resty client trace for the requests fired from
// the client using `httptrace.ClientTrace` and provides insights.
//
// 		client := resty.New().EnableTrace()
//
// 		resp, err := client.R().Get("https://httpbin.org/get")
// 		fmt.Println("Error:", err)
// 		fmt.Println("Trace Info:", resp.Request.TraceInfo())
//
// Also `Request.EnableTrace` available too to get trace info for single request.
//
// Since v2.0.0
func (c *Client) EnableTrace() *Client {
	c.trace = true
	return c
}

// DisableTrace method disables the Resty client trace. Refer to `Client.EnableTrace`.
//
// Since v2.0.0
func (c *Client) DisableTrace() *Client {
	c.trace = false
	return c
}

// IsProxySet method returns the true is proxy is set from resty client otherwise
// false. By default proxy is set from environment, refer to `http.ProxyFromEnvironment`.
func (c *Client) IsProxySet() bool {
	return c.proxyURL != nil
}

// GetClient method returns the current `http.Client` used by the resty client.
func (c *Client) GetClient() *http.Client {
	return c.httpClient
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Client Unexported methods
//_______________________________________________________________________

// Executes method executes the given `Request` object and returns response
// error.
func (c *Client) execute(req *Request) (*Response, error) {
	// Apply Request middleware
	var err error

	// user defined on before request methods
	// to modify the *resty.Request object
	for _, f := range c.udBeforeRequest {
		if err = f(c, req); err != nil {
			return nil, wrapNoRetryErr(err)
		}
	}

	// resty middlewares
	for _, f := range c.beforeRequest {
		if err = f(c, req); err != nil {
			return nil, wrapNoRetryErr(err)
		}
	}

	if hostHeader := req.Header.Get("Host"); hostHeader != "" {
		req.RawRequest.Host = hostHeader
	}

	// call pre-request if defined
	if c.preReqHook != nil {
		if err = c.preReqHook(c, req.RawRequest); err != nil {
			return nil, wrapNoRetryErr(err)
		}
	}

	if err = requestLogger(c, req); err != nil {
		return nil, wrapNoRetryErr(err)
	}

	req.RawRequest.Body = newRequestBodyReleaser(req.RawRequest.Body, req.bodyBuf)

	req.Time = time.Now()
	resp, err := c.httpClient.Do(req.RawRequest)

	response := &Response{
		Request:     req,
		RawResponse: resp,
	}

	if err != nil || req.notParseResponse || c.notParseResponse {
		response.setReceivedAt()
		return response, err
	}

	if !req.isSaveResponse {
		defer closeq(resp.Body)
		body := resp.Body

		// GitHub #142 & #187
		if strings.EqualFold(resp.Header.Get(hdrContentEncodingKey), "gzip") && resp.ContentLength != 0 {
			if _, ok := body.(*gzip.Reader); !ok {
				body, err = gzip.NewReader(body)
				if err != nil {
					response.setReceivedAt()
					return response, err
				}
				defer closeq(body)
			}
		}

		if response.body, err = ioutil.ReadAll(body); err != nil {
			response.setReceivedAt()
			return response, err
		}

		response.size = int64(len(response.body))
	}

	response.setReceivedAt() // after we read the body

	// Apply Response middleware
	for _, f := range c.afterResponse {
		if err = f(c, response); err != nil {
			break
		}
	}

	return response, wrapNoRetryErr(err)
}

// getting TLS client config if not exists then create one
func (c *Client) tlsConfig() (*tls.Config, error) {
	transport, err := c.transport()
	if err != nil {
		return nil, err
	}
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	}
	return transport.TLSClientConfig, nil
}

// Transport method returns `*http.Transport` currently in use or error
// in case currently used `transport` is not a `*http.Transport`.
func (c *Client) transport() (*http.Transport, error) {
	if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
		return transport, nil
	}
	return nil, errors.New("current transport is not an *http.Transport instance")
}

// just an internal helper method
func (c *Client) outputLogTo(w io.Writer) *Client {
	c.log.(*logger).l.SetOutput(w)
	return c
}

// ResponseError is a wrapper for including the server response with an error.
// Neither the err nor the response should be nil.
type ResponseError struct {
	Response *Response
	Err      error
}

func (e *ResponseError) Error() string {
	return e.Err.Error()
}

func (e *ResponseError) Unwrap() error {
	return e.Err
}

// Helper to run onErrorHooks hooks.
// It wraps the error in a ResponseError if the resp is not nil
// so hooks can access it.
func (c *Client) onErrorHooks(req *Request, resp *Response, err error) {
	if err != nil {
		if resp != nil { // wrap with ResponseError
			err = &ResponseError{Response: resp, Err: err}
		}
		for _, h := range c.errorHooks {
			h(req, err)
		}
	}
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// File struct and its methods
//_______________________________________________________________________

// File struct represent file information for multipart request
type File struct {
	Name      string
	ParamName string
	io.Reader
}

// String returns string value of current file details
func (f *File) String() string {
	return fmt.Sprintf("ParamName: %v; FileName: %v", f.ParamName, f.Name)
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// MultipartField struct
//_______________________________________________________________________

// MultipartField struct represent custom data part for multipart request
type MultipartField struct {
	Param       string
	FileName    string
	ContentType string
	io.Reader
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Unexported package methods
//_______________________________________________________________________

func createClient(hc *http.Client) *Client {
	if hc.Transport == nil {
		hc.Transport = createTransport(nil)
	}

	c := &Client{ // not setting lang default values
		QueryParam:             url.Values{},
		FormData:               url.Values{},
		Header:                 http.Header{},
		Cookies:                make([]*http.Cookie, 0),
		RetryWaitTime:          defaultWaitTime,
		RetryMaxWaitTime:       defaultMaxWaitTime,
		PathParams:             make(map[string]string),
		JSONMarshal:            json.Marshal,
		JSONUnmarshal:          json.Unmarshal,
		XMLMarshal:             xml.Marshal,
		XMLUnmarshal:           xml.Unmarshal,
		HeaderAuthorizationKey: http.CanonicalHeaderKey("Authorization"),

		jsonEscapeHTML:     true,
		httpClient:         hc,
		debugBodySizeLimit: math.MaxInt32,
	}

	// Logger
	c.SetLogger(createLogger())

	// default before request middlewares
	c.beforeRequest = []RequestMiddleware{
		parseRequestURL,
		parseRequestHeader,
		parseRequestBody,
		createHTTPRequest,
		addCredentials,
	}

	// user defined request middlewares
	c.udBeforeRequest = []RequestMiddleware{}

	// default after response middlewares
	c.afterResponse = []ResponseMiddleware{
		responseLogger,
		parseResponseBody,
		saveResponseIntoFile,
	}

	return c
}
