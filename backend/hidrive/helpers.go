package hidrive

// This file is for helper-functions which may provide more general and
// specialized functionality than the generic interfaces.
// There are two sections:
// 1. methods bound to Fs
// 2. other functions independent from Fs used throughout the package

// NOTE: Functions accessing paths expect any relative paths
// to be resolved prior to execution with resolvePath(...).

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/hidrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/ranges"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const (
	// MaximumUploadBytes represents the maximum amount of bytes
	// a single upload-operation will support.
	MaximumUploadBytes = 2147483647 // = 2GiB - 1
	// iterationChunkSize represents the chunk size used to iterate directory contents.
	iterationChunkSize = 5000
)

var (
	// retryErrorCodes is a slice of error codes that we will always retry.
	retryErrorCodes = []int{
		429, // Too Many Requests
		500, // Internal Server Error
		502, // Bad Gateway
		503, // Service Unavailable
		504, // Gateway Timeout
		509, // Bandwidth Limit Exceeded
	}
	// ErrorFileExists is returned when a query tries to create a file
	// that already exists.
	ErrorFileExists = errors.New("destination file already exists")
)

// MemberType represents the possible types of entries a directory can contain.
type MemberType string

// possible values for MemberType
const (
	AllMembers       MemberType = "all"
	NoMembers        MemberType = "none"
	DirectoryMembers MemberType = api.HiDriveObjectTypeDirectory
	FileMembers      MemberType = api.HiDriveObjectTypeFile
	SymlinkMembers   MemberType = api.HiDriveObjectTypeSymlink
)

// SortByField represents possible fields to sort entries of a directory by.
type SortByField string

// possible values for SortByField
const (
	descendingSort             string      = "-"
	SortByName                 SortByField = "name"
	SortByModTime              SortByField = "mtime"
	SortByObjectType           SortByField = "type"
	SortBySize                 SortByField = "size"
	SortByNameDescending       SortByField = SortByField(descendingSort) + SortByName
	SortByModTimeDescending    SortByField = SortByField(descendingSort) + SortByModTime
	SortByObjectTypeDescending SortByField = SortByField(descendingSort) + SortByObjectType
	SortBySizeDescending       SortByField = SortByField(descendingSort) + SortBySize
)

var (
	// Unsorted disables sorting and can therefore not be combined with other values.
	Unsorted = []SortByField{"none"}
	// DefaultSorted does not specify how to sort and
	// therefore implies the default sort order.
	DefaultSorted = []SortByField{}
)

// CopyOrMoveOperationType represents the possible types of copy- and move-operations.
type CopyOrMoveOperationType int

// possible values for CopyOrMoveOperationType
const (
	MoveOriginal CopyOrMoveOperationType = iota
	CopyOriginal
	CopyOriginalPreserveModTime
)

// OnExistAction represents possible actions the API should take,
// when a request tries to create a path that already exists.
type OnExistAction string

// possible values for OnExistAction
const (
	// IgnoreOnExist instructs the API not to execute
	// the request in case of a conflict, but to return an error.
	IgnoreOnExist OnExistAction = "ignore"
	// AutoNameOnExist instructs the API to automatically rename
	// any conflicting request-objects.
	AutoNameOnExist OnExistAction = "autoname"
	// OverwriteOnExist instructs the API to overwrite any conflicting files.
	// This can only be used, if the request operates on files directly.
	// (For example when moving/copying a file.)
	// For most requests this action will simply be ignored.
	OverwriteOnExist OnExistAction = "overwrite"
)

