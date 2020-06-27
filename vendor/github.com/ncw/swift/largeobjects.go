package swift

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	gopath "path"
	"strconv"
	"strings"
	"time"
)

// NotLargeObject is returned if an operation is performed on an object which isn't large.
var NotLargeObject = errors.New("Not a large object")

// readAfterWriteTimeout defines the time we wait before an object appears after having been uploaded
var readAfterWriteTimeout = 15 * time.Second

// readAfterWriteWait defines the time to sleep between two retries
var readAfterWriteWait = 200 * time.Millisecond

// largeObjectCreateFile represents an open static or dynamic large object
type largeObjectCreateFile struct {
	conn             *Connection
	container        string
	objectName       string
	currentLength    int64
	filePos          int64
	chunkSize        int64
	segmentContainer string
	prefix           string
	contentType      string
	checkHash        bool
	segments         []Object
	headers          Headers
	minChunkSize     int64
}

func swiftSegmentPath(path string) (string, error) {
	checksum := sha1.New()
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	path = hex.EncodeToString(checksum.Sum(append([]byte(path), random...)))
	return strings.TrimLeft(strings.TrimRight("segments/"+path[0:3]+"/"+path[3:], "/"), "/"), nil
}

func getSegment(segmentPath string, partNumber int) string {
	return fmt.Sprintf("%s/%016d", segmentPath, partNumber)
}

func parseFullPath(manifest string) (container string, prefix string) {
	components := strings.SplitN(manifest, "/", 2)
	container = components[0]
	if len(components) > 1 {
		prefix = components[1]
	}
	return container, prefix
}

func (headers Headers) IsLargeObjectDLO() bool {
	_, isDLO := headers["X-Object-Manifest"]
	return isDLO
}

func (headers Headers) IsLargeObjectSLO() bool {
	_, isSLO := headers["X-Static-Large-Object"]
	return isSLO
}

func (headers Headers) IsLargeObject() bool {
	return headers.IsLargeObjectSLO() || headers.IsLargeObjectDLO()
}

func (c *Connection) getAllSegments(container string, path string, headers Headers) (string, []Object, error) {
	if manifest, isDLO := headers["X-Object-Manifest"]; isDLO {
		segmentContainer, segmentPath := parseFullPath(manifest)
		segments, err := c.getAllDLOSegments(segmentContainer, segmentPath)
		return segmentContainer, segments, err
	}
	if headers.IsLargeObjectSLO() {
		return c.getAllSLOSegments(container, path)
	}
	return "", nil, NotLargeObject
}

// LargeObjectOpts describes how a large object should be created
type LargeObjectOpts struct {
	Container        string  // Name of container to place object
	ObjectName       string  // Name of object
	Flags            int     // Creation flags
	CheckHash        bool    // If set Check the hash
	Hash             string  // If set use this hash to check
	ContentType      string  // Content-Type of the object
	Headers          Headers // Additional headers to upload the object with
	ChunkSize        int64   // Size of chunks of the object, defaults to 10MB if not set
	MinChunkSize     int64   // Minimum chunk size, automatically set for SLO's based on info
	SegmentContainer string  // Name of the container to place segments
	SegmentPrefix    string  // Prefix to use for the segments
	NoBuffer         bool    // Prevents using a bufio.Writer to write segments
}

type LargeObjectFile interface {
	io.Writer
	io.Seeker
	io.Closer
	Size() int64
	Flush() error
}

// largeObjectCreate creates a large object at opts.Container, opts.ObjectName.
//
// opts.Flags can have the following bits set
//   os.TRUNC  - remove the contents of the large object if it exists
//   os.APPEND - write at the end of the large object
func (c *Connection) largeObjectCreate(opts *LargeObjectOpts) (*largeObjectCreateFile, error) {
	var (
		segmentPath      string
		segmentContainer string
		segments         []Object
		currentLength    int64
		err              error
	)

	if opts.SegmentPrefix != "" {
		segmentPath = opts.SegmentPrefix
	} else if segmentPath, err = swiftSegmentPath(opts.ObjectName); err != nil {
		return nil, err
	}

	if info, headers, err := c.Object(opts.Container, opts.ObjectName); err == nil {
		if opts.Flags&os.O_TRUNC != 0 {
			c.LargeObjectDelete(opts.Container, opts.ObjectName)
		} else {
			currentLength = info.Bytes
			if headers.IsLargeObject() {
				segmentContainer, segments, err = c.getAllSegments(opts.Container, opts.ObjectName, headers)
				if err != nil {
					return nil, err
				}
				if len(segments) > 0 {
					segmentPath = gopath.Dir(segments[0].Name)
				}
			} else {
				if err = c.ObjectMove(opts.Container, opts.ObjectName, opts.Container, getSegment(segmentPath, 1)); err != nil {
					return nil, err
				}
				segments = append(segments, info)
			}
		}
	} else if err != ObjectNotFound {
		return nil, err
	}

	// segmentContainer is not empty when the manifest already existed
	if segmentContainer == "" {
		if opts.SegmentContainer != "" {
			segmentContainer = opts.SegmentContainer
		} else {
			segmentContainer = opts.Container + "_segments"
		}
	}

	file := &largeObjectCreateFile{
		conn:             c,
		checkHash:        opts.CheckHash,
		container:        opts.Container,
		objectName:       opts.ObjectName,
		chunkSize:        opts.ChunkSize,
		minChunkSize:     opts.MinChunkSize,
		headers:          opts.Headers,
		segmentContainer: segmentContainer,
		prefix:           segmentPath,
		segments:         segments,
		currentLength:    currentLength,
	}

	if file.chunkSize == 0 {
		file.chunkSize = 10 * 1024 * 1024
	}

	if file.minChunkSize > file.chunkSize {
		file.chunkSize = file.minChunkSize
	}

	if opts.Flags&os.O_APPEND != 0 {
		file.filePos = currentLength
	}

	return file, nil
}

