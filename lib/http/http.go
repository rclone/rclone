// Package http provides a registration interface for http services
package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/spf13/pflag"
)

// Help contains text describing the http server to add to the command
// help.
var Help = `
### Server options

Use --addr to specify which IP address and port the server should
listen on, eg --addr 1.2.3.4:8000 or --addr :8080 to listen to all
IPs.  By default it only listens on localhost.  You can use port
:0 to let the OS choose an available port.

If you set --addr to listen on a public or LAN accessible IP address
then using Authentication is advised - see the next section for info.

--server-read-timeout and --server-write-timeout can be used to
control the timeouts on the server.  Note that this is the total time
for a transfer.

--max-header-bytes controls the maximum number of bytes the server will
accept in the HTTP header.

--baseurl controls the URL prefix that rclone serves from.  By default
rclone will serve from the root.  If you used --baseurl "/rclone" then
rclone would serve from a URL starting with "/rclone/".  This is
useful if you wish to proxy rclone serve.  Rclone automatically
inserts leading and trailing "/" on --baseurl, so --baseurl "rclone",
--baseurl "/rclone" and --baseurl "/rclone/" are all treated
identically.

#### SSL/TLS

By default this will serve over http.  If you want you can serve over
https.  You will need to supply the --cert and --key flags.  If you
wish to do client side certificate validation then you will need to
supply --client-ca also.

--cert should be a either a PEM encoded certificate or a concatenation
of that with the CA certificate.  --key should be the PEM encoded
private key and --client-ca should be the PEM encoded client
certificate authority certificate.
`

// Middleware function signature required by chi.Router.Use()
type Middleware func(http.Handler) http.Handler

// Options contains options for the http Server
type Options struct {
	ListenAddr         string        // Port to listen on
	BaseURL            string        // prefix to strip from URLs
	ServerReadTimeout  time.Duration // Timeout for server reading data
	ServerWriteTimeout time.Duration // Timeout for server writing data
	MaxHeaderBytes     int           // Maximum size of request header
	SslCert            string        // SSL PEM key (concatenation of certificate and CA certificate)
	SslKey             string        // SSL PEM Private key
	ClientCA           string        // Client certificate authority to verify clients with
}

// DefaultOpt is the default values used for Options
var DefaultOpt = Options{
	ListenAddr:         "127.0.0.1:8080",
	ServerReadTimeout:  1 * time.Hour,
	ServerWriteTimeout: 1 * time.Hour,
	MaxHeaderBytes:     4096,
}

// Server interface of http server
type Server interface {
	Router() chi.Router
	Route(pattern string, fn func(r chi.Router)) chi.Router
	Mount(pattern string, h http.Handler)
	Shutdown() error
}

type server struct {
	addrs        []net.Addr
	tlsAddrs     []net.Addr
	listeners    []net.Listener
	tlsListeners []net.Listener
	httpServer   *http.Server
	baseRouter   chi.Router
	closing      *sync.WaitGroup
	useSSL       bool
}

var (
	defaultServer        *server
	defaultServerOptions = DefaultOpt
	defaultServerMutex   sync.Mutex
)

func useSSL(opt Options) bool {
	return opt.SslKey != ""
}

