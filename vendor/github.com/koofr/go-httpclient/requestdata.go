package httpclient

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

type Encoding string

const (
	EncodingJSON = "JSON"
	EncodingXML  = "XML"
	EncodingForm = "Form"
)

type RequestData struct {
	Context          context.Context
	Method           string
	Path             string
	Params           url.Values
	FullURL          string // client.BaseURL + Path or FullURL
	Headers          http.Header
	ReqReader        io.Reader
	ReqEncoding      Encoding
	ReqValue         interface{}
	ReqContentLength int64
	ExpectedStatus   []int
	IgnoreRedirects  bool
	RespEncoding     Encoding
	RespValue        interface{}
	RespConsume      bool
}

func (r *RequestData) CanCopy() bool {
	if r.ReqReader != nil {
		return false
	}

	return true
}

func (r *RequestData) Copy() (ok bool, nr *RequestData) {
	if !r.CanCopy() {
		return false, nil
	}

	nr = &RequestData{
		Method:          r.Method,
		Path:            r.Path,
		FullURL:         r.FullURL,
		ReqEncoding:     r.ReqEncoding,
		ReqValue:        r.ReqValue,
		IgnoreRedirects: r.IgnoreRedirects,
		RespEncoding:    r.RespEncoding,
		RespValue:       r.RespValue,
		RespConsume:     r.RespConsume,
	}

	if r.Params != nil {
		nr.Params = make(url.Values)

		for k, vs := range r.Params {
			nvs := make([]string, len(vs))

			for i, v := range vs {
				nvs[i] = v
			}

			nr.Params[k] = nvs
		}
	}

	if r.Headers != nil {
		nr.Headers = make(http.Header)

		for k, vs := range r.Headers {
			nvs := make([]string, len(vs))

			for i, v := range vs {
				nvs[i] = v
			}

			nr.Headers[k] = nvs
		}
	}

	if r.ExpectedStatus != nil {
		nr.ExpectedStatus = make([]int, len(r.ExpectedStatus))

		for i, v := range r.ExpectedStatus {
			nr.ExpectedStatus[i] = v
		}
	}

	return true, nr
}
