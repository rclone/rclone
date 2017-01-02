// Show that we can use `Expect: 100-continue` in go versions >= 1.6
//
// +build go1.6

package swift

import (
	"net/http"
	"time"
)

// DisableExpectContinue indicates whether this version of go supports
// expect continue.
const DisableExpectContinue = false

// SetExpectContinueTimeout sets ExpectContinueTimeout in the
// transport to the default value if not set.
func SetExpectContinueTimeout(transport *http.Transport, t time.Duration) {
	if transport.ExpectContinueTimeout == 0 {
		transport.ExpectContinueTimeout = t
	}
}
