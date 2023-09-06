// Go 1.0 compatibility functions

//go:build !go1.1
// +build !go1.1

package swift

import (
	"log"
	"net/http"
	"time"
)

// Cancel the request - doesn't work under < go 1.1
func cancelRequest(transport http.RoundTripper, req *http.Request) {
	log.Printf("Tried to cancel a request but couldn't - recompile with go 1.1")
}

// Reset a timer - Doesn't work properly < go 1.1
//
// This is quite hard to do properly under go < 1.1 so we do a crude
// approximation and hope that everyone upgrades to go 1.1 quickly
func resetTimer(t *time.Timer, d time.Duration) {
	t.Stop()
	// Very likely this doesn't actually work if we are already
	// selecting on t.C.  However we've stopped the original timer
	// so won't break transfers but may not time them out :-(
	*t = *time.NewTimer(d)
}
