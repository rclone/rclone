// HTTP parts go1.8+

//+build go1.8

package http

import (
	"net/http"
	"time"
)

// Initialise the http.Server for pre go1.8
func initServer(s *http.Server) {
	s.ReadHeaderTimeout = 10 * time.Second // time to send the headers
	s.IdleTimeout = 60 * time.Second       // time to keep idle connections open
}
