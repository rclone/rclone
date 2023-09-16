package azurefiles

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/file"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
)

func (o *Object) ModTime(ctx context.Context) time.Time {
	if o.metaData == nil {
		resp, err := o.fileClient().GetProperties(ctx, nil)
		if err != nil {
			slog.Warn("got an error while trying to fetch properties for %s : err", o.remote, err)
			return time.Now()
		}
		o.metaData = resp.Metadata
	}
	return modTimeFromMetadata(o.metaData)
}

func (o *Object) Size() int64 {
	return *o.properties.contentLength
}

// TODO: make this readonly
func (o *Object) Fs() fs.Info {
	return o.c
}

// TODO: returning hex encoded string because rclone/hash/multihasher uses hex encoding
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	if ty != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	resp, err := o.fileClient().GetProperties(context.TODO(), nil)
	if err != nil {
		return "", fmt.Errorf("while getting hash for remote=\"%s\" : %w", o.remote, err)
	}
	return hex.EncodeToString(resp.ContentMD5), nil
}

// TODO: what does this mean?
func (o *Object) Storable() bool {
	return true
}

type Object struct {
	common
}

type properties struct {
	changeTime    *time.Time
	contentLength *int64
	// lastAccessTime *time.Time
}

// func (o *Object) fetchMetadata() error {

// 	return o.fileClient().GetProperties()
// }

func (o *Object) fileClient() *file.Client {
	return o.c.RootDirClient.NewFileClient(o.remote)
}

// TODO: change the modTime property on the local object as well
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	// if o.isDir {
	// 	return fmt.Errorf("cannot set ModTime on directory %s", o.remote)
	// }
	tStr := fmt.Sprintf("%d", t.Unix())
	if o.metaData == nil {
		o.metaData = make(map[string]*string)
	}

	o.metaData[modTimeKey] = &tStr
	metaDataOptions := file.SetMetadataOptions{
		Metadata: o.metaData,
	}
	_, err := o.fileClient().SetMetadata(ctx, &metaDataOptions)
	if err != nil {
		return fmt.Errorf("unable to SetModTime on remote=\"%s\" : %w", o.remote, err)
	}
	return nil
}

func (o *Object) Remove(ctx context.Context) error {
	// TODO: should the options for delete not be nil. Depends on behaviour expected by consumers
	if _, err := o.fileClient().Delete(ctx, nil); err != nil {
		return fmt.Errorf("unable to delete remote=\"%s\" : %w", o.remote, err)
	}
	return nil
}

// TODO: implement options. understand purpose of options
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	resp, err := o.fileClient().DownloadStream(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open remote=\"%s\" : %w", o.remote, err)
	}
	return resp.Body, nil
}

// TODO: implement options. understand purpose of options. what is the purpose of src objectInfo.
// TODO: set metadata options from src. Hint at the local backend
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// TODO: File upload options should be included. Atleast two options is important:= Concurrency, Chunksize
	// TODO: content MD5 not set
	if err := uploadStreamSetMd5(ctx, o.fileClient(), in, options...); err != nil {
		return err
	}
	// Set the mtime. copied from all/local.go rclone backend
	if err := o.SetModTime(ctx, src.ModTime(ctx)); err != nil {
		return fmt.Errorf("unable to upload. cannot setModTime on remote=\"%s\" : %w", o.remote, err)
	}
	return nil
}

func uploadStreamSetMd5(ctx context.Context, fc *file.Client, in io.Reader, options ...fs.OpenOption) error {
	hasher := md5.New()
	teedReader := io.TeeReader(in, hasher)
	if err := fc.UploadStream(ctx, teedReader, nil); err != nil {
		return fmt.Errorf("unable to upload. cannot upload stream : %w", err)
	}

	md5Hash := hasher.Sum(nil)

	fc.SetHTTPHeaders(ctx, &file.SetHTTPHeadersOptions{
		HTTPHeaders: &file.HTTPHeaders{
			ContentMD5: md5Hash,
		},
	})

	return nil
}

// TODO: implment the hash function. First implement and test on Update function, then on the Put function
// using base64.StdEncoding.DecodeString for hashse because that is what azureblob uses
