package swift

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
)

// StaticLargeObjectCreateFile represents an open static large object
type StaticLargeObjectCreateFile struct {
	largeObjectCreateFile
}

// SLONotSupported is returned as an error when Static Large Objects are not supported.
//
//nolint:stylecheck
var SLONotSupported = errors.New("SLO not supported")

type swiftSegment struct {
	Path string `json:"path,omitempty"`
	Etag string `json:"etag,omitempty"`
	Size int64  `json:"size_bytes,omitempty"`
	// When uploading a manifest, the attributes must be named `path`, `etag` and `size_bytes`
	// but when querying the JSON content of a manifest with the `multipart-manifest=get`
	// parameter, Swift names those attributes `name`, `hash` and `bytes`.
	// We use all the different attributes names in this structure to be able to use
	// the same structure for both uploading and retrieving.
	Name         string `json:"name,omitempty"`
	Hash         string `json:"hash,omitempty"`
	Bytes        int64  `json:"bytes,omitempty"`
	ContentType  string `json:"content_type,omitempty"`
	LastModified string `json:"last_modified,omitempty"`
}

// StaticLargeObjectCreateFile creates a static large object returning
// an object which satisfies io.Writer, io.Seeker, io.Closer and
// io.ReaderFrom.  The flags are as passed to the largeObjectCreate
// method.
func (c *Connection) StaticLargeObjectCreateFile(ctx context.Context, opts *LargeObjectOpts) (LargeObjectFile, error) {
	info, err := c.cachedQueryInfo(ctx)
	if err != nil || !info.SupportsSLO() {
		return nil, SLONotSupported
	}
	realMinChunkSize := info.SLOMinSegmentSize()
	if realMinChunkSize > opts.MinChunkSize {
		opts.MinChunkSize = realMinChunkSize
	}
	lo, err := c.largeObjectCreate(ctx, opts)
	if err != nil {
		return nil, err
	}
	return withBuffer(opts, &StaticLargeObjectCreateFile{
		largeObjectCreateFile: *lo,
	}), nil
}

// StaticLargeObjectCreate creates or truncates an existing static
// large object returning a writeable object. This sets opts.Flags to
// an appropriate value before calling StaticLargeObjectCreateFile
func (c *Connection) StaticLargeObjectCreate(ctx context.Context, opts *LargeObjectOpts) (LargeObjectFile, error) {
	opts.Flags = os.O_TRUNC | os.O_CREATE
	return c.StaticLargeObjectCreateFile(ctx, opts)
}

// StaticLargeObjectDelete deletes a static large object and all of its segments.
func (c *Connection) StaticLargeObjectDelete(ctx context.Context, container string, path string) error {
	info, err := c.cachedQueryInfo(ctx)
	if err != nil || !info.SupportsSLO() {
		return SLONotSupported
	}
	return c.LargeObjectDelete(ctx, container, path)
}

// StaticLargeObjectMove moves a static large object from srcContainer, srcObjectName to dstContainer, dstObjectName
func (c *Connection) StaticLargeObjectMove(ctx context.Context, srcContainer string, srcObjectName string, dstContainer string, dstObjectName string) error {
	swiftInfo, err := c.cachedQueryInfo(ctx)
	if err != nil || !swiftInfo.SupportsSLO() {
		return SLONotSupported
	}
	info, headers, err := c.Object(ctx, srcContainer, srcObjectName)
	if err != nil {
		return err
	}

	container, segments, err := c.getAllSegments(ctx, srcContainer, srcObjectName, headers)
	if err != nil {
		return err
	}

	//copy only metadata during move (other headers might not be safe for copying)
	headers = headers.ObjectMetadata().ObjectHeaders()

	if err := c.createSLOManifest(ctx, dstContainer, dstObjectName, info.ContentType, container, segments, headers); err != nil {
		return err
	}

	if err := c.ObjectDelete(ctx, srcContainer, srcObjectName); err != nil {
		return err
	}

	return nil
}

// createSLOManifest creates a static large object manifest
func (c *Connection) createSLOManifest(ctx context.Context, container string, path string, contentType string, segmentContainer string, segments []Object, h Headers) error {
	sloSegments := make([]swiftSegment, len(segments))
	for i, segment := range segments {
		sloSegments[i].Path = fmt.Sprintf("%s/%s", segmentContainer, segment.Name)
		sloSegments[i].Etag = segment.Hash
		sloSegments[i].Size = segment.Bytes
	}

	content, err := json.Marshal(sloSegments)
	if err != nil {
		return err
	}

	values := url.Values{}
	values.Set("multipart-manifest", "put")
	if _, err := c.objectPut(ctx, container, path, bytes.NewBuffer(content), false, "", contentType, h, values); err != nil {
		return err
	}

	return nil
}

func (file *StaticLargeObjectCreateFile) Close() error {
	return file.CloseWithContext(context.Background())
}

func (file *StaticLargeObjectCreateFile) CloseWithContext(ctx context.Context) error {
	return file.Flush(ctx)
}

func (file *StaticLargeObjectCreateFile) Flush(ctx context.Context) error {
	if err := file.conn.createSLOManifest(ctx, file.container, file.objectName, file.contentType, file.segmentContainer, file.segments, file.headers); err != nil {
		return err
	}
	return file.conn.waitForSegmentsToShowUp(ctx, file.container, file.objectName, file.Size())
}

func (c *Connection) getAllSLOSegments(ctx context.Context, container, path string) (string, []Object, error) {
	var (
		segmentList      []swiftSegment
		segments         []Object
		segPath          string
		segmentContainer string
	)

	values := url.Values{}
	values.Set("multipart-manifest", "get")

	file, _, err := c.objectOpen(ctx, container, path, true, nil, values)
	if err != nil {
		return "", nil, err
	}

	content, err := ioutil.ReadAll(file)
	if err != nil {
		return "", nil, err
	}

	err = json.Unmarshal(content, &segmentList)
	if err != nil {
		return "", nil, err
	}
	for _, segment := range segmentList {
		segmentContainer, segPath, err = parseFullPath(segment.Name[1:])
		if err != nil {
			return "", nil, err
		}
		segments = append(segments, Object{
			Name:  segPath,
			Bytes: segment.Bytes,
			Hash:  segment.Hash,
		})
	}

	return segmentContainer, segments, nil
}