// shouldRetry returns a boolean as to whether this resp and err deserve to be retried.
// It tries to expire/invalidate the token, if necessary.
// It returns the err as a convenience.
func (f *Fs) shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	if resp != nil && (resp.StatusCode == 401 || isHTTPError(err, 401)) && len(resp.Header["Www-Authenticate"]) > 0 {
		fs.Debugf(f, "Token might be invalid: %v", err)
		if f.tokenRenewer != nil {
			iErr := f.tokenRenewer.Expire()
			if iErr == nil {
				return true, err
			}
		}
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// resolvePath resolves the given (relative) path and
// returns a path suitable for API-calls.
// This will consider the root-path of the fs and any needed prefixes.
//
// Any relative paths passed to functions that access these paths should
// be resolved with this first!
func (f *Fs) resolvePath(objectPath string) string {
	resolved := path.Join(f.opt.RootPrefix, f.root, f.opt.Enc.FromStandardPath(objectPath))
	return resolved
}

// iterateOverDirectory calls the given function callback
// on each item found in a given directory.
//
// If callback ever returns true then this exits early with found = true.
func (f *Fs) iterateOverDirectory(ctx context.Context, directory string, searchOnly MemberType, callback func(*api.HiDriveObject) bool, fields []string, sortBy []SortByField) (found bool, err error) {
	parameters := api.NewQueryParameters()
	parameters.SetPath(directory)
	parameters.AddFields("members.", fields...)
	parameters.AddFields("", api.DirectoryContentFields...)
	parameters.Set("members", string(searchOnly))
	for _, v := range sortBy {
		// The explicit conversion is necessary for each element.
		parameters.AddList("sort", ",", string(v))
	}

	opts := rest.Opts{
		Method:     "GET",
		Path:       "/dir",
		Parameters: parameters.Values,
	}

	iterateContent := func(result *api.DirectoryContent, err error) (bool, error) {
		if err != nil {
			return false, err
		}
		for _, item := range result.Entries {
			item.Name = f.opt.Enc.ToStandardName(item.Name)
			if callback(&item) {
				return true, nil
			}
		}
		return false, nil
	}
	return f.paginateDirectoryAccess(ctx, &opts, iterationChunkSize, 0, iterateContent)
}

// paginateDirectoryAccess executes requests specified via ctx and opts
// which should produce api.DirectoryContent.
// This will paginate the requests using limit starting at the given offset.
//
// The given function callback is called on each api.DirectoryContent found
// along with any errors that occurred.
// If callback ever returns true then this exits early with found = true.
// If callback ever returns an error then this exits early with that error.
func (f *Fs) paginateDirectoryAccess(ctx context.Context, opts *rest.Opts, limit int64, offset int64, callback func(*api.DirectoryContent, error) (bool, error)) (found bool, err error) {
	for {
		opts.Parameters.Set("limit", strconv.FormatInt(offset, 10)+","+strconv.FormatInt(limit, 10))

		var result api.DirectoryContent
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, opts, nil, &result)
			return f.shouldRetry(ctx, resp, err)
		})

		found, err = callback(&result, err)
		if found || err != nil {
			return found, err
		}

		offset += int64(len(result.Entries))
		if offset >= result.TotalCount || limit > int64(len(result.Entries)) {
			break
		}
	}
	return false, nil
}

