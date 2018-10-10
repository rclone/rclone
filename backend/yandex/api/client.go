package src

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

//Client struct
type Client struct {
	token      string
	basePath   string
	HTTPClient *http.Client
}

//NewClient creates new client
func NewClient(token string, client ...*http.Client) *Client {
	return newClientInternal(
		token,
		"https://cloud-api.yandex.com/v1/disk", //also "https://cloud-api.yandex.net/v1/disk" "https://cloud-api.yandex.ru/v1/disk"
		client...)
}

func newClientInternal(token string, basePath string, client ...*http.Client) *Client {
	c := &Client{
		token:    token,
		basePath: basePath,
	}
	if len(client) != 0 {
		c.HTTPClient = client[0]
	} else {
		c.HTTPClient = http.DefaultClient
	}
	return c
}

//ErrorHandler type
type ErrorHandler func(*http.Response) error

var defaultErrorHandler ErrorHandler = func(resp *http.Response) error {
	if resp.StatusCode/100 == 5 {
		return errors.New("server error")
	}

	if resp.StatusCode/100 == 4 {
		var response DiskClientError
		contents, _ := ioutil.ReadAll(resp.Body)
		err := json.Unmarshal(contents, &response)
		if err != nil {
			return err
		}
		return response
	}

	if resp.StatusCode/100 == 3 {
		return errors.New("redirect error")
	}
	return nil
}

func (HTTPRequest *HTTPRequest) run(client *Client) ([]byte, error) {
	var err error
	values := make(url.Values)
	for k, v := range HTTPRequest.Parameters {
		values.Set(k, fmt.Sprintf("%v", v))
	}

	var req *http.Request
	if HTTPRequest.Method == "POST" {
		// TODO json serialize
		req, err = http.NewRequest(
			"POST",
			client.basePath+HTTPRequest.Path,
			strings.NewReader(values.Encode()))
		if err != nil {
			return nil, err
		}
		// TODO
		// req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(
			HTTPRequest.Method,
			client.basePath+HTTPRequest.Path+"?"+values.Encode(),
			nil)
		if err != nil {
			return nil, err
		}
	}

	for headerName := range HTTPRequest.Headers {
		var headerValues = HTTPRequest.Headers[headerName]
		for _, headerValue := range headerValues {
			req.Header.Set(headerName, headerValue)
		}
	}
	return runRequest(client, req)
}

func runRequest(client *Client, req *http.Request) ([]byte, error) {
	return runRequestWithErrorHandler(client, req, defaultErrorHandler)
}

func runRequestWithErrorHandler(client *Client, req *http.Request, errorHandler ErrorHandler) (out []byte, err error) {
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer CheckClose(resp.Body, &err)

	return checkResponseForErrorsWithErrorHandler(resp, errorHandler)
}

func checkResponseForErrorsWithErrorHandler(resp *http.Response, errorHandler ErrorHandler) ([]byte, error) {
	if resp.StatusCode/100 > 2 {
		return nil, errorHandler(resp)
	}
	return ioutil.ReadAll(resp.Body)
}

// CheckClose is a utility function used to check the return from
// Close in a defer statement.
func CheckClose(c io.Closer, err *error) {
	cerr := c.Close()
	if *err == nil {
		*err = cerr
	}
}
