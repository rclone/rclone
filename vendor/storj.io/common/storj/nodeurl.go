// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"net/url"
	"strings"

	"github.com/zeebo/errs"
)

var (
	// ErrNodeURL is used when something goes wrong with a node url.
	ErrNodeURL = errs.Class("node URL error")
)

// NodeURL defines a structure for connecting to a node.
type NodeURL struct {
	ID      NodeID
	Address string
}

// ParseNodeURL parses node URL string.
//
// Examples:
//
//    raw IP:
//      33.20.0.1:7777
//      [2001:db8:1f70::999:de8:7648:6e8]:7777
//
//    with NodeID:
//      12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@33.20.0.1:7777
//      12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@[2001:db8:1f70::999:de8:7648:6e8]:7777
//
//    without host:
//      12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7@
func ParseNodeURL(s string) (NodeURL, error) {
	if s == "" {
		return NodeURL{}, nil
	}
	if !strings.HasPrefix(s, "storj://") {
		if !strings.Contains(s, "://") {
			s = "storj://" + s
		}
	}

	u, err := url.Parse(s)
	if err != nil {
		return NodeURL{}, ErrNodeURL.Wrap(err)
	}
	if u.Scheme != "" && u.Scheme != "storj" {
		return NodeURL{}, ErrNodeURL.New("unknown scheme %q", u.Scheme)
	}

	var node NodeURL
	if u.User != nil {
		node.ID, err = NodeIDFromString(u.User.String())
		if err != nil {
			return NodeURL{}, ErrNodeURL.Wrap(err)
		}
	}
	node.Address = u.Host

	return node, nil
}

// IsZero returns whether the url is empty.
func (url NodeURL) IsZero() bool {
	return url == NodeURL{}
}

// String converts NodeURL to a string.
func (url NodeURL) String() string {
	if url.ID.IsZero() {
		return url.Address
	}
	return url.ID.String() + "@" + url.Address
}

// Set implements flag.Value interface.
func (url *NodeURL) Set(s string) error {
	parsed, err := ParseNodeURL(s)
	if err != nil {
		return ErrNodeURL.Wrap(err)
	}

	*url = parsed
	return nil
}

// Type implements pflag.Value.
func (NodeURL) Type() string { return "storj.NodeURL" }

// NodeURLs defines a comma delimited flag for defining a list node url-s.
type NodeURLs []NodeURL

// ParseNodeURLs parses comma delimited list of node urls.
func ParseNodeURLs(s string) (NodeURLs, error) {
	var urls NodeURLs
	if s == "" {
		return nil, nil
	}

	for _, s := range strings.Split(s, ",") {
		u, err := ParseNodeURL(s)
		if err != nil {
			return nil, ErrNodeURL.Wrap(err)
		}
		urls = append(urls, u)
	}

	return urls, nil
}

// String converts NodeURLs to a string.
func (urls NodeURLs) String() string {
	var xs []string
	for _, u := range urls {
		xs = append(xs, u.String())
	}
	return strings.Join(xs, ",")
}

// Set implements flag.Value interface.
func (urls *NodeURLs) Set(s string) error {
	parsed, err := ParseNodeURLs(s)
	if err != nil {
		return ErrNodeURL.Wrap(err)
	}

	*urls = parsed
	return nil
}

// Type implements pflag.Value.
func (NodeURLs) Type() string { return "storj.NodeURLs" }
