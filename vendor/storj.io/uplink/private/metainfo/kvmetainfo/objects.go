// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"
	"errors"
	"strings"

	"storj.io/common/encryption"
	"storj.io/common/paths"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/uplink/private/metainfo"
	"storj.io/uplink/private/storage/segments"
	"storj.io/uplink/private/storage/streams"
)

var contentTypeKey = "content-type"

// GetObject returns information about an object.
func (db *DB) GetObject(ctx context.Context, bucket storj.Bucket, path storj.Path) (info storj.Object, err error) {
	defer mon.Task()(&ctx)(&err)

	_, info, err = db.getInfo(ctx, bucket, path)

	return info, err
}

// GetObjectStream returns interface for reading the object stream.
func (db *DB) GetObjectStream(ctx context.Context, bucket storj.Bucket, object storj.Object) (stream ReadOnlyStream, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket.Name == "" {
		return nil, storj.ErrNoBucket.New("")
	}

	if object.Path == "" {
		return nil, storj.ErrNoPath.New("")
	}

	return &readonlyStream{
		db:   db,
		info: object,
	}, nil
}

// CreateObject creates an uploading object and returns an interface for uploading Object information.
func (db *DB) CreateObject(ctx context.Context, bucket storj.Bucket, path storj.Path, createInfo *CreateObject) (object MutableObject, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket.Name == "" {
		return nil, storj.ErrNoBucket.New("")
	}

	if path == "" {
		return nil, storj.ErrNoPath.New("")
	}

	info := storj.Object{
		Bucket: bucket,
		Path:   path,
	}

	if createInfo != nil {
		info.Metadata = createInfo.Metadata
		info.ContentType = createInfo.ContentType
		info.Expires = createInfo.Expires
		info.RedundancyScheme = createInfo.RedundancyScheme
		info.EncryptionParameters = createInfo.EncryptionParameters
	}

	// TODO: autodetect content type from the path extension
	// if info.ContentType == "" {}

	return &mutableObject{
		db:   db,
		info: info,
	}, nil
}

// ModifyObject modifies a committed object.
func (db *DB) ModifyObject(ctx context.Context, bucket storj.Bucket, path storj.Path) (object MutableObject, err error) {
	defer mon.Task()(&ctx)(&err)
	return nil, errors.New("not implemented")
}

// DeleteObject deletes an object from database.
func (db *DB) DeleteObject(ctx context.Context, bucket storj.Bucket, path storj.Path) (_ storj.Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket.Name == "" {
		return storj.Object{}, storj.ErrNoBucket.New("")
	}

	if len(path) == 0 {
		return storj.Object{}, storj.ErrNoPath.New("")
	}

	info, err := db.streams.Delete(ctx, storj.JoinPaths(bucket.Name, path))
	if err != nil {
		return storj.Object{}, err
	}

	encPath, err := encryption.EncryptPathWithStoreCipher(bucket.Name, paths.NewUnencrypted(path), db.encStore)
	if err != nil {
		return storj.Object{}, err
	}

	_, obj, err := objectFromInfo(ctx, bucket, path, encPath, info, db.encStore)
	return obj, err
}

// ModifyPendingObject creates an interface for updating a partially uploaded object.
func (db *DB) ModifyPendingObject(ctx context.Context, bucket storj.Bucket, path storj.Path) (object MutableObject, err error) {
	defer mon.Task()(&ctx)(&err)
	return nil, errors.New("not implemented")
}

// ListPendingObjects lists pending objects in bucket based on the ListOptions.
func (db *DB) ListPendingObjects(ctx context.Context, bucket storj.Bucket, options storj.ListOptions) (list storj.ObjectList, err error) {
	defer mon.Task()(&ctx)(&err)
	return storj.ObjectList{}, errors.New("not implemented")
}

