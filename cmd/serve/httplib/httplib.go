// Package httplib provides common functionality for http servers
package httplib

import (
	"fmt"
	"log"
	"net/http"

	auth "github.com/abbot/go-http-auth"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/pflag"
)

// Globals
var (
	bindAddress  = "localhost:8080"
	htPasswdFile = ""
	realm        = "rclone"
	basicUser    = ""
	basicPass    = ""
)

// AddFlags adds the http server specific flags
func AddFlags(flagSet *pflag.FlagSet) {
	flagSet.StringVarP(&bindAddress, "addr", "", bindAddress, "IPaddress:Port to bind server to.")
	flagSet.StringVarP(&htPasswdFile, "htpasswd", "", htPasswdFile, "File to use for htpasswd authentication.")
	flagSet.StringVarP(&realm, "realm", "", realm, "Realm name for authentication.")
	flagSet.StringVarP(&basicUser, "user", "", basicUser, "User name for authentication.")
	flagSet.StringVarP(&basicPass, "pass", "", basicPass, "Password for authentication.")
}

// Help contains text describing the http server to add to the command
// help.
var Help = `
### Server options

Use --addr to specify which IP address and port the server should
listen on, eg --addr 1.2.3.4:8000 or --addr :8080 to listen to all
IPs.  By default it only listens on localhost.

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

Use --realm to set the authentication realm.`

// Server contains info about the running http server
type Server struct {
	bindAddress     string
	httpServer      *http.Server
	basicUser       string
	basicPassHashed string
}

// singleUserProvider provides the encrypted password for a single user
func (s *Server) singleUserProvider(user, realm string) string {
	if user == s.basicUser {
		return s.basicPassHashed
	}
	return ""
}

// NewServer creates an http server
func NewServer(handler http.Handler) *Server {
	s := &Server{
		bindAddress: bindAddress,
	}

	// Use htpasswd if required on everything
	if htPasswdFile != "" || basicUser != "" {
		var secretProvider auth.SecretProvider
		if htPasswdFile != "" {
			fs.Infof(nil, "Using %q as htpasswd storage", htPasswdFile)
			secretProvider = auth.HtpasswdFileProvider(htPasswdFile)
		} else {
			fs.Infof(nil, "Using --user %s --pass XXXX as authenticated user", basicUser)
			s.basicUser = basicUser
			s.basicPassHashed = string(auth.MD5Crypt([]byte(basicPass), []byte("dlPL2MqE"), []byte("$1$")))
			secretProvider = s.singleUserProvider
		}
		authenticator := auth.NewBasicAuthenticator(realm, secretProvider)
		handler = auth.JustCheck(authenticator, handler.ServeHTTP)
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