// fetchMetadataForPath reads the metadata from the path.
func (f *Fs) fetchMetadataForPath(ctx context.Context, path string, fields []string) (*api.HiDriveObject, error) {
	parameters := api.NewQueryParameters()
	parameters.SetPath(path)
	parameters.AddFields("", fields...)

	opts := rest.Opts{
		Method:     "GET",
		Path:       "/meta",
		Parameters: parameters.Values,
	}

	var result api.HiDriveObject
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// copyOrMove copies or moves a directory or file
// from the source-path to the destination-path.
//
// The operation will only be successful
// if the parent-directory of the destination-path exists.
//
// NOTE: Use the explicit methods instead of directly invoking this method.
// (Those are: copyDirectory, moveDirectory, copyFile, moveFile.)
func (f *Fs) copyOrMove(ctx context.Context, isDirectory bool, operationType CopyOrMoveOperationType, source string, destination string, onExist OnExistAction) (*api.HiDriveObject, error) {
	parameters := api.NewQueryParameters()
	parameters.Set("src", source)
	parameters.Set("dst", destination)
	if onExist == AutoNameOnExist ||
		(onExist == OverwriteOnExist && !isDirectory) {
		parameters.Set("on_exist", string(onExist))
	}

	endpoint := "/"
	if isDirectory {
		endpoint += "dir"
	} else {
		endpoint += "file"
	}
	switch operationType {
	case MoveOriginal:
		endpoint += "/move"
	case CopyOriginalPreserveModTime:
		parameters.Set("preserve_mtime", strconv.FormatBool(true))
		fallthrough
	case CopyOriginal:
		endpoint += "/copy"
	}

	opts := rest.Opts{
		Method:     "POST",
		Path:       endpoint,
		Parameters: parameters.Values,
	}

	var result api.HiDriveObject
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// moveDirectory moves the directory at the source-path to the destination-path and
// returns the resulting api-object if successful.
//
// The operation will only be successful
// if the parent-directory of the destination-path exists.
func (f *Fs) moveDirectory(ctx context.Context, source string, destination string, onExist OnExistAction) (*api.HiDriveObject, error) {
	return f.copyOrMove(ctx, true, MoveOriginal, source, destination, onExist)
}

// copyFile copies the file at the source-path to the destination-path and
// returns the resulting api-object if successful.
//
// The operation will only be successful
// if the parent-directory of the destination-path exists.
//
// NOTE: This operation will expand sparse areas in the content of the source-file
// to blocks of 0-bytes in the destination-file.
func (f *Fs) copyFile(ctx context.Context, source string, destination string, onExist OnExistAction) (*api.HiDriveObject, error) {
	return f.copyOrMove(ctx, false, CopyOriginalPreserveModTime, source, destination, onExist)
}

// moveFile moves the file at the source-path to the destination-path and
// returns the resulting api-object if successful.
//
// The operation will only be successful
// if the parent-directory of the destination-path exists.
//
// NOTE: This operation may expand sparse areas in the content of the source-file
// to blocks of 0-bytes in the destination-file.
func (f *Fs) moveFile(ctx context.Context, source string, destination string, onExist OnExistAction) (*api.HiDriveObject, error) {
	return f.copyOrMove(ctx, false, MoveOriginal, source, destination, onExist)
}

// createDirectory creates the directory at the given path and
// returns the resulting api-object if successful.
//
// The directory will only be created if its parent-directory exists.
// This returns fs.ErrorDirNotFound if the parent-directory is not found.
// This returns fs.ErrorDirExists if the directory already exists.
func (f *Fs) createDirectory(ctx context.Context, directory string, onExist OnExistAction) (*api.HiDriveObject, error) {
	parameters := api.NewQueryParameters()
	parameters.SetPath(directory)
	if onExist == AutoNameOnExist {
		parameters.Set("on_exist", string(onExist))
	}

	opts := rest.Opts{
		Method:     "POST",
		Path:       "/dir",
		Parameters: parameters.Values,
	}

	var result api.HiDriveObject
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})

	switch {
	case err == nil:
		return &result, nil
	case isHTTPError(err, 404):
		return nil, fs.ErrorDirNotFound
	case isHTTPError(err, 409):
		return nil, fs.ErrorDirExists
	}
	return nil, err
}

// createDirectories creates the directory at the given path
// along with any missing parent directories and
// returns the resulting api-object (of the created directory) if successful.
//
// This returns fs.ErrorDirExists if the directory already exists.
//
// If an error occurs while the parent directories are being created,
// any directories already created will NOT be deleted again.
func (f *Fs) createDirectories(ctx context.Context, directory string, onExist OnExistAction) (*api.HiDriveObject, error) {
	result, err := f.createDirectory(ctx, directory, onExist)
	if err == nil {
		return result, nil
	}
	if err != fs.ErrorDirNotFound {
		return nil, err
	}
	parentDirectory := path.Dir(directory)
	_, err = f.createDirectories(ctx, parentDirectory, onExist)
	if err != nil && err != fs.ErrorDirExists {
		return nil, err
	}
	// NOTE: Ignoring fs.ErrorDirExists does no harm,
	// since it does not mean the child directory cannot be created.
	return f.createDirectory(ctx, directory, onExist)
}

