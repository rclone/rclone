package fshttp

import (
	"fmt"
	"io"
	"net/http"
	"sync"
)

// FaultInjector decides whether to fail req instead of sending it.
//
// It returns a non-zero HTTP status code to synthesise an error
// response, a non-nil error to fail the request at the transport, or
// (0, nil) to send the request normally. When a fault is injected the
// request body is drained and closed as it would be by a real round
// trip, but nothing is sent to the server.
type FaultInjector func(req *http.Request) (statusCode int, err error)

var (
	faultInjectorMu sync.RWMutex
	faultInjector   FaultInjector
)

// SetFaultInjector installs f as the fault injector for every Transport,
// or removes it if f is nil.
//
// This is intended for tests which need to check how callers cope with
// transient HTTP failures, such as whether an upload is retried
// correctly after a 5xx.
func SetFaultInjector(f FaultInjector) {
	faultInjectorMu.Lock()
	defer faultInjectorMu.Unlock()
	faultInjector = f
}

// injectFault consults the fault injector and returns the synthesised
// response or error for req, or (nil, nil) if it should be sent.
func injectFault(req *http.Request) (*http.Response, error) {
	faultInjectorMu.RLock()
	f := faultInjector
	faultInjectorMu.RUnlock()
	if f == nil {
		return nil, nil
	}
	statusCode, err := f(req)
	if statusCode == 0 && err == nil {
		return nil, nil
	}
	if req.Body != nil {
		_, _ = io.Copy(io.Discard, req.Body)
		_ = req.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		StatusCode:    statusCode,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          http.NoBody,
		ContentLength: 0,
		Request:       req,
	}, nil
}
