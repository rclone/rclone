//+build !go1.8

package rest

import (
	"net/http"

	"github.com/pkg/errors"
)

// ClientWithHeaderReset makes a new http client which resets the
// headers passed in on redirect
//
// This is only needed for go < go1.8
func ClientWithHeaderReset(c *http.Client, headers map[string]string) *http.Client {
	if len(headers) == 0 {
		return c
	}
	clientCopy := *c
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		// Reset the headers in the new request
		for k, v := range headers {
			if v != "" {
				req.Header.Set(k, v)
			}
		}
		return nil
	}
	return &clientCopy
}
