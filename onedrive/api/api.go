// Package api implements the API for one drive
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ncw/rclone/fs"
)

const (
	rootURL = "https://api.onedrive.com/v1.0" // root URL for requests
)

// Client contains the info to sustain the API
type Client struct {
	c *http.Client
}

// NewClient takes an oauth http.Client and makes a new api instance
func NewClient(c *http.Client) *Client {
	return &Client{
		c: c,
	}
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
}

// checkClose is a utility function used to check the return from
// Close in a defer statement.
func checkClose(c io.Closer, err *error) {
	cerr := c.Close()
	if *err == nil {
		*err = cerr
	}
}

// decodeJSON decodes resp.Body into json
func (api *Client) decodeJSON(resp *http.Response, result interface{}) (err error) {
	defer checkClose(resp.Body, &err)
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
		url = rootURL + opts.Path
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
	req.Header.Add("User-Agent", fs.UserAgent)
	resp, err = api.c.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// Decode error response
		errResponse := new(Error)
		err = api.decodeJSON(resp, &errResponse)
		if err != nil {
			return resp, err
		}
		return resp, errResponse
	}
	if opts.NoResponse {
		return resp, resp.Body.Close()
	}
	return resp, nil
}

// CallJSON runs Call and decodes the body as a JSON object into result
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
	err = api.decodeJSON(resp, response)
	return resp, err
}
