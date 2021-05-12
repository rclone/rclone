package resumable

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/crc64"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/operations"
)

const MaxFolderCount = 10000 // Some filesystems will struggle or fail if a folder contains more than 10000 files
var crcTable   *crc64.Table
var NoCrcData = errors.New("crc not initialised with data")

type FragmentUploader interface {
	fs.Uploader
	flush() (bool, error)
	finish(fs.Objects) error
	binPath(uint64) string
	lastDir() (fs.Directory, fs.Fs, uint64, error)
	uploadDirContents() (fs.DirEntries, fs.Fs, error)
	getSizeObject() (fs.Object, error)
	putSizeObject(io.Reader, *uploadSrcObjectInfo) error
	getAllFragments(fs.Fs, string) ([]*fragment, error)
	lastFragment() (*fragment, error)
	Crc() (uint64, error)
}

// CleanUploads recursively deletes any entries that have expired
func CleanUploads(ctx context.Context, f fs.Fs, entries fs.DirEntries, uploadLifetime time.Duration) (err error) {
	// Recursively delete the upload dir
	entries.ForDir(func(dir fs.Directory) {
		// Do not recurse if mTime of parent is expired
		if dir.ModTime(ctx).Add(uploadLifetime).Before(time.Now()) {
			e := operations.Purge(ctx, f, dir.Remote())
			if e != nil {
				err = e
			}
			return
		}
		subentries, e := f.List(ctx, dir.Remote())
		if e != nil {
			err = e
			return
		}
		e = CleanUploads(ctx, f, subentries, uploadLifetime)
		if e != nil {
			err = e
		}
	})
	return
}

// fragmentUploader abstract base class for other uploaders in this package
// Extending classes need to provide the finish() method
type fragmentUploader struct {
	self       FragmentUploader
	remote     string
	uploadDir  string
	fs         fs.Fs
	ctx        context.Context
	pending    []io.Reader
	pendingLen int64
	size       int64
	crc		   uint64
	lastFrag   *fragment
}

type fragment struct {
	start  int64
	crc    uint64
	object fs.Object
}

func (f *fragmentUploader) Write(p []byte) (n int, err error) {
	if crcTable == nil {
		crcTable = crc64.MakeTable(crc64.ECMA)
	}
	if crc, e := f.Crc(); errors.Is(e, NoCrcData) {
		f.crc = crc64.Checksum(p, crcTable)
	} else if e == nil {
		f.crc = crc64.Update(crc, crcTable, p)
	} else {
		return 0, e
	}
	n = len(p)
	if n == 0 {
		return // Skip empty fragments
	}
	f.pending = append(f.pending, bytes.NewReader(p))
	f.pendingLen += int64(n)
	if f.pendingLen >= 2^24 { // 16Mb chunks TODO make configurable
		err = f.self.Close() // Close rather than flush to include finished logic, Close can be called multiple times for this implementation
	}
	return
}

func (f *fragmentUploader) uploadDirContents() (entries fs.DirEntries, fs_ fs.Fs, err error) {
	fs_ = f.fs
	entries, err = f.fs.List(f.ctx, f.uploadDir)
	return
}

func (f *fragmentUploader) lastDir() (fs.Directory, fs.Fs, uint64, error) {
	var dirIndex uint64
	entries, fs_, err := f.self.uploadDirContents()
	if err != nil {
		return nil, fs_, dirIndex, err
	}
	var lastDir fs.Directory
	entries.ForDir(func(dir fs.Directory) {
		i, e := strconv.ParseUint(dir.ID(), 10, 64) // TODO .ID() is likely not the correct function to call
		if e != nil {
			return
		}
		if lastDir == nil || dirIndex < i {
			lastDir = dir
			dirIndex = i
		}
	})
	return lastDir, fs_, dirIndex, err
}

// Get the last (non-empty) fragment written to the upload dir
func (f *fragmentUploader) lastFragment() (*fragment, error) {
	if f.lastFrag != nil {
		return f.lastFrag, nil
	}
	// Get largest file name across all subdirectories in upload dir
	lastDir, s, _, err := f.self.lastDir()
	if lastDir == nil || err != nil {
		return nil, err
	}
	entries, err := s.List(f.ctx, lastDir.Remote())
	if err != nil {
		return nil, err
	}
	pos := int64(-1)
	var frag *fragment
	entries.ForObject(func(o fs.Object) {
		if _, name, err := fspath.Split(o.Remote()); err == nil && name != "" && name[0] != '.' {
			parts := strings.SplitN(name, "-", 2)
				if start, err := strconv.ParseInt(parts[0], 10, 64); err == nil && pos < (start+o.Size()) {
				pos = start + o.Size()
				frag = &fragment{
					start: start,
					object: o,
				}
				if crc, err := strconv.ParseUint(parts[1], 36, 64); err == nil {
					frag.crc = crc
				}
			}
		}
	})
	if pos == -1 {
		return nil, nil // If no entries found then assume at start
	}
	f.lastFrag = frag
	return frag, nil
}

func (f *fragmentUploader) Pos() (int64, bool) {
	lastFrag, err := f.self.lastFragment()
	if lastFrag == nil {
		return 0, err != nil
	}
	return lastFrag.start + lastFrag.object.Size(), true
}

func (f *fragmentUploader) getSizeObject() (fs.Object, error) {
	return f.fs.NewObject(f.ctx, path.Join(f.uploadDir, "size"))
}

