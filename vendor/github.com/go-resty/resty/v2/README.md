<p align="center">
<h1 align="center">Resty</h1>
<p align="center">Simple HTTP and REST client library for Go (inspired by Ruby rest-client)</p>
<p align="center"><a href="#features">Features</a> section describes in detail about Resty capabilities</p>
</p>
<p align="center">
<p align="center"><a href="https://github.com/go-resty/resty/actions/workflows/ci.yml?query=branch%3Amaster"><img src="https://github.com/go-resty/resty/actions/workflows/ci.yml/badge.svg" alt="Build Status"></a> <a href="https://codecov.io/gh/go-resty/resty/branch/master"><img src="https://codecov.io/gh/go-resty/resty/branch/master/graph/badge.svg" alt="Code Coverage"></a> <a href="https://goreportcard.com/report/go-resty/resty"><img src="https://goreportcard.com/badge/go-resty/resty" alt="Go Report Card"></a> <a href="https://github.com/go-resty/resty/releases/latest"><img src="https://img.shields.io/badge/version-2.7.0-blue.svg" alt="Release Version"></a> <a href="https://pkg.go.dev/github.com/go-resty/resty/v2"><img src="https://pkg.go.dev/badge/github.com/go-resty/resty" alt="GoDoc"></a> <a href="LICENSE"><img src="https://img.shields.io/github/license/go-resty/resty.svg" alt="License"></a> <a href="https://github.com/avelino/awesome-go"><img src="https://awesome.re/mentioned-badge.svg" alt="Mentioned in Awesome Go"></a></p>
</p>
<p align="center">
<h4 align="center">Resty Communication Channels</h4>
<p align="center"><a href="https://gitter.im/go_resty/community?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge"><img src="https://badges.gitter.im/go_resty/community.svg" alt="Chat on Gitter - Resty Community"></a> <a href="https://twitter.com/go_resty"><img src="https://img.shields.io/badge/twitter-@go__resty-55acee.svg" alt="Twitter @go_resty"></a></p>
</p>

