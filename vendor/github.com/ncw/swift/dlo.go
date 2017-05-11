package swift

import (
	"os"
)

// DynamicLargeObjectCreateFile represents an open static large object
type DynamicLargeObjectCreateFile struct {
	largeObjectCreateFile
}

// DynamicLargeObjectCreateFile creates a dynamic large object
// returning an object which satisfies io.Writer, io.Seeker, io.Closer
// and io.ReaderFrom.  The flags are as passes to the
// largeObjectCreate method.
func (c *Connection) DynamicLargeObjectCreateFile(opts *LargeObjectOpts) (LargeObjectFile, error) {
	lo, err := c.largeObjectCreate(opts)
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
func (c *Connection) DynamicLargeObjectCreate(opts *LargeObjectOpts) (LargeObjectFile, error) {
	opts.Flags = os.O_TRUNC | os.O_CREATE
	return c.DynamicLargeObjectCreateFile(opts)
}

// DynamicLargeObjectDelete deletes a dynamic large object and all of its segments.
func (c *Connection) DynamicLargeObjectDelete(container string, path string) error {
	return c.LargeObjectDelete(container, path)
}

// DynamicLargeObjectMove moves a dynamic large object from srcContainer, srcObjectName to dstContainer, dstObjectName
func (c *Connection) DynamicLargeObjectMove(srcContainer string, srcObjectName string, dstContainer string, dstObjectName string) error {
	info, headers, err := c.Object(dstContainer, srcObjectName)
	if err != nil {
		return err
	}

	segmentContainer, segmentPath := parseFullPath(headers["X-Object-Manifest"])
	if err := c.createDLOManifest(dstContainer, dstObjectName, segmentContainer+"/"+segmentPath, info.ContentType); err != nil {
		return err
	}

	if err := c.ObjectDelete(srcContainer, srcObjectName); err != nil {
		return err
	}

	return nil
}

// createDLOManifest creates a dynamic large object manifest
func (c *Connection) createDLOManifest(container string, objectName string, prefix string, contentType string) error {
	headers := make(Headers)
	headers["X-Object-Manifest"] = prefix
	manifest, err := c.ObjectCreate(container, objectName, false, "", contentType, headers)
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
	return file.Flush()
}

func (file *DynamicLargeObjectCreateFile) Flush() error {
	err := file.conn.createDLOManifest(file.container, file.objectName, file.segmentContainer+"/"+file.prefix, file.contentType)
	if err != nil {
		return err
	}
	return file.conn.waitForSegmentsToShowUp(file.container, file.objectName, file.Size())
}

func (c *Connection) getAllDLOSegments(segmentContainer, segmentPath string) ([]Object, error) {
	//a simple container listing works 99.9% of the time
	segments, err := c.ObjectsAll(segmentContainer, &ObjectsOpts{Prefix: segmentPath})
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
		segment, _, err := c.Object(segmentContainer, segmentName)
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
