package middleware

import (
	"net/http"
	"strings"
)

// PageRoute is a simple middleware which allows you to route a static GET request
// at the middleware stack level.
func PageRoute(path string, handler http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && strings.EqualFold(r.URL.Path, path) {
				handler.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
