// Package rest implements a simple REST wrapper
//
// All methods are safe for concurrent calling.
package rest

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"sync"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

// Client contains the info to sustain the API
type Client struct {
	mu           sync.RWMutex
	c            *http.Client
	rootURL      string
	errorHandler func(resp *http.Response) error
	headers      map[string]string
}

// NewClient takes an oauth http.Client and makes a new api instance
func NewClient(c *http.Client) *Client {
	api := &Client{
		c:            c,
		errorHandler: defaultErrorHandler,
		headers:      make(map[string]string),
	}
	return api
}

// ReadBody reads resp.Body into result, closing the body
func ReadBody(resp *http.Response) (result []byte, err error) {
	defer fs.CheckClose(resp.Body, &err)
	return ioutil.ReadAll(resp.Body)
}

// defaultErrorHandler doesn't attempt to parse the http body, just
// returns it in the error message
func defaultErrorHandler(resp *http.Response) (err error) {
	body, err := ReadBody(resp)
	if err != nil {
		return errors.Wrap(err, "error reading error out of body")
	}
	return errors.Errorf("HTTP error %v (%v) returned body: %q", resp.StatusCode, resp.Status, body)
}

// SetErrorHandler sets the handler to decode an error response when
// the HTTP status code is not 2xx.  The handler should close resp.Body.
func (api *Client) SetErrorHandler(fn func(resp *http.Response) error) *Client {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.errorHandler = fn
	return api
}

// SetRoot sets the default RootURL.  You can override this on a per
// call basis using the RootURL field in Opts.
func (api *Client) SetRoot(RootURL string) *Client {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.rootURL = RootURL
	return api
}

// SetHeader sets a header for all requests
func (api *Client) SetHeader(key, value string) *Client {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.headers[key] = value
	return api
}

// Opts contains parameters for Call, CallJSON etc
type Opts struct {
	Method                string // GET, POST etc
	Path                  string // relative to RootURL
	RootURL               string // override RootURL passed into SetRoot()
	Body                  io.Reader
	NoResponse            bool // set to close Body
	ContentType           string
	ContentLength         *int64
	ContentRange          string
	ExtraHeaders          map[string]string
	UserName              string // username for Basic Auth
	Password              string // password for Basic Auth
	Options               []fs.OpenOption
	IgnoreStatus          bool       // if set then we don't check error status or parse error body
	MultipartMetadataName string     // set the following 3 vars
	MultipartContentName  string     // and Body and pass in request
	MultipartFileName     string     // for multipart upload
	Parameters            url.Values // any parameters for the final URL
}

// Copy creates a copy of the options
func (o *Opts) Copy() *Opts {
	newOpts := *o
	return &newOpts
}

// DecodeJSON decodes resp.Body into result
func DecodeJSON(resp *http.Response, result interface{}) (err error) {
	defer fs.CheckClose(resp.Body, &err)
	decoder := json.NewDecoder(resp.Body)
	return decoder.Decode(result)
}

// ClientWithHeaderReset makes a new http client which resets the
// headers passed in on redirect
//
// FIXME This is now unecessary with go1.8
func ClientWithHeaderReset(c *http.Client, headers map[string]string) *http.Client {
	if len(headers) == 0 {
		return c
	}
	clientCopy := *c
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		// Reset the headers in the new request
		for k, v := range headers {
			if v != "" {
				req.Header.Set(k, v)
			}
		}
		return nil
	}
	return &clientCopy
}

// Call makes the call and returns the http.Response
//
// if err != nil then resp.Body will need to be closed
//
// it will return resp if at all possible, even if err is set
func (api *Client) Call(opts *Opts) (resp *http.Response, err error) {
	api.mu.RLock()
	defer api.mu.RUnlock()
	if opts == nil {
		return nil, errors.New("call() called with nil opts")
	}
	url := api.rootURL
	if opts.RootURL != "" {
		url = opts.RootURL
	}
	if url == "" {
		return nil, errors.New("RootURL not set")
	}
	url += opts.Path
	if opts.Parameters != nil && len(opts.Parameters) > 0 {
		url += "?" + opts.Parameters.Encode()
	}
	req, err := http.NewRequest(opts.Method, url, opts.Body)
	if err != nil {
		return
	}
	headers := make(map[string]string)
	// Set default headers
	for k, v := range api.headers {
		headers[k] = v
	}
	if opts.ContentType != "" {
		headers["Content-Type"] = opts.ContentType
	}
	if opts.ContentLength != nil {
		req.ContentLength = *opts.ContentLength
	}
	if opts.ContentRange != "" {
		headers["Content-Range"] = opts.ContentRange
	}
	// Set any extra headers
	if opts.ExtraHeaders != nil {
		for k, v := range opts.ExtraHeaders {
			headers[k] = v
		}
	}
	// add any options to the headers
	fs.OpenOptionAddHeaders(opts.Options, headers)
	// Now set the headers
	for k, v := range headers {
		if v != "" {
			req.Header.Add(k, v)
		}
	}
	if opts.UserName != "" || opts.Password != "" {
		req.SetBasicAuth(opts.UserName, opts.Password)
	}
	c := ClientWithHeaderReset(api.c, headers)
	api.mu.RUnlock()
	resp, err = c.Do(req)
	api.mu.RLock()
	if err != nil {
		return nil, err
	}
	if !opts.IgnoreStatus {
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return resp, api.errorHandler(resp)
		}
	}
	if opts.NoResponse {
		return resp, resp.Body.Close()
	}
	return resp, nil
}

// CallJSON runs Call and decodes the body as a JSON object into response (if not nil)
//
// If request is not nil then it will be JSON encoded as the body of the request
//
// It will return resp if at all possible, even if err is set
func (api *Client) CallJSON(opts *Opts, request interface{}, response interface{}) (resp *http.Response, err error) {
	var requestBody []byte
	// Marshal the request if given
	if request != nil {
		opts = opts.Copy()
		requestBody, err = json.Marshal(request)
		opts.ContentType = "application/json"
		if err != nil {
			return nil, err
		}
		// Set the body up as a JSON object if required
		if opts.Body == nil {
			opts.Body = bytes.NewBuffer(requestBody)
		}
	}
	errChan := make(chan error, 1)
	isMultipart := opts.MultipartMetadataName != "" && opts.Body != nil && request != nil
	if isMultipart {
		bodyReader, bodyWriter := io.Pipe()
		writer := multipart.NewWriter(bodyWriter)
		opts.ContentType = writer.FormDataContentType()
		in := opts.Body
		opts.Body = bodyReader
		go func() {
			defer func() { _ = bodyWriter.Close() }()
			var err error

			// Create the first part
			err = writer.WriteField(opts.MultipartMetadataName, string(requestBody))
			if err != nil {
				errChan <- err
				return
			}

			// Add the file part
			part, err := writer.CreateFormFile(opts.MultipartContentName, opts.MultipartFileName)
			if err != nil {
				errChan <- err
				return
			}

			// Copy it in
			if _, err := io.Copy(part, in); err != nil {
				errChan <- err
				return
			}
			errChan <- writer.Close()
		}()
	}
	resp, err = api.Call(opts)
	if err != nil {
		return resp, err
	}
	if response == nil || opts.NoResponse {
		return resp, nil
	}
	if isMultipart {
		err = <-errChan
		if err != nil {
			return resp, err
		}
	}
	err = DecodeJSON(resp, response)
	return resp, err
}
