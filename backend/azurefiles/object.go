package azurefiles

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"log/slog"
	"path"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/file"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
)

const ONE_MB_IN_BYTES = 1048576

func objectInstance(f *Fs, remote string, md map[string]*string, contentLength *int64, contentType *string) Object {
	return Object{common: common{
		f:        f,
		remote:   remote,
		metaData: md,
		properties: properties{
			contentType:   contentType,
			contentLength: contentLength,
		},
	}}
}

func (o *Object) ModTime(ctx context.Context) time.Time {
	if o.metaData == nil {
		resp, err := o.fileClient().GetProperties(ctx, nil)
		if err != nil {
			slog.Warn("got an error while trying to fetch properties for %s : err", o.remote, err)
			return time.Now()
		}
		o.metaData = resp.Metadata
	}
	t, err := modTimeFromMetadata(o.metaData)
	if err != nil {
		return time.Now()
	}
	return t
}

// What happens of content length is empty
func (o *Object) Size() int64 {
	return *o.properties.contentLength
}

// TODO: make this readonly
func (o *Object) Fs() fs.Info {
	return o.f
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
	contentLength *int64
	contentType   *string
	// lastAccessTime *time.Time
}

func (o *Object) fileClient() *file.Client {
	return o.f.fileClientFromEncodedPathRelativeToShareRoot(o.f.encodePath(path.Join(o.f.root, o.remote)))
}

// TODO: change the modTime property on the local object as well
// FIX modTime on local objhect should change only if the modTime is successfully modified on the remote object
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	tStr := modTimeToString(t)
	if o.metaData == nil {
		o.metaData = make(map[string]*string)
	}

	setCaseInvariantMetaDataValue(o.metaData, modTimeKey, tStr)
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

// TODO: check for mandatory options and the other options
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	downloadStreamOptions := file.DownloadStreamOptions{}
	for _, opt := range options {
		switch v := opt.(type) {
		case *fs.SeekOption:
			httpRange := file.HTTPRange{
				Offset: v.Offset,
			}
			downloadStreamOptions.Range = httpRange
		case *fs.RangeOption:
			var start *int64
			var end *int64
			if v.Start >= 0 {
				start = &v.Start
			}
			if v.End >= 0 {
				end = &v.End
			}

			fhr := file.HTTPRange{}
			if start != nil && end != nil {
				fhr.Offset = *start
				fhr.Count = *end - *start + 1
			} else if start != nil && end == nil {
				fhr.Offset = *start
			} else if start == nil && end != nil {
				fhr.Offset = *o.contentLength - *end
			}

			downloadStreamOptions.Range = fhr
		}
	}
	resp, err := o.fileClient().DownloadStream(ctx, &downloadStreamOptions)
	if err != nil {
		return nil, fmt.Errorf("could not open remote=\"%s\" : %w", o.remote, err)
	}
	return resp.Body, nil
}

// TODO: implement options. understand purpose of options. what is the purpose of src objectInfo.
// TODO: set metadata options from src. Hint look at the local backend
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// TODO: File upload options should be included. Atleast two options is important:= Concurrency, Chunksize

	if src.Size() > maxFileSize {
		return fmt.Errorf("max supported file size is 4TB. provided size is %d", src.Size())
	}

	// TODO: is this fileSize is required. in the put function fileSize was passed as an argumen to f.Create
	// however f.Create required the largest file size and not the size of the file being uploaded
	fileSize := maxFileSize / 2 // FIXME: remove this d reduction in maxFileSize
	if src.Size() >= 0 {
		fileSize = src.Size()
	}
	fc := o.fileClient()
	if _, err := fc.SetHTTPHeaders(ctx, &file.SetHTTPHeadersOptions{
		FileContentLength: &fileSize,
	}); err != nil {
		return err
	}
	o.contentLength = &fileSize

	if err := uploadStreamSetMd5(ctx, fc, in, src, options...); err != nil {
		return err
	}
	// Set the mtime. copied from all/local.go rclone backend
	updatedModTime := src.ModTime(ctx)
	if err := o.SetModTime(ctx, updatedModTime); err != nil {
		return fmt.Errorf("unable to upload. cannot setModTime on remote=\"%s\" : %w", o.remote, err)
	}
	return nil
}

// cannot set modTime header here because setHTTPHeaders does not allow setting metadata
func uploadStreamSetMd5(ctx context.Context, fc *file.Client, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	hasher := md5.New()
	byteCounter := ByteCounter{}
	teedReader := io.TeeReader(in, io.MultiWriter(hasher, &byteCounter))

	// TODO: set concurrency level
	uploadStreamOptions := file.UploadStreamOptions{
		ChunkSize: chunkSize(options...),
	}

	if err := fc.UploadStream(ctx, teedReader, &uploadStreamOptions); err != nil {
		return fmt.Errorf("unable to upload. cannot upload stream : %w", err)
	}

	md5Hash := hasher.Sum(nil)
	bytesWritten := byteCounter.count
	contentType := objectInfoMimeType(ctx, src)

	// TODO: test contentType
	_, err := fc.SetHTTPHeaders(ctx, &file.SetHTTPHeadersOptions{
		FileContentLength: &bytesWritten,
		HTTPHeaders: &file.HTTPHeaders{
			ContentMD5:  md5Hash,
			ContentType: &contentType,
		},
	})
	if err != nil {
		log.Print(err)
		return err
	}

	return nil
}

func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.common.String()
}

type ByteCounter struct {
	count int64
}

func (bc *ByteCounter) Write(p []byte) (n int, err error) {
	lenP := len(p)
	bc.count += int64(lenP)
	return lenP, nil
}

// TODO: implment the hash function. First implement and test on Update function, then on the Put function
// using base64.StdEncoding.DecodeString for hashse because that is what azureblob uses

func (o *Object) MimeType(ctx context.Context) string {
	if o.properties.contentType == nil {
		return ""
	}
	return *o.properties.contentType
}

func objectInfoMimeType(ctx context.Context, oi fs.ObjectInfo) string {
	if mo, ok := oi.(fs.MimeTyper); ok {
		return mo.MimeType(ctx)
	}
	return ""
}

func chunkSize(options ...fs.OpenOption) int64 {
	for _, option := range options {
		if chunkOpt, ok := option.(*fs.ChunkOption); ok {
			return chunkOpt.ChunkSize
		}
	}
	return 1048576
}