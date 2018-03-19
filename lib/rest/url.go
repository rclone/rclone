package rest

import (
	"net/url"

	"github.com/pkg/errors"
)

// URLJoin joins a URL and a path returning a new URL
//
// path should be URL escaped
func URLJoin(base *url.URL, path string) (*url.URL, error) {
	rel, err := url.Parse(path)
	if err != nil {
		return nil, errors.Wrapf(err, "Error parsing %q as URL", path)
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
