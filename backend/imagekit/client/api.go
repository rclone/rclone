package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

type ApiError struct {
	Message string            `json:"message"`
	Reason  string            `json:"reason"`
	Errors  map[string]string `json:"errors"`
	err     error             `json:"-"`
}

func (e ApiError) Error() string {
	return e.Message
}

func parseError(r io.ReadCloser, embed error) (err error) {
	var body []byte
	body, err = io.ReadAll(r)

	if err != nil {
		return errors.New("undefined error")
	}

	var ikError = &ApiError{}

	err = json.Unmarshal(body, ikError)
	if err != nil {
		return err
	}

	ikError.err = embed
	return ikError
}

// ParseError returns error object by parsing the http response body if applicable otherwise returns core error such as ErrUnauthorized, ErrServer etc.
func ParseError(resp *http.Response) error {
	var err error
	var code = resp.StatusCode

	if code > 199 && code < 300 {
		return nil
	}

	switch code {
	case 400:
		err = parseError(resp.Body, errors.New("bad request"))
	case 401:
		return errors.New("unauthorized")
	case 403:
		err = parseError(resp.Body, errors.New("forbidden"))
	case 404:
		err = errors.New("not found")
	case 429:
		err = errors.New("too many requests")
	case 500, 502, 503, 504:
		err = errors.New("server error")
	default:
		err = errors.New("undefined error")
	}
	return err
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
