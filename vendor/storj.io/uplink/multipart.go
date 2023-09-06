// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"crypto/rand"
	"errors"
	"math"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jtolio/eventkit"
	"github.com/zeebo/errs"

	"storj.io/common/base58"
	"storj.io/common/encryption"
	"storj.io/common/leak"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/uplink/private/eestream/scheduler"
	"storj.io/uplink/private/metaclient"
	"storj.io/uplink/private/storage/streams"
	"storj.io/uplink/private/stream"
	"storj.io/uplink/private/testuplink"
)

// ErrUploadIDInvalid is returned when the upload ID is invalid.
var ErrUploadIDInvalid = errors.New("upload ID invalid")

// UploadInfo contains information about an upload.
type UploadInfo struct {
	UploadID string
	Key      string

	IsPrefix bool

	System SystemMetadata
	Custom CustomMetadata
}

// CommitUploadOptions options for committing multipart upload.
type CommitUploadOptions struct {
	CustomMetadata CustomMetadata
}

// BeginUpload begins a new multipart upload to bucket and key.
//
// Use UploadPart to upload individual parts.
//
// Use CommitUpload to finish the upload.
//
// Use AbortUpload to cancel the upload at any time.
//
// UploadObject is a convenient way to upload single part objects.
func (project *Project) BeginUpload(ctx context.Context, bucket, key string, options *UploadOptions) (info UploadInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	switch {
	case bucket == "":
		return UploadInfo{}, errwrapf("%w (%q)", ErrBucketNameInvalid, bucket)
	case key == "":
		return UploadInfo{}, errwrapf("%w (%q)", ErrObjectKeyInvalid, key)
	}

	if options == nil {
		options = &UploadOptions{}
	}

	encPath, err := encryptPath(project, bucket, key)
	if err != nil {
		return UploadInfo{}, packageError.Wrap(err)
	}

	metainfoClient, err := project.dialMetainfoClient(ctx)
	if err != nil {
		return UploadInfo{}, packageError.Wrap(err)
	}
	defer func() { err = errs.Combine(err, metainfoClient.Close()) }()

	response, err := metainfoClient.BeginObject(ctx, metaclient.BeginObjectParams{
		Bucket:               []byte(bucket),
		EncryptedObjectKey:   []byte(encPath.Raw()),
		ExpiresAt:            options.Expires,
		EncryptionParameters: project.encryptionParameters,
	})
	if err != nil {
		return UploadInfo{}, convertKnownErrors(err, bucket, key)
	}

	encodedStreamID := base58.CheckEncode(response.StreamID[:], 1)
	return UploadInfo{
		Key:      key,
		UploadID: encodedStreamID,
		System: SystemMetadata{
			Expires: options.Expires,
		},
	}, nil
}

// CommitUpload commits a multipart upload to bucket and key started with BeginUpload.
//
// uploadID is an upload identifier returned by BeginUpload.
func (project *Project) CommitUpload(ctx context.Context, bucket, key, uploadID string, opts *CommitUploadOptions) (object *Object, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO add completedPart to options when we will have implementation for that

	switch {
	case bucket == "":
		return nil, errwrapf("%w (%q)", ErrBucketNameInvalid, bucket)
	case key == "":
		return nil, errwrapf("%w (%q)", ErrObjectKeyInvalid, key)
	case uploadID == "":
		return nil, packageError.Wrap(ErrUploadIDInvalid)
	}

	decodedStreamID, version, err := base58.CheckDecode(uploadID)
	if err != nil || version != 1 {
		return nil, packageError.Wrap(ErrUploadIDInvalid)
	}

	id, err := storj.StreamIDFromBytes(decodedStreamID)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	if opts == nil {
		opts = &CommitUploadOptions{}
	}

	commitObjParams, err := project.fillMetadata(bucket, key, id, opts.CustomMetadata)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	metainfoClient, err := project.dialMetainfoClient(ctx)
	if err != nil {
		return nil, packageError.Wrap(err)
	}
	defer func() { err = errs.Combine(err, metainfoClient.Close()) }()

	err = metainfoClient.CommitObject(ctx, commitObjParams)
	if err != nil {
		return nil, convertKnownErrors(err, bucket, key)
	}

	// TODO return real object after committing
	return &Object{
		Key: key,
	}, nil
}

