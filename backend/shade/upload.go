//multipart upload for shade

package shade

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"sync"

	"github.com/rclone/rclone/backend/shade/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/chunksize"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/rclone/rclone/lib/rest"
)

var warnStreamUpload sync.Once

type shadeChunkWriter struct {
	initToken        string
	chunkSize        int64
	size             int64
	f                *Fs
	o                *Object
	completedParts   []api.CompletedPart
	completedPartsMu sync.Mutex
}

// uploadMultipart handles multipart upload for larger files
func (o *Object) uploadMultipart(ctx context.Context, src fs.ObjectInfo, in io.Reader, options ...fs.OpenOption) error {

	chunkWriter, err := multipart.UploadMultipart(ctx, src, in, multipart.UploadMultipartOptions{
		Open:        o.fs,
		OpenOptions: options,
	})
	if err != nil {
		return err
	}

	var shadeWriter = chunkWriter.(*shadeChunkWriter)
	o.size = shadeWriter.size
	return nil
}

// OpenChunkWriter returns the chunk size and a ChunkWriter
//
// Pass in the remote and the src object
// You can also use options to hint at the desired chunk size
func (f *Fs) OpenChunkWriter(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {
	// Temporary Object under construction
	o := &Object{
		fs:     f,
		remote: remote,
	}

	uploadParts := f.opt.MaxUploadParts
	if uploadParts < 1 {
		uploadParts = 1
	} else if uploadParts > maxUploadParts {
		uploadParts = maxUploadParts
	}
	size := src.Size()
	fs.FixRangeOption(options, size)

	// calculate size of parts
	chunkSize := f.opt.ChunkSize

	// size can be -1 here meaning we don't know the size of the incoming file. We use ChunkSize
	// buffers here (default 64 MB). With a maximum number of parts (10,000) this will be a file of
	// 640 GB.
	if size == -1 {
		warnStreamUpload.Do(func() {
			fs.Logf(f, "Streaming uploads using chunk size %v will have maximum file size of %v",
				chunkSize, fs.SizeSuffix(int64(chunkSize)*int64(uploadParts)))
		})
	} else {
		chunkSize = chunksize.Calculator(src, size, uploadParts, chunkSize)
	}

	token, err := o.fs.refreshJWTToken(ctx)
	if err != nil {
		return info, nil, fmt.Errorf("failed to get token: %w", err)
	}

	err = f.ensureParentDirectories(ctx, remote)
	if err != nil {
		return info, nil, fmt.Errorf("failed to ensure parent directories: %w", err)
	}

	fullPath := remote
	if f.root != "" {
		fullPath = path.Join(f.root, remote)
	}

	// Initiate multipart upload
	type initRequest struct {
		Path     string `json:"path"`
		PartSize int64  `json:"partSize"`
	}
	reqBody := initRequest{
		Path:     fullPath,
		PartSize: int64(chunkSize),
	}

	var initResp struct {
		Token string `json:"token"`
	}

	opts := rest.Opts{
		Method:  "POST",
		Path:    fmt.Sprintf("/%s/upload/multipart", o.fs.drive),
		RootURL: o.fs.endpoint,
		ExtraHeaders: map[string]string{
			"Authorization": "Bearer " + token,
		},
		Options: options,
	}

	err = o.fs.pacer.Call(func() (bool, error) {
		res, err := o.fs.srv.CallJSON(ctx, &opts, reqBody, &initResp)
		if err != nil {
			return res != nil && res.StatusCode == http.StatusTooManyRequests, err
		}
		return false, nil
	})

	if err != nil {
		return info, nil, fmt.Errorf("failed to initiate multipart upload: %w", err)
	}

	chunkWriter := &shadeChunkWriter{
		initToken: initResp.Token,
		chunkSize: int64(chunkSize),
		size:      size,
		f:         f,
		o:         o,
	}
	info = fs.ChunkWriterInfo{
		ChunkSize:         int64(chunkSize),
		Concurrency:       f.opt.Concurrency,
		LeavePartsOnError: false,
	}
	return info, chunkWriter, err
}

// WriteChunk will write chunk number with reader bytes, where chunk number >= 0
func (s *shadeChunkWriter) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (bytesWritten int64, err error) {

	token, err := s.f.refreshJWTToken(ctx)
	if err != nil {
		return 0, err
	}

	// Read chunk
	var chunk bytes.Buffer
	n, err := io.Copy(&chunk, reader)

	if n == 0 {
		return 0, nil
	}

	if err != nil {
		return 0, fmt.Errorf("failed to read chunk: %w", err)
	}
	// Get presigned URL for this part
	var partURL api.PartURL

	partOpts := rest.Opts{
		Method:  "POST",
		Path:    fmt.Sprintf("/%s/upload/multipart/part/%d?token=%s", s.f.drive, chunkNumber+1, url.QueryEscape(s.initToken)),
		RootURL: s.f.endpoint,
		ExtraHeaders: map[string]string{
			"Authorization": "Bearer " + token,
		},
	}

	err = s.f.pacer.Call(func() (bool, error) {
		res, err := s.f.srv.CallJSON(ctx, &partOpts, nil, &partURL)
		if err != nil {
			return res != nil && res.StatusCode == http.StatusTooManyRequests, err
		}
		return false, nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to get part URL: %w", err)
	}
	opts := rest.Opts{
		Method:        "PUT",
		RootURL:       partURL.URL,
		Body:          &chunk,
		ContentType:   "",
		ContentLength: &n,
	}

	// Add headers
	var uploadRes *http.Response
	if len(partURL.Headers) > 0 {
		opts.ExtraHeaders = make(map[string]string)
		for k, v := range partURL.Headers {
			opts.ExtraHeaders[k] = v
		}
	}

	err = s.f.pacer.Call(func() (bool, error) {
		uploadRes, err = s.f.srv.Call(ctx, &opts)
		if err != nil {
			return uploadRes != nil && uploadRes.StatusCode == http.StatusTooManyRequests, err
		}
		return false, nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to upload part %d: %w", chunk, err)
	}

	if uploadRes.StatusCode != http.StatusOK && uploadRes.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(uploadRes.Body)
		fs.CheckClose(uploadRes.Body, &err)
		return 0, fmt.Errorf("part upload failed with status %d: %s", uploadRes.StatusCode, string(body))
	}

	// Get ETag from response
	etag := uploadRes.Header.Get("ETag")
	fs.CheckClose(uploadRes.Body, &err)

	s.completedPartsMu.Lock()
	defer s.completedPartsMu.Unlock()
	s.completedParts = append(s.completedParts, api.CompletedPart{
		PartNumber: int32(chunkNumber + 1),
		ETag:       etag,
	})
	return n, nil
}

// Close complete chunked writer finalising the file.
func (s *shadeChunkWriter) Close(ctx context.Context) error {

	// Complete multipart upload
	sort.Slice(s.completedParts, func(i, j int) bool {
		return s.completedParts[i].PartNumber < s.completedParts[j].PartNumber
	})

	type completeRequest struct {
		Parts []api.CompletedPart `json:"parts"`
	}
	var completeBody completeRequest

	if s.completedParts == nil {
		completeBody = completeRequest{Parts: []api.CompletedPart{}}
	} else {
		completeBody = completeRequest{Parts: s.completedParts}
	}

	token, err := s.f.refreshJWTToken(ctx)
	if err != nil {
		return err
	}

	completeOpts := rest.Opts{
		Method:  "POST",
		Path:    fmt.Sprintf("/%s/upload/multipart/complete?token=%s", s.f.drive, url.QueryEscape(s.initToken)),
		RootURL: s.f.endpoint,
		ExtraHeaders: map[string]string{
			"Authorization": "Bearer " + token,
		},
	}

	var response http.Response

	err = s.f.pacer.Call(func() (bool, error) {
		res, err := s.f.srv.CallJSON(ctx, &completeOpts, completeBody, &response)

		if err != nil && res == nil {
			return false, err
		}

		if res.StatusCode == http.StatusTooManyRequests {
			return true, err // Retry on 429
		}

		if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(res.Body)
			return false, fmt.Errorf("complete multipart failed with status %d: %s", res.StatusCode, string(body))
		}

		return false, nil
	})

	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	return nil
}

// Abort chunk write
//
// You can and should call Abort without calling Close.
func (s *shadeChunkWriter) Abort(ctx context.Context) error {
	token, err := s.f.refreshJWTToken(ctx)
	if err != nil {
		return err
	}

	opts := rest.Opts{
		Method:  "POST",
		Path:    fmt.Sprintf("/%s/upload/abort/multipart?token=%s", s.f.drive, url.QueryEscape(s.initToken)),
		RootURL: s.f.endpoint,
		ExtraHeaders: map[string]string{
			"Authorization": "Bearer " + token,
		},
	}

	err = s.f.pacer.Call(func() (bool, error) {
		res, err := s.f.srv.Call(ctx, &opts)
		if err != nil {
			fs.Debugf(s.f, "Failed to abort multipart upload: %v", err)
			return false, nil // Don't retry abort
		}
		if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
			fs.Debugf(s.f, "Abort returned status %d", res.StatusCode)
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("failed to abort multipart upload: %w", err)
	}
	return nil
}
