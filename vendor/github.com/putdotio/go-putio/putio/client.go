package putio

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

const (
	defaultUserAgent = "go-putio"
	defaultMediaType = "application/json"
	defaultBaseURL   = "https://api.put.io"
	defaultUploadURL = "https://upload.put.io"
)

// Client manages communication with Put.io v2 API.
type Client struct {
	// HTTP client used to communicate with Put.io API
	client *http.Client

	// Base URL for API requests
	BaseURL *url.URL

	// base url for upload requests
	uploadURL *url.URL

	// User agent for client
	UserAgent string

	// Override host header for API requests
	Host string

	// ExtraHeaders are passed to the API server on every request.
	ExtraHeaders http.Header

	// Services used for communicating with the API
	Account   *AccountService
	Files     *FilesService
	Transfers *TransfersService
	Zips      *ZipsService
	Friends   *FriendsService
	Events    *EventsService
}

// NewClient returns a new Put.io API client, using the htttpClient, which must
// be a new Oauth2 enabled http.Client. If httpClient is not defined, default
// HTTP client is used.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	baseURL, _ := url.Parse(defaultBaseURL)
	uploadURL, _ := url.Parse(defaultUploadURL)
	c := &Client{
		client:       httpClient,
		BaseURL:      baseURL,
		uploadURL:    uploadURL,
		UserAgent:    defaultUserAgent,
		ExtraHeaders: make(http.Header),
	}

	c.Account = &AccountService{client: c}
	c.Files = &FilesService{client: c}
	c.Transfers = &TransfersService{client: c}
	c.Zips = &ZipsService{client: c}
	c.Friends = &FriendsService{client: c}
	c.Events = &EventsService{client: c}

	return c
}

func (c *Client) ValidateToken(ctx context.Context) (userID *int64, err error) {
	req, err := c.NewRequest(ctx, "GET", "/v2/oauth2/validate", nil)
	if err != nil {
		return
	}
	var r struct {
		UserID *int64 `json:"user_id"`
	}
	resp, err := c.Do(req, &r)
	defer resp.Body.Close()
	return r.UserID, err
}

// NewRequest creates an API request. A relative URL can be provided via
// relURL, which will be resolved to the BaseURL of the Client.
func (c *Client) NewRequest(ctx context.Context, method, relURL string, body io.Reader) (*http.Request, error) {
	rel, err := url.Parse(relURL)
	if err != nil {
		return nil, err
	}

	var u *url.URL
	// XXX: workaroud for upload endpoint. upload method has a different base url,
	// so we've a special case for testing purposes.
	if relURL == "/v2/files/upload" {
		u = c.uploadURL.ResolveReference(rel)
	} else {
		u = c.BaseURL.ResolveReference(rel)
	}

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	req.Header.Set("Accept", defaultMediaType)
	req.Header.Set("User-Agent", c.UserAgent)

	if c.Host != "" {
		req.Host = c.Host
	}

	// merge headers with extra headers
	for header, values := range c.ExtraHeaders {
		for _, value := range values {
			req.Header.Add(header, value)
		}
	}

	return req, nil
}

// Do sends an API request and returns the API response. The API response is
// JSON decoded and stored in the value pointed to by v, or returned as an
// error if an API error has occurred. Response body is closed at all cases except
// v is nil. If v is nil, response body is not closed and the body can be used
// for streaming.
func (c *Client) Do(r *http.Request, v interface{}) (*http.Response, error) {
	resp, err := c.client.Do(r)
	if err != nil {
		return nil, err
	}

	err = checkResponse(resp)
	if err != nil {
		// close the body at all times if there is an http error
		resp.Body.Close()
		return resp, err
	}

	if v == nil {
		return resp, nil
	}

	// close the body for all cases from here
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(v)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// checkResponse is the entrypoint to reading the API response. If the response
// status code is not in success range, it will try to return a structured
// error.
func checkResponse(r *http.Response) error {
	if r.StatusCode >= 200 && r.StatusCode <= 399 {
		return nil
	}

	// server possibly returns json and more details
	er := &ErrorResponse{Response: r}

	er.Body, er.ParseError = ioutil.ReadAll(r.Body)
	if er.ParseError != nil {
		return er
	}
	if r.Header.Get("content-type") == "application/json" {
		er.ParseError = json.Unmarshal(er.Body, er)
		if er.ParseError != nil {
			return er
		}
	}
	return er
}
