// Package rest implements a simple REST wrapper
package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ncw/rclone/fs"
)

// Client contains the info to sustain the API
type Client struct {
	c            *http.Client
	rootURL      string
	errorHandler func(resp *http.Response) error
}

// NewClient takes an oauth http.Client and makes a new api instance
func NewClient(c *http.Client, rootURL string) *Client {
	return &Client{
		c:            c,
		rootURL:      rootURL,
		errorHandler: defaultErrorHandler,
	}
}

// defaultErrorHandler doesn't attempt to parse the http body
func defaultErrorHandler(resp *http.Response) (err error) {
	defer fs.CheckClose(resp.Body, &err)
	return fmt.Errorf("HTTP error %v (%v) returned", resp.StatusCode, resp.Status)
}

// SetErrorHandler sets the handler to decode an error response when
// the HTTP status code is not 2xx.  The handler should close resp.Body.
func (api *Client) SetErrorHandler(fn func(resp *http.Response) error) {
	api.errorHandler = fn
}

// Opts contains parameters for Call, CallJSON etc
type Opts struct {
	Method        string
	Path          string
	Absolute      bool // Path is absolute
	Body          io.Reader
	NoResponse    bool // set to close Body
	ContentType   string
	ContentLength *int64
	ContentRange  string
	ExtraHeaders  map[string]string
}

// DecodeJSON decodes resp.Body into result
func DecodeJSON(resp *http.Response, result interface{}) (err error) {
	defer fs.CheckClose(resp.Body, &err)
	decoder := json.NewDecoder(resp.Body)
	return decoder.Decode(result)
}

// Call makes the call and returns the http.Response
//
// if err != nil then resp.Body will need to be closed
//
// it will return resp if at all possible, even if err is set
func (api *Client) Call(opts *Opts) (resp *http.Response, err error) {
	if opts == nil {
		return nil, fmt.Errorf("call() called with nil opts")
	}
	var url string
	if opts.Absolute {
		url = opts.Path
	} else {
		url = api.rootURL + opts.Path
	}
	req, err := http.NewRequest(opts.Method, url, opts.Body)
	if err != nil {
		return
	}
	if opts.ContentType != "" {
		req.Header.Add("Content-Type", opts.ContentType)
	}
	if opts.ContentLength != nil {
		req.ContentLength = *opts.ContentLength
	}
	if opts.ContentRange != "" {
		req.Header.Add("Content-Range", opts.ContentRange)
	}
	if opts.ExtraHeaders != nil {
		for k, v := range opts.ExtraHeaders {
			req.Header.Add(k, v)
		}
	}
	req.Header.Add("User-Agent", fs.UserAgent)
	resp, err = api.c.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp, api.errorHandler(resp)
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
	// Set the body up as a JSON object if required
	if opts.Body == nil && request != nil {
		body, err := json.Marshal(request)
		if err != nil {
			return nil, err
		}
		var newOpts = *opts
		newOpts.Body = bytes.NewBuffer(body)
		newOpts.ContentType = "application/json"
		opts = &newOpts
	}
	resp, err = api.Call(opts)
	if err != nil {
		return resp, err
	}
	if opts.NoResponse {
		return resp, nil
	}
	err = DecodeJSON(resp, response)
	return resp, err
}