// NewServer instantiates a new http server using provided listeners and options
// This function is provided if the default http server does not meet a services requirements and should not generally be used
// A http server can listen using multiple listeners. For example, a listener for port 80, and a listener for port 443.
// tlsListeners are ignored if opt.SslKey is not provided
func NewServer(listeners, tlsListeners []net.Listener, opt Options) (Server, error) {
	// Validate input
	if len(listeners) == 0 && len(tlsListeners) == 0 {
		return nil, errors.New("Can't create server without listeners")
	}

	// Prepare TLS config
	var tlsConfig *tls.Config = nil

	useSSL := useSSL(opt)
	if (opt.SslCert != "") != useSSL {
		err := errors.New("Need both -cert and -key to use SSL")
		log.Fatalf(err.Error())
		return nil, err
	}

	if useSSL {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS10, // disable SSL v3.0 and earlier
		}
	} else if len(listeners) == 0 && len(tlsListeners) != 0 {
		return nil, errors.New("No SslKey or non-tlsListeners")
	}

	if opt.ClientCA != "" {
		if !useSSL {
			err := errors.New("Can't use --client-ca without --cert and --key")
			log.Fatalf(err.Error())
			return nil, err
		}
		certpool := x509.NewCertPool()
		pem, err := ioutil.ReadFile(opt.ClientCA)
		if err != nil {
			log.Fatalf("Failed to read client certificate authority: %v", err)
			return nil, err
		}
		if !certpool.AppendCertsFromPEM(pem) {
			err := errors.New("Can't parse client certificate authority")
			log.Fatalf(err.Error())
			return nil, err
		}
		tlsConfig.ClientCAs = certpool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	// Ignore passing "/" for BaseURL
	opt.BaseURL = strings.Trim(opt.BaseURL, "/")
	if opt.BaseURL != "" {
		opt.BaseURL = "/" + opt.BaseURL
	}

	// Build base router
	var router chi.Router = chi.NewRouter()
	router.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	})
	router.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	})

	handler := router.(http.Handler)
	if opt.BaseURL != "" {
		handler = http.StripPrefix(opt.BaseURL, handler)
	}

	// Serve on listeners
	httpServer := &http.Server{
		Handler:           handler,
		ReadTimeout:       opt.ServerReadTimeout,
		WriteTimeout:      opt.ServerWriteTimeout,
		MaxHeaderBytes:    opt.MaxHeaderBytes,
		ReadHeaderTimeout: 10 * time.Second, // time to send the headers
		IdleTimeout:       60 * time.Second, // time to keep idle connections open
		TLSConfig:         tlsConfig,
	}

	addrs, tlsAddrs := make([]net.Addr, len(listeners)), make([]net.Addr, len(tlsListeners))

	wg := &sync.WaitGroup{}

	for i, l := range listeners {
		addrs[i] = l.Addr()
	}

	if useSSL {
		for i, l := range tlsListeners {
			tlsAddrs[i] = l.Addr()
		}
	}

	return &server{addrs, tlsAddrs, listeners, tlsListeners, httpServer, router, wg, useSSL}, nil
}

func (s *server) Serve() {
	serve := func(l net.Listener) {
		defer s.closing.Done()
		if err := s.httpServer.Serve(l); err != http.ErrServerClosed && err != nil {
			log.Fatalf(err.Error())
		}
	}

	s.closing.Add(len(s.listeners))
	for _, l := range s.listeners {
		go serve(l)
	}

	if s.useSSL {
		s.closing.Add(len(s.tlsListeners))
		for _, l := range s.tlsListeners {
			go serve(l)
		}
	}
}

// Router returns the server base router
func (s *server) Router() chi.Router {
	return s.baseRouter
}

// Route mounts a sub-Router along a `pattern` string.
func (s *server) Route(pattern string, fn func(r chi.Router)) chi.Router {
	return s.baseRouter.Route(pattern, fn)
}

// Mount attaches another http.Handler along ./pattern/*
func (s *server) Mount(pattern string, h http.Handler) {
	s.baseRouter.Mount(pattern, h)
}

// Shutdown gracefully shuts down the server
func (s *server) Shutdown() error {
	if err := s.httpServer.Shutdown(context.Background()); err != nil {
		return err
	}
	s.closing.Wait()
	return nil
}

//---- Default HTTP server convenience functions ----

// Router returns the server base router
func Router() (chi.Router, error) {
	if err := start(); err != nil {
		return nil, err
	}
	return defaultServer.baseRouter, nil
}

// Route mounts a sub-Router along a `pattern` string.
func Route(pattern string, fn func(r chi.Router)) (chi.Router, error) {
	if err := start(); err != nil {
		return nil, err
	}
	return defaultServer.Route(pattern, fn), nil
}

