package auth

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"

	auth "github.com/abbot/go-http-auth"
	"github.com/rclone/rclone/fs"
	httplib "github.com/rclone/rclone/lib/http"
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

type contextUserType struct{}

// ContextUserKey is a simple context key for storing the username of the request
var ContextUserKey = &contextUserType{}

type contextAuthType struct{}

// ContextAuthKey is a simple context key for storing info returned by CustomAuthFn
var ContextAuthKey = &contextAuthType{}

// LoggedBasicAuth extends BasicAuth to include access logging
type LoggedBasicAuth struct {
	auth.BasicAuth
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
func NewLoggedBasicAuthenticator(realm string, secrets auth.SecretProvider) *LoggedBasicAuth {
	return &LoggedBasicAuth{BasicAuth: auth.BasicAuth{Realm: realm, Secrets: secrets}}
}

// Helper to generate required interface for middleware
func basicAuth(authenticator *LoggedBasicAuth) httplib.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if username := authenticator.CheckAuth(r); username == "" {
				authenticator.RequireAuth(w, r)
			} else {
				r = r.WithContext(context.WithValue(r.Context(), ContextUserKey, username))
				next.ServeHTTP(w, r)
			}
		})
	}
}

// HtPasswdAuth instantiates middleware that authenticates against the passed htpasswd file
func HtPasswdAuth(path, realm string) httplib.Middleware {
	fs.Infof(nil, "Using %q as htpasswd storage", path)
	secretProvider := auth.HtpasswdFileProvider(path)
	authenticator := NewLoggedBasicAuthenticator(realm, secretProvider)
	return basicAuth(authenticator)
}

// SingleAuth instantiates middleware that authenticates for a single user
func SingleAuth(user, pass, realm string) httplib.Middleware {
	fs.Infof(nil, "Using --user %s --pass XXXX as authenticated user", user)
	pass = string(auth.MD5Crypt([]byte(pass), []byte("dlPL2MqE"), []byte("$1$")))
	secretProvider := func(user, realm string) string {
		if user == user {
			return pass
		}
		return ""
	}
	authenticator := NewLoggedBasicAuthenticator(realm, secretProvider)
	return basicAuth(authenticator)
}

// CustomAuth instantiates middleware that authenticates using a custom function
func CustomAuth(fn CustomAuthFn, realm string) httplib.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := parseAuthorization(r)
			if ok {
				value, err := fn(user, pass)
				if err != nil {
					fs.Infof(r.URL.Path, "%s: Auth failed from %s: %v", r.RemoteAddr, user, err)
					auth.NewBasicAuthenticator(realm, func(user, realm string) string { return "" }).RequireAuth(w, r) //Reuse BasicAuth error reporting
					return
				}
				if value != nil {
					r = r.WithContext(context.WithValue(r.Context(), ContextAuthKey, value))
				}
			}
		})
	}
}
