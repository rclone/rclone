// Package httplib provides common functionality for http servers
package httplib

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	auth "github.com/abbot/go-http-auth"
	"github.com/ncw/rclone/fs"
)

// Globals
var ()

// Help contains text describing the http server to add to the command
// help.
var Help = `
### Server options

Use --addr to specify which IP address and port the server should
listen on, eg --addr 1.2.3.4:8000 or --addr :8080 to listen to all
IPs.  By default it only listens on localhost.

--server-read-timeout and --server-write-timeout can be used to
control the timeouts on the server.  Note that this is the total time
for a transfer.

--max-header-bytes controls the maximum number of bytes the server will
accept in the HTTP header.

#### Authentication

By default this will serve files without needing a login.

You can either use an htpasswd file which can take lots of users, or
set a single username and password with the --user and --pass flags.

Use --htpasswd /path/to/htpasswd to provide an htpasswd file.  This is
in standard apache format and supports MD5, SHA1 and BCrypt for basic
authentication.  Bcrypt is recommended.

To create an htpasswd file:

    touch htpasswd
    htpasswd -B htpasswd user
    htpasswd -B htpasswd anotherUser

The password file can be updated while rclone is running.

Use --realm to set the authentication realm.

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

// Options contains options for the http Server
type Options struct {
	ListenAddr         string        // Port to listen on
	ServerReadTimeout  time.Duration // Timeout for server reading data
	ServerWriteTimeout time.Duration // Timeout for server writing data
	MaxHeaderBytes     int           // Maximum size of request header
	SslCert            string        // SSL PEM key (concatenation of certificate and CA certificate)
	SslKey             string        // SSL PEM Private key
	ClientCA           string        // Client certificate authority to verify clients with
	HtPasswd           string        // htpasswd file - if not provided no authentication is done
	Realm              string        // realm for authentication
	BasicUser          string        // single username for basic auth if not using Htpasswd
	BasicPass          string        // password for BasicUser
}

// DefaultOpt is the default values used for Options
var DefaultOpt = Options{
	ListenAddr:         "localhost:8080",
	Realm:              "rclone",
	ServerReadTimeout:  1 * time.Hour,
	ServerWriteTimeout: 1 * time.Hour,
	MaxHeaderBytes:     4096,
}

// Server contains info about the running http server
type Server struct {
	Opt             Options
	handler         http.Handler // original handler
	httpServer      *http.Server
	basicPassHashed string
	useSSL          bool // if server is configured for SSL/TLS
}

// singleUserProvider provides the encrypted password for a single user
func (s *Server) singleUserProvider(user, realm string) string {
	if user == s.Opt.BasicUser {
		return s.basicPassHashed
	}
	return ""
}

// NewServer creates an http server.  The opt can be nil in which case
// the default options will be used.
func NewServer(handler http.Handler, opt *Options) *Server {
	s := &Server{
		handler: handler,
	}

	// Make a copy of the options
	if opt != nil {
		s.Opt = *opt
	} else {
		s.Opt = DefaultOpt
	}

	// Use htpasswd if required on everything
	if s.Opt.HtPasswd != "" || s.Opt.BasicUser != "" {
		var secretProvider auth.SecretProvider
		if s.Opt.HtPasswd != "" {
			fs.Infof(nil, "Using %q as htpasswd storage", s.Opt.HtPasswd)
			secretProvider = auth.HtpasswdFileProvider(s.Opt.HtPasswd)
		} else {
			fs.Infof(nil, "Using --user %s --pass XXXX as authenticated user", s.Opt.BasicUser)
			s.basicPassHashed = string(auth.MD5Crypt([]byte(s.Opt.BasicPass), []byte("dlPL2MqE"), []byte("$1$")))
			secretProvider = s.singleUserProvider
		}
		authenticator := auth.NewBasicAuthenticator(s.Opt.Realm, secretProvider)
		handler = auth.JustCheck(authenticator, handler.ServeHTTP)
	}

	s.useSSL = s.Opt.SslKey != ""
	if (s.Opt.SslCert != "") != s.useSSL {
		log.Fatalf("Need both -cert and -key to use SSL")
	}

	// FIXME make a transport?
	s.httpServer = &http.Server{
		Addr:           s.Opt.ListenAddr,
		Handler:        handler,
		ReadTimeout:    s.Opt.ServerReadTimeout,
		WriteTimeout:   s.Opt.ServerWriteTimeout,
		MaxHeaderBytes: s.Opt.MaxHeaderBytes,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS10, // disable SSL v3.0 and earlier
		},
	}
	// go version specific initialisation
	initServer(s.httpServer)

	if s.Opt.ClientCA != "" {
		if !s.useSSL {
			log.Fatalf("Can't use --client-ca without --cert and --key")
		}
		certpool := x509.NewCertPool()
		pem, err := ioutil.ReadFile(s.Opt.ClientCA)
		if err != nil {
			log.Fatalf("Failed to read client certificate authority: %v", err)
		}
		if !certpool.AppendCertsFromPEM(pem) {
			log.Fatalf("Can't parse client certificate authority")
		}
		s.httpServer.TLSConfig.ClientCAs = certpool
		s.httpServer.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return s
}

// Serve runs the server - doesn't return
func (s *Server) Serve() {
	var err error
	if s.useSSL {
		err = s.httpServer.ListenAndServeTLS(s.Opt.SslCert, s.Opt.SslKey)
	} else {
		err = s.httpServer.ListenAndServe()
	}
	log.Printf("Error while serving HTTP: %v", err)
}

// Close shuts the running server down
func (s *Server) Close() {
	err := closeServer(s.httpServer)
	if err != nil {
		log.Printf("Error on closing HTTP server: %v", err)
	}
}

// URL returns the serving address of this server
func (s *Server) URL() string {
	proto := "http"
	if s.useSSL {
		proto = "https"
	}
	return fmt.Sprintf("%s://%s/", proto, s.Opt.ListenAddr)
}
