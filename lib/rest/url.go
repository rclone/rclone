package rest

import (
	"fmt"
	"net/url"
	"strings"
)

// URLJoin joins a URL and a path returning a new URL
//
// path should be URL escaped
func URLJoin(base *url.URL, path string) (*url.URL, error) {
	rel, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("error parsing %q as URL: %w", path, err)
	}
	return base.ResolveReference(rel), nil
}

// URLPathEscape escapes URL path the in string using URL escaping rules
//
// This mimics url.PathEscape which only available from go 1.8
func URLPathEscape(in string) string {
	var u url.URL
	u.Path = in
	return u.String()
}

// URLPathEscapeAll escapes URL path the in string using URL escaping rules
//
// It escapes every character except [A-Za-z0-9] and /
func URLPathEscapeAll(in string) string {
	var b strings.Builder
	b.Grow(len(in) * 3) // worst case: every byte escaped
	const hex = "0123456789ABCDEF"
	for i := range len(in) {
		c := in[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '/' {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0F])
		}
	}
	return b.String()
}
