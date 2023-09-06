// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package edge

import (
	"net/url"
	"strings"
)

// ShareURLOptions contains options how to present the data data exposed through Linksharing.
type ShareURLOptions struct {
	// If set it creates a link directly to the data instead of an to intermediate landing page.
	// This URL can then be passed to a download command or embedded on a webpage.
	Raw bool
}

// JoinShareURL creates a linksharing URL from parts. The existence or accessibility of the target
// is not checked, it might not exist or be inaccessible.
//
// Example result is https://link.storjshare.io/s/l5pucy3dmvzxgs3fpfewix27l5pq/mybucket/myprefix/myobject
//
// The baseURL is the url of the linksharing service, e.g. https://link.storjshare.io. The accessKeyID
// can be obtained by calling RegisterAccess. It must be associated with public visibility.
// The bucket is optional, leave it blank to share the entire project. The object key is also optional,
// if empty shares the entire bucket. It can also be a prefix, in which case it must end with a "/".
func JoinShareURL(baseURL string, accessKeyID string, bucket string, key string, options *ShareURLOptions) (string, error) {
	if accessKeyID == "" {
		return "", uplinkError.New("accessKeyID is required")
	}

	if bucket == "" && key != "" {
		return "", uplinkError.New("bucket is required if key is specified")
	}

	if options == nil {
		options = &ShareURLOptions{}
	}

	if options.Raw {
		if key == "" {
			return "", uplinkError.New("key is required for a raw download link")
		}
		if key[len(key)-1:] == "/" {
			// This is error can be removed if it is too limiting.
			// Because the result could be used as a base for a known folder structure.
			return "", uplinkError.New("a raw download link can not be a prefix")
		}
	}

	result, err := url.ParseRequestURI(baseURL)
	if err != nil {
		return "", uplinkError.New("invalid base url: %q", baseURL)
	}

	result.Path = strings.Trim(result.Path, "/")
	if options.Raw {
		result.Path += "/raw/"
	} else {
		result.Path += "/s/"
	}

	result.Path += accessKeyID

	if bucket != "" {
		result.Path += "/" + bucket
	}

	if key != "" {
		result.Path += "/" + key
	}

	return result.String(), nil
}