func (project *Project) fillMetadata(bucket, key string, id storj.StreamID, metadata CustomMetadata) (metaclient.CommitObjectParams, error) {
	commitObjParams := metaclient.CommitObjectParams{StreamID: id}
	if len(metadata) == 0 {
		return commitObjParams, nil
	}

	metadataBytes, err := pb.Marshal(&pb.SerializableMeta{
		UserDefined: metadata.Clone(),
	})
	if err != nil {
		return metaclient.CommitObjectParams{}, packageError.Wrap(err)
	}

	streamInfo, err := pb.Marshal(&pb.StreamInfo{
		Metadata: metadataBytes,
	})
	if err != nil {
		return metaclient.CommitObjectParams{}, packageError.Wrap(err)
	}

	derivedKey, err := deriveContentKey(project, bucket, key)
	if err != nil {
		return metaclient.CommitObjectParams{}, packageError.Wrap(err)
	}

	var metadataKey storj.Key
	// generate random key for encrypting the segment's content
	_, err = rand.Read(metadataKey[:])
	if err != nil {
		return metaclient.CommitObjectParams{}, packageError.Wrap(err)
	}

	var encryptedKeyNonce storj.Nonce
	// generate random nonce for encrypting the metadata key
	_, err = rand.Read(encryptedKeyNonce[:])
	if err != nil {
		return metaclient.CommitObjectParams{}, packageError.Wrap(err)
	}

	encryptionParameters := project.encryptionParameters
	encryptedKey, err := encryption.EncryptKey(&metadataKey, encryptionParameters.CipherSuite, derivedKey, &encryptedKeyNonce)
	if err != nil {
		return metaclient.CommitObjectParams{}, packageError.Wrap(err)
	}

	// encrypt metadata with the content encryption key and zero nonce.
	encryptedStreamInfo, err := encryption.Encrypt(streamInfo, encryptionParameters.CipherSuite, &metadataKey, &storj.Nonce{})
	if err != nil {
		return metaclient.CommitObjectParams{}, packageError.Wrap(err)
	}

	// TODO should we commit StreamMeta or commit only encrypted StreamInfo
	streamMetaBytes, err := pb.Marshal(&pb.StreamMeta{
		EncryptedStreamInfo: encryptedStreamInfo,
	})
	if err != nil {
		return metaclient.CommitObjectParams{}, packageError.Wrap(err)
	}

	commitObjParams.EncryptedMetadataEncryptedKey = encryptedKey
	commitObjParams.EncryptedMetadataNonce = encryptedKeyNonce
	commitObjParams.EncryptedMetadata = streamMetaBytes

	return commitObjParams, nil
}

// UploadPart uploads a part with partNumber to a multipart upload started with BeginUpload.
//
// uploadID is an upload identifier returned by BeginUpload.
func (project *Project) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber uint32) (_ *PartUpload, err error) {
	upload := &PartUpload{
		bucket: bucket,
		key:    key,
		part: &Part{
			PartNumber: partNumber,
		},
		stats:  newOperationStats(ctx, project.access.satelliteURL),
		eTagCh: make(chan []byte, 1),
	}
	upload.task = mon.TaskNamed("PartUpload")(&ctx)
	defer func() {
		if err != nil {
			upload.stats.flagFailure(err)
			upload.emitEvent(false)
		}
	}()
	defer upload.stats.trackWorking()()
	defer mon.Task()(&ctx)(&err)

	switch {
	case bucket == "":
		return nil, errwrapf("%w (%q)", ErrBucketNameInvalid, bucket)
	case key == "":
		return nil, errwrapf("%w (%q)", ErrObjectKeyInvalid, key)
	case uploadID == "":
		return nil, packageError.Wrap(ErrUploadIDInvalid)
	case partNumber >= math.MaxInt32:
		return nil, packageError.New("partNumber should be less than max(int32)")
	}

	decodedStreamID, version, err := base58.CheckDecode(uploadID)
	if err != nil || version != 1 {
		return nil, packageError.Wrap(ErrUploadIDInvalid)
	}

	if encPath, err := encryptPath(project, bucket, key); err == nil {
		upload.stats.encPath = encPath
	}

	ctx, cancel := context.WithCancel(ctx)
	upload.cancel = cancel

	streams, err := project.getStreamsStore(ctx)
	if err != nil {
		return nil, convertKnownErrors(err, bucket, key)
	}
	upload.streams = streams

	if project.concurrentSegmentUploadConfig == nil {
		upload.upload = stream.NewUploadPart(ctx, bucket, key, decodedStreamID, partNumber, upload.eTagCh, streams)
	} else {
		sched := scheduler.New(project.concurrentSegmentUploadConfig.SchedulerOptions)
		u, err := streams.UploadPart(ctx, bucket, key, decodedStreamID, int32(partNumber), upload.eTagCh, sched)
		if err != nil {
			return nil, convertKnownErrors(err, bucket, key)
		}
		upload.upload = u
	}

	upload.tracker = project.tracker.Child("upload-part", 1)
	return upload, nil
}

