//+build go1.8

package rest

import (
	"net/http"
)

// ClientWithHeaderReset makes a new http client which resets the
// headers passed in on redirect
//
// This is now unecessary with go1.8 so becomes a no-op
func ClientWithHeaderReset(c *http.Client, headers map[string]string) *http.Client {
	return c
}
