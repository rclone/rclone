// Go 1.1 and later compatibility functions
//
//go:build go1.1
// +build go1.1

package swift

import (
	"net/http"
	"time"
)

// Cancel the request
func cancelRequest(transport http.RoundTripper, req *http.Request) {
	if tr, ok := transport.(interface {
		CancelRequest(*http.Request)
	}); ok {
		tr.CancelRequest(req)
	}
}

// Reset a timer
func resetTimer(t *time.Timer, d time.Duration) {
	t.Reset(d)
}