// deleteDirectory deletes the directory at the given path.
//
// If recursive is false, the directory will only be deleted if it is empty.
// If recursive is true, the directory will be deleted regardless of its content.
// This returns fs.ErrorDirNotFound if the directory is not found.
// This returns fs.ErrorDirectoryNotEmpty if the directory is not empty and
// recursive is false.
func (f *Fs) deleteDirectory(ctx context.Context, directory string, recursive bool) error {
	parameters := api.NewQueryParameters()
	parameters.SetPath(directory)
	parameters.Set("recursive", strconv.FormatBool(recursive))

	opts := rest.Opts{
		Method:     "DELETE",
		Path:       "/dir",
		Parameters: parameters.Values,
		NoResponse: true,
	}

	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return f.shouldRetry(ctx, resp, err)
	})

	switch {
	case isHTTPError(err, 404):
		return fs.ErrorDirNotFound
	case isHTTPError(err, 409):
		return fs.ErrorDirectoryNotEmpty
	}
	return err
}

// deleteObject deletes the object/file at the given path.
//
// This returns fs.ErrorObjectNotFound if the object is not found.
func (f *Fs) deleteObject(ctx context.Context, path string) error {
	parameters := api.NewQueryParameters()
	parameters.SetPath(path)

	opts := rest.Opts{
		Method:     "DELETE",
		Path:       "/file",
		Parameters: parameters.Values,
		NoResponse: true,
	}

	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return f.shouldRetry(ctx, resp, err)
	})

	if isHTTPError(err, 404) {
		return fs.ErrorObjectNotFound
	}
	return err
}

// createFile creates a file at the given path
// with the content of the io.ReadSeeker.
// This guarantees that existing files will not be overwritten.
// The maximum size of the content is limited by MaximumUploadBytes.
// The io.ReadSeeker should be resettable by seeking to its start.
// If modTime is not the zero time instant,
// it will be set as the file's modification time after the operation.
//
// This returns fs.ErrorDirNotFound
// if the parent directory of the file is not found.
// This returns ErrorFileExists if a file already exists at the specified path.
func (f *Fs) createFile(ctx context.Context, path string, content io.ReadSeeker, modTime time.Time, onExist OnExistAction) (*api.HiDriveObject, error) {
	parameters := api.NewQueryParameters()
	parameters.SetFileInDirectory(path)
	if onExist == AutoNameOnExist {
		parameters.Set("on_exist", string(onExist))
	}

	var err error
	if !modTime.IsZero() {
		err = parameters.SetTime("mtime", modTime)
		if err != nil {
			return nil, err
		}
	}

	opts := rest.Opts{
		Method:      "POST",
		Path:        "/file",
		Body:        content,
		ContentType: "application/octet-stream",
		Parameters:  parameters.Values,
	}

	var result api.HiDriveObject
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		// Reset the reading index (in case this is a retry).
		if _, err = content.Seek(0, io.SeekStart); err != nil {
			return false, err
		}
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})

	switch {
	case err == nil:
		return &result, nil
	case isHTTPError(err, 404):
		return nil, fs.ErrorDirNotFound
	case isHTTPError(err, 409):
		return nil, ErrorFileExists
	}
	return nil, err
}