// LargeObjectDelete deletes the large object named by container, path
func (c *Connection) LargeObjectDelete(container string, objectName string) error {
	_, headers, err := c.Object(container, objectName)
	if err != nil {
		return err
	}

	var objects [][]string
	if headers.IsLargeObject() {
		segmentContainer, segments, err := c.getAllSegments(container, objectName, headers)
		if err != nil {
			return err
		}
		for _, obj := range segments {
			objects = append(objects, []string{segmentContainer, obj.Name})
		}
	}
	objects = append(objects, []string{container, objectName})

	info, err := c.cachedQueryInfo()
	if err == nil && info.SupportsBulkDelete() && len(objects) > 0 {
		filenames := make([]string, len(objects))
		for i, obj := range objects {
			filenames[i] = obj[0] + "/" + obj[1]
		}
		_, err = c.doBulkDelete(filenames, nil)
		// Don't fail on ObjectNotFound because eventual consistency
		// makes this situation normal.
		if err != nil && err != Forbidden && err != ObjectNotFound {
			return err
		}
	} else {
		for _, obj := range objects {
			if err := c.ObjectDelete(obj[0], obj[1]); err != nil {
				return err
			}
		}
	}

	return nil
}

// LargeObjectGetSegments returns all the segments that compose an object
// If the object is a Dynamic Large Object (DLO), it just returns the objects
// that have the prefix as indicated by the manifest.
// If the object is a Static Large Object (SLO), it retrieves the JSON content
// of the manifest and return all the segments of it.
func (c *Connection) LargeObjectGetSegments(container string, path string) (string, []Object, error) {
	_, headers, err := c.Object(container, path)
	if err != nil {
		return "", nil, err
	}

	return c.getAllSegments(container, path, headers)
}

// Seek sets the offset for the next write operation
func (file *largeObjectCreateFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		file.filePos = offset
	case 1:
		file.filePos += offset
	case 2:
		file.filePos = file.currentLength + offset
	default:
		return -1, fmt.Errorf("invalid value for whence")
	}
	if file.filePos < 0 {
		return -1, fmt.Errorf("negative offset")
	}
	return file.filePos, nil
}

func (file *largeObjectCreateFile) Size() int64 {
	return file.currentLength
}

func withLORetry(expectedSize int64, fn func() (Headers, int64, error)) (err error) {
	endTimer := time.NewTimer(readAfterWriteTimeout)
	defer endTimer.Stop()
	waitingTime := readAfterWriteWait
	for {
		var headers Headers
		var sz int64
		if headers, sz, err = fn(); err == nil {
			if !headers.IsLargeObjectDLO() || (expectedSize == 0 && sz > 0) || expectedSize == sz {
				return
			}
		} else {
			return
		}
		waitTimer := time.NewTimer(waitingTime)
		select {
		case <-endTimer.C:
			waitTimer.Stop()
			err = fmt.Errorf("Timeout expired while waiting for object to have size == %d, got: %d", expectedSize, sz)
			return
		case <-waitTimer.C:
			waitingTime *= 2
		}
	}
}

func (c *Connection) waitForSegmentsToShowUp(container, objectName string, expectedSize int64) (err error) {
	err = withLORetry(expectedSize, func() (Headers, int64, error) {
		var info Object
		var headers Headers
		info, headers, err = c.objectBase(container, objectName)
		if err != nil {
			return headers, 0, err
		}
		return headers, info.Bytes, nil
	})
	return
}

