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

// TODO: maybe use this in the result of list. or replace all instances where object instances are created
func objectInstance(f *Fs, remote string, contentLength int64, md5Hash []byte, lwt time.Time) Object {
	return Object{common: common{
		f:      f,
		remote: remote,
		properties: properties{
			contentLength: contentLength,
			md5Hash:       md5Hash,
			lastWriteTime: lwt,
		},
	}}
}

// Size of object in bytes
func (o *Object) Size() int64 {
	return o.properties.contentLength
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.f
}

// Hash returns the MD5 of an object returning a lowercase hex string
//
// May make a network request becaue the [fs.List] method does not
// return MD5 hashes for DirEntry
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	if ty != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	if len(o.common.properties.md5Hash) == 0 {
		props, err := o.fileClient().GetProperties(ctx, nil)
		if err != nil {
			return "", fmt.Errorf("unable to fetch properties to determine hash")
		}
		o.common.properties.md5Hash = props.ContentMD5
	}
	return hex.EncodeToString(o.common.properties.md5Hash), nil
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Object describes a Azure File Share File not a Directory
type Object struct {
	common
}

// These fields have pointer types because it seems to
// TODO: descide whether these could be pointer or not
type properties struct {
	contentLength int64
	md5Hash       []byte
	lastWriteTime time.Time
}

func (o *Object) fileClient() *file.Client {
	decodedFullPath := o.f.decodedFullPath(o.remote)
	fullEncodedPath := o.f.encodePath(decodedFullPath)
	return o.f.fileClientFromEncodedPathRelativeToShareRoot(fullEncodedPath)
}

// SetModTime sets the modification time
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	smbProps := file.SMBProperties{
		LastWriteTime: &t,
	}
	setHeadersOptions := file.SetHTTPHeadersOptions{
		SMBProperties: &smbProps,
	}
	_, err := o.fileClient().SetHTTPHeaders(ctx, &setHeadersOptions)
	if err != nil {
		return fmt.Errorf("unable to set modTime : %w", err)
	}
	o.lastWriteTime = t
	return nil
}

// ModTime returns the modification time of the object
//
// Returns time.Now() if not present
// TODO: convert o.lastWriteTime to *time.Time so that one can know when it has
// been explicitly set
func (o *Object) ModTime(ctx context.Context) time.Time {
	if o.lastWriteTime.Unix() <= 1 {
		return time.Now()
	}
	return o.lastWriteTime
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	// TODO: should the options for delete not be nil. Depends on behaviour expected by consumers
	if _, err := o.fileClient().Delete(ctx, nil); err != nil {
		return fmt.Errorf("unable to delete remote=\"%s\" : %w", o.remote, err)
	}
	return nil
}

// Open an object for read
//
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
				fhr.Offset = o.contentLength - *end
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

func (o *Object) upload(ctx context.Context, in io.Reader, src fs.ObjectInfo, isDestNewlyCreated bool, options ...fs.OpenOption) error {
	if src.Size() > fourTbInBytes {
		return fmt.Errorf("max supported file size is 4TB. provided size is %d", src.Size())
	} else if src.Size() < 0 {
		return fmt.Errorf("files with unknown sizes are not supported")
	}

	fc := o.fileClient()

	if !isDestNewlyCreated {
		if src.Size() != o.Size() {
			if _, resizeErr := fc.Resize(ctx, src.Size(), nil); resizeErr != nil {
				return fmt.Errorf("unable to resize while trying to update. %w ", resizeErr)
			}
		}
	}

	var md5Hash []byte
	hashToBeComputed := false
	if hashStr, err := src.Hash(ctx, hash.MD5); err != nil || hashStr == "" {
		hashToBeComputed = true
	} else {
		var decodeErr error
		md5Hash, decodeErr = hex.DecodeString(hashStr)
		if decodeErr != nil {
			hashToBeComputed = true
			msg := fmt.Sprintf("should not happen. Error while decoding hex encoded md5 '%s'. Error is %s",
				hashStr, decodeErr.Error())
			slog.Error(msg)
		}
	}
	var uploadErr error
	if hashToBeComputed {
		md5Hash, uploadErr = uploadStreamAndComputeHash(ctx, fc, in, src, options...)
	} else {
		uploadErr = uploadStream(ctx, fc, in, src, options...)
	}
	if uploadErr != nil {
		return fmt.Errorf("while uploading %s : %w", src.Remote(), uploadErr)
	}

	modTime := src.ModTime(ctx)
	if err := uploadSizeHashLWT(ctx, fc, src.Size(), md5Hash, modTime); err != nil {

		return fmt.Errorf("while setting size hash and last write time for %s : %w", src.Remote(), err)
	}
	o.properties.contentLength = src.Size()
	o.properties.md5Hash = md5Hash
	o.properties.lastWriteTime = modTime
	return nil
}

// Update the object with the contents of the io.Reader, modTime, size and MD5 hash
// Does not create a new object
//
// TODO: implement options. understand purpose of options
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return o.upload(ctx, in, src, false, options...)
}

// cannot set modTime header here because setHTTPHeaders does not allow setting metadata
func uploadStream(ctx context.Context, fc *file.Client, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// TODO: set concurrency level
	uploadStreamOptions := file.UploadStreamOptions{
		ChunkSize: chunkSize(options...),
	}

	if err := fc.UploadStream(ctx, in, &uploadStreamOptions); err != nil {
		return fmt.Errorf("unable to upload. cannot upload stream : %w", err)
	}
	return nil
}

func uploadStreamAndComputeHash(ctx context.Context, fc *file.Client, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) ([]byte, error) {
	hasher := md5.New()
	teeReader := io.TeeReader(in, hasher)
	err := uploadStream(ctx, fc, teeReader, src, options...)
	if err != nil {
		return []byte{}, err
	}
	return hasher.Sum(nil), nil

}

// the function is named with prefix 'upload' since it indicates that things will be modified on the server
func uploadSizeHashLWT(ctx context.Context, fc *file.Client, size int64, hash []byte, lwt time.Time) error {
	smbProps := file.SMBProperties{
		LastWriteTime: &lwt,
	}
	httpHeaders := &file.HTTPHeaders{
		ContentMD5: hash,
	}
	_, err := fc.SetHTTPHeaders(ctx, &file.SetHTTPHeadersOptions{
		FileContentLength: &size,
		SMBProperties:     &smbProps,
		HTTPHeaders:       httpHeaders,
	})
	if err != nil {
		return fmt.Errorf("while setting size, hash, lastWriteTime : %w", err)
	}
	return nil
}

func chunkSize(options ...fs.OpenOption) int64 {
	for _, option := range options {
		if chunkOpt, ok := option.(*fs.ChunkOption); ok {
			return chunkOpt.ChunkSize
		}
	}
	return 1048576
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.common.String()
}
