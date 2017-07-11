package rest

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/Jeffail/gabs"
)

// Method contains the supported HTTP verbs.
type Method string

// Supported HTTP verbs.
const (
	Get    Method = "GET"
	Post   Method = "POST"
	Put    Method = "PUT"
	Patch  Method = "PATCH"
	Delete Method = "DELETE"
)

// Request holds the request to an API Call.
type Request struct {
	Method      Method
	BaseURL     string // e.g. https://api.service.com
	Headers     map[string]string
	QueryParams map[string]string
	Body        []byte
}

// Response holds the response from an API call.
type Response struct {
	StatusCode int         // e.g. 200
	Headers    http.Header // e.g. map[X-Rate-Limit:[600]]
	Body       string      // e.g. {"result: success"}
	JSON       *gabs.Container
}

// ParseJSON parses the response body to JSON container.
func (r *Response) ParseJSON() error {
	if strings.Contains(r.Headers.Get("Content-Type"), "application/json") {
		json, err := gabs.ParseJSON([]byte(r.Body))
		if err != nil {
			return err
		}

		r.JSON = json
		return nil
	}

	return errors.New("response body is not JSON")
}

// DefaultClient is used if no custom HTTP client is defined
var DefaultClient = &Client{HTTPClient: http.DefaultClient}

// Client allows modification of client headers, redirect policy
// and other settings
// See https://golang.org/pkg/net/http
type Client struct {
	HTTPClient *http.Client
}

// The following functions enable the ability to define a
// custom HTTP Client

// MakeRequest makes the API call.
func (c *Client) MakeRequest(req *http.Request) (*http.Response, error) {
	return c.HTTPClient.Do(req)
}

// API is the main interface to the API.
func (c *Client) API(r *Request) (*Response, error) {
	// Build the HTTP request object.
	req, err := BuildRequestObject(r)
	if err != nil {
		return nil, err
	}

	// Build the HTTP client and make the request.
	res, err := c.MakeRequest(req)
	if err != nil {
		return nil, err
	}

	// Build Response object.
	response, err := BuildResponse(res)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// AddQueryParameters adds query parameters to the URL.
func AddQueryParameters(baseURL string, queryParams map[string]string) string {
	baseURL += "?"
	params := url.Values{}
	for key, value := range queryParams {
		params.Add(key, value)
	}
	return baseURL + params.Encode()
}

// BuildRequestObject creates the HTTP request object.
func BuildRequestObject(r *Request) (*http.Request, error) {
	// Add any query parameters to the URL.
	if len(r.QueryParams) != 0 {
		r.BaseURL = AddQueryParameters(r.BaseURL, r.QueryParams)
	}
	req, err := http.NewRequest(string(r.Method), r.BaseURL, bytes.NewBuffer(r.Body))
	for key, value := range r.Headers {
		req.Header.Set(key, value)
	}
	_, exists := req.Header["Content-Type"]
	if len(r.Body) > 0 && !exists {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, err
}

// BuildResponse builds the response struct.
func BuildResponse(r *http.Response) (*Response, error) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	response := Response{
		StatusCode: r.StatusCode,
		Body:       string(body),
		Headers:    r.Header,
	}

	return &response, nil
}

// MakeRequest makes the API call.
func MakeRequest(r *http.Request) (*http.Response, error) {
	return DefaultClient.HTTPClient.Do(r)
}

// API is the main interface to the API.
func API(request *Request) (*Response, error) {
	return DefaultClient.API(request)
}
