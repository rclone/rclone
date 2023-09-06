// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"sort"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/base58"
	"storj.io/common/encryption"
	"storj.io/common/storj"
	"storj.io/uplink/private/metaclient"
)

// ListUploadsOptions options for listing uncommitted uploads.
type ListUploadsOptions struct {
	// Prefix allows to filter uncommitted uploads by a key prefix. If not empty, it must end with slash.
	Prefix string
	// Cursor sets the starting position of the iterator.
	// The first item listed will be the one after the cursor.
	// Cursor is relative to Prefix.
	Cursor string
	// Recursive iterates the objects without collapsing prefixes.
	Recursive bool

	// System includes SystemMetadata in the results.
	System bool
	// Custom includes CustomMetadata in the results.
	Custom bool
}

// UploadIterator is an iterator over a collection of uncommitted uploads.
type UploadIterator struct {
	ctx           context.Context
	project       *Project
	bucket        string
	options       metaclient.ListOptions
	uploadOptions ListUploadsOptions
	list          *metaclient.ObjectList
	position      int
	completed     bool
	err           error
	listObjects   func(tx context.Context, db *metaclient.DB, bucket string, options metaclient.ListOptions) (metaclient.ObjectList, error)
}

func listObjects(ctx context.Context, db *metaclient.DB, bucket string, options metaclient.ListOptions) (metaclient.ObjectList, error) {
	return db.ListObjects(ctx, bucket, options)
}

func listPendingObjectStreams(ctx context.Context, db *metaclient.DB, bucket string, options metaclient.ListOptions) (metaclient.ObjectList, error) {
	return db.ListPendingObjectStreams(ctx, bucket, options)
}

// Next prepares next entry for reading.
// It returns false if the end of the iteration is reached and there are no more uploads, or if there is an error.
func (uploads *UploadIterator) Next() bool {
	if uploads.err != nil {
		uploads.completed = true
		return false
	}

	if uploads.list == nil {
		more := uploads.loadNext()
		uploads.completed = !more
		return more
	}

	if uploads.position >= len(uploads.list.Items)-1 {
		if !uploads.list.More {
			uploads.completed = true
			return false
		}
		more := uploads.loadNext()
		uploads.completed = !more
		return more
	}

	uploads.position++

	return true
}

func (uploads *UploadIterator) loadNext() bool {
	ok, err := uploads.tryLoadNext()
	if err != nil {
		uploads.err = err
		return false
	}
	return ok
}

