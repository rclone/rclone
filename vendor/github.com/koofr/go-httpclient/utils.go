package httpclient

import (
	"net/url"
	"strings"
)

func EscapePath(path string) string {
	u := url.URL{
		Path: path,
	}

	return strings.Replace(u.String(), "+", "%2b", -1)
}
