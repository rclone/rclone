// HTTP parts pre go1.8

//+build !go1.8

package http

import (
	"net/http"
)

// Initialise the http.Server for pre go1.8
func initServer(s *http.Server) {
}