// overwriteFile updates the content of the file at the given path
// with the content of the io.ReadSeeker.
// If the file does not exist it will be created.
// The maximum size of the content is limited by MaximumUploadBytes.
// The io.ReadSeeker should be resettable by seeking to its start.
// If modTime is not the zero time instant,
// it will be set as the file's modification time after the operation.
//
// This returns fs.ErrorDirNotFound
// if the parent directory of the file is not found.
func (f *Fs) overwriteFile(ctx context.Context, path string, content io.ReadSeeker, modTime time.Time) (*api.HiDriveObject, error) {
	parameters := api.NewQueryParameters()
	parameters.SetFileInDirectory(path)

	var err error
	if !modTime.IsZero() {
		err = parameters.SetTime("mtime", modTime)
		if err != nil {
			return nil, err
		}
	}

	opts := rest.Opts{
		Method:      "PUT",
		Path:        "/file",
		Body:        content,
		ContentType: "application/octet-stream",
		Parameters:  parameters.Values,
	}

	var result api.HiDriveObject
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		// Reset the reading index (in case this is a retry).
		if _, err = content.Seek(0, io.SeekStart); err != nil {
			return false, err
		}
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})

	switch {
	case err == nil:
		return &result, nil
	case isHTTPError(err, 404):
		return nil, fs.ErrorDirNotFound
	}
	return nil, err
}

// uploadFileChunked updates the content of the existing file at the given path
// with the content of the io.Reader.
// Returns the position of the last successfully written byte, stopping before the first failed write.
// If nothing was written this will be 0.
// Returns the resulting api-object if successful.
//
// Replaces the file contents by uploading multiple chunks of the given size in parallel.
// Therefore this can and be used to upload files of any size efficiently.
// The number of parallel transfers is limited by transferLimit which should larger than 0.
// If modTime is not the zero time instant,
// it will be set as the file's modification time after the operation.
//
// NOTE: This method uses updateFileChunked and may create sparse files,
// if the upload of a chunk fails unexpectedly.
// See note about sparse files in patchFile.
// If any of the uploads fail, the process will be aborted and
// the first error that occurred will be returned.
// This is not an atomic operation,
// therefore if the upload fails the file may be partially modified.
//
// This returns fs.ErrorObjectNotFound if the object is not found.
func (f *Fs) uploadFileChunked(ctx context.Context, path string, content io.Reader, modTime time.Time, chunkSize int, transferLimit int64) (okSize uint64, info *api.HiDriveObject, err error) {
	okSize, err = f.updateFileChunked(ctx, path, content, 0, chunkSize, transferLimit)

	if err == nil {
		info, err = f.resizeFile(ctx, path, okSize, modTime)
	}
	return okSize, info, err
}