## News

  * v2.7.0 [released](https://github.com/go-resty/resty/releases/tag/v2.7.0) and tagged on Nov 03, 2021.
  * v2.0.0 [released](https://github.com/go-resty/resty/releases/tag/v2.0.0) and tagged on Jul 16, 2019.
  * v1.12.0 [released](https://github.com/go-resty/resty/releases/tag/v1.12.0) and tagged on Feb 27, 2019.
  * v1.0 released and tagged on Sep 25, 2017. - Resty's first version was released on Sep 15, 2015 then it grew gradually as a very handy and helpful library. Its been a two years since first release. I'm very thankful to Resty users and its [contributors](https://github.com/go-resty/resty/graphs/contributors).

## Features

  * GET, POST, PUT, DELETE, HEAD, PATCH, OPTIONS, etc.
  * Simple and chainable methods for settings and request
  * [Request](https://pkg.go.dev/github.com/go-resty/resty/v2#Request) Body can be `string`, `[]byte`, `struct`, `map`, `slice` and `io.Reader` too
    * Auto detects `Content-Type`
    * Buffer less processing for `io.Reader`
    * Native `*http.Request` instance may be accessed during middleware and request execution via `Request.RawRequest`
    * Request Body can be read multiple times via `Request.RawRequest.GetBody()`
  * [Response](https://pkg.go.dev/github.com/go-resty/resty/v2#Response) object gives you more possibility
    * Access as `[]byte` array - `response.Body()` OR Access as `string` - `response.String()`
    * Know your `response.Time()` and when we `response.ReceivedAt()`
  * Automatic marshal and unmarshal for `JSON` and `XML` content type
    * Default is `JSON`, if you supply `struct/map` without header `Content-Type`
    * For auto-unmarshal, refer to -
        - Success scenario [Request.SetResult()](https://pkg.go.dev/github.com/go-resty/resty/v2#Request.SetResult) and [Response.Result()](https://pkg.go.dev/github.com/go-resty/resty/v2#Response.Result).
        - Error scenario [Request.SetError()](https://pkg.go.dev/github.com/go-resty/resty/v2#Request.SetError) and [Response.Error()](https://pkg.go.dev/github.com/go-resty/resty/v2#Response.Error).
        - Supports [RFC7807](https://tools.ietf.org/html/rfc7807) - `application/problem+json` & `application/problem+xml`
    * Resty provides an option to override [JSON Marshal/Unmarshal and XML Marshal/Unmarshal](#override-json--xml-marshalunmarshal)
  * Easy to upload one or more file(s) via `multipart/form-data`
    * Auto detects file content type
  * Request URL [Path Params (aka URI Params)](https://pkg.go.dev/github.com/go-resty/resty/v2#Request.SetPathParams)
  * Backoff Retry Mechanism with retry condition function [reference](retry_test.go)
  * Resty client HTTP & REST [Request](https://pkg.go.dev/github.com/go-resty/resty/v2#Client.OnBeforeRequest) and [Response](https://pkg.go.dev/github.com/go-resty/resty/v2#Client.OnAfterResponse) middlewares
  * `Request.SetContext` supported
  * Authorization option of `BasicAuth` and `Bearer` token
  * Set request `ContentLength` value for all request or particular request
  * Custom [Root Certificates](https://pkg.go.dev/github.com/go-resty/resty/v2#Client.SetRootCertificate) and Client [Certificates](https://pkg.go.dev/github.com/go-resty/resty/v2#Client.SetCertificates)
  * Download/Save HTTP response directly into File, like `curl -o` flag. See [SetOutputDirectory](https://pkg.go.dev/github.com/go-resty/resty/v2#Client.SetOutputDirectory) & [SetOutput](https://pkg.go.dev/github.com/go-resty/resty/v2#Request.SetOutput).
  * Cookies for your request and CookieJar support
  * SRV Record based request instead of Host URL
  * Client settings like `Timeout`, `RedirectPolicy`, `Proxy`, `TLSClientConfig`, `Transport`, etc.
  * Optionally allows GET request with payload, see [SetAllowGetMethodPayload](https://pkg.go.dev/github.com/go-resty/resty/v2#Client.SetAllowGetMethodPayload)
  * Supports registering external JSON library into resty, see [how to use](https://github.com/go-resty/resty/issues/76#issuecomment-314015250)
  * Exposes Response reader without reading response (no auto-unmarshaling) if need be, see [how to use](https://github.com/go-resty/resty/issues/87#issuecomment-322100604)
  * Option to specify expected `Content-Type` when response `Content-Type` header missing. Refer to [#92](https://github.com/go-resty/resty/issues/92)
  * Resty design
    * Have client level settings & options and also override at Request level if you want to
    * Request and Response middleware
    * Create Multiple clients if you want to `resty.New()`
    * Supports `http.RoundTripper` implementation, see [SetTransport](https://pkg.go.dev/github.com/go-resty/resty/v2#Client.SetTransport)
    * goroutine concurrent safe
    * Resty Client trace, see [Client.EnableTrace](https://pkg.go.dev/github.com/go-resty/resty/v2#Client.EnableTrace) and [Request.EnableTrace](https://pkg.go.dev/github.com/go-resty/resty/v2#Request.EnableTrace)
      * Since v2.4.0, trace info contains a `RequestAttempt` value, and the `Request` object contains an `Attempt` attribute
    * Debug mode - clean and informative logging presentation
    * Gzip - Go does it automatically also resty has fallback handling too
    * Works fine with `HTTP/2` and `HTTP/1.1`
  * [Bazel support](#bazel-support)
  * Easily mock Resty for testing, [for e.g.](#mocking-http-requests-using-httpmock-library)
  * Well tested client library

### Included Batteries

  * Redirect Policies - see [how to use](#redirect-policy)
    * NoRedirectPolicy
    * FlexibleRedirectPolicy
    * DomainCheckRedirectPolicy
    * etc. [more info](redirect.go)
  * Retry Mechanism [how to use](#retries)
    * Backoff Retry
    * Conditional Retry
    * Since v2.6.0, Retry Hooks - [Client](https://pkg.go.dev/github.com/go-resty/resty/v2#Client.AddRetryHook), [Request](https://pkg.go.dev/github.com/go-resty/resty/v2#Request.AddRetryHook)
  * SRV Record based request instead of Host URL [how to use](resty_test.go#L1412)
  * etc (upcoming - throw your idea's [here](https://github.com/go-resty/resty/issues)).


#### Supported Go Versions

Initially Resty started supporting `go modules` since `v1.10.0` release.

Starting Resty v2 and higher versions, it fully embraces [go modules](https://github.com/golang/go/wiki/Modules) package release. It requires a Go version capable of understanding `/vN` suffixed imports:

- 1.9.7+
- 1.10.3+
- 1.11+


## It might be beneficial for your project :smile:

Resty author also published following projects for Go Community.

  * [aah framework](https://aahframework.org) - A secure, flexible, rapid Go web framework.
  * [THUMBAI](https://thumbai.app) - Go Mod Repository, Go Vanity Service and Simple Proxy Server.
  * [go-model](https://github.com/jeevatkm/go-model) - Robust & Easy to use model mapper and utility methods for Go `struct`.


## Installation

```bash
# Go Modules
require github.com/go-resty/resty/v2 v2.7.0
```

## Usage

The following samples will assist you to become as comfortable as possible with resty library.

```go
// Import resty into your code and refer it as `resty`.
import "github.com/go-resty/resty/v2"
```

#### Simple GET

```go
// Create a Resty Client
client := resty.New()

resp, err := client.R().
    EnableTrace().
    Get("https://httpbin.org/get")

// Explore response object
fmt.Println("Response Info:")
fmt.Println("  Error      :", err)
fmt.Println("  Status Code:", resp.StatusCode())
fmt.Println("  Status     :", resp.Status())
fmt.Println("  Proto      :", resp.Proto())
fmt.Println("  Time       :", resp.Time())
fmt.Println("  Received At:", resp.ReceivedAt())
fmt.Println("  Body       :\n", resp)
fmt.Println()

// Explore trace info
fmt.Println("Request Trace Info:")
ti := resp.Request.TraceInfo()
fmt.Println("  DNSLookup     :", ti.DNSLookup)
fmt.Println("  ConnTime      :", ti.ConnTime)
fmt.Println("  TCPConnTime   :", ti.TCPConnTime)
fmt.Println("  TLSHandshake  :", ti.TLSHandshake)
fmt.Println("  ServerTime    :", ti.ServerTime)
fmt.Println("  ResponseTime  :", ti.ResponseTime)
fmt.Println("  TotalTime     :", ti.TotalTime)
fmt.Println("  IsConnReused  :", ti.IsConnReused)
fmt.Println("  IsConnWasIdle :", ti.IsConnWasIdle)
fmt.Println("  ConnIdleTime  :", ti.ConnIdleTime)
fmt.Println("  RequestAttempt:", ti.RequestAttempt)
fmt.Println("  RemoteAddr    :", ti.RemoteAddr.String())

/* Output
Response Info:
  Error      : <nil>
  Status Code: 200
  Status     : 200 OK
  Proto      : HTTP/2.0
  Time       : 457.034718ms
  Received At: 2020-09-14 15:35:29.784681 -0700 PDT m=+0.458137045
  Body       :
  {
    "args": {},
    "headers": {
      "Accept-Encoding": "gzip",
      "Host": "httpbin.org",
      "User-Agent": "go-resty/2.4.0 (https://github.com/go-resty/resty)",
      "X-Amzn-Trace-Id": "Root=1-5f5ff031-000ff6292204aa6898e4de49"
    },
    "origin": "0.0.0.0",
    "url": "https://httpbin.org/get"
  }

Request Trace Info:
  DNSLookup     : 4.074657ms
  ConnTime      : 381.709936ms
  TCPConnTime   : 77.428048ms
  TLSHandshake  : 299.623597ms
  ServerTime    : 75.414703ms
  ResponseTime  : 79.337µs
  TotalTime     : 457.034718ms
  IsConnReused  : false
  IsConnWasIdle : false
  ConnIdleTime  : 0s
  RequestAttempt: 1
  RemoteAddr    : 3.221.81.55:443
*/
```

#### Enhanced GET

```go
// Create a Resty Client
client := resty.New()

resp, err := client.R().
      SetQueryParams(map[string]string{
          "page_no": "1",
          "limit": "20",
          "sort":"name",
          "order": "asc",
          "random":strconv.FormatInt(time.Now().Unix(), 10),
      }).
      SetHeader("Accept", "application/json").
      SetAuthToken("BC594900518B4F7EAC75BD37F019E08FBC594900518B4F7EAC75BD37F019E08F").
      Get("/search_result")


// Sample of using Request.SetQueryString method
resp, err := client.R().
      SetQueryString("productId=232&template=fresh-sample&cat=resty&source=google&kw=buy a lot more").
      SetHeader("Accept", "application/json").
      SetAuthToken("BC594900518B4F7EAC75BD37F019E08FBC594900518B4F7EAC75BD37F019E08F").
      Get("/show_product")


// If necessary, you can force response content type to tell Resty to parse a JSON response into your struct
resp, err := client.R().
      SetResult(result).
      ForceContentType("application/json").
      Get("v2/alpine/manifests/latest")
```

#### Various POST method combinations

```go
// Create a Resty Client
client := resty.New()

// POST JSON string
// No need to set content type, if you have client level setting
resp, err := client.R().
      SetHeader("Content-Type", "application/json").
      SetBody(`{"username":"testuser", "password":"testpass"}`).
      SetResult(&AuthSuccess{}).    // or SetResult(AuthSuccess{}).
      Post("https://myapp.com/login")

// POST []byte array
// No need to set content type, if you have client level setting
resp, err := client.R().
      SetHeader("Content-Type", "application/json").
      SetBody([]byte(`{"username":"testuser", "password":"testpass"}`)).
      SetResult(&AuthSuccess{}).    // or SetResult(AuthSuccess{}).
      Post("https://myapp.com/login")

// POST Struct, default is JSON content type. No need to set one
resp, err := client.R().
      SetBody(User{Username: "testuser", Password: "testpass"}).
      SetResult(&AuthSuccess{}).    // or SetResult(AuthSuccess{}).
      SetError(&AuthError{}).       // or SetError(AuthError{}).
      Post("https://myapp.com/login")

// POST Map, default is JSON content type. No need to set one
resp, err := client.R().
      SetBody(map[string]interface{}{"username": "testuser", "password": "testpass"}).
      SetResult(&AuthSuccess{}).    // or SetResult(AuthSuccess{}).
      SetError(&AuthError{}).       // or SetError(AuthError{}).
      Post("https://myapp.com/login")

// POST of raw bytes for file upload. For example: upload file to Dropbox
fileBytes, _ := ioutil.ReadFile("/Users/jeeva/mydocument.pdf")

// See we are not setting content-type header, since go-resty automatically detects Content-Type for you
resp, err := client.R().
      SetBody(fileBytes).
      SetContentLength(true).          // Dropbox expects this value
      SetAuthToken("<your-auth-token>").
      SetError(&DropboxError{}).       // or SetError(DropboxError{}).
      Post("https://content.dropboxapi.com/1/files_put/auto/resty/mydocument.pdf") // for upload Dropbox supports PUT too

// Note: resty detects Content-Type for request body/payload if content type header is not set.
//   * For struct and map data type defaults to 'application/json'
//   * Fallback is plain text content type
```

#### Sample PUT

You can use various combinations of `PUT` method call like demonstrated for `POST`.

```go
// Note: This is one sample of PUT method usage, refer POST for more combination

// Create a Resty Client
client := resty.New()

// Request goes as JSON content type
// No need to set auth token, error, if you have client level settings
resp, err := client.R().
      SetBody(Article{
        Title: "go-resty",
        Content: "This is my article content, oh ya!",
        Author: "Jeevanandam M",
        Tags: []string{"article", "sample", "resty"},
      }).
      SetAuthToken("C6A79608-782F-4ED0-A11D-BD82FAD829CD").
      SetError(&Error{}).       // or SetError(Error{}).
      Put("https://myapp.com/article/1234")
```

#### Sample PATCH

You can use various combinations of `PATCH` method call like demonstrated for `POST`.

```go
// Note: This is one sample of PUT method usage, refer POST for more combination

// Create a Resty Client
client := resty.New()

// Request goes as JSON content type
// No need to set auth token, error, if you have client level settings
resp, err := client.R().
      SetBody(Article{
        Tags: []string{"new tag1", "new tag2"},
      }).
      SetAuthToken("C6A79608-782F-4ED0-A11D-BD82FAD829CD").
      SetError(&Error{}).       // or SetError(Error{}).
      Patch("https://myapp.com/articles/1234")
```

#### Sample DELETE, HEAD, OPTIONS

```go
// Create a Resty Client
client := resty.New()

// DELETE a article
// No need to set auth token, error, if you have client level settings
resp, err := client.R().
      SetAuthToken("C6A79608-782F-4ED0-A11D-BD82FAD829CD").
      SetError(&Error{}).       // or SetError(Error{}).
      Delete("https://myapp.com/articles/1234")

// DELETE a articles with payload/body as a JSON string
// No need to set auth token, error, if you have client level settings
resp, err := client.R().
      SetAuthToken("C6A79608-782F-4ED0-A11D-BD82FAD829CD").
      SetError(&Error{}).       // or SetError(Error{}).
      SetHeader("Content-Type", "application/json").
      SetBody(`{article_ids: [1002, 1006, 1007, 87683, 45432] }`).
      Delete("https://myapp.com/articles")

// HEAD of resource
// No need to set auth token, if you have client level settings
resp, err := client.R().
      SetAuthToken("C6A79608-782F-4ED0-A11D-BD82FAD829CD").
      Head("https://myapp.com/videos/hi-res-video")

// OPTIONS of resource
// No need to set auth token, if you have client level settings
resp, err := client.R().
      SetAuthToken("C6A79608-782F-4ED0-A11D-BD82FAD829CD").
      Options("https://myapp.com/servers/nyc-dc-01")
```

#### Override JSON & XML Marshal/Unmarshal

User could register choice of JSON/XML library into resty or write your own. By default resty registers standard `encoding/json` and `encoding/xml` respectively.
```go
// Example of registering json-iterator
import jsoniter "github.com/json-iterator/go"

json := jsoniter.ConfigCompatibleWithStandardLibrary

client := resty.New()
client.JSONMarshal = json.Marshal
client.JSONUnmarshal = json.Unmarshal

// similarly user could do for XML too with -
client.XMLMarshal
client.XMLUnmarshal
```

### Multipart File(s) upload

#### Using io.Reader

```go
profileImgBytes, _ := ioutil.ReadFile("/Users/jeeva/test-img.png")
notesBytes, _ := ioutil.ReadFile("/Users/jeeva/text-file.txt")

// Create a Resty Client
client := resty.New()

resp, err := client.R().
      SetFileReader("profile_img", "test-img.png", bytes.NewReader(profileImgBytes)).
      SetFileReader("notes", "text-file.txt", bytes.NewReader(notesBytes)).
      SetFormData(map[string]string{
          "first_name": "Jeevanandam",
          "last_name": "M",
      }).
      Post("http://myapp.com/upload")
```

#### Using File directly from Path

```go
// Create a Resty Client
client := resty.New()

// Single file scenario
resp, err := client.R().
      SetFile("profile_img", "/Users/jeeva/test-img.png").
      Post("http://myapp.com/upload")

// Multiple files scenario
resp, err := client.R().
      SetFiles(map[string]string{
        "profile_img": "/Users/jeeva/test-img.png",
        "notes": "/Users/jeeva/text-file.txt",
      }).
      Post("http://myapp.com/upload")

// Multipart of form fields and files
resp, err := client.R().
      SetFiles(map[string]string{
        "profile_img": "/Users/jeeva/test-img.png",
        "notes": "/Users/jeeva/text-file.txt",
      }).
      SetFormData(map[string]string{
        "first_name": "Jeevanandam",
        "last_name": "M",
        "zip_code": "00001",
        "city": "my city",
        "access_token": "C6A79608-782F-4ED0-A11D-BD82FAD829CD",
      }).
      Post("http://myapp.com/profile")
```

#### Sample Form submission

```go
// Create a Resty Client
client := resty.New()

// just mentioning about POST as an example with simple flow
// User Login
resp, err := client.R().
      SetFormData(map[string]string{
        "username": "jeeva",
        "password": "mypass",
      }).
      Post("http://myapp.com/login")

// Followed by profile update
resp, err := client.R().
      SetFormData(map[string]string{
        "first_name": "Jeevanandam",
        "last_name": "M",
        "zip_code": "00001",
        "city": "new city update",
      }).
      Post("http://myapp.com/profile")

// Multi value form data
criteria := url.Values{
  "search_criteria": []string{"book", "glass", "pencil"},
}
resp, err := client.R().
      SetFormDataFromValues(criteria).
      Post("http://myapp.com/search")
```

#### Save HTTP Response into File

```go
// Create a Resty Client
client := resty.New()

// Setting output directory path, If directory not exists then resty creates one!
// This is optional one, if you're planning using absoule path in
// `Request.SetOutput` and can used together.
client.SetOutputDirectory("/Users/jeeva/Downloads")

// HTTP response gets saved into file, similar to curl -o flag
_, err := client.R().
          SetOutput("plugin/ReplyWithHeader-v5.1-beta.zip").
          Get("http://bit.ly/1LouEKr")

// OR using absolute path
// Note: output directory path is not used for absolute path
_, err := client.R().
          SetOutput("/MyDownloads/plugin/ReplyWithHeader-v5.1-beta.zip").
          Get("http://bit.ly/1LouEKr")
```

#### Request URL Path Params

Resty provides easy to use dynamic request URL path params. Params can be set at client and request level. Client level params value can be overridden at request level.

```go
// Create a Resty Client
client := resty.New()

client.R().SetPathParams(map[string]string{
   "userId": "sample@sample.com",
   "subAccountId": "100002",
}).
Get("/v1/users/{userId}/{subAccountId}/details")

// Result:
//   Composed URL - /v1/users/sample@sample.com/100002/details
```

#### Request and Response Middleware

Resty provides middleware ability to manipulate for Request and Response. It is more flexible than callback approach.

```go
// Create a Resty Client
client := resty.New()

// Registering Request Middleware
client.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
    // Now you have access to Client and current Request object
    // manipulate it as per your need

    return nil  // if its success otherwise return error
  })

// Registering Response Middleware
client.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
    // Now you have access to Client and current Response object
    // manipulate it as per your need

    return nil  // if its success otherwise return error
  })
```

#### OnError Hooks

Resty provides OnError hooks that may be called because:

- The client failed to send the request due to connection timeout, TLS handshake failure, etc...
- The request was retried the maximum amount of times, and still failed.

If there was a response from the server, the original error will be wrapped in `*resty.ResponseError` which contains the last response received.

```go
// Create a Resty Client
client := resty.New()

client.OnError(func(req *resty.Request, err error) {
  if v, ok := err.(*resty.ResponseError); ok {
    // v.Response contains the last response from the server
    // v.Err contains the original error
  }
  // Log the error, increment a metric, etc...
})
```

#### Redirect Policy

Resty provides few ready to use redirect policy(s) also it supports multiple policies together.

```go
// Create a Resty Client
client := resty.New()

// Assign Client Redirect Policy. Create one as per you need
client.SetRedirectPolicy(resty.FlexibleRedirectPolicy(15))

// Wanna multiple policies such as redirect count, domain name check, etc
client.SetRedirectPolicy(resty.FlexibleRedirectPolicy(20),
                        resty.DomainCheckRedirectPolicy("host1.com", "host2.org", "host3.net"))
```

##### Custom Redirect Policy

Implement [RedirectPolicy](redirect.go#L20) interface and register it with resty client. Have a look [redirect.go](redirect.go) for more information.

```go
// Create a Resty Client
client := resty.New()

// Using raw func into resty.SetRedirectPolicy
client.SetRedirectPolicy(resty.RedirectPolicyFunc(func(req *http.Request, via []*http.Request) error {
  // Implement your logic here

  // return nil for continue redirect otherwise return error to stop/prevent redirect
  return nil
}))

//---------------------------------------------------

// Using struct create more flexible redirect policy
type CustomRedirectPolicy struct {
  // variables goes here
}

func (c *CustomRedirectPolicy) Apply(req *http.Request, via []*http.Request) error {
  // Implement your logic here

  // return nil for continue redirect otherwise return error to stop/prevent redirect
  return nil
}

// Registering in resty
client.SetRedirectPolicy(CustomRedirectPolicy{/* initialize variables */})
```

#### Custom Root Certificates and Client Certificates

```go
// Create a Resty Client
client := resty.New()

// Custom Root certificates, just supply .pem file.
// you can add one or more root certificates, its get appended
client.SetRootCertificate("/path/to/root/pemFile1.pem")
client.SetRootCertificate("/path/to/root/pemFile2.pem")
// ... and so on!

// Adding Client Certificates, you add one or more certificates
// Sample for creating certificate object
// Parsing public/private key pair from a pair of files. The files must contain PEM encoded data.
cert1, err := tls.LoadX509KeyPair("certs/client.pem", "certs/client.key")
if err != nil {
  log.Fatalf("ERROR client certificate: %s", err)
}
// ...

// You add one or more certificates
client.SetCertificates(cert1, cert2, cert3)
```

#### Custom Root Certificates and Client Certificates from string

```go
// Custom Root certificates from string
// You can pass you certificates throught env variables as strings
// you can add one or more root certificates, its get appended
client.SetRootCertificateFromString("-----BEGIN CERTIFICATE-----content-----END CERTIFICATE-----")
client.SetRootCertificateFromString("-----BEGIN CERTIFICATE-----content-----END CERTIFICATE-----")
// ... and so on!

// Adding Client Certificates, you add one or more certificates
// Sample for creating certificate object
// Parsing public/private key pair from a pair of files. The files must contain PEM encoded data.
cert1, err := tls.X509KeyPair([]byte("-----BEGIN CERTIFICATE-----content-----END CERTIFICATE-----"), []byte("-----BEGIN CERTIFICATE-----content-----END CERTIFICATE-----"))
if err != nil {
  log.Fatalf("ERROR client certificate: %s", err)
}
// ...

// You add one or more certificates
client.SetCertificates(cert1, cert2, cert3)
```

#### Proxy Settings - Client as well as at Request Level

Default `Go` supports Proxy via environment variable `HTTP_PROXY`. Resty provides support via `SetProxy` & `RemoveProxy`.
Choose as per your need.

**Client Level Proxy** settings applied to all the request

```go
// Create a Resty Client
client := resty.New()

// Setting a Proxy URL and Port
client.SetProxy("http://proxyserver:8888")

// Want to remove proxy setting
client.RemoveProxy()
```

#### Retries

Resty uses [backoff](http://www.awsarchitectureblog.com/2015/03/backoff.html)
to increase retry intervals after each attempt.

Usage example:

```go
// Create a Resty Client
client := resty.New()

// Retries are configured per client
client.
    // Set retry count to non zero to enable retries
    SetRetryCount(3).
    // You can override initial retry wait time.
    // Default is 100 milliseconds.
    SetRetryWaitTime(5 * time.Second).
    // MaxWaitTime can be overridden as well.
    // Default is 2 seconds.
    SetRetryMaxWaitTime(20 * time.Second).
    // SetRetryAfter sets callback to calculate wait time between retries.
    // Default (nil) implies exponential backoff with jitter
    SetRetryAfter(func(client *resty.Client, resp *resty.Response) (time.Duration, error) {
        return 0, errors.New("quota exceeded")
    })
```

Above setup will result in resty retrying requests returned non nil error up to
3 times with delay increased after each attempt.

You can optionally provide client with [custom retry conditions](https://pkg.go.dev/github.com/go-resty/resty/v2#RetryConditionFunc):

```go
// Create a Resty Client
client := resty.New()

client.AddRetryCondition(
    // RetryConditionFunc type is for retry condition function
    // input: non-nil Response OR request execution error
    func(r *resty.Response, err error) bool {
        return r.StatusCode() == http.StatusTooManyRequests
    },
)
```

Above example will make resty retry requests ended with `429 Too Many Requests`
status code.

Multiple retry conditions can be added.

It is also possible to use `resty.Backoff(...)` to get arbitrary retry scenarios
implemented. [Reference](retry_test.go).

#### Allow GET request with Payload

```go
// Create a Resty Client
client := resty.New()

// Allow GET request with Payload. This is disabled by default.
client.SetAllowGetMethodPayload(true)
```

#### Wanna Multiple Clients

```go
// Here you go!
// Client 1
client1 := resty.New()
client1.R().Get("http://httpbin.org")
// ...

// Client 2
client2 := resty.New()
client2.R().Head("http://httpbin.org")
// ...

// Bend it as per your need!!!
```

#### Remaining Client Settings & its Options

```go
// Create a Resty Client
client := resty.New()

// Unique settings at Client level
//--------------------------------
// Enable debug mode
client.SetDebug(true)

// Assign Client TLSClientConfig
// One can set custom root-certificate. Refer: http://golang.org/pkg/crypto/tls/#example_Dial
client.SetTLSClientConfig(&tls.Config{ RootCAs: roots })

// or One can disable security check (https)
client.SetTLSClientConfig(&tls.Config{ InsecureSkipVerify: true })

// Set client timeout as per your need
client.SetTimeout(1 * time.Minute)


// You can override all below settings and options at request level if you want to
//--------------------------------------------------------------------------------
// Host URL for all request. So you can use relative URL in the request
client.SetHostURL("http://httpbin.org")

// Headers for all request
client.SetHeader("Accept", "application/json")
client.SetHeaders(map[string]string{
        "Content-Type": "application/json",
        "User-Agent": "My custom User Agent String",
      })

// Cookies for all request
client.SetCookie(&http.Cookie{
      Name:"go-resty",
      Value:"This is cookie value",
      Path: "/",
      Domain: "sample.com",
      MaxAge: 36000,
      HttpOnly: true,
      Secure: false,
    })
client.SetCookies(cookies)

// URL query parameters for all request
client.SetQueryParam("user_id", "00001")
client.SetQueryParams(map[string]string{ // sample of those who use this manner
      "api_key": "api-key-here",
      "api_secert": "api-secert",
    })
client.R().SetQueryString("productId=232&template=fresh-sample&cat=resty&source=google&kw=buy a lot more")

// Form data for all request. Typically used with POST and PUT
client.SetFormData(map[string]string{
    "access_token": "BC594900-518B-4F7E-AC75-BD37F019E08F",
  })

// Basic Auth for all request
client.SetBasicAuth("myuser", "mypass")

// Bearer Auth Token for all request
client.SetAuthToken("BC594900518B4F7EAC75BD37F019E08FBC594900518B4F7EAC75BD37F019E08F")

// Enabling Content length value for all request
client.SetContentLength(true)

// Registering global Error object structure for JSON/XML request
client.SetError(&Error{})    // or resty.SetError(Error{})
```

#### Unix Socket

```go
unixSocket := "/var/run/my_socket.sock"

// Create a Go's http.Transport so we can set it in resty.
transport := http.Transport{
	Dial: func(_, _ string) (net.Conn, error) {
		return net.Dial("unix", unixSocket)
	},
}

// Create a Resty Client
client := resty.New()

// Set the previous transport that we created, set the scheme of the communication to the
// socket and set the unixSocket as the HostURL.
client.SetTransport(&transport).SetScheme("http").SetHostURL(unixSocket)

// No need to write the host's URL on the request, just the path.
client.R().Get("/index.html")
```

#### Bazel Support

Resty can be built, tested and depended upon via [Bazel](https://bazel.build).
For example, to run all tests:

```shell
bazel test :resty_test
```

#### Mocking http requests using [httpmock](https://github.com/jarcoal/httpmock) library

In order to mock the http requests when testing your application you
could use the `httpmock` library.

When using the default resty client, you should pass the client to the library as follow:

```go
// Create a Resty Client
client := resty.New()

// Get the underlying HTTP Client and set it to Mock
httpmock.ActivateNonDefault(client.GetClient())
```

More detailed example of mocking resty http requests using ginko could be found [here](https://github.com/jarcoal/httpmock#ginkgo--resty-example).

## Versioning

Resty releases versions according to [Semantic Versioning](http://semver.org)

  * Resty v2 does not use `gopkg.in` service for library versioning.
  * Resty fully adapted to `go mod` capabilities since `v1.10.0` release.
  * Resty v1 series was using `gopkg.in` to provide versioning. `gopkg.in/resty.vX` points to appropriate tagged versions; `X` denotes version series number and it's a stable release for production use. For e.g. `gopkg.in/resty.v0`.
  * Development takes place at the master branch. Although the code in master should always compile and test successfully, it might break API's. I aim to maintain backwards compatibility, but sometimes API's and behavior might be changed to fix a bug.

## Contribution

I would welcome your contribution! If you find any improvement or issue you want to fix, feel free to send a pull request, I like pull requests that include test cases for fix/enhancement. I have done my best to bring pretty good code coverage. Feel free to write tests.

BTW, I'd like to know what you think about `Resty`. Kindly open an issue or send me an email; it'd mean a lot to me.

## Creator

[Jeevanandam M.](https://github.com/jeevatkm) (jeeva@myjeeva.com)

## Core Team

Have a look on [Members](https://github.com/orgs/go-resty/people) page.

## Contributors

Have a look on [Contributors](https://github.com/go-resty/resty/graphs/contributors) page.

## License

Resty released under MIT license, refer [LICENSE](LICENSE) file.
