package swift

import (
	"context"
	"os"
	"strings"
)

// DynamicLargeObjectCreateFile represents an open static large object
type DynamicLargeObjectCreateFile struct {
	largeObjectCreateFile
}

// DynamicLargeObjectCreateFile creates a dynamic large object
// returning an object which satisfies io.Writer, io.Seeker, io.Closer
// and io.ReaderFrom.  The flags are as passes to the
// largeObjectCreate method.
func (c *Connection) DynamicLargeObjectCreateFile(ctx context.Context, opts *LargeObjectOpts) (LargeObjectFile, error) {
	lo, err := c.largeObjectCreate(ctx, opts)
	if err != nil {
		return nil, err
	}

	return withBuffer(opts, &DynamicLargeObjectCreateFile{
		largeObjectCreateFile: *lo,
	}), nil
}

// DynamicLargeObjectCreate creates or truncates an existing dynamic
// large object returning a writeable object.  This sets opts.Flags to
// an appropriate value before calling DynamicLargeObjectCreateFile
func (c *Connection) DynamicLargeObjectCreate(ctx context.Context, opts *LargeObjectOpts) (LargeObjectFile, error) {
	opts.Flags = os.O_TRUNC | os.O_CREATE
	return c.DynamicLargeObjectCreateFile(ctx, opts)
}

// DynamicLargeObjectDelete deletes a dynamic large object and all of its segments.
func (c *Connection) DynamicLargeObjectDelete(ctx context.Context, container string, path string) error {
	return c.LargeObjectDelete(ctx, container, path)
}

// DynamicLargeObjectMove moves a dynamic large object from srcContainer, srcObjectName to dstContainer, dstObjectName
func (c *Connection) DynamicLargeObjectMove(ctx context.Context, srcContainer string, srcObjectName string, dstContainer string, dstObjectName string) error {
	info, headers, err := c.Object(ctx, srcContainer, srcObjectName)
	if err != nil {
		return err
	}

	segmentContainer, segmentPath, err := parseFullPath(headers["X-Object-Manifest"])
	if err != nil {
		return err
	}

	if err := c.createDLOManifest(ctx, dstContainer, dstObjectName, segmentContainer+"/"+segmentPath, info.ContentType, sanitizeLargeObjectMoveHeaders(headers)); err != nil {
		return err
	}

	if err := c.ObjectDelete(ctx, srcContainer, srcObjectName); err != nil {
		return err
	}

	return nil
}

func sanitizeLargeObjectMoveHeaders(headers Headers) Headers {
	sanitizedHeaders := make(map[string]string, len(headers))
	for k, v := range headers {
		if strings.HasPrefix(k, "X-") { //Some of the fields does not effect the request e,g, X-Timestamp, X-Trans-Id, X-Openstack-Request-Id. Open stack will generate new ones anyway.
			sanitizedHeaders[k] = v
		}
	}
	return sanitizedHeaders
}

// createDLOManifest creates a dynamic large object manifest
func (c *Connection) createDLOManifest(ctx context.Context, container string, objectName string, prefix string, contentType string, headers Headers) error {
	if headers == nil {
		headers = make(Headers)
	}
	headers["X-Object-Manifest"] = prefix
	manifest, err := c.ObjectCreate(ctx, container, objectName, false, "", contentType, headers)
	if err != nil {
		return err
	}

	if err := manifest.Close(); err != nil {
		return err
	}

	return nil
}

// Close satisfies the io.Closer interface
func (file *DynamicLargeObjectCreateFile) Close() error {
	return file.CloseWithContext(context.Background())
}

func (file *DynamicLargeObjectCreateFile) CloseWithContext(ctx context.Context) error {
	return file.Flush(ctx)
}

func (file *DynamicLargeObjectCreateFile) Flush(ctx context.Context) error {
	err := file.conn.createDLOManifest(ctx, file.container, file.objectName, file.segmentContainer+"/"+file.prefix, file.contentType, file.headers)
	if err != nil {
		return err
	}
	return file.conn.waitForSegmentsToShowUp(ctx, file.container, file.objectName, file.Size())
}

func (c *Connection) getAllDLOSegments(ctx context.Context, segmentContainer, segmentPath string) ([]Object, error) {
	//a simple container listing works 99.9% of the time
	segments, err := c.ObjectsAll(ctx, segmentContainer, &ObjectsOpts{Prefix: segmentPath})
	if err != nil {
		return nil, err
	}

	hasObjectName := make(map[string]struct{})
	for _, segment := range segments {
		hasObjectName[segment.Name] = struct{}{}
	}

	//The container listing might be outdated (i.e. not contain all existing
	//segment objects yet) because of temporary inconsistency (Swift is only
	//eventually consistent!). Check its completeness.
	segmentNumber := 0
	for {
		segmentNumber++
		segmentName := getSegment(segmentPath, segmentNumber)
		if _, seen := hasObjectName[segmentName]; seen {
			continue
		}

		//This segment is missing in the container listing. Use a more reliable
		//request to check its existence. (HEAD requests on segments are
		//guaranteed to return the correct metadata, except for the pathological
		//case of an outage of large parts of the Swift cluster or its network,
		//since every segment is only written once.)
		segment, _, err := c.Object(ctx, segmentContainer, segmentName)
		switch err {
		case nil:
			//found new segment -> add it in the correct position and keep
			//going, more might be missing
			if segmentNumber <= len(segments) {
				segments = append(segments[:segmentNumber], segments[segmentNumber-1:]...)
				segments[segmentNumber-1] = segment
			} else {
				segments = append(segments, segment)
			}
			continue
		case ObjectNotFound:
			//This segment is missing. Since we upload segments sequentially,
			//there won't be any more segments after it.
			return segments, nil
		default:
			return nil, err //unexpected error
		}
	}
}
