// Package files implements io/fs objects
package files

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	stdfs "io/fs"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/mholt/archives"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/operations"
)

// fill tar.Header with metadata if available (too bad username/groupname is not available)
func metadataToHeader(metadata fs.Metadata, header *tar.Header) {
	var val string
	var ok bool
	var err error
	var mode, uid, gid int64
	var atime, ctime time.Time
	var uname, gname string
	// check if metadata is valid
	if metadata != nil {
		// mode
		val, ok = metadata["mode"]
		if !ok {
			mode = 0644
		} else {
			mode, err = strconv.ParseInt(val, 8, 64)
			if err != nil {
				mode = 0664
			}
		}
		// uid
		val, ok = metadata["uid"]
		if !ok {
			uid = 0
		} else {
			uid, err = strconv.ParseInt(val, 10, 32)
			if err != nil {
				uid = 0
			}
		}
		// gid
		val, ok = metadata["gid"]
		if !ok {
			gid = 0
		} else {
			gid, err = strconv.ParseInt(val, 10, 32)
			if err != nil {
				gid = 0
			}
		}
		// access time
		val, ok := metadata["atime"]
		if !ok {
			atime = time.Unix(0, 0)
		} else {
			atime, err = time.Parse(time.RFC3339Nano, val)
			if err != nil {
				atime = time.Unix(0, 0)
			}
		}
		// set uname/gname
		if uid == 0 {
			uname = "root"
		} else {
			uname = strconv.FormatInt(uid, 10)
		}
		if gid == 0 {
			gname = "root"
		} else {
			gname = strconv.FormatInt(gid, 10)
		}
	} else {
		mode = 0644
		uid = 0
		gid = 0
		uname = "root"
		gname = "root"
		atime = header.ModTime
		ctime = header.ModTime
	}
	// set values
	header.Mode = mode
	header.Uid = int(uid)
	header.Gid = int(gid)
	header.Uname = uname
	header.Gname = gname
	header.AccessTime = atime
	header.ChangeTime = ctime
}

// structs for fs.FileInfo,fs.File,SeekableFile

type fileInfoImpl struct {
	header *tar.Header
}

type fileImpl struct {
	entry    stdfs.FileInfo
	ctx      context.Context
	reader   io.ReadSeekCloser
	transfer *accounting.Transfer
	err      error
}

func newFileInfo(ctx context.Context, entry fs.DirEntry, prefix string, metadata fs.Metadata) stdfs.FileInfo {
	var fi = new(fileInfoImpl)

	fi.header = new(tar.Header)
	if prefix != "" {
		fi.header.Name = path.Join(strings.TrimPrefix(prefix, "/"), entry.Remote())
	} else {
		fi.header.Name = entry.Remote()
	}
	fi.header.Size = entry.Size()
	fi.header.ModTime = entry.ModTime(ctx)
	// set metadata
	metadataToHeader(metadata, fi.header)
	// flag if directory
	_, isDir := entry.(fs.Directory)
	if isDir {
		fi.header.Mode = int64(stdfs.ModeDir) | fi.header.Mode
	}

	return fi
}

func (a *fileInfoImpl) Name() string {
	return a.header.Name
}

func (a *fileInfoImpl) Size() int64 {
	return a.header.Size
}

func (a *fileInfoImpl) Mode() stdfs.FileMode {
	return stdfs.FileMode(a.header.Mode)
}

func (a *fileInfoImpl) ModTime() time.Time {
	return a.header.ModTime
}

func (a *fileInfoImpl) IsDir() bool {
	return (a.header.Mode & int64(stdfs.ModeDir)) != 0
}

func (a *fileInfoImpl) Sys() any {
	return a.header
}

func (a *fileInfoImpl) String() string {
	return fmt.Sprintf("Name=%v Size=%v IsDir=%v UID=%v GID=%v", a.Name(), a.Size(), a.IsDir(), a.header.Uid, a.header.Gid)
}

// create a fs.File compatible struct
func newFile(ctx context.Context, obj fs.Object, fi stdfs.FileInfo) (stdfs.File, error) {
	var f = new(fileImpl)
	// create stdfs.File
	f.entry = fi
	f.ctx = ctx
	f.err = nil
	// create transfer
	f.transfer = accounting.Stats(ctx).NewTransfer(obj, nil)
	// get open options
	var options []fs.OpenOption
	for _, option := range fs.GetConfig(ctx).DownloadHeaders {
		options = append(options, option)
	}
	// open file
	f.reader, f.err = operations.Open(ctx, obj, options...)
	if f.err != nil {
		defer f.transfer.Done(ctx, f.err)
		return nil, f.err
	}
	// Account the transfer
	f.reader = f.transfer.Account(ctx, f.reader)

	return f, f.err
}

func (a *fileImpl) Stat() (stdfs.FileInfo, error) {
	return a.entry, nil
}

func (a *fileImpl) Read(data []byte) (int, error) {
	if a.reader == nil {
		a.err = fmt.Errorf("file %s not open", a.entry.Name())
		return 0, a.err
	}
	i, err := a.reader.Read(data)
	a.err = err
	return i, a.err
}

func (a *fileImpl) Close() error {
	// close file
	if a.reader == nil {
		a.err = fmt.Errorf("file %s not open", a.entry.Name())
	} else {
		a.err = a.reader.Close()
	}
	// close transfer
	a.transfer.Done(a.ctx, a.err)

	return a.err
}

// NewArchiveFileInfo will take a fs.DirEntry and return a archives.Fileinfo
func NewArchiveFileInfo(ctx context.Context, entry fs.DirEntry, prefix string, metadata fs.Metadata) archives.FileInfo {
	fi := newFileInfo(ctx, entry, prefix, metadata)

	return archives.FileInfo{
		FileInfo:      fi,
		NameInArchive: fi.Name(),
		LinkTarget:    "",
		Open: func() (stdfs.File, error) {
			obj, isObject := entry.(fs.Object)
			if isObject {
				return newFile(ctx, obj, fi)
			}
			return nil, fmt.Errorf("%s is not a file", fi.Name())
		},
	}

}
