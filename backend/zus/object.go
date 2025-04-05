package zus

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path"
	"strings"
	"time"

	"github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/zboxcore/sdk"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
)

type Object struct {
	fs        *Fs
	remote    string
	modTime   time.Time
	size      int64
	encrypted bool
}

// String returns a description of the Object
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}

	return o.Remote()
}

// Remote returns the remote path
func (o *Object) Remote() string {
	if o.fs.root == "/" {
		return o.remote
	}

	return strings.TrimPrefix(o.remote, o.fs.root+"/")
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	return o.size
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, ty hash.Type) (_ string, err error) {
	return "", hash.ErrUnsupported
}

// Storable says whether this object can be stored
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the metadata on the object to set the modification date
func (o *Object) SetModTime(ctx context.Context, t time.Time) (err error) {
	return fs.ErrorCantSetModTime
}

func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	var (
		rangeStart int64
		rangeEnd   int64 = -1
	)

	for _, option := range options {
		switch opt := option.(type) {
		case *fs.RangeOption:
			rangeStart = opt.Start
			rangeEnd = opt.End
		case *fs.SeekOption:
			if opt.Offset > 0 {
				rangeStart = opt.Offset
			} else {
				rangeStart = o.size + opt.Offset
			}
		default:
			if option.Mandatory() {
				fs.Errorf(o, "Unsupported mandatory option: %v", option)

				return nil, errors.New("unsupported mandatory option")
			}

		}
	}
	return o.fs.alloc.DownloadObject(ctx, o.remote, rangeStart, rangeEnd)
}

func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	for _, option := range options {
		if option.Mandatory() {
			fs.Errorf(o.fs, "Unsupported mandatory option: %v", option)

			return errors.New("unsupported mandatory option")
		}
	}
	mp := make(map[string]string)
	modified := src.ModTime(ctx)
	mp["rclone:mtime"] = modified.Format(time.RFC3339)
	marshal, err := json.Marshal(mp)
	if err != nil {
		return err
	}
	fileMeta := sdk.FileMeta{
		Path:       "",
		RemotePath: o.remote,
		ActualSize: src.Size(),
		RemoteName: path.Base(o.remote),
		CustomMeta: string(marshal),
	}
	isStreamUpload := src.Size() == -1
	if isStreamUpload {
		fileMeta.ActualSize = 0
	}
	rb := &ReaderBytes{
		reader: in,
	}
	opRequest := sdk.OperationRequest{
		OperationType: constants.FileOperationUpdate,
		FileReader:    rb,
		Workdir:       o.fs.opts.WorkDir,
		RemotePath:    o.remote,
		FileMeta:      fileMeta,
		Opts: []sdk.ChunkedUploadOption{
			sdk.WithChunkNumber(120),
			sdk.WithEncrypt(o.fs.opts.Encrypt),
		},
		StreamUpload: isStreamUpload,
	}
	err = o.fs.alloc.DoMultiOperation([]sdk.OperationRequest{opRequest})
	if err != nil {
		return err
	}
	o.modTime = modified
	o.size = rb.size
	o.encrypted = o.fs.opts.Encrypt

	return nil
}

func (o *Object) put(ctx context.Context, in io.Reader, src fs.ObjectInfo, toUpdate bool) (err error) {
	mp := make(map[string]string)
	modified := src.ModTime(ctx)
	mp["rclone:mtime"] = modified.Format(time.RFC3339)
	marshal, err := json.Marshal(mp)
	if err != nil {
		return err
	}
	fileMeta := sdk.FileMeta{
		Path:       "",
		RemotePath: o.remote,
		ActualSize: src.Size(),
		RemoteName: path.Base(o.remote),
		CustomMeta: string(marshal),
	}
	isStreamUpload := src.Size() == -1
	if isStreamUpload {
		fileMeta.ActualSize = 0
	}
	rb := &ReaderBytes{
		reader: in,
	}
	opRequest := sdk.OperationRequest{
		OperationType: constants.FileOperationInsert,
		FileReader:    rb,
		Workdir:       o.fs.opts.WorkDir,
		RemotePath:    o.remote,
		FileMeta:      fileMeta,
		Opts: []sdk.ChunkedUploadOption{
			sdk.WithChunkNumber(120),
			sdk.WithEncrypt(o.fs.opts.Encrypt),
		},
		StreamUpload: isStreamUpload,
	}
	if toUpdate {
		opRequest.OperationType = constants.FileOperationUpdate
	}
	err = o.fs.alloc.DoMultiOperation([]sdk.OperationRequest{opRequest})
	if err != nil {
		return err
	}
	o.modTime = modified
	o.size = rb.size
	o.encrypted = o.fs.opts.Encrypt
	return nil
}

func (o *Object) Remove(ctx context.Context) (err error) {
	opRequest := sdk.OperationRequest{
		OperationType: constants.FileOperationDelete,
		RemotePath:    o.remote,
	}
	err = o.fs.alloc.DoMultiOperation([]sdk.OperationRequest{opRequest})
	return err
}

type ReaderBytes struct {
	reader io.Reader
	size   int64
}

func (r *ReaderBytes) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	r.size += int64(n)
	return n, nil
}
