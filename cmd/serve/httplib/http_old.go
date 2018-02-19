// HTTP parts pre go1.8

//+build !go1.8

package httplib

import (
	"net/http"
)

// Initialise the http.Server for pre go1.8
func initServer(s *http.Server) {
}

// closeServer closes the server in a non graceful way
func closeServer(s *http.Server) error {
	return nil
}
