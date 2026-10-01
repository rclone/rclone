package dosya

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/dosya/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/sync/errgroup"
)

// sourceMTimeHeader is the request header the upload endpoints read to set a
// file's modification time (Unix seconds); see lib/upload/source-timestamps.ts.
const sourceMTimeHeader = "x-dosya-source-mtime"

// modTimeHeaders returns the ExtraHeaders that carry a source modification time
// on an upload, or nil when there is nothing plausible to send. The server
// clamps implausible values itself, but a zero time is never worth a header.
func modTimeHeaders(modTime time.Time) map[string]string {
	if modTime.IsZero() {
		return nil
	}
	return map[string]string{sourceMTimeHeader: strconv.FormatInt(modTime.Unix(), 10)}
}

// workspaceHintHeader names the workspace a by-id request is about. The API
// maps an id to its workspace through an index the workspace fills about a
// second after a write, so an id this client was handed moments ago can still
// miss; the hint lets the server look for it in the named workspace instead.
// It only locates: every route still checks membership against the workspace
// it resolved, and a workspace-pinned key's hint is ignored unless it matches.
const workspaceHintHeader = "X-Dosya-Workspace"

// byIDHeaders returns the ExtraHeaders every /api/files/:id and
// /api/folders/:id call carries, or nil when no workspace is configured.
func (f *Fs) byIDHeaders() map[string]string {
	if f.opt.WorkspaceID == "" {
		return nil
	}
	return map[string]string{workspaceHintHeader: f.opt.WorkspaceID}
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	retry := fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes)
	if retry {
		if d, ok := retryAfter(resp); ok {
			// The API answers 503 + Retry-After while a workspace is being
			// moved between storage homes, and Cloudflare sends it with 429.
			// The pacer's own backoff tops out at maxSleep, far short of that.
			fs.Debugf(nil, "Server asked to retry after %v", d)
			err = pacer.RetryAfterError(err, d)
		}
	}
	return retry, err
}

// maxRetryAfter caps how long a single Retry-After is honoured for
const maxRetryAfter = 5 * time.Minute

// retryAfter reads a 429 or 503 response's Retry-After header, which the API
// sends as whole seconds
func retryAfter(resp *http.Response) (time.Duration, bool) {
	if resp == nil || (resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusServiceUnavailable) {
		return 0, false
	}
	seconds, err := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After")))
	if err != nil || seconds <= 0 {
		return 0, false
	}
	return min(time.Duration(seconds)*time.Second, maxRetryAfter), true
}

// errorHandler decodes a non-2xx response into an *api.Error so callers can
// act on the machine-readable error_code rather than the message text.
func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		return fmt.Errorf("error reading error out of body: %w", err)
	}
	apiErr := &api.Error{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
	}
	if json.Unmarshal(body, apiErr) != nil || apiErr.Message == "" {
		// Not the API's JSON shape (e.g. an edge response that never
		// reached the API) - keep the body text so it isn't lost.
		apiErr.ErrorCode = ""
		apiErr.Message = strings.TrimSpace(string(body))
	}
	return apiErr
}

// The workspace caps the number of in-flight uploads per user. The API
// refuses an upload/init past that cap with a 400 carrying
// error_code "concurrent_upload_limit", and documents it as an error to
// wait on rather than give up on: slots free as uploads finish, and the
// server marks sessions idle for 10 minutes as failed, so waiting up to
// that long guarantees progress unless the workspace really is saturated.
// A 400 is deliberately outside the pacer's retry codes, so the wait
// lives in initUpload instead of shouldRetry.
var (
	concurrentUploadRetryDelay = 5 * time.Second
	concurrentUploadWaitMax    = 10 * time.Minute
)

// isConcurrentUploadLimit reports whether err is the API refusing an
// upload because the workspace's concurrent upload limit is reached
func isConcurrentUploadLimit(err error) bool {
	var apiErr *api.Error
	return errors.As(err, &apiErr) && apiErr.ErrorCode == "concurrent_upload_limit"
}

// listPageSize is the largest page GET /api/files will serve
const listPageSize = 500

