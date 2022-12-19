package restic

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// declare a few helper functions

// wantFunc tests the HTTP response in res and marks the test as errored if something is incorrect.
type wantFunc func(t testing.TB, res *httptest.ResponseRecorder)

// newRequest returns a new HTTP request with the given params
func newRequest(t testing.TB, method, path string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	req.Header.Add("Accept", resticAPIV2)
	return req
}

// wantCode returns a function which checks that the response has the correct HTTP status code.
func wantCode(code int) wantFunc {
	return func(t testing.TB, res *httptest.ResponseRecorder) {
		assert.Equal(t, code, res.Code)
	}
}

// wantBody returns a function which checks that the response has the data in the body.
func wantBody(body string) wantFunc {
	return func(t testing.TB, res *httptest.ResponseRecorder) {
		assert.NotNil(t, res.Body)
		assert.Equal(t, res.Body.Bytes(), []byte(body))
	}
}

// checkRequest uses f to process the request and runs the checker functions on the result.
func checkRequest(t testing.TB, f http.HandlerFunc, req *http.Request, want []wantFunc) {
	rr := httptest.NewRecorder()
	f(rr, req)

	for _, fn := range want {
		fn(t, rr)
	}
}

// TestRequest is a sequence of HTTP requests with (optional) tests for the response.
type TestRequest struct {
	req  *http.Request
	want []wantFunc
}