// AbortUpload aborts a multipart upload started with BeginUpload.
//
// uploadID is an upload identifier returned by BeginUpload.
func (project *Project) AbortUpload(ctx context.Context, bucket, key, uploadID string) (err error) {
	defer mon.Task()(&ctx)(&err)

	switch {
	case bucket == "":
		return errwrapf("%w (%q)", ErrBucketNameInvalid, bucket)
	case key == "":
		return errwrapf("%w (%q)", ErrObjectKeyInvalid, key)
	case uploadID == "":
		return packageError.Wrap(ErrUploadIDInvalid)
	}

	decodedStreamID, version, err := base58.CheckDecode(uploadID)
	if err != nil || version != 1 {
		return packageError.Wrap(ErrUploadIDInvalid)
	}

	id, err := storj.StreamIDFromBytes(decodedStreamID)
	if err != nil {
		return packageError.Wrap(err)
	}

	encPath, err := encryptPath(project, bucket, key)
	if err != nil {
		return convertKnownErrors(err, bucket, key)
	}

	metainfoClient, err := project.dialMetainfoClient(ctx)
	if err != nil {
		return convertKnownErrors(err, bucket, key)
	}
	defer func() { err = errs.Combine(err, metainfoClient.Close()) }()

	_, err = metainfoClient.BeginDeleteObject(ctx, metaclient.BeginDeleteObjectParams{
		Bucket:             []byte(bucket),
		EncryptedObjectKey: []byte(encPath.Raw()),
		// TODO remove it or set to 0 when satellite side will be fixed
		Version:  1,
		StreamID: id,
		Status:   int32(pb.Object_UPLOADING),
	})
	return convertKnownErrors(err, bucket, key)
}

// ListUploadParts returns an iterator over the parts of a multipart upload started with BeginUpload.
func (project *Project) ListUploadParts(ctx context.Context, bucket, key, uploadID string, options *ListUploadPartsOptions) *PartIterator {
	defer mon.Task()(&ctx)(nil)

	opts := metaclient.ListSegmentsParams{}

	if options != nil {
		opts.Cursor = metaclient.SegmentPosition{
			PartNumber: int32(options.Cursor),
			// cursor needs to be last segment in a part
			// satellite can accept uint32 as segment index
			// but protobuf is defined as int32 for now
			Index: math.MaxInt32,
		}
	}

	parts := PartIterator{
		ctx:      ctx,
		project:  project,
		bucket:   bucket,
		key:      key,
		options:  opts,
		uploadID: uploadID,
	}

	switch {
	case parts.bucket == "":
		parts.err = errwrapf("%w (%q)", ErrBucketNameInvalid, parts.bucket)
		return &parts
	case parts.key == "":
		parts.err = errwrapf("%w (%q)", ErrObjectKeyInvalid, parts.key)
		return &parts
	case parts.uploadID == "":
		parts.err = packageError.Wrap(ErrUploadIDInvalid)
		return &parts
	}

	decodedStreamID, version, err := base58.CheckDecode(uploadID)
	if err != nil || version != 1 {
		parts.err = packageError.Wrap(ErrUploadIDInvalid)
		return &parts
	}

	parts.options.StreamID = decodedStreamID
	return &parts
}

// ListUploads returns an iterator over the uncommitted uploads in bucket.
// Both multipart and regular uploads are returned. An object may not be
// visible through ListUploads until it has a committed part.
func (project *Project) ListUploads(ctx context.Context, bucket string, options *ListUploadsOptions) *UploadIterator {
	defer mon.Task()(&ctx)(nil)

	opts := metaclient.ListOptions{
		Direction: metaclient.After,
		Status:    int32(pb.Object_UPLOADING), // TODO: define object status constants in storj package?
	}

	if options != nil {
		opts.Prefix = options.Prefix
		opts.Cursor = options.Cursor
		opts.Recursive = options.Recursive
		opts.IncludeSystemMetadata = options.System
		opts.IncludeCustomMetadata = options.Custom
	}

	opts.Limit = testuplink.GetListLimit(ctx)

	uploads := UploadIterator{
		ctx:     ctx,
		project: project,
		bucket:  bucket,
		options: opts,
	}

	if opts.Prefix != "" && !strings.HasSuffix(opts.Prefix, "/") {
		uploads.listObjects = listPendingObjectStreams
	} else {
		uploads.listObjects = listObjects
	}

	if options != nil {
		uploads.uploadOptions = *options
	}

	return &uploads
}

// Part part metadata.
type Part struct {
	PartNumber uint32
	// Size plain size of a part.
	Size     int64
	Modified time.Time
	ETag     []byte
}