// ListObjects lists objects in bucket based on the ListOptions.
func (db *DB) ListObjects(ctx context.Context, bucket storj.Bucket, options storj.ListOptions) (list storj.ObjectList, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket.Name == "" {
		return storj.ObjectList{}, storj.ErrNoBucket.New("")
	}

	if options.Prefix != "" && !strings.HasSuffix(options.Prefix, "/") {
		return storj.ObjectList{}, errClass.New("prefix should end with slash")
	}

	var startAfter string
	switch options.Direction {
	// TODO for now we are supporting only storj.After
	// case storj.Forward:
	// 	// forward lists forwards from cursor, including cursor
	// 	startAfter = keyBefore(options.Cursor)
	case storj.After:
		// after lists forwards from cursor, without cursor
		startAfter = options.Cursor
	default:
		return storj.ObjectList{}, errClass.New("invalid direction %d", options.Direction)
	}

	// TODO: we should let libuplink users be able to determine what metadata fields they request as well
	// metaFlags := meta.All
	// if db.pathCipher(bucket) == storj.EncNull || db.pathCipher(bucket) == storj.EncNullBase64URL {
	// 	metaFlags = meta.None
	// }

	// TODO use flags with listing
	// if metaFlags&meta.Size != 0 {
	// Calculating the stream's size require also the user-defined metadata,
	// where stream store keeps info about the number of segments and their size.
	// metaFlags |= meta.UserDefined
	// }

	// Remove the trailing slash from list prefix.
	// Otherwise, if we the list prefix is `/bob/`, the encrypted list
	// prefix results in `enc("")/enc("bob")/enc("")`. This is an incorrect
	// encrypted prefix, what we really want is `enc("")/enc("bob")`.
	prefix := streams.ParsePath(storj.JoinPaths(bucket.Name, strings.TrimSuffix(options.Prefix, "/")))
	prefixKey, err := encryption.DerivePathKey(prefix.Bucket(), streams.PathForKey(prefix.UnencryptedPath().Raw()), db.encStore)
	if err != nil {
		return storj.ObjectList{}, errClass.Wrap(err)
	}

	encPrefix, err := encryption.EncryptPathWithStoreCipher(prefix.Bucket(), prefix.UnencryptedPath(), db.encStore)
	if err != nil {
		return storj.ObjectList{}, errClass.Wrap(err)
	}

	// We have to encrypt startAfter but only if it doesn't contain a bucket.
	// It contains a bucket if and only if the prefix has no bucket. This is why it is a raw
	// string instead of a typed string: it's either a bucket or an unencrypted path component
	// and that isn't known at compile time.
	needsEncryption := prefix.Bucket() != ""
	var base *encryption.Base
	if needsEncryption {
		_, _, base = db.encStore.LookupEncrypted(prefix.Bucket(), encPrefix)

		startAfter, err = encryption.EncryptPathRaw(startAfter, db.pathCipher(base.PathCipher), prefixKey)
		if err != nil {
			return storj.ObjectList{}, errClass.Wrap(err)
		}
	}

	items, more, err := db.metainfo.ListObjects(ctx, metainfo.ListObjectsParams{
		Bucket:          []byte(bucket.Name),
		EncryptedPrefix: []byte(encPrefix.Raw()),
		EncryptedCursor: []byte(startAfter),
		Limit:           int32(options.Limit),
		Recursive:       options.Recursive,
	})
	if err != nil {
		return storj.ObjectList{}, errClass.Wrap(err)
	}

	list = storj.ObjectList{
		Bucket: bucket.Name,
		Prefix: options.Prefix,
		More:   more,
		Items:  make([]storj.Object, 0, len(items)),
	}

	for _, item := range items {
		var path streams.Path
		var itemPath string

		if needsEncryption {
			itemPath, err = encryption.DecryptPathRaw(string(item.EncryptedPath), db.pathCipher(base.PathCipher), prefixKey)
			if err != nil {
				// skip items that cannot be decrypted
				if encryption.ErrDecryptFailed.Has(err) {
					continue
				}
				return storj.ObjectList{}, errClass.Wrap(err)
			}

			// TODO(jeff): this shouldn't be necessary if we handled trailing slashes
			// appropriately. there's some issues with list.
			fullPath := prefix.UnencryptedPath().Raw()
			if len(fullPath) > 0 && fullPath[len(fullPath)-1] != '/' {
				fullPath += "/"
			}
			fullPath += itemPath

			path = streams.CreatePath(prefix.Bucket(), paths.NewUnencrypted(fullPath))
		} else {
			itemPath = string(item.EncryptedPath)
			path = streams.CreatePath(string(item.EncryptedPath), paths.Unencrypted{})
		}

		stream, streamMeta, err := streams.TypedDecryptStreamInfo(ctx, item.EncryptedMetadata, path, db.encStore)
		if err != nil {
			// skip items that cannot be decrypted
			if encryption.ErrDecryptFailed.Has(err) {
				continue
			}
			return storj.ObjectList{}, errClass.Wrap(err)
		}

		object, err := objectFromMeta(bucket, itemPath, item, stream, streamMeta)
		if err != nil {
			return storj.ObjectList{}, errClass.Wrap(err)
		}

		list.Items = append(list.Items, object)
	}

	return list, nil
}

