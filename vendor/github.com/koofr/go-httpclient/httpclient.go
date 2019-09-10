package httpclient

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

var XmlHeaderBytes []byte = []byte(xml.Header)

type ErrorHandlerFunc func(*http.Response, error) error
type PostHookFunc func(*http.Request, *http.Response) error

type HTTPClient struct {
	BaseURL          *url.URL
	Headers          http.Header
	Client           *http.Client
	PostHooks        map[int]PostHookFunc
	errorHandler     ErrorHandlerFunc
	rateLimited      bool
	rateLimitChan    chan struct{}
	rateLimitTimeout time.Duration
}

func New() (httpClient *HTTPClient) {
	return &HTTPClient{
		Client:    HttpClient,
		Headers:   make(http.Header),
		PostHooks: make(map[int]PostHookFunc),
	}
}

func Insecure() (httpClient *HTTPClient) {
	httpClient = New()
	httpClient.Client = InsecureHttpClient
	return httpClient
}

var DefaultClient = New()

func (c *HTTPClient) SetPostHook(onStatus int, hook PostHookFunc) {
	c.PostHooks[onStatus] = hook
}

func (c *HTTPClient) SetErrorHandler(handler ErrorHandlerFunc) {
	c.errorHandler = handler
}

func (c *HTTPClient) SetRateLimit(limit int, timeout time.Duration) {
	c.rateLimited = true
	c.rateLimitChan = make(chan struct{}, limit)

	for i := 0; i < limit; i++ {
		c.rateLimitChan <- struct{}{}
	}

	c.rateLimitTimeout = timeout
}

func (c *HTTPClient) buildURL(req *RequestData) *url.URL {
	bu := c.BaseURL

	rpath := req.Path

	if strings.HasSuffix(bu.Path, "/") && strings.HasPrefix(rpath, "/") {
		rpath = rpath[1:]
	}

	opaque := EscapePath(bu.Path + rpath)

	u := &url.URL{
		Scheme: bu.Scheme,
		Host:   bu.Host,
		Opaque: opaque,
	}

	if req.Params != nil {
		u.RawQuery = req.Params.Encode()
	}

	return u
}

func (c *HTTPClient) setHeaders(req *RequestData, httpReq *http.Request) {
	switch req.RespEncoding {
	case EncodingJSON:
		httpReq.Header.Set("Accept", "application/json")
	case EncodingXML:
		httpReq.Header.Set("Accept", "application/xml")
	}

	if c.Headers != nil {
		for key, values := range c.Headers {
			for _, value := range values {
				httpReq.Header.Set(key, value)
			}
		}
	}

	if req.Headers != nil {
		for key, values := range req.Headers {
			for _, value := range values {
				httpReq.Header.Set(key, value)
			}
		}
	}
}

func (c *HTTPClient) checkStatus(req *RequestData, response *http.Response) (err error) {
	if req.ExpectedStatus != nil {
		statusOk := false

		for _, status := range req.ExpectedStatus {
			if response.StatusCode == status {
				statusOk = true
			}
		}

		if !statusOk {
			lr := io.LimitReader(response.Body, 10*1024)
			contentBytes, _ := ioutil.ReadAll(lr)
			content := string(contentBytes)

			err = InvalidStatusError{
				Expected: req.ExpectedStatus,
				Got:      response.StatusCode,
				Headers:  response.Header,
				Content:  content,
			}

			return err
		}
	}

	return nil
}

func (c *HTTPClient) unmarshalResponse(req *RequestData, response *http.Response) (err error) {
	var buf []byte

	switch req.RespEncoding {
	case EncodingJSON:
		defer response.Body.Close()

		if buf, err = ioutil.ReadAll(response.Body); err != nil {
			return err
		}

		err = json.Unmarshal(buf, req.RespValue)

		if err != nil {
			return err
		}

		return nil

	case EncodingXML:
		defer response.Body.Close()

		if buf, err = ioutil.ReadAll(response.Body); err != nil {
			return err
		}

		err = xml.Unmarshal(buf, req.RespValue)

		if err != nil {
			return err
		}

		return nil
	}

	switch req.RespValue.(type) {
	case *[]byte:
		defer response.Body.Close()

		if buf, err = ioutil.ReadAll(response.Body); err != nil {
			return err
		}

		respVal := req.RespValue.(*[]byte)
		*respVal = buf

		return nil
	}

	if req.RespConsume {
		defer response.Body.Close()
		ioutil.ReadAll(response.Body)
	}

	return nil
}