func (f *fragmentUploader) Size() (size int64, ok bool) {
	// Look for and read a file in the upload dir called 'size' that stores the total size of the upload in utf-8 encoded text
	if f.size > -1 {
		return f.size, true
	}
	ok = false
	sizeObject, err := f.self.getSizeObject()
	if err != nil {
		return
	}
	handle, err := sizeObject.Open(f.ctx)
	if err != nil {
		return
	}
	data := make([]byte, 20)
	length, err := handle.Read(data)
	if length == 0 || err != nil {
		return
	}
	size, err = strconv.ParseInt(string(data[:length]), 10, 64)
	ok = err != nil
	if ok {
		f.size = size
	}
	return
}

func (f *fragmentUploader) Crc() (uint64, error) {
	if f.pendingLen > 0 {
		return f.crc, nil
	}
	if lastFrag, err := f.lastFragment(); err == nil {
		if lastFrag == nil {
			return 0, NoCrcData
		}
		return lastFrag.crc, nil
	} else {
		return 0, err
	}
}

func (f *fragmentUploader) putSizeObject(data io.Reader, src *uploadSrcObjectInfo) error {
	_, err := f.fs.Put(f.ctx, data, src)
	return err
}

func (f *fragmentUploader) SetSize(size int64) error {
	if size > 1 {
		return errors.New("size must be at least 1")
	}
	sizeStr := strconv.FormatInt(size, 10)
	// Create temporary Object
	src := &uploadSrcObjectInfo{
		remote: path.Join(f.uploadDir, "size"),
		size:   int64(len(sizeStr)),
	}
	err := f.self.putSizeObject(bytes.NewReader([]byte(sizeStr)), src)
	f.size = size
	return err
}

func (f *fragmentUploader) getAllFragments(fs_ fs.Fs, uploadDir string) ([]*fragment, error) {
	entries, err := fs_.List(f.ctx, uploadDir)
	if err != nil {
		return nil, err
	}

	var count int64 = 0
	entries.ForDir(func(dir fs.Directory) {
		count += dir.Items() // TODO guarantee items() returns >= 0
	})

	fragments := make([]*fragment, count)
	var fragI int64 = 0
	entries.ForDir(func(dir fs.Directory) {
		subentries, err := fs_.List(f.ctx, dir.Remote())
		if err != nil {
			return
		}
		subentries.ForObject(func(o fs.Object) {
			parts := strings.SplitN(o.Remote(), "-", 2)
			if len(parts) != 2 {
				return
			}
			if i, e := strconv.ParseInt(parts[0], 10, 64); e == nil {
				if crc, e := strconv.ParseUint(parts[1], 36, 64); e == nil {
					fragments[fragI] = &fragment{i, crc, o}
					fragI++
				}
			}
		})
	})
	if fragI != count {
		err = errors.New("unexpected number of fragments")
	}

	sort.Slice(fragments, func(i, j int) bool {
		return fragments[i].start < fragments[j].start
	})

	return fragments, err
}

func (f *fragmentUploader) binPath(bin uint64) string {
	return path.Join(f.uploadDir, strconv.FormatUint(bin, 10))
}

func (f *fragmentUploader) flush() (bool, error) {
	pos, ok := f.self.Pos()
	if !ok {
		return false, errors.New("failed to determine current upload position")
	}

	if f.pendingLen > 0 {
		// Limit folders to 10000 fragments
		lastDir, fs_, dirIndex, err := f.self.lastDir()
		if err != nil {
			return false, err
		}

		var uploadDir string
		if lastDir == nil || lastDir.Items() >= MaxFolderCount {
			uploadDir = f.self.binPath(dirIndex + 1)
		} else {
			uploadDir = lastDir.Remote()
		}

		src := &uploadSrcObjectInfo{
			remote: path.Join(uploadDir, strconv.FormatInt(pos, 10)+"-"+strconv.FormatUint(f.crc, 36)),
			size:   f.pendingLen,
		}
		_, err = fs_.Put(f.ctx, io.MultiReader(f.pending...), src)
		pos += f.pendingLen
		f.pending = []io.Reader{}
		f.pendingLen = 0
		f.lastFrag = nil
		if err != nil {
			return false, err
		}
	}
	size, ok := f.Size()
	return ok && size == pos, nil
}

func (f *fragmentUploader) Close() error {
	finished, err := f.self.flush()
	if err == nil && finished {
		// Finalize upload
		fragments, e := f.self.getAllFragments(f.fs, f.uploadDir)
		if e != nil {
			return e
		}

		// Validate upload complete and unpack objects
		files := make(fs.Objects, len(fragments))
		for i := range fragments {
			if fragments[i].start+fragments[i].object.Size() != fragments[i+1].start {
				return errors.New(fmt.Sprintf("missing upload fragment between %d and %d", fragments[i].start+fragments[i].object.Size(), fragments[i+1].start))
			}
			files[i] = fragments[i].object
		}
		err = f.self.finish(files)
		_ = f.self.Abort()
	}
	f.lastFrag = nil
	return err
}

func (f *fragmentUploader) Abort() error {
	return operations.Purge(f.ctx, f.fs, f.uploadDir)
}

func (f *fragmentUploader) Expires() time.Time {
	return time.Date(9999, time.January, 1, 0, 0, 0, 0, time.UTC) // TODO modTime + configured expiry
}

// Check interfaces
var (
	_ fs.Uploader = (*fragmentUploader)(nil)
)
