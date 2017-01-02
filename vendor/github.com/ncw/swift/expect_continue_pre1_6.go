// Show that we can't use `Expect: 100-continue` in go versions < 1.6
//
// +build !go1.6

package swift

import (
	"net/http"
	"time"
)

// DisableExpectContinue indicates whether this version of go supports
// expect continue.
const DisableExpectContinue = true

// SetExpectContinueTimeout sets ExpectContinueTimeout in the
// transport to the default value if not set.
func SetExpectContinueTimeout(transport *http.Transport, t time.Duration) {
	// can't do anything so ignore
}
