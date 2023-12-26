package xpan

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/rclone/rclone/backend/xpan/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/rest"
)

// Check the interfaces are satisfied
var (
	_ fs.Object = (*Object)(nil)
	_ fs.IDer   = (*Object)(nil)
)

// Object xpan remote file object
type Object struct {
	fs   *Fs
	item *api.Item
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
	return o.fs.removeLeadingRoot(o.item.Path)
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (o *Object) ModTime(context.Context) time.Time {
	if o.item == nil {
		return time.Time{}
	}
	return time.Unix(int64(o.item.LocalModifyTime), 0)
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	if o.item == nil {
		return -1
	}
	return int64(o.item.Size)
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", nil
}

// Storable says whether this object can be stored
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the metadata on the object to set the modification date
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	params, err := o.fs.newReqParams("filemetas")
	if err != nil {
		return nil, err
	}
	params.Set("fsids", fmt.Sprintf("[%d]", o.item.FsID))
	params.Set("dlink", "1")
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/rest/2.0/xpan/multimedia",
		Parameters: params,
	}

	var fileMetaResponse api.FileMetaResponse
	err = o.fs.pacer.Call(func() (bool, error) {
		_, err := o.fs.srv.CallJSON(ctx, &opts, nil, &fileMetaResponse)
		return false, err
	})
	if err != nil {
		return nil, err
	}
	if fileMetaResponse.ErrorNumber != 0 {
		return nil, api.Err(fileMetaResponse.ErrorNumber)
	}
	if len(fileMetaResponse.List) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	r := newDownloadReader(
		ctx, o.fs, int64(fileMetaResponse.List[0].Size), fileMetaResponse.List[0].Dlink)
	return io.NopCloser(r), nil
}

// Update in to the object with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
// return an error or update the object properly (rather than e.g. calling panic).
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	o.fs.tokenRenewer.Start()
	defer o.fs.tokenRenewer.Stop()
	copiedItem := *o.item
	copiedObj := *o
	copiedObj.item = &copiedItem
	copiedItem.LocalModifyTime = uint(src.ModTime(ctx).Unix())
	copiedItem.Size = uint(src.Size())
	o.item, err = o.fs.multipartUpload(ctx, in, &copiedObj, options)
	return
}

// Remove removes this object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.filemanager(
		ctx, "delete", fmt.Sprintf("[\"%s\"]", o.fs.absolutePath(o.Remote())))
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return fmt.Sprintf("%d", o.item.FsID)
}