func (db *DB) pathCipher(pathCipher storj.CipherSuite) storj.CipherSuite {
	if db.encStore.EncryptionBypass {
		return storj.EncNullBase64URL
	}
	return pathCipher
}

type object struct {
	fullpath        streams.Path
	bucket          string
	encPath         paths.Encrypted
	lastSegmentMeta segments.Meta
	streamInfo      *pb.StreamInfo
	streamMeta      pb.StreamMeta
}

func (db *DB) getInfo(ctx context.Context, bucket storj.Bucket, path storj.Path) (obj object, info storj.Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket.Name == "" {
		return object{}, storj.Object{}, storj.ErrNoBucket.New("")
	}

	if path == "" {
		return object{}, storj.Object{}, storj.ErrNoPath.New("")
	}

	encPath, err := encryption.EncryptPathWithStoreCipher(bucket.Name, paths.NewUnencrypted(path), db.encStore)
	if err != nil {
		return object{}, storj.Object{}, err
	}

	objectInfo, err := db.metainfo.GetObject(ctx, metainfo.GetObjectParams{
		Bucket:        []byte(bucket.Name),
		EncryptedPath: []byte(encPath.Raw()),
	})
	if err != nil {
		return object{}, storj.Object{}, err
	}

	return objectFromInfo(ctx, bucket, path, encPath, objectInfo, db.encStore)
}

func objectFromInfo(ctx context.Context, bucket storj.Bucket, path storj.Path, encPath paths.Encrypted, objectInfo storj.ObjectInfo, encStore *encryption.Store) (object, storj.Object, error) {
	if objectInfo.Bucket == "" { // zero objectInfo
		return object{}, storj.Object{}, nil
	}

	fullpath := streams.CreatePath(bucket.Name, paths.NewUnencrypted(path))
	lastSegmentMeta := segments.Meta{
		Modified:   objectInfo.Created,
		Expiration: objectInfo.Expires,
		Size:       objectInfo.Size,
		Data:       objectInfo.Metadata,
	}

	streamInfo, streamMeta, err := streams.TypedDecryptStreamInfo(ctx, lastSegmentMeta.Data, fullpath, encStore)
	if err != nil {
		return object{}, storj.Object{}, err
	}

	info, err := objectStreamFromMeta(bucket, path, objectInfo.StreamID, lastSegmentMeta, streamInfo, streamMeta, objectInfo.Stream.RedundancyScheme)
	if err != nil {
		return object{}, storj.Object{}, err
	}

	return object{
		fullpath:        fullpath,
		bucket:          bucket.Name,
		encPath:         encPath,
		lastSegmentMeta: lastSegmentMeta,
		streamInfo:      streamInfo,
		streamMeta:      streamMeta,
	}, info, nil
}

