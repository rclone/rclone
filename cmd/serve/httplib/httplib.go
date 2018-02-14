// Package httplib provides common functionality for http servers
package httplib

import (
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/pflag"
)

// Globals
var (
	bindAddress = "localhost:8080"
)

// AddFlags adds the http server specific flags
func AddFlags(flagSet *pflag.FlagSet) {
	flagSet.StringVarP(&bindAddress, "addr", "", bindAddress, "IPaddress:Port to bind server to.")
}

// Help contains text describing the http server to add to the command
// help.
var Help = `
### Server options

Use --addr to specify which IP address and port the server should
listen on, eg --addr 1.2.3.4:8000 or --addr :8080 to listen to all
IPs.  By default it only listens on localhost.
`

// Server contains info about the running http server
type Server struct {
	bindAddress string
	httpServer  *http.Server
}

// NewServer creates an http server
func NewServer(handler http.Handler) *Server {
	s := &Server{
		bindAddress: bindAddress,
	}
	// FIXME make a transport?
	s.httpServer = &http.Server{
		Addr:           s.bindAddress,
		Handler:        handler,
		MaxHeaderBytes: 1 << 20,
	}
	// go version specific initialisation
	initServer(s.httpServer)
	return s
}

// SetBindAddress overrides the config flag
func (s *Server) SetBindAddress(addr string) {
	s.bindAddress = addr
	s.httpServer.Addr = addr
}

// Serve runs the server - doesn't return
func (s *Server) Serve() {
	log.Fatal(s.httpServer.ListenAndServe())
}

// URL returns the serving address of this server
func (s *Server) URL() string {
	return fmt.Sprintf("http://%s/", s.bindAddress)
}
