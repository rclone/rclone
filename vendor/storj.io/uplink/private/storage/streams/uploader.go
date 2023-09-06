// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/encryption"
	"storj.io/common/paths"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/uplink/private/metaclient"
	"storj.io/uplink/private/storage/streams/pieceupload"
	"storj.io/uplink/private/storage/streams/segmentupload"
	"storj.io/uplink/private/storage/streams/splitter"
	"storj.io/uplink/private/storage/streams/streamupload"
	"storj.io/uplink/private/testuplink"
)

// At a high level, uploads are composed of two pieces: a SegmentSource and an
// UploaderBackend. The SegmentSource is responsible for providing the UploaderBackend
// with encrypted segments to upload along with the metadata necessary to upload them,
// and the UploaderBackend is responsible for uploading the all of segments as fast and as
// reliably as possible.
//
// One main reason for this split is to create a "light upload" where a smaller client
// can be the SegmentSource, encrypting and splitting the data, and a third party server
// can be the UploaderBackend, performing the Reed-Solomon encoding and uploading the pieces
// to nodes and issuing the correct RPCs to the satellite.
//
// Concretely, the SegmentSource is implemented by the splitter package, and the
// UploaderBackend is implemented by the streamupload package. The Uploader in this package
// creates and combines those two to perform the uploads without the help of a third
// party server.
//
// The splitter package exports the Splitter type which implements SegmentSource and has
// these methods (only Next is part of the SegmentSource interface):
//
//  * Write([]byte) (int, error): the standard io.Writer interface.
//  * Finish(error): informs the splitter that no more writes are coming
//                   with a potential error if there was one.
//  * Next(context.Context) (Segment, error): blocks until enough data has been
//                                            written to know if the segment should
//                                            be inline and then returns the next
//                                            segment to upload.
//
// where the Segment type is an interface that mainly allows one to get a reader
// for the data with the `Reader() io.Reader` method and has many other methods
// for getting the metadata an UploaderBackend would need to upload the segment.
// This means the Segment is somewhat like a promise type, where not necessarily
// all of the data is available immediately, but it will be available eventually.
//
// The Next method on the SegmentSource is used by the UploaderBackend when it wants
// a new segment to upload, and the Write and Finish calls are used by the client
// providing the data to upload. The splitter.Splitter has the property that there
// is bounded write-ahead: Write calls will block until enough data has been read from
// the io.Reader returned by the Segment. This provides backpressure to the client
//  avoiding the need for large buffers.
//
// The UploaderBackend ends up calling one of UploadObject or UploadPart
// in the streamupload package. Both of those end up dispatching to the
// same logic in uploadSegments which creates and uses many smaller helper
// types with small responsibilities. The main helper types are:
//
//  * streambatcher.Batcher: Wraps the metaclient.Batch api to keep track of
//                           plain bytes uploaded and the stream id of the upload,
//                           because sometimes the stream id is not known until
//                           after a BeginObject call.
//  * batchaggregator.Aggregator: Aggregates individual metaclient.BatchItems
//                                into larger batches, lazily flushing them
//                                to reduce round trips to the satellite.
//  * segmenttracker.Tracker: Some data is only available after the final segment
//                            is uploaded, like the ETag. But, since segments can
//                            be uploaded in parallel and finish out of order, we
//                            cannot commit any segment until we are sure it is not
//                            the last segment. This type holds CommitSegment calls
//                            until it knows the ETag will be available and then
//                            flushes them when possible.
//
// The uploadSegments function simply calls Next on the SegmentSource, issuing
// any RPCs necessary to begin the segment and passes it to a provided SegmentUploader.
// multiple segments to be uploaded in parallel thanks to the SegmentUploader interface.
// It has a method to begin uploading a segment that blocks until enough resources are
// available, and returns a handle that can be Waited on that returns when the segment
// is finished uploading. The function thus calls Begin synchronously with the loop
// getting the next segment, and then calls Wait asynchronously in a goroutine. Thus,
// the function is limited on the input side by the Write calls to the source, and
// limited on the output side on the Begin call blocking for enough resources.
// Finally, when all of the segments are finished indicated by Next returning a nil
// segment, it issues some more necessary RPCs to finish off the upload, and returns the
// result.
//
// The SegmentUploader is responsible for uploading an individual segment and also has
// many smaller helper types:
//
//  * pieceupload.LimitsExchanger: This allows the segment upload to request more
//                                 nodes to upload to when some of the piece uploads
//                                 fail, making segment uploads resilient.
//  * pieceupload.Manager: Responsible for handing out pieces to upload and in charge
//                         of using the LimitsExchanger when necessary. It keeps track
//                         of which pieces have succeeded and which have failed for
//                         final inspection, ensuring enough pieces are successful for
//                         an upload and constructing a FinishSegment RPC telling the
//                         satellite which pieces were successful.
//  * pieceupload.PiecePutter: This is the interface that actually performs an individual
//                             piece upload to a single node. It will grab a piece from
//                             the Manager and attempt to upload it, looping until it is
//                             either canceled or succeeds in uploading some piece to
//                             some node, both potentially different every iteration.
//  * scheduler.Scheduler: This is how the SegmentUploader ensures that it operates
//                         within some amount of resources. At time of writing, it is
//                         a priority semaphore in the sense that each segment upload
//                         grabs a Handle that can be used to grab a Resource. The total
//                         number of Resources is limited, and when a Resource is put
//                         back, the earliest acquired Handle is given priority. This
//                         ensures that earlier started segments finish more quickly
//                         keeping overall resource usage low.
//
// The SegmentUploader simply acquires a Handle from the scheduler, and for each piece
// acquires a Resource from the Handle, launches a goroutine that attempts to upload
// a piece and returns the Resource when it is done, and returns a type that knows how
// to wait for all of those goroutines to finish and return the upload results.

