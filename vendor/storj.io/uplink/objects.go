// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"

	"storj.io/common/storj"
)

// ListObjectsOptions defines object listing options.
type ListObjectsOptions struct {
	// Prefix allows to filter objects by a key prefix. If not empty, it must end with slash.
	Prefix string
	// Cursor sets the starting position of the iterator. The first item listed will be the one after the cursor.
	Cursor string
	// Recursive iterates the objects without collapsing prefixes.
	Recursive bool

	// System includes SystemMetadata in the results.
	System bool
	// Custom includes CustomMetadata in the results.
	Custom bool
}

// ListObjects returns an iterator over the objects.
func (project *Project) ListObjects(ctx context.Context, bucket string, options *ListObjectsOptions) *ObjectIterator {
	defer mon.Func().RestartTrace(&ctx)(nil)

	b := storj.Bucket{Name: bucket, PathCipher: storj.EncAESGCM}
	opts := storj.ListOptions{
		Direction: storj.After,
	}

	if options != nil {
		opts.Prefix = options.Prefix
		opts.Cursor = options.Cursor
		opts.Recursive = options.Recursive
	}

	objects := ObjectIterator{
		ctx:     ctx,
		project: project,
		bucket:  b,
		options: opts,
	}

	if options != nil {
		objects.objOptions = *options
	}

	return &objects
}

// ObjectIterator is an iterator over a collection of objects or prefixes.
type ObjectIterator struct {
	ctx        context.Context
	project    *Project
	bucket     storj.Bucket
	options    storj.ListOptions
	objOptions ListObjectsOptions
	list       *storj.ObjectList
	position   int
	completed  bool
	err        error
}

// Next prepares next Object for reading.
// It returns false if the end of the iteration is reached and there are no more objects, or if there is an error.
func (objects *ObjectIterator) Next() bool {
	if objects.err != nil {
		objects.completed = true
		return false
	}

	if objects.list == nil {
		more := objects.loadNext()
		objects.completed = !more
		return more
	}

	if objects.position >= len(objects.list.Items)-1 {
		if !objects.list.More {
			objects.completed = true
			return false
		}
		more := objects.loadNext()
		objects.completed = !more
		return more
	}

	objects.position++

	return true
}

func (objects *ObjectIterator) loadNext() bool {
	list, err := objects.project.db.ListObjects(objects.ctx, objects.bucket, objects.options)
	if err != nil {
		objects.err = convertKnownErrors(err, objects.bucket.Name, "")
		return false
	}
	objects.list = &list
	if list.More {
		objects.options = objects.options.NextPage(list)
	}
	objects.position = 0
	return len(list.Items) > 0
}

// Err returns error, if one happened during iteration.
func (objects *ObjectIterator) Err() error {
	return packageError.Wrap(objects.err)
}

// Item returns the current object in the iterator.
func (objects *ObjectIterator) Item() *Object {
	item := objects.item()
	if item == nil {
		return nil
	}

	key := item.Path
	if len(objects.options.Prefix) > 0 {
		key = objects.options.Prefix + item.Path
	}

	obj := Object{
		Key:      key,
		IsPrefix: item.IsPrefix,
	}

	// TODO: Make this filtering on the satellite
	if objects.objOptions.System {
		obj.System = SystemMetadata{
			Created:       item.Created,
			Expires:       item.Expires,
			ContentLength: item.Size,
		}
	}

	// TODO: Make this filtering on the satellite
	if objects.objOptions.Custom {
		obj.Custom = item.Metadata
	}

	return &obj
}

func (objects *ObjectIterator) item() *storj.Object {
	if objects.completed {
		return nil
	}

	if objects.err != nil {
		return nil
	}

	if objects.list == nil {
		return nil
	}

	if len(objects.list.Items) == 0 {
		return nil
	}

	return &objects.list.Items[objects.position]
}