// listFilesAndFolders lists files and folders in a directory
//
// The API pages files (at most listPageSize per page) but returns every
// folder on each page, so the files from all pages are gathered and the
// folders are taken from the first page only. Pages are ordered by name,
// which the API tie-breaks on id, so offsets are stable between requests.
func (f *Fs) listFilesAndFolders(ctx context.Context, folderID string) (*api.ListResponse, error) {
	var listing *api.ListResponse
	for page := 1; ; page++ {
		opts := rest.Opts{
			Method: "GET",
			Path:   "/api/files",
			Parameters: map[string][]string{
				"workspace_id": {f.opt.WorkspaceID},
				"per_page":     {strconv.Itoa(listPageSize)},
				"page":         {strconv.Itoa(page)},
				"sort":         {"name"},
				"dir":          {"asc"},
			},
		}
		if folderID != "" && folderID != rootID {
			opts.Parameters["folder_id"] = []string{folderID}
		}

		var result api.ListResponse
		err := f.pacer.Call(func() (bool, error) {
			resp, err := f.rest.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return nil, fmt.Errorf("couldn't list files: %w", err)
		}
		if !result.OK {
			return nil, fmt.Errorf("API error: %s", result.Error)
		}
		if listing == nil {
			listing = &result
		} else {
			listing.Files = append(listing.Files, result.Files...)
		}
		if page >= result.Pagination.TotalPages || len(result.Files) == 0 {
			return listing, nil
		}
	}
}

// createFolder creates a folder
func (f *Fs) createFolder(ctx context.Context, name string, parentID string) (*api.CreateFolderResponse, error) {
	request := api.CreateFolderRequest{
		WorkspaceID: f.opt.WorkspaceID,
		Name:        name,
	}
	if parentID != "" && parentID != rootID {
		request.ParentID = &parentID
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/folders",
	}

	var result api.CreateFolderResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't create folder: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}

// removeFolder deletes a folder
func (f *Fs) removeFolder(ctx context.Context, folderID string) error {
	opts := rest.Opts{
		Method:       "DELETE",
		Path:         "/api/folders/" + folderID,
		ExtraHeaders: f.byIDHeaders(),
	}

	// The first DELETE moves the folder to the trash, where its files still
	// count against the workspace quota. A DELETE on the trashed folder
	// purges it permanently, but a big subtree is purged in bounded passes
	// (202, complete=false), so keep calling until the purge is complete.
	for pass := 0; ; pass++ {
		var result api.DeleteFolderResponse
		err := f.pacer.Call(func() (bool, error) {
			resp, err := f.rest.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			if pass == 0 {
				return fmt.Errorf("couldn't remove folder: %w", err)
			}
			return fmt.Errorf("couldn't permanently remove folder: %w", err)
		}
		if !result.OK {
			return fmt.Errorf("API error: %s", result.Error)
		}
		if result.Permanent && result.Complete {
			return nil
		}
		if pass >= maxFolderPurgePasses {
			return fmt.Errorf("couldn't permanently remove folder: purge still incomplete after %d passes", pass)
		}
	}
}

// maxFolderPurgePasses bounds the DELETE calls spent purging one folder
const maxFolderPurgePasses = 1000

// renameFolder renames a folder
func (f *Fs) renameFolder(ctx context.Context, folderID string, newName string) error {
	request := api.RenameFolderRequest{Name: newName}
	opts := rest.Opts{
		Method:       "PUT",
		Path:         "/api/folders/" + folderID + "/rename",
		ExtraHeaders: f.byIDHeaders(),
	}

	var result api.Response
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't rename folder: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("API error: %s", result.Error)
	}
	return nil
}

// moveFolder moves a folder to a new parent
func (f *Fs) moveFolder(ctx context.Context, folderID string, newParentID string, newName string) (renamed bool, err error) {
	request := api.MoveFolderRequest{Name: newName}
	if newParentID != "" && newParentID != rootID {
		request.ParentID = &newParentID
	}
	opts := rest.Opts{
		Method:       "PUT",
		Path:         "/api/folders/" + folderID + "/move",
		ExtraHeaders: f.byIDHeaders(),
	}

	var result api.MoveFolderResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return false, fmt.Errorf("couldn't move folder: %w", err)
	}
	if !result.OK {
		return false, fmt.Errorf("API error: %s", result.Error)
	}
	// Servers that predate the name field ignore it and answer without a
	// name, leaving the caller to rename separately
	return newName != "" && result.Name == newName, nil
}

// deleteFile deletes a file (soft delete first, then permanent)
func (f *Fs) deleteFile(ctx context.Context, fileID string) error {
	opts := rest.Opts{
		Method:       "DELETE",
		Path:         "/api/files/" + fileID,
		ExtraHeaders: f.byIDHeaders(),
	}

	var result api.DeleteFileResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't delete file: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("API error: %s", result.Error)
	}

	// The first DELETE soft-deletes; a second one on an already soft-deleted
	// file purges it from the database and R2. Answered into its own value so
	// a field the second response omits can't be read off the first.
	if !result.Permanent {
		var permanent api.DeleteFileResponse
		err = f.pacer.Call(func() (bool, error) {
			resp, err := f.rest.CallJSON(ctx, &opts, nil, &permanent)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return fmt.Errorf("couldn't permanently delete file: %w", err)
		}
		if !permanent.OK {
			return fmt.Errorf("API error: %s", permanent.Error)
		}
	}
	return nil
}

// renameFile renames a file
func (f *Fs) renameFile(ctx context.Context, fileID string, newName string) error {
	request := api.RenameFileRequest{Name: newName}
	opts := rest.Opts{
		Method:       "PUT",
		Path:         "/api/files/" + fileID + "/rename",
		ExtraHeaders: f.byIDHeaders(),
	}

	var result api.RenameFileResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't rename file: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("API error: %s", result.Error)
	}
	return nil
}