// Mount attaches another http.Handler along ./pattern/*
func Mount(pattern string, h http.Handler) error {
	if err := start(); err != nil {
		return err
	}
	defaultServer.Mount(pattern, h)
	return nil
}

// Restart or start the default http server using the default options and no handlers
func Restart() error {
	if e := Shutdown(); e != nil {
		return e
	}

	return start()
}

// Start the default server
func start() error {
	defaultServerMutex.Lock()
	defer defaultServerMutex.Unlock()

	if defaultServer != nil {
		// Server already started, do nothing
		return nil
	}

	var err error
	var l net.Listener
	l, err = net.Listen("tcp", defaultServerOptions.ListenAddr)
	if err != nil {
		return err
	}

	var s Server
	if useSSL(defaultServerOptions) {
		s, err = NewServer([]net.Listener{}, []net.Listener{l}, defaultServerOptions)
	} else {
		s, err = NewServer([]net.Listener{l}, []net.Listener{}, defaultServerOptions)
	}
	if err != nil {
		return err
	}
	defaultServer = s.(*server)
	defaultServer.Serve()
	return nil
}

// Shutdown gracefully shuts down the default http server
func Shutdown() error {
	defaultServerMutex.Lock()
	defer defaultServerMutex.Unlock()
	if defaultServer != nil {
		s := defaultServer
		defaultServer = nil
		return s.Shutdown()
	}
	return nil
}

// GetOptions thread safe getter for the default server options
func GetOptions() Options {
	defaultServerMutex.Lock()
	defer defaultServerMutex.Unlock()
	return defaultServerOptions
}

// SetOptions thread safe setter for the default server options
func SetOptions(opt Options) {
	defaultServerMutex.Lock()
	defer defaultServerMutex.Unlock()
	defaultServerOptions = opt
}

//---- Utility functions ----

// URL of default http server
func URL() string {
	if defaultServer == nil {
		panic("Server not running")
	}
	for _, a := range defaultServer.addrs {
		return fmt.Sprintf("http://%s%s/", a.String(), defaultServerOptions.BaseURL)
	}
	for _, a := range defaultServer.tlsAddrs {
		return fmt.Sprintf("https://%s%s/", a.String(), defaultServerOptions.BaseURL)
	}
	panic("Server is running with no listener")
}

//---- Command line flags ----

// AddFlagsPrefix adds flags for the httplib
func AddFlagsPrefix(flagSet *pflag.FlagSet, prefix string, Opt *Options) {
	flags.StringVarP(flagSet, &Opt.ListenAddr, prefix+"addr", "", Opt.ListenAddr, "IPaddress:Port or :Port to bind server to.")
	flags.DurationVarP(flagSet, &Opt.ServerReadTimeout, prefix+"server-read-timeout", "", Opt.ServerReadTimeout, "Timeout for server reading data")
	flags.DurationVarP(flagSet, &Opt.ServerWriteTimeout, prefix+"server-write-timeout", "", Opt.ServerWriteTimeout, "Timeout for server writing data")
	flags.IntVarP(flagSet, &Opt.MaxHeaderBytes, prefix+"max-header-bytes", "", Opt.MaxHeaderBytes, "Maximum size of request header")
	flags.StringVarP(flagSet, &Opt.SslCert, prefix+"cert", "", Opt.SslCert, "SSL PEM key (concatenation of certificate and CA certificate)")
	flags.StringVarP(flagSet, &Opt.SslKey, prefix+"key", "", Opt.SslKey, "SSL PEM Private key")
	flags.StringVarP(flagSet, &Opt.ClientCA, prefix+"client-ca", "", Opt.ClientCA, "Client certificate authority to verify clients with")
	flags.StringVarP(flagSet, &Opt.BaseURL, prefix+"baseurl", "", Opt.BaseURL, "Prefix for URLs - leave blank for root.")

}

// AddFlags adds flags for the httplib
func AddFlags(flagSet *pflag.FlagSet) {
	AddFlagsPrefix(flagSet, "", &defaultServerOptions)
}
