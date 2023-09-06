package hdfs

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	hdfs "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_hdfs"
	"github.com/colinmarc/hdfs/v2/internal/transfer"
	"google.golang.org/protobuf/proto"
)

// A FileReader represents an existing file or directory in HDFS. It implements
// io.Reader, io.ReaderAt, io.Seeker, and io.Closer, and can only be used for
// reads. For writes, see FileWriter and Client.Create.
type FileReader struct {
	client *Client
	name   string
	info   os.FileInfo

	blocks      []*hdfs.LocatedBlockProto
	blockReader *transfer.BlockReader
	deadline    time.Time
	offset      int64

	readdirLast string

	closed bool
}

// Open returns an FileReader which can be used for reading.
func (c *Client) Open(name string) (*FileReader, error) {
	info, err := c.getFileInfo(name)
	if err != nil {
		return nil, &os.PathError{"open", name, interpretException(err)}
	}

	return &FileReader{
		client: c,
		name:   name,
		info:   info,
		closed: false,
	}, nil
}

// Name returns the name of the file.
func (f *FileReader) Name() string {
	return f.info.Name()
}

// Stat returns the FileInfo structure describing file.
func (f *FileReader) Stat() os.FileInfo {
	return f.info
}

// SetDeadline sets the deadline for future Read, ReadAt, and Checksum calls. A
// zero value for t means those calls will not time out.
func (f *FileReader) SetDeadline(t time.Time) error {
	f.deadline = t
	if f.blockReader != nil {
		return f.blockReader.SetDeadline(t)
	}

	// Return the error at connection time.
	return nil
}

// Checksum returns HDFS's internal "MD5MD5CRC32C" checksum for a given file.
//
// Internally to HDFS, it works by calculating the MD5 of all the CRCs (which
// are stored alongside the data) for each block, and then calculating the MD5
// of all of those.
func (f *FileReader) Checksum() ([]byte, error) {
	if f.info.IsDir() {
		return nil, &os.PathError{
			"checksum",
			f.name,
			errors.New("is a directory"),
		}
	}

	if f.blocks == nil {
		err := f.getBlocks()
		if err != nil {
			return nil, err
		}
	}

	// Hadoop calculates this by writing the checksums out to a byte array, which
	// is automatically padded with zeroes out to the next  power of 2
	// (with a minimum of 32)... and then takes the MD5 of that array, including
	// the zeroes. This is pretty shady business, but we want to track
	// the 'hadoop fs -checksum' behavior if possible.
	paddedLength := 32
	totalLength := 0
	checksum := md5.New()

	for _, block := range f.blocks {
		d, err := f.client.wrapDatanodeDial(f.client.options.DatanodeDialFunc,
			block.GetBlockToken())
		if err != nil {
			return nil, err
		}

		cr := &transfer.ChecksumReader{
			Block:               block,
			UseDatanodeHostname: f.client.options.UseDatanodeHostname,
			DialFunc:            d,
		}

		err = cr.SetDeadline(f.deadline)
		if err != nil {
			return nil, err
		}

		blockChecksum, err := cr.ReadChecksum()
		if err != nil {
			return nil, err
		}

		checksum.Write(blockChecksum)
		totalLength += len(blockChecksum)
		if paddedLength < totalLength {
			paddedLength *= 2
		}
	}

	checksum.Write(make([]byte, paddedLength-totalLength))
	return checksum.Sum(nil), nil
}

// Seek implements io.Seeker.
//
// The seek is virtual - it starts a new block read at the new position.
func (f *FileReader) Seek(offset int64, whence int) (int64, error) {
	if f.closed {
		return 0, io.ErrClosedPipe
	}

	var off int64
	switch whence {
	case io.SeekStart:
		off = offset
	case io.SeekCurrent:
		off = f.offset + offset
	case io.SeekEnd:
		off = f.info.Size() + offset
	default:
		return f.offset, fmt.Errorf("invalid whence: %d", whence)
	}

	if off < 0 || off > f.info.Size() {
		return f.offset, fmt.Errorf("invalid resulting offset: %d", off)
	}

	if f.blockReader != nil {
		// If the seek is within the next few chunks, it's much more
		// efficient to throw away a few bytes than to reconnect and start
		// a read at the new offset.
		err := f.blockReader.Skip(off - f.offset)
		if err != nil {
			// It isn't possible to skip forward in the current block, so reset such
			// that we can reconnect at the new offset.
			f.blockReader.Close()
			f.blockReader = nil
		}
	}

	f.offset = off
	return f.offset, nil
}

// Read implements io.Reader.
func (f *FileReader) Read(b []byte) (int, error) {
	if f.closed {
		return 0, io.ErrClosedPipe
	}

	if f.info.IsDir() {
		return 0, &os.PathError{
			"read",
			f.name,
			errors.New("is a directory"),
		}
	}

	if f.offset >= f.info.Size() {
		return 0, io.EOF
	}

	if len(b) == 0 {
		return 0, nil
	}

	if f.blocks == nil {
		err := f.getBlocks()
		if err != nil {
			return 0, err
		}
	}

	for {
		if f.blockReader == nil {
			err := f.getNewBlockReader()
			if err != nil {
				return 0, err
			}
		}

		n, err := f.blockReader.Read(b)
		f.offset += int64(n)

		if err != nil && err != io.EOF {
			f.blockReader.Close()
			f.blockReader = nil
			return n, err
		} else if n > 0 {
			return n, nil
		}

		err = f.blockReader.Close()
		f.blockReader = nil
		if err != nil {
			return n, err
		}
	}
}