// PartUpload is a part upload to started multipart upload.
type PartUpload struct {
	mu      sync.Mutex
	closed  bool
	aborted bool
	cancel  context.CancelFunc
	upload  streamUpload
	bucket  string
	key     string
	part    *Part
	streams *streams.Store
	eTagCh  chan []byte

	stats operationStats
	task  func(*error)

	tracker leak.Ref
}

// Write uploads len(p) bytes from p to the object's data stream.
// It returns the number of bytes written from p (0 <= n <= len(p))
// and any error encountered that caused the write to stop early.
func (upload *PartUpload) Write(p []byte) (int, error) {
	track := upload.stats.trackWorking()
	n, err := upload.upload.Write(p)
	upload.mu.Lock()
	upload.stats.bytes += int64(n)
	upload.stats.flagFailure(err)
	track()
	upload.mu.Unlock()
	return n, convertKnownErrors(err, upload.bucket, upload.key)
}

// SetETag sets ETag for a part.
func (upload *PartUpload) SetETag(eTag []byte) error {
	upload.mu.Lock()
	defer upload.mu.Unlock()

	if upload.part.ETag != nil {
		return packageError.New("etag already set")
	}

	if upload.aborted {
		return errwrapf("%w: upload aborted", ErrUploadDone)
	}
	if upload.closed {
		return errwrapf("%w: already committed", ErrUploadDone)
	}

	upload.part.ETag = eTag
	upload.eTagCh <- eTag
	return nil
}

// Commit commits a part.
//
// Returns ErrUploadDone when either Abort or Commit has already been called.
func (upload *PartUpload) Commit() error {
	track := upload.stats.trackWorking()
	upload.mu.Lock()
	defer upload.mu.Unlock()

	if upload.aborted {
		return errwrapf("%w: already aborted", ErrUploadDone)
	}

	if upload.closed {
		return errwrapf("%w: already committed", ErrUploadDone)
	}

	upload.closed = true

	// ETag must not be sent after a call to commit. The upload code waits on
	// the channel before committing the last segment. Closing the channel
	// allows the upload code to unblock if no eTag has been set. Not all
	// multipart uploaders care about setting the eTag so we can't assume it
	// has been set.
	close(upload.eTagCh)

	err := errs.Combine(
		upload.upload.Commit(),
		upload.streams.Close(),
		upload.tracker.Close(),
	)
	upload.stats.flagFailure(err)
	track()
	upload.emitEvent(false)

	return convertKnownErrors(err, upload.bucket, upload.key)
}

// Abort aborts the part upload.
//
// Returns ErrUploadDone when either Abort or Commit has already been called.
func (upload *PartUpload) Abort() error {
	track := upload.stats.trackWorking()
	upload.mu.Lock()
	defer upload.mu.Unlock()

	if upload.closed {
		return errwrapf("%w: already committed", ErrUploadDone)
	}

	if upload.aborted {
		return errwrapf("%w: already aborted", ErrUploadDone)
	}

	upload.aborted = true
	upload.cancel()

	err := errs.Combine(
		upload.upload.Abort(),
		upload.streams.Close(),
		upload.tracker.Close(),
	)
	upload.stats.flagFailure(err)
	track()
	upload.emitEvent(true)

	return convertKnownErrors(err, upload.bucket, upload.key)
}

// Info returns the last information about the uploaded part.
func (upload *PartUpload) Info() *Part {
	if meta := upload.upload.Meta(); meta != nil {
		upload.part.Size = meta.Size
		upload.part.Modified = meta.Modified
	}
	return upload.part
}

func (upload *PartUpload) emitEvent(aborted bool) {
	message, err := upload.stats.err()
	upload.task(&err)

	evs.Event("part-upload",
		eventkit.Int64("bytes", upload.stats.bytes),
		eventkit.Duration("user-elapsed", time.Since(upload.stats.start)),
		eventkit.Duration("working-elapsed", upload.stats.working),
		eventkit.Bool("success", err == nil),
		eventkit.String("error", message),
		eventkit.Bool("aborted", aborted),
		eventkit.String("arch", runtime.GOARCH),
		eventkit.String("os", runtime.GOOS),
		eventkit.Int64("cpus", int64(runtime.NumCPU())),
		eventkit.Int64("quic-rollout", int64(upload.stats.quicRollout)),
		eventkit.String("satellite", upload.stats.satellite),
		eventkit.Bytes("path-checksum", pathChecksum(upload.stats.encPath)),
		eventkit.Int64("noise-version", noiseVersion),
		// segment count
		// ram available
	)
}