// MetainfoUpload are the metainfo methods needed to upload a stream.
type MetainfoUpload interface {
	metaclient.Batcher
	RetryBeginSegmentPieces(ctx context.Context, params metaclient.RetryBeginSegmentPiecesParams) (metaclient.RetryBeginSegmentPiecesResponse, error)
	io.Closer
}

// Uploader uploads object or part streams.
type Uploader struct {
	metainfo             MetainfoUpload
	piecePutter          pieceupload.PiecePutter
	segmentSize          int64
	encStore             *encryption.Store
	encryptionParameters storj.EncryptionParameters
	inlineThreshold      int
	longTailMargin       int

	// The backend is fixed to the real backend in production but is overridden
	// for testing.
	backend uploaderBackend
}

// NewUploader constructs a new stream putter.
func NewUploader(metainfo MetainfoUpload, piecePutter pieceupload.PiecePutter, segmentSize int64, encStore *encryption.Store, encryptionParameters storj.EncryptionParameters, inlineThreshold, longTailMargin int) (*Uploader, error) {
	switch {
	case segmentSize <= 0:
		return nil, errs.New("segment size must be larger than 0")
	case encryptionParameters.BlockSize <= 0:
		return nil, errs.New("encryption block size must be larger than 0")
	case inlineThreshold <= 0:
		return nil, errs.New("inline threshold must be larger than 0")
	}
	return &Uploader{
		metainfo:             metainfo,
		piecePutter:          piecePutter,
		segmentSize:          segmentSize,
		encStore:             encStore,
		encryptionParameters: encryptionParameters,
		inlineThreshold:      inlineThreshold,
		longTailMargin:       longTailMargin,
		backend:              realUploaderBackend{},
	}, nil
}

// Close closes the underlying resources for the uploader.
func (u *Uploader) Close() error {
	return u.metainfo.Close()
}

var uploadCounter int64

