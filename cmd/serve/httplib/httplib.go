// Package httplib provides common functionality for http servers
package httplib

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	auth "github.com/abbot/go-http-auth"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd/serve/httplib/serve/data"
	"github.com/rclone/rclone/fs"
)

// Globals
var ()

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
	listener        net.Listener
	waitChan        chan struct{} // for waiting on the listener to close
	httpServer      *http.Server
	basicPassHashed string
	useSSL          bool               // if server is configured for SSL/TLS
	usingAuth       bool               // set if authentication is configured
	HTMLTemplate    *template.Template // HTML template for web interface
}

type contextUserType struct{}

// ContextUserKey is a simple context key
var ContextUserKey = &contextUserType{}

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
		oldHandler := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// No auth wanted for OPTIONS method
			if r.Method == "OPTIONS" {
				oldHandler.ServeHTTP(w, r)
				return
			}
			if username := authenticator.CheckAuth(r); username == "" {
				authHeader := r.Header.Get(authenticator.Headers.V().Authorization)
				if authHeader != "" {
					s := strings.SplitN(authHeader, " ", 2)
					var userName = "UNKNOWN"
					if len(s) == 2 && s[0] == "Basic" {
						b, err := base64.StdEncoding.DecodeString(s[1])
						if err == nil {
							userName = strings.SplitN(string(b), ":", 2)[0]
						}
					}
					fs.Infof(r.URL.Path, "%s: Unauthorized request from %s", r.RemoteAddr, userName)
				} else {
					fs.Infof(r.URL.Path, "%s: Basic auth challenge sent", r.RemoteAddr)
				}
				authenticator.RequireAuth(w, r)
			} else {
				r = r.WithContext(context.WithValue(r.Context(), ContextUserKey, username))
				oldHandler.ServeHTTP(w, r)
			}
		})
		s.usingAuth = true
	}

	s.useSSL = s.Opt.SslKey != ""
	if (s.Opt.SslCert != "") != s.useSSL {
		log.Fatalf("Need both -cert and -key to use SSL")
	}

	// FIXME make a transport?
	s.httpServer = &http.Server{
		Addr:              s.Opt.ListenAddr,
		Handler:           handler,
		ReadTimeout:       s.Opt.ServerReadTimeout,
		WriteTimeout:      s.Opt.ServerWriteTimeout,
		MaxHeaderBytes:    s.Opt.MaxHeaderBytes,
		ReadHeaderTimeout: 10 * time.Second, // time to send the headers
		IdleTimeout:       60 * time.Second, // time to keep idle connections open
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS10, // disable SSL v3.0 and earlier
		},
	}

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

	htmlTemplate, templateErr := data.GetTemplate()
	if templateErr != nil {
		log.Fatalf(templateErr.Error())
	}
	s.HTMLTemplate = htmlTemplate

	return s
}

// Serve runs the server - returns an error only if
// the listener was not started; does not block, so
// use s.Wait() to block on the listener indefinitely.
func (s *Server) Serve() error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return errors.Wrapf(err, "start server failed")
	}
	s.listener = ln
	s.waitChan = make(chan struct{})
	go func() {
		var err error
		if s.useSSL {
			// hacky hack to get this to work with old Go versions, which
			// don't have ServeTLS on http.Server; see PR #2194.
			type tlsServer interface {
				ServeTLS(ln net.Listener, cert, key string) error
			}
			srvIface := interface{}(s.httpServer)
			if tlsSrv, ok := srvIface.(tlsServer); ok {
				// yay -- we get easy TLS support with HTTP/2
				err = tlsSrv.ServeTLS(s.listener, s.Opt.SslCert, s.Opt.SslKey)
			} else {
				// oh well -- we can still do TLS but might not have HTTP/2
				tlsConfig := new(tls.Config)
				tlsConfig.Certificates = make([]tls.Certificate, 1)
				tlsConfig.Certificates[0], err = tls.LoadX509KeyPair(s.Opt.SslCert, s.Opt.SslKey)
				if err != nil {
					log.Printf("Error loading key pair: %v", err)
				}
				tlsLn := tls.NewListener(s.listener, tlsConfig)
				err = s.httpServer.Serve(tlsLn)
			}
		} else {
			err = s.httpServer.Serve(s.listener)
		}
		if err != nil {
			log.Printf("Error on serving HTTP server: %v", err)
		}
	}()
	return nil
}

// Wait blocks while the listener is open.
func (s *Server) Wait() {
	<-s.waitChan
}

// Close shuts the running server down
func (s *Server) Close() {
	err := s.httpServer.Close()
	if err != nil {
		log.Printf("Error on closing HTTP server: %v", err)
		return
	}
	close(s.waitChan)
}

// URL returns the serving address of this server
func (s *Server) URL() string {
	proto := "http"
	if s.useSSL {
		proto = "https"
	}
	addr := s.Opt.ListenAddr
	if s.listener != nil {
		// prefer actual listener address; required if using 0-port
		// (i.e. port assigned by operating system)
		addr = s.listener.Addr().String()
	}
	return fmt.Sprintf("%s://%s/", proto, addr)
}

// UsingAuth returns true if authentication is required
func (s *Server) UsingAuth() bool {
	return s.usingAuth
}
