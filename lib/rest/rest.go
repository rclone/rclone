// Package rest implements a simple REST wrapper
//
// All methods are safe for concurrent calling.
package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"sync"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/readers"
)

// Client contains the info to sustain the API
type Client struct {
	mu           sync.RWMutex
	c            *http.Client
	rootURL      string
	errorHandler func(resp *http.Response) error
	headers      map[string]string
	signer       SignerFn
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
// returns it in the error message closing resp.Body
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
// Start the key with "*" for don't canonicalise
func (api *Client) SetHeader(key, value string) *Client {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.headers[key] = value
	return api
}

// RemoveHeader unsets a header for all requests
func (api *Client) RemoveHeader(key string) *Client {
	api.mu.Lock()
	defer api.mu.Unlock()
	delete(api.headers, key)
	return api
}

// SignerFn is used to sign an outgoing request
type SignerFn func(*http.Request) error

// SetSigner sets a signer for all requests
func (api *Client) SetSigner(signer SignerFn) *Client {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.signer = signer
	return api
}

// SetUserPass creates an Authorization header for all requests with
// the UserName and Password passed in
func (api *Client) SetUserPass(UserName, Password string) *Client {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.SetBasicAuth(UserName, Password)
	api.SetHeader("Authorization", req.Header.Get("Authorization"))
	return api
}

// SetCookie creates a Cookies Header for all requests with the supplied
// cookies passed in.
// All cookies have to be supplied at once, all cookies will be overwritten
// on a new call to the method
func (api *Client) SetCookie(cks ...*http.Cookie) *Client {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	for _, ck := range cks {
		req.AddCookie(ck)
	}
	api.SetHeader("Cookie", req.Header.Get("Cookie"))
	return api
}

// Opts contains parameters for Call, CallJSON, etc.
type Opts struct {
	Method                string // GET, POST, etc.
	Path                  string // relative to RootURL
	RootURL               string // override RootURL passed into SetRoot()
	Body                  io.Reader
	NoResponse            bool // set to close Body
	ContentType           string
	ContentLength         *int64
	ContentRange          string
	ExtraHeaders          map[string]string // extra headers, start them with "*" for don't canonicalise
	UserName              string            // username for Basic Auth
	Password              string            // password for Basic Auth
	Options               []fs.OpenOption
	IgnoreStatus          bool       // if set then we don't check error status or parse error body
	MultipartParams       url.Values // if set do multipart form upload with attached file
	MultipartMetadataName string     // ..this is used for the name of the metadata form part if set
	MultipartContentName  string     // ..name of the parameter which is the attached file
	MultipartFileName     string     // ..name of the file for the attached file
	Parameters            url.Values // any parameters for the final URL
	TransferEncoding      []string   // transfer encoding, set to "identity" to disable chunked encoding
	Close                 bool       // set to close the connection after this transaction
	NoRedirect            bool       // if this is set then the client won't follow redirects
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

// DecodeXML decodes resp.Body into result
func DecodeXML(resp *http.Response, result interface{}) (err error) {
	defer fs.CheckClose(resp.Body, &err)
	decoder := xml.NewDecoder(resp.Body)
	return decoder.Decode(result)
}

// ClientWithNoRedirects makes a new http client which won't follow redirects
func ClientWithNoRedirects(c *http.Client) *http.Client {
	clientCopy := *c
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &clientCopy
}

// Call makes the call and returns the http.Response
//
// if err == nil then resp.Body will need to be closed unless
// opt.NoResponse is set
//
// if err != nil then resp.Body will have been closed
//
// it will return resp if at all possible, even if err is set
func (api *Client) Call(ctx context.Context, opts *Opts) (resp *http.Response, err error) {
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
	body := readers.NoCloser(opts.Body)
	// If length is set and zero then nil out the body to stop use
	// use of chunked encoding and insert a "Content-Length: 0"
	// header.
	//
	// If we don't do this we get "Content-Length" headers for all
	// files except 0 length files.
	if opts.ContentLength != nil && *opts.ContentLength == 0 {
		body = nil
	}
	req, err := http.NewRequest(opts.Method, url, body)
	if err != nil {
		return
	}
	req = req.WithContext(ctx) // go1.13 can use NewRequestWithContext
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
	if len(opts.TransferEncoding) != 0 {
		req.TransferEncoding = opts.TransferEncoding
	}
	if opts.Close {
		req.Close = true
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
		if k != "" && v != "" {
			if k[0] == '*' {
				// Add non-canonical version if header starts with *
				k = k[1:]
				req.Header[k] = append(req.Header[k], v)
			} else {
				req.Header.Add(k, v)
			}
		}
	}

	if opts.UserName != "" || opts.Password != "" {
		req.SetBasicAuth(opts.UserName, opts.Password)
	}
	var c *http.Client
	if opts.NoRedirect {
		c = ClientWithNoRedirects(api.c)
	} else {
		c = api.c
	}
	if api.signer != nil {
		api.mu.RUnlock()
		err = api.signer(req)
		api.mu.RLock()
		if err != nil {
			return nil, errors.Wrap(err, "signer failed")
		}
	}
	api.mu.RUnlock()
	resp, err = c.Do(req)
	api.mu.RLock()
	if err != nil {
		return nil, err
	}
	if !opts.IgnoreStatus {
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			err = api.errorHandler(resp)
			if err.Error() == "" {
				// replace empty errors with something
				err = errors.Errorf("http error %d: %v", resp.StatusCode, resp.Status)
			}
			return resp, err
		}
	}
	if opts.NoResponse {
		return resp, resp.Body.Close()
	}
	return resp, nil
}

// MultipartUpload creates an io.Reader which produces an encoded a
// multipart form upload from the params passed in and the  passed in
//
// in - the body of the file (may be nil)
// params - the form parameters
// fileName - is the name of the attached file
// contentName - the name of the parameter for the file
//
// the int64 returned is the overhead in addition to the file contents, in case Content-Length is required
//
// NB This doesn't allow setting the content type of the attachment
func MultipartUpload(in io.Reader, params url.Values, contentName, fileName string) (io.ReadCloser, string, int64, error) {
	bodyReader, bodyWriter := io.Pipe()
	writer := multipart.NewWriter(bodyWriter)
	contentType := writer.FormDataContentType()

	// Create a Multipart Writer as base for calculating the Content-Length
	buf := &bytes.Buffer{}
	dummyMultipartWriter := multipart.NewWriter(buf)
	err := dummyMultipartWriter.SetBoundary(writer.Boundary())
	if err != nil {
		return nil, "", 0, err
	}

	for key, vals := range params {
		for _, val := range vals {
			err := dummyMultipartWriter.WriteField(key, val)
			if err != nil {
				return nil, "", 0, err
			}
		}
	}
	if in != nil {
		_, err = dummyMultipartWriter.CreateFormFile(contentName, fileName)
		if err != nil {
			return nil, "", 0, err
		}
	}

	err = dummyMultipartWriter.Close()
	if err != nil {
		return nil, "", 0, err
	}

	multipartLength := int64(buf.Len())

	// Pump the data in the background
	go func() {
		var err error

		for key, vals := range params {
			for _, val := range vals {
				err = writer.WriteField(key, val)
				if err != nil {
					_ = bodyWriter.CloseWithError(errors.Wrap(err, "create metadata part"))
					return
				}
			}
		}

		if in != nil {
			part, err := writer.CreateFormFile(contentName, fileName)
			if err != nil {
				_ = bodyWriter.CloseWithError(errors.Wrap(err, "failed to create form file"))
				return
			}

			_, err = io.Copy(part, in)
			if err != nil {
				_ = bodyWriter.CloseWithError(errors.Wrap(err, "failed to copy data"))
				return
			}
		}

		err = writer.Close()
		if err != nil {
			_ = bodyWriter.CloseWithError(errors.Wrap(err, "failed to close form"))
			return
		}

		_ = bodyWriter.Close()
	}()

	return bodyReader, contentType, multipartLength, nil
}

// CallJSON runs Call and decodes the body as a JSON object into response (if not nil)
//
// If request is not nil then it will be JSON encoded as the body of the request
//
// If response is not nil then the response will be JSON decoded into
// it and resp.Body will be closed.
//
// If response is nil then the resp.Body will be closed only if
// opts.NoResponse is set.
//
// If (opts.MultipartParams or opts.MultipartContentName) and
// opts.Body are set then CallJSON will do a multipart upload with a
// file attached.  opts.MultipartContentName is the name of the
// parameter and opts.MultipartFileName is the name of the file.  If
// MultpartContentName is set, and request != nil is supplied, then
// the request will be marshalled into JSON and added to the form with
// parameter name MultipartMetadataName.
//
// It will return resp if at all possible, even if err is set
func (api *Client) CallJSON(ctx context.Context, opts *Opts, request interface{}, response interface{}) (resp *http.Response, err error) {
	return api.callCodec(ctx, opts, request, response, json.Marshal, DecodeJSON, "application/json")
}

// CallXML runs Call and decodes the body as an XML object into response (if not nil)
//
// If request is not nil then it will be XML encoded as the body of the request
//
// If response is not nil then the response will be XML decoded into
// it and resp.Body will be closed.
//
// If response is nil then the resp.Body will be closed only if
// opts.NoResponse is set.
//
// See CallJSON for a description of MultipartParams and related opts
//
// It will return resp if at all possible, even if err is set
func (api *Client) CallXML(ctx context.Context, opts *Opts, request interface{}, response interface{}) (resp *http.Response, err error) {
	return api.callCodec(ctx, opts, request, response, xml.Marshal, DecodeXML, "application/xml")
}

type marshalFn func(v interface{}) ([]byte, error)
type decodeFn func(resp *http.Response, result interface{}) (err error)

func (api *Client) callCodec(ctx context.Context, opts *Opts, request interface{}, response interface{}, marshal marshalFn, decode decodeFn, contentType string) (resp *http.Response, err error) {
	var requestBody []byte
	// Marshal the request if given
	if request != nil {
		requestBody, err = marshal(request)
		if err != nil {
			return nil, err
		}
		// Set the body up as a marshalled object if no body passed in
		if opts.Body == nil {
			opts = opts.Copy()
			opts.ContentType = contentType
			opts.Body = bytes.NewBuffer(requestBody)
		}
	}
	if opts.MultipartParams != nil || opts.MultipartContentName != "" {
		params := opts.MultipartParams
		if params == nil {
			params = url.Values{}
		}
		if opts.MultipartMetadataName != "" {
			params.Add(opts.MultipartMetadataName, string(requestBody))
		}
		opts = opts.Copy()

		var overhead int64
		opts.Body, opts.ContentType, overhead, err = MultipartUpload(opts.Body, params, opts.MultipartContentName, opts.MultipartFileName)
		if err != nil {
			return nil, err
		}
		if opts.ContentLength != nil {
			*opts.ContentLength += overhead
		}
	}
	resp, err = api.Call(ctx, opts)
	if err != nil {
		return resp, err
	}
	// if opts.NoResponse is set, resp.Body will have been closed by Call()
	if response == nil || opts.NoResponse {
		return resp, nil
	}
	err = decode(resp, response)
	return resp, err
}
