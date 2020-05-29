// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

// ErrObjectKeyInvalid is returned when the object key is invalid.
var ErrObjectKeyInvalid = errors.New("object key invalid")

// ErrObjectNotFound is returned when the object is not found.
var ErrObjectNotFound = errors.New("object not found")

// Object contains information about an object.
type Object struct {
	Key string
	// IsPrefix indicates whether the Key is a prefix for other objects.
	IsPrefix bool

	System SystemMetadata
	Custom CustomMetadata
}

// SystemMetadata contains information about the object that cannot be changed directly.
type SystemMetadata struct {
	Created       time.Time
	Expires       time.Time
	ContentLength int64
}

// CustomMetadata contains custom user metadata about the object.
//
// The keys and values in custom metadata are expected to be valid UTF-8.
//
// When choosing a custom key for your application start it with a prefix "app:key",
// as an example application named "Image Board" might use a key "image-board:title".
type CustomMetadata map[string]string

// Clone makes a deep clone.
func (meta CustomMetadata) Clone() CustomMetadata {
	r := CustomMetadata{}
	for k, v := range meta {
		r[k] = v
	}
	return r
}

// Verify verifies whether CustomMetadata contains only "utf-8".
func (meta CustomMetadata) Verify() error {
	var invalid []string
	for k, v := range meta {
		if !utf8.ValidString(k) || !utf8.ValidString(v) {
			invalid = append(invalid, fmt.Sprintf("not utf-8 %q=%q", k, v))
		}
		if strings.IndexByte(k, 0) >= 0 || strings.IndexByte(v, 0) >= 0 {
			invalid = append(invalid, fmt.Sprintf("contains 0 byte: %q=%q", k, v))
		}
		if k == "" {
			invalid = append(invalid, "empty key")
		}
	}

	if len(invalid) > 0 {
		return errs.New("invalid pairs %v", invalid)
	}

	return nil
}

// StatObject returns information about an object at the specific key.
func (project *Project) StatObject(ctx context.Context, bucket, key string) (info *Object, err error) {
	defer mon.Func().RestartTrace(&ctx)(&err)

	b := storj.Bucket{Name: bucket}
	obj, err := project.db.GetObject(ctx, b, key)
	if err != nil {
		return nil, convertKnownErrors(err, bucket, key)
	}

	return convertObject(&obj), nil
}

// DeleteObject deletes the object at the specific key.
func (project *Project) DeleteObject(ctx context.Context, bucket, key string) (deleted *Object, err error) {
	defer mon.Func().RestartTrace(&ctx)(&err)

	b := storj.Bucket{Name: bucket}
	obj, err := project.db.DeleteObject(ctx, b, key)
	if err != nil {
		return nil, convertKnownErrors(err, bucket, key)
	}
	return convertObject(&obj), nil
}

// convertObject converts storj.Object to uplink.Object.
func convertObject(obj *storj.Object) *Object {
	if obj.Bucket.Name == "" { // zero object
		return nil
	}

	return &Object{
		Key: obj.Path,
		System: SystemMetadata{
			Created:       obj.Created,
			Expires:       obj.Expires,
			ContentLength: obj.Size,
		},
		Custom: obj.Metadata,
	}
}