func objectFromMeta(bucket storj.Bucket, path storj.Path, listItem storj.ObjectListItem, stream *pb.StreamInfo, streamMeta pb.StreamMeta) (storj.Object, error) {
	object := storj.Object{
		Version:  0, // TODO:
		Bucket:   bucket,
		Path:     path,
		IsPrefix: listItem.IsPrefix,

		Created:  listItem.CreatedAt, // TODO: use correct field
		Modified: listItem.CreatedAt, // TODO: use correct field
		Expires:  listItem.ExpiresAt,
	}

	err := updateObjectWithStream(&object, stream, streamMeta)
	if err != nil {
		return storj.Object{}, err
	}

	return object, nil
}

func objectStreamFromMeta(bucket storj.Bucket, path storj.Path, streamID storj.StreamID, lastSegment segments.Meta, stream *pb.StreamInfo, streamMeta pb.StreamMeta, redundancyScheme storj.RedundancyScheme) (storj.Object, error) {
	var nonce storj.Nonce
	var encryptedKey storj.EncryptedPrivateKey
	if streamMeta.LastSegmentMeta != nil {
		copy(nonce[:], streamMeta.LastSegmentMeta.KeyNonce)
		encryptedKey = streamMeta.LastSegmentMeta.EncryptedKey
	}

	rv := storj.Object{
		Version:  0, // TODO:
		Bucket:   bucket,
		Path:     path,
		IsPrefix: false,

		Created:  lastSegment.Modified,   // TODO: use correct field
		Modified: lastSegment.Modified,   // TODO: use correct field
		Expires:  lastSegment.Expiration, // TODO: use correct field

		Stream: storj.Stream{
			ID: streamID,

			RedundancyScheme: redundancyScheme,
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.CipherSuite(streamMeta.EncryptionType),
				BlockSize:   streamMeta.EncryptionBlockSize,
			},
			LastSegment: storj.LastSegment{
				EncryptedKeyNonce: nonce,
				EncryptedKey:      encryptedKey,
			},
		},
	}

	err := updateObjectWithStream(&rv, stream, streamMeta)
	if err != nil {
		return storj.Object{}, err
	}

	return rv, nil
}

func updateObjectWithStream(object *storj.Object, stream *pb.StreamInfo, streamMeta pb.StreamMeta) error {
	if stream == nil {
		return nil
	}

	serializableMeta := pb.SerializableMeta{}
	err := pb.Unmarshal(stream.Metadata, &serializableMeta)
	if err != nil {
		return err
	}

	// ensure that the map is not nil
	if serializableMeta.UserDefined == nil {
		serializableMeta.UserDefined = map[string]string{}
	}

	_, found := serializableMeta.UserDefined[contentTypeKey]
	if !found && serializableMeta.ContentType != "" {
		serializableMeta.UserDefined[contentTypeKey] = serializableMeta.ContentType
	}

	segmentCount := numberOfSegments(stream, streamMeta)
	object.Metadata = serializableMeta.UserDefined
	object.Stream.Size = ((segmentCount - 1) * stream.SegmentsSize) + stream.LastSegmentSize
	object.Stream.SegmentCount = segmentCount
	object.Stream.FixedSegmentSize = stream.SegmentsSize
	object.Stream.LastSegment.Size = stream.LastSegmentSize

	return nil
}

type mutableObject struct {
	db   *DB
	info storj.Object
}

func (object *mutableObject) Info() storj.Object { return object.info }

func (object *mutableObject) CreateStream(ctx context.Context) (_ MutableStream, err error) {
	defer mon.Task()(&ctx)(&err)
	return &mutableStream{
		db:   object.db,
		info: object.info,
	}, nil
}

func (object *mutableObject) ContinueStream(ctx context.Context) (_ MutableStream, err error) {
	defer mon.Task()(&ctx)(&err)
	return nil, errors.New("not implemented")
}

func (object *mutableObject) DeleteStream(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return errors.New("not implemented")
}

func (object *mutableObject) Commit(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, info, err := object.db.getInfo(ctx, object.info.Bucket, object.info.Path)
	object.info = info
	return err
}

func numberOfSegments(stream *pb.StreamInfo, streamMeta pb.StreamMeta) int64 {
	if streamMeta.NumberOfSegments > 0 {
		return streamMeta.NumberOfSegments
	}
	return stream.DeprecatedNumberOfSegments
}
