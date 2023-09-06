package utils

import (
	"net/url"
	"strings"
)

// URLQueryEscape escapes the original string.
func URLQueryEscape(origin string) string {
	escaped := url.QueryEscape(origin)
	escaped = strings.Replace(escaped, "%2F", "/", -1)
	escaped = strings.Replace(escaped, "%3D", "=", -1)
	escaped = strings.Replace(escaped, "+", "%20", -1)
	return escaped
}

// URLQueryUnescape unescapes the escaped string.
func URLQueryUnescape(escaped string) (string, error) {
	escaped = strings.Replace(escaped, "/", "%2F", -1)
	escaped = strings.Replace(escaped, "=", "%3D", -1)
	escaped = strings.Replace(escaped, "%20", " ", -1)
	return url.QueryUnescape(escaped)
}
