package client

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
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