// updateFileChunked updates the content of the existing file at the given path
// starting at the given offset.
// Returns the position of the last successfully written byte, stopping before the first failed write.
// If nothing was written this will be 0.
//
// Replaces the file contents starting from the given byte offset
// with the content of the io.Reader.
// If the offset is beyond the file end, the file is extended up to the offset.
//
// The upload is done multiple chunks of the given size in parallel.
// Therefore this can and be used to upload files of any size efficiently.
// The number of parallel transfers is limited by transferLimit which should larger than 0.
//
// NOTE: Because it is inefficient to set the modification time with every chunk,
// setting it to a specific value must be done in a separate request
// after this operation finishes.
//
// NOTE: This method uses patchFile and may create sparse files,
// especially if the upload of a chunk fails unexpectedly.
// See note about sparse files in patchFile.
// If any of the uploads fail, the process will be aborted and
// the first error that occurred will be returned.
// This is not an atomic operation,
// therefore if the upload fails the file may be partially modified.
//
// This returns fs.ErrorObjectNotFound if the object is not found.
func (f *Fs) updateFileChunked(ctx context.Context, path string, content io.Reader, offset uint64, chunkSize int, transferLimit int64) (okSize uint64, err error) {
	var (
		okChunksMu sync.Mutex // protects the variables below
		okChunks   []ranges.Range
	)
	g, gCtx := errgroup.WithContext(ctx)
	transferSemaphore := semaphore.NewWeighted(transferLimit)

	var readErr error
	startMoreTransfers := true
	zeroTime := time.Time{}
	for chunk := uint64(0); startMoreTransfers; chunk++ {
		// Acquire semaphore to limit number of transfers in parallel.
		readErr = transferSemaphore.Acquire(gCtx, 1)
		if readErr != nil {
			break
		}

		// Read a chunk of data.
		chunkReader, bytesRead, readErr := readerForChunk(content, chunkSize)
		if bytesRead < chunkSize {
			startMoreTransfers = false
		}
		if readErr != nil || bytesRead <= 0 {
			break
		}

		// Transfer the chunk.
		chunkOffset := uint64(chunkSize)*chunk + offset
		g.Go(func() error {
			// After this upload is done,
			// signal that another transfer can be started.
			defer transferSemaphore.Release(1)
			uploadErr := f.patchFile(gCtx, path, cachedReader(chunkReader), chunkOffset, zeroTime)
			if uploadErr == nil {
				// Remember successfully written chunks.
				okChunksMu.Lock()
				okChunks = append(okChunks, ranges.Range{Pos: int64(chunkOffset), Size: int64(bytesRead)})
				okChunksMu.Unlock()
				fs.Debugf(f, "Done uploading chunk of size %v at offset %v.", bytesRead, chunkOffset)
			} else {
				fs.Infof(f, "Error while uploading chunk at offset %v. Error is %v.", chunkOffset, uploadErr)
			}
			return uploadErr
		})
	}

	if readErr != nil {
		// Log the error in case it is later ignored because of an upload-error.
		fs.Infof(f, "Error while reading/preparing to upload a chunk. Error is %v.", readErr)
	}

	err = g.Wait()

	// Compute the first continuous range of the file content,
	// which does not contain any failed chunks.
	// Do not forget to add the file content up to the starting offset,
	// which is presumed to be already correct.
	rs := ranges.Ranges{}
	rs.Insert(ranges.Range{Pos: 0, Size: int64(offset)})
	for _, chunkRange := range okChunks {
		rs.Insert(chunkRange)
	}
	if len(rs) > 0 && rs[0].Pos == 0 {
		okSize = uint64(rs[0].Size)
	}

	if err != nil {
		return okSize, err
	}
	if readErr != nil {
		return okSize, readErr
	}

	return okSize, nil
}

// patchFile updates the content of the existing file at the given path
// starting at the given offset.
//
// Replaces the file contents starting from the given byte offset
// with the content of the io.ReadSeeker.
// If the offset is beyond the file end, the file is extended up to the offset.
// The maximum size of the update is limited by MaximumUploadBytes.
// The io.ReadSeeker should be resettable by seeking to its start.
// If modTime is not the zero time instant,
// it will be set as the file's modification time after the operation.
//
// NOTE: By extending the file up to the offset this may create sparse files,
// which allocate less space on the file system than their apparent size indicates,
// since holes between data chunks are "real" holes
// and not regions made up of consecutive 0-bytes.
// Subsequent operations (such as copying data)
// usually expand the holes into regions of 0-bytes.
//
// This returns fs.ErrorObjectNotFound if the object is not found.
func (f *Fs) patchFile(ctx context.Context, path string, content io.ReadSeeker, offset uint64, modTime time.Time) error {
	parameters := api.NewQueryParameters()
	parameters.SetPath(path)
	parameters.Set("offset", strconv.FormatUint(offset, 10))

	if !modTime.IsZero() {
		err := parameters.SetTime("mtime", modTime)
		if err != nil {
			return err
		}
	}

	opts := rest.Opts{
		Method:      "PATCH",
		Path:        "/file",
		Body:        content,
		ContentType: "application/octet-stream",
		Parameters:  parameters.Values,
		NoResponse:  true,
	}

	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		// Reset the reading index (in case this is a retry).
		_, err = content.Seek(0, io.SeekStart)
		if err != nil {
			return false, err
		}
		resp, err = f.srv.Call(ctx, &opts)
		if isHTTPError(err, 423) {
			return true, err
		}
		return f.shouldRetry(ctx, resp, err)
	})

	if isHTTPError(err, 404) {
		return fs.ErrorObjectNotFound
	}
	return err
}

