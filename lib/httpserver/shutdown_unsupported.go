// +build go1.7,!go1.8

package httpserver

import "net/http"

func stopServer(server *http.Server) {}
