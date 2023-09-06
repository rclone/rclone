package middleware

import (
	"net/http"
	"strings"
)

// ContentCharset generates a handler that writes a 415 Unsupported Media Type response if none of the charsets match.
// An empty charset will allow requests with no Content-Type header or no specified charset.
func ContentCharset(charsets ...string) func(next http.Handler) http.Handler {
	for i, c := range charsets {
		charsets[i] = strings.ToLower(c)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !contentEncoding(r.Header.Get("Content-Type"), charsets...) {
				w.WriteHeader(http.StatusUnsupportedMediaType)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Check the content encoding against a list of acceptable values.
func contentEncoding(ce string, charsets ...string) bool {
	_, ce = split(strings.ToLower(ce), ";")
	_, ce = split(ce, "charset=")
	ce, _ = split(ce, ";")
	for _, c := range charsets {
		if ce == c {
			return true
		}
	}

	return false
}

// Split a string in two parts, cleaning any whitespace.
func split(str, sep string) (string, string) {
	var a, b string
	var parts = strings.SplitN(str, sep, 2)
	a = strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		b = strings.TrimSpace(parts[1])
	}

	return a, b
}