func (uploads *UploadIterator) tryLoadNext() (ok bool, err error) {
	db, err := dialMetainfoDB(uploads.ctx, uploads.project)
	if err != nil {
		return false, convertKnownErrors(err, uploads.bucket, "")
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	list, err := uploads.listObjects(uploads.ctx, db, uploads.bucket, uploads.options)
	if err != nil {
		return false, convertKnownErrors(err, uploads.bucket, "")
	}
	uploads.list = &list
	if list.More {
		uploads.options = uploads.options.NextPage(list)
	}
	uploads.position = 0
	return len(list.Items) > 0, nil
}

// Err returns error, if one happened during iteration.
func (uploads *UploadIterator) Err() error {
	return packageError.Wrap(uploads.err)
}

// Item returns the current entry in the iterator.
func (uploads *UploadIterator) Item() *UploadInfo {
	item := uploads.item()
	if item == nil {
		return nil
	}

	key := item.Path
	if len(uploads.options.Prefix) > 0 && strings.HasSuffix(uploads.options.Prefix, "/") {
		key = uploads.options.Prefix + item.Path
	}

	obj := UploadInfo{
		Key:      key,
		IsPrefix: item.IsPrefix,
		UploadID: base58.CheckEncode(item.Stream.ID, 1),
	}

	// TODO: Make this filtering on the satellite
	if uploads.uploadOptions.System {
		obj.System = SystemMetadata{
			Created:       item.Created,
			Expires:       item.Expires,
			ContentLength: item.Size,
		}
	}

	// TODO: Make this filtering on the satellite
	if uploads.uploadOptions.Custom {
		obj.Custom = item.Metadata
	}

	return &obj
}

func (uploads *UploadIterator) item() *metaclient.Object {
	if uploads.completed {
		return nil
	}

	if uploads.err != nil {
		return nil
	}

	if uploads.list == nil {
		return nil
	}

	if len(uploads.list.Items) == 0 {
		return nil
	}

	return &uploads.list.Items[uploads.position]
}

// ListUploadPartsOptions options for listing upload parts.
type ListUploadPartsOptions struct {
	// Cursor sets the starting position of the iterator.
	// The first item listed will be the one after the cursor.
	Cursor uint32
}

// PartIterator is an iterator over a collection of parts of an upload.
type PartIterator struct {
	ctx       context.Context
	project   *Project
	bucket    string
	key       string
	uploadID  string
	options   metaclient.ListSegmentsParams
	items     []*Part
	more      bool
	position  int
	lastPart  *Part
	completed bool
	err       error
}

// Next prepares next entry for reading.
func (parts *PartIterator) Next() bool {
	if parts.err != nil {
		parts.completed = true
		return false
	}

	if len(parts.items) == 0 {
		more := parts.loadNext()
		parts.completed = !more
		return more
	}

	if parts.position >= len(parts.items)-1 {
		if !parts.more {
			parts.completed = true
			return false
		}
		more := parts.loadNext()
		parts.completed = !more
		return more
	}

	parts.position++

	return true
}

func (parts *PartIterator) loadNext() bool {
	ok, err := parts.tryLoadNext()
	if err != nil {
		parts.err = err
		return false
	}
	return ok
}

func (parts *PartIterator) tryLoadNext() (ok bool, err error) {
	metainfoClient, err := parts.project.dialMetainfoClient(parts.ctx)
	if err != nil {
		return false, convertKnownErrors(err, parts.bucket, parts.key)
	}
	defer func() { err = errs.Combine(err, metainfoClient.Close()) }()

	partsMap := make(map[uint32]*Part)
	// put into map last part from previous listing
	if parts.lastPart != nil {
		partsMap[parts.lastPart.PartNumber] = parts.lastPart
	}

	parts.position = 0
	parts.items = parts.items[:0]

	for {
		list, err := metainfoClient.ListSegments(parts.ctx, parts.options)
		if err != nil {
			return false, convertKnownErrors(err, parts.bucket, parts.key)
		}

		for _, item := range list.Items {
			var etag []byte
			if item.EncryptedETag != nil {
				// ETag will be only with last segment in a part
				etag, err = decryptETag(parts.project, parts.bucket, parts.key, list.EncryptionParameters, item)
				if err != nil {
					return false, convertKnownErrors(err, parts.bucket, parts.key)
				}
			}

			partNumber := uint32(item.Position.PartNumber)
			_, exists := partsMap[partNumber]
			if !exists {
				partsMap[partNumber] = &Part{
					PartNumber: partNumber,
					Size:       item.PlainSize,
					Modified:   item.CreatedAt,
					ETag:       etag,
				}
			} else {
				partsMap[partNumber].Size += item.PlainSize
				if item.CreatedAt.After(partsMap[partNumber].Modified) {
					partsMap[partNumber].Modified = item.CreatedAt
				}
				// The satellite returns the segments ordered by position. So it is
				// OK to just overwrite the ETag with the one from the next segment.
				// Eventually, the map will contain the ETag of the last segment,
				// which is the part's ETag.
				partsMap[partNumber].ETag = etag
			}
		}

		if list.More && len(list.Items) > 0 {
			item := list.Items[len(list.Items)-1]
			parts.options.Cursor = item.Position
		}

		parts.more = list.More

		// stop this loop when there is no next page or we have more than single
		// result in map. Single result means that we still cannot present result
		// as more segment from this part can be still not listed.
		if !parts.more || len(partsMap) == 0 || len(partsMap) > 1 {
			break
		}
	}

	for _, part := range partsMap {
		parts.items = append(parts.items, part)
	}
	sort.Slice(parts.items, func(i, k int) bool {
		return parts.items[i].PartNumber < parts.items[k].PartNumber
	})

	if parts.more && len(parts.items) > 0 {
		// remove last part from results as it may be incomplete and rest
		// of segments will be in next chunk of results, removed part
		// will be added to next chunk and updated if needed
		parts.lastPart = parts.items[len(parts.items)-1]
		parts.items = parts.items[:len(parts.items)-1]
	}

	return len(parts.items) > 0, nil
}

// Item returns the current entry in the iterator.
func (parts *PartIterator) Item() *Part {
	if parts.completed {
		return nil
	}

	if parts.err != nil {
		return nil
	}

	if len(parts.items) == 0 {
		return nil
	}

	return parts.items[parts.position]
}

// Err returns error, if one happened during iteration.
func (parts *PartIterator) Err() error {
	return packageError.Wrap(parts.err)
}

func decryptETag(project *Project, bucket, key string, encryptionParameters storj.EncryptionParameters, segment metaclient.SegmentListItem) ([]byte, error) {
	if segment.EncryptedETag == nil {
		return nil, nil
	}

	derivedKey, err := deriveContentKey(project, bucket, key)
	if err != nil {
		return nil, err
	}

	contentKey, err := encryption.DecryptKey(segment.EncryptedKey, encryptionParameters.CipherSuite, derivedKey, &segment.EncryptedKeyNonce)
	if err != nil {
		return nil, err
	}

	// Derive another key from the randomly generated content key to decrypt
	// the segment's ETag.
	etagKey, err := deriveETagKey(contentKey)
	if err != nil {
		return nil, err
	}

	return encryption.Decrypt(segment.EncryptedETag, encryptionParameters.CipherSuite, etagKey, &storj.Nonce{})
}

// TODO move it to be accesible here and from streams/store.go.
func deriveETagKey(key *storj.Key) (*storj.Key, error) {
	return encryption.DeriveKey(key, "storj-etag-v1")
}