// moveFile moves a file to a new folder and, if newName is non-empty,
// renames it in the same request
func (f *Fs) moveFile(ctx context.Context, fileID string, folderID string, newName string) error {
	request := api.MoveFileRequest{}
	if folderID != "" && folderID != rootID {
		request.FolderID = &folderID
	}
	if newName != "" {
		request.Name = &newName
	}
	opts := rest.Opts{
		Method:       "PUT",
		Path:         "/api/files/" + fileID + "/move",
		ExtraHeaders: f.byIDHeaders(),
	}

	var result api.Response
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("couldn't move file: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("API error: %s", result.Error)
	}
	return nil
}

// copyFile copies a file to a folder
func (f *Fs) copyFile(ctx context.Context, fileID string, folderID string, name string) (*api.CopyFileResponse, error) {
	request := api.CopyFileRequest{Name: name}
	if folderID != "" && folderID != rootID {
		request.FolderID = &folderID
	}
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/api/files/" + fileID + "/copy",
		ExtraHeaders: f.byIDHeaders(),
	}

	var result api.CopyFileResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't copy file: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}

// downloadFile downloads a file by its ID
//
// The download endpoint answers 302 with a presigned R2 URL in Location, so
// this is two calls. Both go through the rest client, which is built on
// fshttp, so --timeout, --contimeout, --ca-cert, --no-check-certificate,
// --user-agent, --bind, --dump and the proxy settings apply to downloads as
// they do to every other call, and errorHandler and shouldRetry decide the
// errors and the retries uniformly.
func (f *Fs) downloadFile(ctx context.Context, fileID string, options []fs.OpenOption) (io.ReadCloser, error) {
	// NoRedirect stops the client following the 302 with the API's
	// Authorization header attached. A 302 is not a 2xx, so Call reports it
	// as an error (having read and closed the body); the Location header
	// is what actually matters, and finding it clears the error, as in
	// backend/linkbox.
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/api/files/" + fileID + "/download",
		ExtraHeaders: f.byIDHeaders(),
		NoRedirect:   true,
	}
	var downloadURL string
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.Call(ctx, &opts)
		if resp != nil {
			if location := resp.Header.Get("Location"); location != "" {
				downloadURL, err = location, nil
			}
		}
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't get download URL: %w", err)
	}
	if downloadURL == "" {
		return nil, errors.New("no download URL in redirect response")
	}

	// Fetch the object from the presigned URL. It carries its own
	// credentials and R2 refuses a request that also has an Authorization
	// header, so the client-level bearer token is removed by setting the
	// header empty: the rest client skips a header with an empty value.
	opts = rest.Opts{
		Method:       "GET",
		RootURL:      downloadURL,
		Options:      options,
		ExtraHeaders: map[string]string{"Authorization": ""},
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		var err error
		resp, err = f.rest.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't download file: %w", err)
	}
	return resp.Body, nil
}

