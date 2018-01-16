// Copyright (c) 2015 Serge Gebhardt. All rights reserved.
//
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

package acd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

const (
	// LibraryVersion is the current version of this library
	LibraryVersion     = "0.1.0"
	defaultMetadataURL = "https://drive.amazonaws.com/drive/v1/"
	defaultContentURL  = "https://content-na.drive.amazonaws.com/cdproxy/"
	userAgent          = "go-acd/" + LibraryVersion
)

// A Client manages communication with the Amazon Cloud Drive API.
type Client struct {
	// HTTP client used to communicate with the API.
	httpClient *http.Client

	// Metadata URL for API requests. Defaults to the public Amazon Cloud Drive API.
	// MetadataURL should always be specified with a trailing slash.
	MetadataURL *url.URL

	// Content URL for API requests. Defaults to the public Amazon Cloud Drive API.
	// ContentURL should always be specified with a trailing slash.
	ContentURL *url.URL

	// User agent used when communicating with the API.
	UserAgent string

	// Services used for talking to different parts of the API.
	Account *AccountService
	Nodes   *NodesService
	Changes *ChangesService
}

// NewClient returns a new Amazon Cloud Drive API client. If a nil httpClient is
// provided, http.DefaultClient will be used. To use API methods which require
// authentication, provide an http.Client that will perform the authentication
// for you (such as that provided by the golang.org/x/oauth2 library).
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	metadataURL, _ := url.Parse(defaultMetadataURL)
	contentURL, _ := url.Parse(defaultContentURL)

	c := &Client{
		httpClient:  httpClient,
		MetadataURL: metadataURL,
		ContentURL:  contentURL,
		UserAgent:   userAgent,
	}

	c.Account = &AccountService{client: c}
	c.Nodes = &NodesService{client: c}
	c.Changes = &ChangesService{client: c}

	return c
}

// NewMetadataRequest creates an API request for metadata. A relative URL can be
// provided in urlStr, in which case it is resolved relative to the MetadataURL
// of the Client. Relative URLs should always be specified without a preceding
// slash. If specified, the value pointed to by body is JSON encoded and included
// as the request body.
func (c *Client) NewMetadataRequest(method, urlStr string, body interface{}) (*http.Request, error) {
	return c.newRequest(c.MetadataURL, method, urlStr, body)
}

// NewContentRequest creates an API request for content. A relative URL can be
// provided in urlStr, in which case it is resolved relative to the ContentURL
// of the Client. Relative URLs should always be specified without a preceding
// slash. If specified, the value pointed to by body is JSON encoded and included
// as the request body.
func (c *Client) NewContentRequest(method, urlStr string, body interface{}) (*http.Request, error) {
	return c.newRequest(c.ContentURL, method, urlStr, body)
}

// newRequest creates an API request. A relative URL can be provided in urlStr,
// in which case it is resolved relative to base URL.
// Relative URLs should always be specified without a preceding slash. If
// specified, the value pointed to by body is JSON encoded and included as the
// request body.
func (c *Client) newRequest(base *url.URL, method, urlStr string, body interface{}) (*http.Request, error) {
	rel, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	u := base.ResolveReference(rel)

	bodyReader, ok := body.(io.Reader)
	if !ok && body != nil {
		buf := &bytes.Buffer{}
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
		bodyReader = buf
	}

	req, err := http.NewRequest(method, u.String(), bodyReader)
	if err != nil {
		return nil, err
	}

	//	req.Header.Add("Accept", mediaTypeV3)
	if c.UserAgent != "" {
		req.Header.Add("User-Agent", c.UserAgent)
	}
	return req, nil
}

// Do sends an API request and returns the API response. The API response is
// JSON decoded and stored in the value pointed to by v, or returned as an
// error if an API error has occurred. If v implements the io.Writer
// interface, the raw response body will be written to v, without attempting to
// first decode it. If v is nil then the resp.Body won't be closed - this is
// your responsibility.
//
func (c *Client) Do(req *http.Request, v interface{}) (*http.Response, error) {
	//buf, _ := httputil.DumpRequest(req, true)
	//buf, _ := httputil.DumpRequest(req, false)
	//log.Printf("req = %s", string(buf))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if v != nil {
		defer resp.Body.Close()
	}
	//buf, _ = httputil.DumpResponse(resp, true)
	//buf, _ = httputil.DumpResponse(resp, false)
	//log.Printf("resp = %s", string(buf))

	err = CheckResponse(resp)
	if err != nil {
		// even though there was an error, we still return the response
		// in case the caller wants to inspect it further.  We do close the
		// Body though
		if v == nil {
			resp.Body.Close()
		}
		return resp, err
	}

	if v != nil {
		if w, ok := v.(io.Writer); ok {
			io.Copy(w, resp.Body)
		} else {
			err = json.NewDecoder(resp.Body).Decode(v)
		}
	}
	return resp, err
}

// CheckResponse checks the API response for errors, and returns them if
// present.  A response is considered an error if it has a status code outside
// the 200 range.
func CheckResponse(r *http.Response) error {
	c := r.StatusCode
	if 200 <= c && c <= 299 {
		return nil
	}

	errBody := ""
	if data, err := ioutil.ReadAll(r.Body); err == nil {
		errBody = strings.TrimSpace(string(data))
	}

	errMsg := fmt.Sprintf("HTTP code %v: %q: ", c, r.Status)
	if errBody == "" {
		errMsg += "no response body"
	} else {
		errMsg += fmt.Sprintf("response body: %q", errBody)
	}

	return errors.New(errMsg)
}
