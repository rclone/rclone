package http

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"

	goauth "github.com/abbot/go-http-auth"
	"github.com/rclone/rclone/fs"
)

// parseAuthorization parses the Authorization header into user, pass
// it returns a boolean as to whether the parse was successful
func parseAuthorization(r *http.Request) (user, pass string, ok bool) {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		s := strings.SplitN(authHeader, " ", 2)
		if len(s) == 2 && s[0] == "Basic" {
			b, err := base64.StdEncoding.DecodeString(s[1])
			if err == nil {
				parts := strings.SplitN(string(b), ":", 2)
				user = parts[0]
				if len(parts) > 1 {
					pass = parts[1]
					ok = true
				}
			}
		}
	}
	return
}

// LoggedBasicAuth simply wraps the goauth.BasicAuth struct
type LoggedBasicAuth struct {
	goauth.BasicAuth
}

// CheckAuth extends BasicAuth.CheckAuth to emit a log entry for unauthorised requests
func (a *LoggedBasicAuth) CheckAuth(r *http.Request) string {
	username := a.BasicAuth.CheckAuth(r)
	if username == "" {
		user, _, _ := parseAuthorization(r)
		fs.Infof(r.URL.Path, "%s: Unauthorized request from %s", r.RemoteAddr, user)
	}
	return username
}

// NewLoggedBasicAuthenticator instantiates a new instance of LoggedBasicAuthenticator
func NewLoggedBasicAuthenticator(realm string, secrets goauth.SecretProvider) *LoggedBasicAuth {
	return &LoggedBasicAuth{BasicAuth: goauth.BasicAuth{Realm: realm, Secrets: secrets}}
}

// Helper to generate required interface for middleware
func basicAuth(authenticator *LoggedBasicAuth) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// skip auth for unix socket
			if IsUnixSocket(r) {
				next.ServeHTTP(w, r)
				return
			}
			// skip auth for CORS preflight
			if r.Method == "OPTIONS" {
				next.ServeHTTP(w, r)
				return
			}

			username := authenticator.CheckAuth(r)
			if username == "" {
				authenticator.RequireAuth(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKeyUser, username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MiddlewareAuthCertificateUser instantiates middleware that extracts the authenticated user via client certificate common name
func MiddlewareAuthCertificateUser() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, cert := range r.TLS.PeerCertificates {
				if cert.Subject.CommonName != "" {
					r = r.WithContext(context.WithValue(r.Context(), ctxKeyUser, cert.Subject.CommonName))
					next.ServeHTTP(w, r)
					return
				}
			}
			code := http.StatusUnauthorized
			w.Header().Set("Content-Type", "text/plain")
			http.Error(w, http.StatusText(code), code)
		})
	}
}

// MiddlewareAuthHtpasswd instantiates middleware that authenticates against the passed htpasswd file
func MiddlewareAuthHtpasswd(path, realm string) Middleware {
	fs.Infof(nil, "Using %q as htpasswd storage", path)
	secretProvider := goauth.HtpasswdFileProvider(path)
	authenticator := NewLoggedBasicAuthenticator(realm, secretProvider)
	return basicAuth(authenticator)
}

// MiddlewareAuthBasic instantiates middleware that authenticates for a single user
func MiddlewareAuthBasic(user, pass, realm, salt string) Middleware {
	fs.Infof(nil, "Using --user %s --pass XXXX as authenticated user", user)
	pass = string(goauth.MD5Crypt([]byte(pass), []byte(salt), []byte("$1$")))
	secretProvider := func(u, r string) string {
		if user == u {
			return pass
		}
		return ""
	}
	authenticator := NewLoggedBasicAuthenticator(realm, secretProvider)
	return basicAuth(authenticator)
}

// MiddlewareAuthCustom instantiates middleware that authenticates using a custom function
func MiddlewareAuthCustom(fn CustomAuthFn, realm string, userFromContext bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// skip auth for unix socket
			if IsUnixSocket(r) {
				next.ServeHTTP(w, r)
				return
			}
			// skip auth for CORS preflight
			if r.Method == "OPTIONS" {
				next.ServeHTTP(w, r)
				return
			}

			user, pass, ok := parseAuthorization(r)
			if !ok && userFromContext {
				user, ok = CtxGetUser(r.Context())
			}

			if !ok {
				code := http.StatusUnauthorized
				w.Header().Set("Content-Type", "text/plain")
				w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm=%q, charset="UTF-8"`, realm))
				http.Error(w, http.StatusText(code), code)
				return
			}

			value, err := fn(user, pass)
			if err != nil {
				fs.Infof(r.URL.Path, "%s: Auth failed from %s: %v", r.RemoteAddr, user, err)
				goauth.NewBasicAuthenticator(realm, func(user, realm string) string { return "" }).RequireAuth(w, r) //Reuse BasicAuth error reporting
				return
			}

			if value != nil {
				r = r.WithContext(context.WithValue(r.Context(), ctxKeyAuth, value))
			}

			next.ServeHTTP(w, r)
		})
	}
}

var onlyOnceWarningAllowOrigin sync.Once

// MiddlewareCORS instantiates middleware that handles basic CORS protections for rcd
func MiddlewareCORS(allowOrigin string) Middleware {
	onlyOnceWarningAllowOrigin.Do(func() {
		if allowOrigin == "*" {
			fs.Logf(nil, "Warning: Allow origin set to *. This can cause serious security problems.")
		}
	})

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// skip cors for unix sockets
			if IsUnixSocket(r) {
				next.ServeHTTP(w, r)
				return
			}

			if allowOrigin != "" {
				w.Header().Add("Access-Control-Allow-Origin", allowOrigin)
				w.Header().Add("Access-Control-Allow-Headers", "authorization, Content-Type")
				w.Header().Add("Access-Control-Allow-Methods", "COPY, DELETE, GET, HEAD, LOCK, MKCOL, MOVE, OPTIONS, POST, PROPFIND, PROPPATCH, PUT, TRACE, UNLOCK")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// MiddlewareStripPrefix instantiates middleware that removes the BaseURL from the path
func MiddlewareStripPrefix(prefix string) Middleware {
	return func(next http.Handler) http.Handler {
		stripPrefixHandler := http.StripPrefix(prefix, next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow OPTIONS on the root only
			if r.URL.Path == "/" && r.Method == "OPTIONS" {
				next.ServeHTTP(w, r)
				return
			}
			stripPrefixHandler.ServeHTTP(w, r)
		})
	}
}
