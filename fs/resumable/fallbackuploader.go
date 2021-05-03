package resumable

import (
	"context"
	"io"
	"path"
	"strconv"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/errors"
)

type fallbackUploader struct {
	fragmentUploader
	fallbackUploadDir     string
	fallbackFs            fs.Fs
	minSize, maxFragments int64
	partialFallback       bool // partialFallback is manipulated to force flush(), uploadDirContents(), and binPath() to use parent implementation
}

// NewFallbackUploader returns an fs.Uploader that stages upload chunks to an alternate fs.Fs while uploading. Upon completion
// the chunks are concatenated and streamed to the target fs.Fs
func NewFallbackUploader(remote, uploadDir, fallbackUploadDir string, fs_, fallbackFs fs.Fs, ctx context.Context, minSize, maxFragments int64) fs.Uploader {
	_, concatable := fs_.(ConcatenatorFs)
	self := &fallbackUploader{
		fragmentUploader{
			nil,
			remote,
			uploadDir,
			fs_,
			ctx,
			[]io.Reader{},
			0,
			-1,
		},
		fallbackUploadDir,
		fallbackFs,
		minSize,
		maxFragments,
		concatable,
	}
	self.self = self
	return self
}

func (f *fallbackUploader) uploadDirContents() (entries fs.DirEntries, fs fs.Fs, err error) {
	if f.partialFallback {
		fs = f.fallbackFs
		entries, err = fs.List(f.ctx, f.fallbackUploadDir)
		return
	} else {
		return f.fragmentUploader.uploadDirContents()
	}
}

func (f *fallbackUploader) Pos() (pos int64, ok bool) {
	if f.partialFallback {
		var fallbackPos int64
		fallbackPos, ok = f.fragmentUploader.Pos()
		if !ok {
			return
		}
		f.partialFallback = false
		pos, ok = f.fragmentUploader.Pos()
		f.partialFallback = true
		if pos < fallbackPos {
			pos = fallbackPos
		}
		return
	} else {
		return f.fragmentUploader.Pos()
	}
}

func (f *fallbackUploader) getSizeObject() (fs.Object, error) {
	if f.partialFallback {
		return f.fragmentUploader.getSizeObject()
	} else {
		return f.fallbackFs.NewObject(f.ctx, path.Join(f.fallbackUploadDir, "size"))
	}
}

func (f *fallbackUploader) putSizeObject(data io.Reader, src *uploadSrcObjectInfo) error {
	if f.partialFallback {
		return f.fragmentUploader.putSizeObject(data, src)
	} else {
		src.remote = path.Join(f.fallbackUploadDir, "size")
		_, err := f.fallbackFs.Put(f.ctx, data, src)
		return err
	}
}

func (f *fallbackUploader) binPath(bin uint64) string {
	if f.partialFallback {
		return f.fragmentUploader.binPath(bin)
	} else {
		return path.Join(f.fallbackUploadDir, strconv.FormatUint(bin, 10))
	}
}

func (f *fallbackUploader) flush() (bool, error) {
	if f.partialFallback {
		defer func() { f.partialFallback = true }()
		totalFragments := int64(-1)
		if f.maxFragments > 0 {
			totalFragments = 0
			for _, f.partialFallback = range []bool{true, false} { // Total for fallback and main
				lastDir, _, dirIndex, err := f.lastDir()
				if err != nil {
					return false, err
				}
				if dirIndex > 0 {
					totalFragments += (int64(dirIndex) - 1) * MaxFolderCount
				}
				totalFragments += lastDir.Items()
			}
		}
		f.partialFallback = false
		lastFragmentSize := int64(0)
		if f.minSize > 0 {
			// Check to make sure the last fragment size on the fallback is big enough to eventually upload otherwise provide fallback with enough contiguous data
			frag, err := f.lastFragment()
			if err != nil {
				return false, err
			}
			lastFragmentSize = frag.object.Size()
		}
		f.partialFallback = (f.pendingLen >= f.minSize && lastFragmentSize >= f.minSize) && totalFragments < f.maxFragments
	}
	return f.fragmentUploader.flush() // the fs that is written to is determined by f.partialFallback
}

func (f *fallbackUploader) finish(fragments fs.Objects) error {
	if f.partialFallback {
		catFs, ok := f.fs.(ConcatenatorFs)
		if !ok {
			panic("initial interface check in NewFallbackUploader() for ConcatenatorFs falsely returned true") // Seriously, if this ever gets hit, something went very very wrong
		}
		_, err := catFs.Concat(f.ctx, fragments, f.remote)
		return err
	}

	// Target Fs doesn't support concat, stream everything
	size, ok := f.Size()
	if !ok {
		return errors.New("finish() called before SetSize() or value can't be recovered")
	}

	readCloser := NewConcatReader(f.ctx, fragments)
	defer readCloser.Close()

	_, err := f.fs.Put(f.ctx, readCloser, &uploadSrcObjectInfo{
		remote: f.remote,
		size:   size,
	})
	return err
}

func (f *fallbackUploader) Abort() error {
	err := operations.Purge(f.ctx, f.fallbackFs, f.fallbackUploadDir)
	if err != nil {
		_ = f.fragmentUploader.Abort()
	} else {
		return f.fragmentUploader.Abort()
	}
	return err
}

// Check interfaces
var (
	_ FragmentUploader = (*fallbackUploader)(nil)
	_ fs.Uploader      = (*fallbackUploader)(nil)
)