// UploadObject starts an upload of an object to the given location. The object
// contents can be written to the returned upload, which can then be committed.
func (u *Uploader) UploadObject(ctx context.Context, bucket, unencryptedKey string, metadata Metadata, expiration time.Time, sched segmentupload.Scheduler) (_ *Upload, err error) {
	ctx = testuplink.WithLogWriterContext(ctx, "upload", fmt.Sprint(atomic.AddInt64(&uploadCounter, 1)))
	testuplink.Log(ctx, "Starting upload")
	defer testuplink.Log(ctx, "Done starting upload")

	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	done := make(chan uploadResult, 1)

	derivedKey, err := encryption.DeriveContentKey(bucket, paths.NewUnencrypted(unencryptedKey), u.encStore)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	encPath, err := encryption.EncryptPathWithStoreCipher(bucket, paths.NewUnencrypted(unencryptedKey), u.encStore)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	split, err := splitter.New(splitter.Options{
		Split:      u.segmentSize,
		Minimum:    int64(u.inlineThreshold),
		Params:     u.encryptionParameters,
		Key:        derivedKey,
		PartNumber: 0,
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}
	go func() {
		<-ctx.Done()
		split.Finish(ctx.Err())
	}()

	beginObject := &metaclient.BeginObjectParams{
		Bucket:               []byte(bucket),
		EncryptedObjectKey:   []byte(encPath.Raw()),
		ExpiresAt:            expiration,
		EncryptionParameters: u.encryptionParameters,
	}

	uploader := segmentUploader{metainfo: u.metainfo, piecePutter: u.piecePutter, sched: sched, longTailMargin: u.longTailMargin}

	encMeta := u.newEncryptedMetadata(metadata, derivedKey)

	go func() {
		info, err := u.backend.UploadObject(
			ctx,
			split,
			uploader,
			u.metainfo,
			beginObject,
			encMeta,
		)
		// On failure, we need to "finish" the splitter with an error so that
		// outstanding writes to the splitter fail, otherwise the writes will
		// block waiting for the upload to read the stream.
		if err != nil {
			split.Finish(err)
		}
		testuplink.Log(ctx, "Upload finished. err:", err)
		done <- uploadResult{info: info, err: err}
	}()

	return &Upload{
		split:  split,
		done:   done,
		cancel: cancel,
	}, nil
}

// UploadPart starts an upload of a part to the given location for the given
// multipart upload stream. The eTag is an optional channel is used to provide
// the eTag to be encrypted and included in the final segment of the part. The
// eTag should be  sent on the channel only after the contents of the part have
// been fully written to the returned upload, but before calling Commit.
func (u *Uploader) UploadPart(ctx context.Context, bucket, unencryptedKey string, streamID storj.StreamID, partNumber int32, eTag <-chan []byte, sched segmentupload.Scheduler) (_ *Upload, err error) {
	ctx = testuplink.WithLogWriterContext(ctx,
		"upload", fmt.Sprint(atomic.AddInt64(&uploadCounter, 1)),
		"part_number", fmt.Sprint(partNumber),
	)
	testuplink.Log(ctx, "Starting upload")
	defer testuplink.Log(ctx, "Done starting upload")

	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	done := make(chan uploadResult, 1)

	derivedKey, err := encryption.DeriveContentKey(bucket, paths.NewUnencrypted(unencryptedKey), u.encStore)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	split, err := splitter.New(splitter.Options{
		Split:      u.segmentSize,
		Minimum:    int64(u.inlineThreshold),
		Params:     u.encryptionParameters,
		Key:        derivedKey,
		PartNumber: partNumber,
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}
	go func() {
		<-ctx.Done()
		split.Finish(ctx.Err())
	}()

	uploader := segmentUploader{metainfo: u.metainfo, piecePutter: u.piecePutter, sched: sched, longTailMargin: u.longTailMargin}

	go func() {
		info, err := u.backend.UploadPart(
			ctx,
			split,
			uploader,
			u.metainfo,
			streamID,
			eTag,
		)
		// On failure, we need to "finish" the splitter with an error so that
		// outstanding writes to the splitter fail, otherwise the writes will
		// block waiting for the upload to read the stream.
		if err != nil {
			split.Finish(err)
		}
		testuplink.Log(ctx, "Upload finished. err:", err)
		done <- uploadResult{info: info, err: err}
	}()

	return &Upload{
		split:  split,
		done:   done,
		cancel: cancel,
	}, nil
}

func (u *Uploader) newEncryptedMetadata(metadata Metadata, derivedKey *storj.Key) streamupload.EncryptedMetadata {
	return &encryptedMetadata{
		metadata:    metadata,
		segmentSize: u.segmentSize,
		derivedKey:  derivedKey,
		cipherSuite: u.encryptionParameters.CipherSuite,
	}
}

type segmentUploader struct {
	metainfo       MetainfoUpload
	piecePutter    pieceupload.PiecePutter
	sched          segmentupload.Scheduler
	longTailMargin int
}

func (u segmentUploader) Begin(ctx context.Context, beginSegment *metaclient.BeginSegmentResponse, segment splitter.Segment) (streamupload.SegmentUpload, error) {
	return segmentupload.Begin(ctx, beginSegment, segment, limitsExchanger{u.metainfo}, u.piecePutter, u.sched, u.longTailMargin)
}

type limitsExchanger struct {
	metainfo MetainfoUpload
}

func (e limitsExchanger) ExchangeLimits(ctx context.Context, segmentID storj.SegmentID, pieceNumbers []int) (storj.SegmentID, []*pb.AddressedOrderLimit, error) {
	resp, err := e.metainfo.RetryBeginSegmentPieces(ctx, metaclient.RetryBeginSegmentPiecesParams{
		SegmentID:         segmentID,
		RetryPieceNumbers: pieceNumbers,
	})
	if err != nil {
		return nil, nil, err
	}
	return resp.SegmentID, resp.Limits, nil
}

type encryptedMetadata struct {
	metadata    Metadata
	segmentSize int64
	derivedKey  *storj.Key
	cipherSuite storj.CipherSuite
}

func (e *encryptedMetadata) EncryptedMetadata(lastSegmentSize int64) (data []byte, encKey *storj.EncryptedPrivateKey, nonce *storj.Nonce, err error) {
	metadataBytes, err := e.metadata.Metadata()
	if err != nil {
		return nil, nil, nil, err
	}

	streamInfo, err := pb.Marshal(&pb.StreamInfo{
		SegmentsSize:    e.segmentSize,
		LastSegmentSize: lastSegmentSize,
		Metadata:        metadataBytes,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	var metadataKey storj.Key
	if _, err := rand.Read(metadataKey[:]); err != nil {
		return nil, nil, nil, err
	}

	var encryptedMetadataKeyNonce storj.Nonce
	if _, err := rand.Read(encryptedMetadataKeyNonce[:]); err != nil {
		return nil, nil, nil, err
	}

	// encrypt the metadata key with the derived key and the random encrypted key nonce
	encryptedMetadataKey, err := encryption.EncryptKey(&metadataKey, e.cipherSuite, e.derivedKey, &encryptedMetadataKeyNonce)
	if err != nil {
		return nil, nil, nil, err
	}

	// encrypt the stream info with the metadata key and the zero nonce
	encryptedStreamInfo, err := encryption.Encrypt(streamInfo, e.cipherSuite, &metadataKey, &storj.Nonce{})
	if err != nil {
		return nil, nil, nil, err
	}

	streamMeta, err := pb.Marshal(&pb.StreamMeta{
		EncryptedStreamInfo: encryptedStreamInfo,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	return streamMeta, &encryptedMetadataKey, &encryptedMetadataKeyNonce, nil
}

type uploaderBackend interface {
	UploadObject(ctx context.Context, segmentSource streamupload.SegmentSource, segmentUploader streamupload.SegmentUploader, miBatcher metaclient.Batcher, beginObject *metaclient.BeginObjectParams, encMeta streamupload.EncryptedMetadata) (streamupload.Info, error)
	UploadPart(ctx context.Context, segmentSource streamupload.SegmentSource, segmentUploader streamupload.SegmentUploader, miBatcher metaclient.Batcher, streamID storj.StreamID, eTagCh <-chan []byte) (streamupload.Info, error)
}

type realUploaderBackend struct{}

func (realUploaderBackend) UploadObject(ctx context.Context, segmentSource streamupload.SegmentSource, segmentUploader streamupload.SegmentUploader, miBatcher metaclient.Batcher, beginObject *metaclient.BeginObjectParams, encMeta streamupload.EncryptedMetadata) (streamupload.Info, error) {
	return streamupload.UploadObject(ctx, segmentSource, segmentUploader, miBatcher, beginObject, encMeta)
}

func (realUploaderBackend) UploadPart(ctx context.Context, segmentSource streamupload.SegmentSource, segmentUploader streamupload.SegmentUploader, miBatcher metaclient.Batcher, streamID storj.StreamID, eTagCh <-chan []byte) (streamupload.Info, error) {
	return streamupload.UploadPart(ctx, segmentSource, segmentUploader, miBatcher, streamID, eTagCh)
}