// uploadSmallFile uploads a file smaller than 10MB in a single request
func (f *Fs) uploadSmallFile(ctx context.Context, in io.Reader, sessionID string, size int64, modTime time.Time) (*api.UploadCompleteResponse, error) {
	opts := rest.Opts{
		Method:        "PUT",
		Path:          "/api/upload/" + sessionID,
		Body:          in,
		ContentLength: &size,
		ContentType:   "application/octet-stream",
		ExtraHeaders:  modTimeHeaders(modTime),
	}

	// The body is a stream which can't be rewound, so a low level retry
	// would resend whatever is left of it - nothing - and the server
	// would store that as an empty file. Don't retry here; rclone's high
	// level retries redo the whole transfer instead.
	var result api.UploadCompleteResponse
	err := f.pacer.CallNoRetry(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't upload file: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}

// uploadPart uploads a single part of a multipart upload
func (f *Fs) uploadPart(ctx context.Context, part []byte, sessionID string, partNumber int) (*api.UploadPartResponse, error) {
	size := int64(len(part))
	opts := rest.Opts{
		Method:        "PUT",
		Path:          "/api/upload/" + sessionID + "/part/" + strconv.Itoa(partNumber),
		ContentLength: &size,
		ContentType:   "application/octet-stream",
	}

	var result api.UploadPartResponse
	err := f.pacer.Call(func() (bool, error) {
		// A fresh reader per attempt so a retry resends the whole part
		opts.Body = bytes.NewReader(part)
		resp, err := f.rest.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't upload part %d: %w", partNumber, err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}

// completeUpload finalizes a multipart upload
func (f *Fs) completeUpload(ctx context.Context, sessionID string, modTime time.Time) (*api.UploadCompleteResponse, error) {
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/api/upload/" + sessionID + "/complete",
		ExtraHeaders: modTimeHeaders(modTime),
	}

	var result api.UploadCompleteResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't complete upload: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}

// initUpload initializes a new upload session
func (f *Fs) initUpload(ctx context.Context, fileName string, fileSize int64, mimeType string, folderID string, fileID *string) (*api.UploadInitResponse, error) {
	request := api.UploadInitRequest{
		WorkspaceID: f.opt.WorkspaceID,
		FileName:    fileName,
		FileSize:    fileSize,
		MimeType:    mimeType,
		FileID:      fileID,
	}
	if folderID != "" && folderID != rootID {
		request.FolderID = &folderID
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/upload/init",
	}

	var result api.UploadInitResponse
	deadline := time.Now().Add(concurrentUploadWaitMax)
	for {
		result = api.UploadInitResponse{}
		err := f.pacer.Call(func() (bool, error) {
			resp, err := f.rest.CallJSON(ctx, &opts, &request, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err == nil {
			break
		}
		if !isConcurrentUploadLimit(err) || time.Now().After(deadline) {
			return nil, fmt.Errorf("couldn't init upload: %w", err)
		}
		fs.Debugf(f, "Upload slots in use, waiting %v before retrying %q: %v", concurrentUploadRetryDelay, fileName, err)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("couldn't init upload: %w", ctx.Err())
		case <-time.After(concurrentUploadRetryDelay):
		}
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}

// uploadParts reads the source part by part and uploads the parts.
//
// Part 1 goes on its own: the server creates the R2 multipart upload (and a
// new file's id and key) when it receives the first part, so parts sent
// before that has finished would each start an upload of their own. The
// rest go out opt.UploadConcurrency at a time. A part is only read from the
// source once a buffer is free, so at most that many parts are held in
// memory, and a short source is still found by the reader before the part
// it could not fill is sent.
func (f *Fs) uploadParts(ctx context.Context, in *readers.CountingReader, sessionID string, size, partSize int64, totalParts int) error {
	readPart := func(buf []byte, partNum int) ([]byte, error) {
		currentPartSize := partSize
		if partNum == totalParts {
			currentPartSize = size - partSize*int64(totalParts-1)
		}
		part := buf[:currentPartSize]
		if _, err := io.ReadFull(in, part); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil, fmt.Errorf("expected %d bytes in input, but got %d: %w", size, in.BytesRead(), io.ErrUnexpectedEOF)
			}
			return nil, fmt.Errorf("failed to read part %d: %w", partNum, err)
		}
		return part, nil
	}

	concurrency := max(f.opt.UploadConcurrency, 1)
	buffers := make(chan []byte, concurrency)
	buffers <- make([]byte, partSize)
	allocated := 1

	first := <-buffers
	part, err := readPart(first, 1)
	if err != nil {
		return err
	}
	if _, err := f.uploadPart(ctx, part, sessionID, 1); err != nil {
		return err
	}
	buffers <- first

	// A failed read has to take the parts already in flight down with it,
	// or they upload to completion while the error waits on Wait.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	g, gCtx := errgroup.WithContext(ctx)
	var readErr error
	for partNum := 2; partNum <= totalParts; partNum++ {
		// Take a free buffer, allocating one while under the limit.
		var buf []byte
		select {
		case buf = <-buffers:
		default:
			if allocated < concurrency {
				buf = make([]byte, partSize)
				allocated++
			} else {
				select {
				case buf = <-buffers:
				case <-gCtx.Done():
				}
			}
		}
		if buf == nil {
			break // a part failed; its error comes back from Wait
		}
		part, err := readPart(buf, partNum)
		if err != nil {
			readErr = err
			cancel()
			break
		}
		g.Go(func() error {
			defer func() { buffers <- buf }()
			_, err := f.uploadPart(gCtx, part, sessionID, partNum)
			return err
		})
	}
	err = g.Wait()
	if readErr != nil {
		return readErr
	}
	return err
}

// uploadFile handles the full upload flow (small or multipart)
func (f *Fs) uploadFile(ctx context.Context, in io.Reader, remote string, size int64, modTime time.Time, folderID string, fileID *string) (*api.UploadCompleteResponse, error) {
	// Both refusals live here so a new file and a new version of an existing
	// one answer the same way: upload/init needs a byte count up front, and
	// the server stores no zero-length object.
	if size < 0 {
		return nil, errors.New("can't upload files with unknown size")
	}
	if size == 0 {
		return nil, fs.ErrorCantUploadEmptyFiles
	}

	leaf := f.opt.Enc.FromStandardName(path.Base(remote))

	mimeType := fs.MimeTypeFromName(remote)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	initResp, err := f.initUpload(ctx, leaf, size, mimeType, folderID, fileID)
	if err != nil {
		return nil, err
	}

	// Count what the source actually supplies, so a source which ends
	// early is reported as an error rather than stored as a truncated
	// file.
	counter := readers.NewCountingReader(in)

	// Small file upload (single PUT, no resumable)
	if initResp.Resumable == nil {
		resp, err := f.uploadSmallFile(ctx, counter, initResp.SessionID, size, modTime)
		if err != nil {
			return nil, err
		}
		if got := int64(counter.BytesRead()); got != size {
			return nil, fmt.Errorf("expected %d bytes in input, but got %d: %w", size, got, io.ErrUnexpectedEOF)
		}
		return resp, nil
	}

	// Multipart upload. Each part is read fully into memory before it is
	// sent, so a short source is caught before the part goes out and a
	// part which has to be retried is resent with all of its bytes.
	partSize := initResp.Resumable.PartSize
	totalParts := initResp.Resumable.TotalParts
	// Both bounds matter. partSize*(totalParts-1) >= size means a part would
	// have nothing to carry; size > partSize*totalParts means the last part
	// is bigger than partSize, and readPart would slice past its buffer and
	// panic.
	if partSize <= 0 || totalParts <= 0 || partSize*int64(totalParts-1) >= size || size > partSize*int64(totalParts) {
		return nil, fmt.Errorf("invalid multipart layout from server: %d parts of %d bytes for %d bytes", totalParts, partSize, size)
	}
	if err := f.uploadParts(ctx, counter, initResp.SessionID, size, partSize, totalParts); err != nil {
		return nil, err
	}

	return f.completeUpload(ctx, initResp.SessionID, modTime)
}

// createShareLink creates a public share link for a file
func (f *Fs) createShareLink(ctx context.Context, fileID string, expire fs.Duration) (*api.ShareLinkResponse, error) {
	request := api.ShareLinkRequest{}
	if expire < fs.DurationOff {
		days := int(time.Duration(expire).Hours() / 24)
		if days < 1 {
			days = 1
		}
		request.ExpiresInDays = &days
	}

	opts := rest.Opts{
		Method:       "POST",
		Path:         "/api/files/" + fileID + "/share",
		ExtraHeaders: f.byIDHeaders(),
	}

	var result api.ShareLinkResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't create share link: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}

// getWorkspaceInfo fetches workspace storage info
func (f *Fs) getWorkspaceInfo(ctx context.Context) (*api.WorkspaceInfoResponse, error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/api/workspaces/" + f.opt.WorkspaceID,
	}

	var result api.WorkspaceInfoResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't get workspace info: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}
	return &result, nil
}