// ReadAt implements io.ReaderAt.
func (f *FileReader) ReadAt(b []byte, off int64) (int, error) {
	if f.closed {
		return 0, io.ErrClosedPipe
	}

	if off < 0 {
		return 0, &os.PathError{"readat", f.name, errors.New("negative offset")}
	}

	_, err := f.Seek(off, 0)
	if err != nil {
		return 0, err
	}

	n, err := io.ReadFull(f, b)

	// For some reason, os.File.ReadAt returns io.EOF in this case instead of
	// io.ErrUnexpectedEOF.
	if err == io.ErrUnexpectedEOF {
		err = io.EOF
	}

	return n, err
}

// Readdir reads the contents of the directory associated with file and returns
// a slice of up to n os.FileInfo values, as would be returned by Stat, in
// directory order. Subsequent calls on the same file will yield further
// os.FileInfos.
//
// If n > 0, Readdir returns at most n os.FileInfo values. In this case, if
// Readdir returns an empty slice, it will return a non-nil error explaining
// why. At the end of a directory, the error is io.EOF.
//
// If n <= 0, Readdir returns all the os.FileInfo from the directory in a single
// slice. In this case, if Readdir succeeds (reads all the way to the end of
// the directory), it returns the slice and a nil error. If it encounters an
// error before the end of the directory, Readdir returns the os.FileInfo read
// until that point and a non-nil error.
//
// The os.FileInfo values returned will not have block location attached to
// the struct returned by Sys(). To fetch that information, make a separate
// call to Stat.
//
// Note that making multiple calls to Readdir with a smallish n (as you might do
// with the os version) is slower than just requesting everything at once.
// That's because HDFS has no mechanism for limiting the number of entries
// returned; whatever extra entries it returns are simply thrown away.
func (f *FileReader) Readdir(n int) ([]os.FileInfo, error) {
	if f.closed {
		return nil, io.ErrClosedPipe
	}

	if !f.info.IsDir() {
		return nil, &os.PathError{
			"readdir",
			f.name,
			errors.New("the file is not a directory"),
		}
	}

	if n <= 0 {
		f.readdirLast = ""
	}

	res := make([]os.FileInfo, 0)
	for {
		batch, remaining, err := f.readdir()
		if err != nil {
			return nil, &os.PathError{"readdir", f.name, interpretException(err)}
		}

		if len(batch) > 0 {
			f.readdirLast = batch[len(batch)-1].Name()
		}

		res = append(res, batch...)
		if remaining == 0 || (n > 0 && len(res) >= n) {
			break
		}
	}

	if n > 0 {
		if len(res) == 0 {
			return nil, io.EOF
		}

		if len(res) > n {
			res = res[:n]
			f.readdirLast = res[len(res)-1].Name()
		}
	}

	return res, nil
}

func (f *FileReader) readdir() ([]os.FileInfo, int, error) {
	req := &hdfs.GetListingRequestProto{
		Src:          proto.String(f.name),
		StartAfter:   []byte(f.readdirLast),
		NeedLocation: proto.Bool(false),
	}
	resp := &hdfs.GetListingResponseProto{}

	err := f.client.namenode.Execute("getListing", req, resp)
	if err != nil {
		return nil, 0, err
	} else if resp.GetDirList() == nil {
		return nil, 0, os.ErrNotExist
	}

	list := resp.GetDirList().GetPartialListing()
	res := make([]os.FileInfo, 0, len(list))
	for _, status := range list {
		res = append(res, newFileInfo(status, ""))
	}

	remaining := int(resp.GetDirList().GetRemainingEntries())
	return res, remaining, nil
}

// Readdirnames reads and returns a slice of names from the directory f.
//
// If n > 0, Readdirnames returns at most n names. In this case, if Readdirnames
// returns an empty slice, it will return a non-nil error explaining why. At the
// end of a directory, the error is io.EOF.
//
// If n <= 0, Readdirnames returns all the names from the directory in a single
// slice. In this case, if Readdirnames succeeds (reads all the way to the end
// of the directory), it returns the slice and a nil error. If it encounters an
// error before the end of the directory, Readdirnames returns the names read
// until that point and a non-nil error.
func (f *FileReader) Readdirnames(n int) ([]string, error) {
	if f.closed {
		return nil, io.ErrClosedPipe
	}

	fis, err := f.Readdir(n)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(fis))
	for _, fi := range fis {
		names = append(names, fi.Name())
	}

	return names, nil
}

// Close implements io.Closer.
func (f *FileReader) Close() error {
	f.closed = true

	if f.blockReader != nil {
		return f.blockReader.Close()
	}

	return nil
}

func (f *FileReader) getBlocks() error {
	req := &hdfs.GetBlockLocationsRequestProto{
		Src:    proto.String(f.name),
		Offset: proto.Uint64(0),
		Length: proto.Uint64(uint64(f.info.Size())),
	}
	resp := &hdfs.GetBlockLocationsResponseProto{}

	err := f.client.namenode.Execute("getBlockLocations", req, resp)
	if err != nil {
		return err
	}

	f.blocks = resp.GetLocations().GetBlocks()
	return nil
}

func (f *FileReader) getNewBlockReader() error {
	off := uint64(f.offset)
	for _, block := range f.blocks {
		start := block.GetOffset()
		end := start + block.GetB().GetNumBytes()

		if start <= off && off < end {
			dialFunc, err := f.client.wrapDatanodeDial(
				f.client.options.DatanodeDialFunc,
				block.GetBlockToken())
			if err != nil {
				return err
			}

			f.blockReader = &transfer.BlockReader{
				ClientName:          f.client.namenode.ClientName,
				Block:               block,
				Offset:              int64(off - start),
				UseDatanodeHostname: f.client.options.UseDatanodeHostname,
				DialFunc:            dialFunc,
			}

			return f.SetDeadline(f.deadline)
		}
	}

	return errors.New("invalid offset")
}
