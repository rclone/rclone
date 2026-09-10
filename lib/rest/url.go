package rest

import (
	"fmt"
	"net/url"
	"strings"
)

// URLJoin joins a URL and a path returning a new URL
//
// path should be URL escaped
//
// path is resolved as a URL reference so an absolute URL or a
// network-path reference ("//host/path") replaces the scheme or host
// of base. Use URLJoinRoot for a path which must stay on base.
func URLJoin(base *url.URL, path string) (*url.URL, error) {
	rel, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("error parsing %q as URL: %w", path, err)
	}
	return base.ResolveReference(rel), nil
}

// URLJoinRoot joins root, an unescaped path, onto base returning a new URL.
//
// root may only change the path of base. A root which carries a
// scheme, host or userinfo of its own, such as the network-path
// reference "//host/path", returns an error.
func URLJoinRoot(base *url.URL, root string) (*url.URL, error) {
	rel, err := url.Parse(URLPathEscape(root))
	if err != nil {
		return nil, fmt.Errorf("error parsing %q as URL: %w", root, err)
	}
	if rel.Scheme != "" || rel.Opaque != "" || rel.User != nil || rel.Host != "" {
		return nil, fmt.Errorf("root %q must not change the scheme, host or user of %q", root, base.Redacted())
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
// It escapes every character except the RFC 3986 unreserved characters
// [A-Za-z0-9-._~] and the path separator /. Unreserved characters MUST NOT
// be percent-encoded per RFC 3986 §2.3.
func URLPathEscapeAll(in string) string {
	var b strings.Builder
	b.Grow(len(in) * 3) // worst case: every byte escaped
	const hex = "0123456789ABCDEF"
	for i := range len(in) {
		c := in[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '/' ||
			c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0F])
		}
	}
	return b.String()
}