// resizeFile updates the existing file at the given path to be of the given size
// and returns the resulting api-object if successful.
//
// If the given size is smaller than the current filesize,
// the file is cut/truncated at that position.
// If the given size is larger, the file is extended up to that position.
// If modTime is not the zero time instant,
// it will be set as the file's modification time after the operation.
//
// NOTE: By extending the file this may create sparse files,
// which allocate less space on the file system than their apparent size indicates,
// since holes between data chunks are "real" holes
// and not regions made up of consecutive 0-bytes.
// Subsequent operations (such as copying data)
// usually expand the holes into regions of 0-bytes.
//
// This returns fs.ErrorObjectNotFound if the object is not found.
func (f *Fs) resizeFile(ctx context.Context, path string, size uint64, modTime time.Time) (*api.HiDriveObject, error) {
	parameters := api.NewQueryParameters()
	parameters.SetPath(path)
	parameters.Set("size", strconv.FormatUint(size, 10))

	if !modTime.IsZero() {
		err := parameters.SetTime("mtime", modTime)
		if err != nil {
			return nil, err
		}
	}

	opts := rest.Opts{
		Method:     "POST",
		Path:       "/file/truncate",
		Parameters: parameters.Values,
	}

	var result api.HiDriveObject
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})

	switch {
	case err == nil:
		return &result, nil
	case isHTTPError(err, 404):
		return nil, fs.ErrorObjectNotFound
	}
	return nil, err
}

// ------------------------------------------------------------

// isHTTPError compares the numerical status code
// of an api.Error to the given HTTP status.
//
// If the given error is not an api.Error or
// a numerical status code could not be determined, this returns false.
// Otherwise this returns whether the status code of the error is equal to the given status.
func isHTTPError(err error, status int64) bool {
	if apiErr, ok := err.(*api.Error); ok {
		errStatus, decodeErr := apiErr.Code.Int64()
		if decodeErr == nil && errStatus == status {
			return true
		}
	}
	return false
}

// createHiDriveScopes creates oauth-scopes
// from the given user-role and access-permissions.
//
// If the arguments are empty, they will not be included in the result.
func createHiDriveScopes(role string, access string) []string {
	switch {
	case role != "" && access != "":
		return []string{access + "," + role}
	case role != "":
		return []string{role}
	case access != "":
		return []string{access}
	}
	return []string{}
}

// cachedReader returns a version of the reader that caches its contents and
// can therefore be reset using Seek.
func cachedReader(reader io.Reader) io.ReadSeeker {
	bytesReader, ok := reader.(*bytes.Reader)
	if ok {
		return bytesReader
	}

	repeatableReader, ok := reader.(*readers.RepeatableReader)
	if ok {
		return repeatableReader
	}

	return readers.NewRepeatableReader(reader)
}

// readerForChunk reads a chunk of bytes from reader (after handling any accounting).
// Returns a new io.Reader (chunkReader) for that chunk
// and the number of bytes that have been read from reader.
func readerForChunk(reader io.Reader, length int) (chunkReader io.Reader, bytesRead int, err error) {
	// Unwrap any accounting from the input if present.
	reader, wrap := accounting.UnWrap(reader)

	// Read a chunk of data.
	buffer := make([]byte, length)
	bytesRead, err = io.ReadFull(reader, buffer)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		err = nil
	}
	if err != nil {
		return nil, bytesRead, err
	}
	// Truncate unused capacity.
	buffer = buffer[:bytesRead]

	// Use wrap to put any accounting back for chunkReader.
	return wrap(bytes.NewReader(buffer)), bytesRead, nil
}