// Write satisfies the io.Writer interface
func (file *largeObjectCreateFile) Write(buf []byte) (int, error) {
	var sz int64
	var relativeFilePos int
	writeSegmentIdx := 0
	for i, obj := range file.segments {
		if file.filePos < sz+obj.Bytes || (i == len(file.segments)-1 && file.filePos < sz+file.minChunkSize) {
			relativeFilePos = int(file.filePos - sz)
			break
		}
		writeSegmentIdx++
		sz += obj.Bytes
	}
	sizeToWrite := len(buf)
	for offset := 0; offset < sizeToWrite; {
		newSegment, n, err := file.writeSegment(buf[offset:], writeSegmentIdx, relativeFilePos)
		if err != nil {
			return 0, err
		}
		if writeSegmentIdx < len(file.segments) {
			file.segments[writeSegmentIdx] = *newSegment
		} else {
			file.segments = append(file.segments, *newSegment)
		}
		offset += n
		writeSegmentIdx++
		relativeFilePos = 0
	}
	file.filePos += int64(sizeToWrite)
	file.currentLength = 0
	for _, obj := range file.segments {
		file.currentLength += obj.Bytes
	}
	return sizeToWrite, nil
}

func (file *largeObjectCreateFile) writeSegment(buf []byte, writeSegmentIdx int, relativeFilePos int) (*Object, int, error) {
	var (
		readers         []io.Reader
		existingSegment *Object
		segmentSize     int
	)
	segmentName := getSegment(file.prefix, writeSegmentIdx+1)
	sizeToRead := int(file.chunkSize)
	if writeSegmentIdx < len(file.segments) {
		existingSegment = &file.segments[writeSegmentIdx]
		if writeSegmentIdx != len(file.segments)-1 {
			sizeToRead = int(existingSegment.Bytes)
		}
		if relativeFilePos > 0 {
			headers := make(Headers)
			headers["Range"] = "bytes=0-" + strconv.FormatInt(int64(relativeFilePos-1), 10)
			existingSegmentReader, _, err := file.conn.ObjectOpen(file.segmentContainer, segmentName, true, headers)
			if err != nil {
				return nil, 0, err
			}
			defer existingSegmentReader.Close()
			sizeToRead -= relativeFilePos
			segmentSize += relativeFilePos
			readers = []io.Reader{existingSegmentReader}
		}
	}
	if sizeToRead > len(buf) {
		sizeToRead = len(buf)
	}
	segmentSize += sizeToRead
	readers = append(readers, bytes.NewReader(buf[:sizeToRead]))
	if existingSegment != nil && segmentSize < int(existingSegment.Bytes) {
		headers := make(Headers)
		headers["Range"] = "bytes=" + strconv.FormatInt(int64(segmentSize), 10) + "-"
		tailSegmentReader, _, err := file.conn.ObjectOpen(file.segmentContainer, segmentName, true, headers)
		if err != nil {
			return nil, 0, err
		}
		defer tailSegmentReader.Close()
		segmentSize = int(existingSegment.Bytes)
		readers = append(readers, tailSegmentReader)
	}
	segmentReader := io.MultiReader(readers...)
	headers, err := file.conn.ObjectPut(file.segmentContainer, segmentName, segmentReader, true, "", file.contentType, nil)
	if err != nil {
		return nil, 0, err
	}
	return &Object{Name: segmentName, Bytes: int64(segmentSize), Hash: headers["Etag"]}, sizeToRead, nil
}

func withBuffer(opts *LargeObjectOpts, lo LargeObjectFile) LargeObjectFile {
	if !opts.NoBuffer {
		return &bufferedLargeObjectFile{
			LargeObjectFile: lo,
			bw:              bufio.NewWriterSize(lo, int(opts.ChunkSize)),
		}
	}
	return lo
}

type bufferedLargeObjectFile struct {
	LargeObjectFile
	bw *bufio.Writer
}

func (blo *bufferedLargeObjectFile) Close() error {
	err := blo.bw.Flush()
	if err != nil {
		return err
	}
	return blo.LargeObjectFile.Close()
}

func (blo *bufferedLargeObjectFile) Write(p []byte) (n int, err error) {
	return blo.bw.Write(p)
}

func (blo *bufferedLargeObjectFile) Seek(offset int64, whence int) (int64, error) {
	err := blo.bw.Flush()
	if err != nil {
		return 0, err
	}
	return blo.LargeObjectFile.Seek(offset, whence)
}

func (blo *bufferedLargeObjectFile) Size() int64 {
	return blo.LargeObjectFile.Size() + int64(blo.bw.Buffered())
}

func (blo *bufferedLargeObjectFile) Flush() error {
	err := blo.bw.Flush()
	if err != nil {
		return err
	}
	return blo.LargeObjectFile.Flush()
}