func (c *HTTPClient) marshalRequest(req *RequestData) (err error) {
	if req.ReqReader != nil || req.ReqValue == nil {
		return nil
	}

	if req.Headers == nil {
		req.Headers = make(http.Header)
	}

	var buf []byte

	switch req.ReqEncoding {
	case EncodingJSON:
		buf, err = json.Marshal(req.ReqValue)

		if err != nil {
			return err
		}

		req.ReqReader = bytes.NewReader(buf)
		req.Headers.Set("Content-Type", "application/json")
		req.Headers.Set("Content-Length", fmt.Sprintf("%d", len(buf)))

		req.ReqContentLength = int64(len(buf))

		return nil

	case EncodingXML:
		buf, err = xml.Marshal(req.ReqValue)

		if err != nil {
			return err
		}

		buf = append(XmlHeaderBytes, buf...)

		req.ReqReader = bytes.NewReader(buf)
		req.Headers.Set("Content-Type", "application/xml")
		req.Headers.Set("Content-Length", fmt.Sprintf("%d", len(buf)))

		req.ReqContentLength = int64(len(buf))

		return nil

	case EncodingForm:
		if data, ok := req.ReqValue.(url.Values); ok {
			formStr := data.Encode()
			req.ReqReader = strings.NewReader(formStr)
			req.Headers.Set("Content-Type", "application/x-www-form-urlencoded")

			req.Headers.Set("Content-Length", fmt.Sprintf("%d", len(formStr)))

			req.ReqContentLength = int64(len(formStr))

			return nil
		} else {
			return fmt.Errorf("HTTPClient: invalid ReqValue type %T", req.ReqValue)
		}
	}

	return fmt.Errorf("HTTPClient: invalid ReqEncoding: %s", req.ReqEncoding)
}

func (c *HTTPClient) runPostHook(req *http.Request, response *http.Response) (err error) {
	hook, ok := c.PostHooks[response.StatusCode]

	if ok {
		err = hook(req, response)
	}

	return err
}

func (c *HTTPClient) Request(req *RequestData) (response *http.Response, err error) {
	err = c.marshalRequest(req)

	if err != nil {
		return nil, err
	}

	r, err := http.NewRequest(req.Method, req.FullURL, req.ReqReader)

	if err != nil {
		return nil, err
	}

	if req.Context != nil {
		r = r.WithContext(req.Context)
	}

	r.ContentLength = req.ReqContentLength

	if req.FullURL == "" {
		r.URL = c.buildURL(req)
		r.Host = r.URL.Host
	}

	c.setHeaders(req, r)

	if c.rateLimited {
		if c.rateLimitTimeout > 0 {
			select {
			case t := <-c.rateLimitChan:
				defer func() {
					c.rateLimitChan <- t
				}()
			case <-time.After(c.rateLimitTimeout):
				return nil, RateLimitTimeoutError
			}
		} else {
			t := <-c.rateLimitChan
			defer func() {
				c.rateLimitChan <- t
			}()
		}
	}

	isTraceEnabled := os.Getenv("HTTPCLIENT_TRACE") != ""

	if isTraceEnabled {
		requestBytes, _ := httputil.DumpRequestOut(r, true)
		fmt.Println(string(requestBytes))
	}

	if req.IgnoreRedirects {
		transport := c.Client.Transport

		if transport == nil {
			transport = http.DefaultTransport
		}

		response, err = transport.RoundTrip(r)
	} else {
		response, err = c.Client.Do(r)
	}

	if err != nil {
		if req.Context != nil {
			// If we got an error, and the context has been canceled,
			// the context's error is probably more useful.
			select {
			case <-req.Context.Done():
				err = req.Context.Err()
			default:
			}
		}
		if c.errorHandler != nil {
			err = c.errorHandler(response, err)
		}
		return nil, err
	}

	if isTraceEnabled {
		responseBytes, _ := httputil.DumpResponse(response, true)
		fmt.Println(string(responseBytes))
	}

	if err = c.runPostHook(r, response); err != nil {
		return response, err
	}

	if err = c.checkStatus(req, response); err != nil {
		defer response.Body.Close()
		return response, err
	}

	if err = c.unmarshalResponse(req, response); err != nil {
		return response, err
	}

	return response, nil
}
