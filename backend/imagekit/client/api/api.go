package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// HttpClient interface to provide Do(req *http.Request) method
type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// ResponseMetaData is used in response objects to provide metadata
type ResponseMetaData struct {
	Header     http.Header
	StatusCode int
	Body       []byte
}

// Stringer to get printable metadata
func (rm ResponseMetaData) String() string {
	return fmt.Sprintf("%d\n%s\n%v", rm.StatusCode, string(rm.Body), rm.Header)
}

// Response is promoted struct to response objects
type Response struct {
	ResponseMetaData
}

// SetMeta method assigns given metadata
func (resp *Response) SetMeta(meta ResponseMetaData) {
	resp.ResponseMetaData = meta
}

// Body returns raw http response body
func (resp *Response) Body() []byte {
	return resp.ResponseMetaData.Body
}

// ParseError returns error object by parsing the http response body if applicable otherwise returns core error such as ErrUnauthorized, ErrServer etc.
func (resp *Response) ParseError() error {
	var err error
	var code = resp.ResponseMetaData.StatusCode

	if code > 199 && code < 300 {
		return nil
	}

	switch code {
	case 400:
		err = ParseError(resp.ResponseMetaData.Body, ErrBadRequest)
	case 401:
		return ErrUnauthorized
	case 403:
		err = ParseError(resp.ResponseMetaData.Body, ErrForbidden)
	case 404:
		err = ErrNotFound
	case 429:
		err = ErrTooManyRequests
	case 500, 502, 503, 504:
		err = ErrServer
	default:
		err = ErrUndefined
	}
	return err
}

type ApiError struct {
	Message string            `json:"message"`
	Reason  string            `json:"reason"`
	Errors  map[string]string `json:"errors"`
	err     error             `json:"-"`
}

func (e ApiError) Error() string {
	return e.Message
}

func (e ApiError) Unwrap() error {
	return e.err
}

func ParseError(body []byte, embed error) error {
	var ikError = &ApiError{}

	err := json.Unmarshal(body, ikError)
	if err != nil {
		return err
	}

	ikError.err = embed
	return ikError
}

// MetaSetter is an interface to provide type safety to set meta
type MetaSetter interface {
	ParseError() error
	SetMeta(ResponseMetaData)
}

// base64DataRegex is the regular expression for detecting base64 encoded strings.
var base64DataRegex = regexp.MustCompile("^data:([\\w-]+/[\\w\\-+.]+)?(;[\\w-]+=[\\w-]+)*;base64,([a-zA-Z0-9/+\\n=]+)$")

// StructToParams serializes struct to url.Values, which can be further sent to the http client.
func StructToParams(inputStruct interface{}) (url.Values, error) {
	var paramsMap map[string]interface{}
	paramsJSONObj, _ := json.Marshal(inputStruct)
	err := json.Unmarshal(paramsJSONObj, &paramsMap)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	for paramName, value := range paramsMap {
		kind := reflect.ValueOf(value).Kind()

		if kind == reflect.Slice || kind == reflect.Array {
			rVal := reflect.ValueOf(value)
			for i := 0; i < rVal.Len(); i++ {
				item := rVal.Index(i)
				val, err := encodeParamValue(item.Interface())
				if err != nil {
					return nil, err
				}

				arrParamName := fmt.Sprintf("%s[%d]", paramName, i)
				params.Add(arrParamName, val)
			}

			continue
		}

		val, err := encodeParamValue(value)
		if err != nil {
			return nil, err
		}

		params.Add(paramName, val)
	}

	return params, nil
}

func encodeParamValue(value interface{}) (string, error) {
	resBytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}

	res := string(resBytes)
	if strings.HasPrefix(res, "\"") { // FIXME: Fix this dirty hack that prevents double quoting of strings
		res, _ = strconv.Unquote(res)
	}

	return res, nil
}

// BuildPath builds (joins) the URL path from the provided parts.
func BuildPath(parts ...interface{}) string {
	var partsSlice []string

	for _, part := range parts {
		partRes := ""
		switch partVal := part.(type) {
		case string:
			partRes = partVal
		case fmt.Stringer:
			partRes = partVal.String()
		default:
			partRes = fmt.Sprintf("%v", partVal)
		}
		if len(partRes) > 0 {
			partsSlice = append(partsSlice, strings.Trim(partRes, "/"))
		}
	}

	return strings.Join(partsSlice, "/")
}

// DeferredClose is a wrapper around io.Closer.Close method.
func DeferredClose(c io.Closer) {
	if err := c.Close(); err != nil {
		log.Println(err)
	}
}

// DeferredBodyClose closes http response body
func DeferredBodyClose(resp *http.Response) {
	if resp != nil {
		DeferredClose(resp.Body)
	}
}

// SetResponseMeta assigns given http response data to response objects
func SetResponseMeta(httpResp *http.Response, respStruct MetaSetter) {
	if httpResp == nil {
		return
	}

	meta := ResponseMetaData{
		Header:     httpResp.Header,
		StatusCode: httpResp.StatusCode,
	}

	if body, err := io.ReadAll(httpResp.Body); err == nil {
		meta.Body = body
	}
	respStruct.SetMeta(meta)
}

func Bool(b bool) *bool {
	return &b
}

func Int(i int) *int {
	return &i
}

func Float32(f float32) *float32 {
	return &f
}