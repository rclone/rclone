// Upload large files for sharefile
//
// Docs - https://api.sharefile.com/rest/docs/resource.aspx?name=Items#Upload_File

package sharefile

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/sharefile/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/lib/rest"
)

// largeUpload is used to control the upload of large files which need chunking
type largeUpload struct {
	ctx      context.Context
	f        *Fs                      // parent Fs
	o        *Object                  // object being uploaded
	in       io.Reader                // read the data from here
	wrap     accounting.WrapFn        // account parts being transferred
	size     int64                    // total size
	parts    int64                    // calculated number of parts, if known
	info     *api.UploadSpecification // where to post chunks, etc.
	threads  int                      // number of threads to use in upload
	streamed bool                     // set if using streamed upload
}

// newLargeUpload starts an upload of object o from in with metadata in src
func (f *Fs) newLargeUpload(ctx context.Context, o *Object, in io.Reader, src fs.ObjectInfo, info *api.UploadSpecification) (up *largeUpload, err error) {
	size := src.Size()
	parts := int64(-1)
	if size >= 0 {
		parts = size / int64(o.fs.opt.ChunkSize)
		if size%int64(o.fs.opt.ChunkSize) != 0 {
			parts++
		}
	}

	var streamed bool
	switch strings.ToLower(info.Method) {
	case "streamed":
		streamed = true
	case "threaded":
		streamed = false
	default:
		return nil, errors.Errorf("can't use method %q with newLargeUpload", info.Method)
	}

	threads := f.ci.Transfers
	if threads > info.MaxNumberOfThreads {
		threads = info.MaxNumberOfThreads
	}

	// unwrap the accounting from the input, we use wrap to put it
	// back on after the buffering
	in, wrap := accounting.UnWrap(in)
	up = &largeUpload{
		ctx:      ctx,
		f:        f,
		o:        o,
		in:       in,
		wrap:     wrap,
		size:     size,
		threads:  threads,
		info:     info,
		parts:    parts,
		streamed: streamed,
	}
	return up, nil
}

// parse the api.UploadFinishResponse in respBody
func (up *largeUpload) parseUploadFinishResponse(respBody []byte) (err error) {
	var finish api.UploadFinishResponse
	err = json.Unmarshal(respBody, &finish)
	if err != nil {
		// Sometimes the unmarshal fails in which case return the body
		return errors.Errorf("upload: bad response: %q", bytes.TrimSpace(respBody))
	}
	return up.o.checkUploadResponse(up.ctx, &finish)
}

// Transfer a chunk
func (up *largeUpload) transferChunk(ctx context.Context, part int64, offset int64, body []byte, fileHash string) error {
	md5sumRaw := md5.Sum(body)
	md5sum := hex.EncodeToString(md5sumRaw[:])
	size := int64(len(body))

	// Add some more parameters to the ChunkURI
	u := up.info.ChunkURI
	u += fmt.Sprintf("&index=%d&byteOffset=%d&hash=%s&fmt=json",
		part, offset, md5sum,
	)
	if fileHash != "" {
		u += fmt.Sprintf("&finish=true&fileSize=%d&fileHash=%s",
			offset+int64(len(body)),
			fileHash,
		)
	}
	opts := rest.Opts{
		Method:        "POST",
		RootURL:       u,
		ContentLength: &size,
	}
	var respBody []byte
	err := up.f.pacer.Call(func() (bool, error) {
		fs.Debugf(up.o, "Sending chunk %d length %d", part, len(body))
		opts.Body = up.wrap(bytes.NewReader(body))
		resp, err := up.f.srv.Call(ctx, &opts)
		if err != nil {
			fs.Debugf(up.o, "Error sending chunk %d: %v", part, err)
		} else {
			respBody, err = rest.ReadBody(resp)
		}
		// retry all errors now that the multipart upload has started
		return err != nil, err
	})
	if err != nil {
		fs.Debugf(up.o, "Error sending chunk %d: %v", part, err)
		return err
	}
	// If last chunk and using "streamed" transfer, get the response back now
	if up.streamed && fileHash != "" {
		return up.parseUploadFinishResponse(respBody)
	}
	fs.Debugf(up.o, "Done sending chunk %d", part)
	return nil
}

// finish closes off the large upload and reads the metadata
func (up *largeUpload) finish(ctx context.Context) error {
	fs.Debugf(up.o, "Finishing large file upload")
	// For a streamed transfer we will already have read the info
	if up.streamed {
		return nil
	}

	opts := rest.Opts{
		Method:  "POST",
		RootURL: up.info.FinishURI,
	}
	var respBody []byte
	err := up.f.pacer.Call(func() (bool, error) {
		resp, err := up.f.srv.Call(ctx, &opts)
		if err != nil {
			return shouldRetry(ctx, resp, err)
		}
		respBody, err = rest.ReadBody(resp)
		// retry all errors now that the multipart upload has started
		return err != nil, err
	})
	if err != nil {
		return err
	}
	return up.parseUploadFinishResponse(respBody)
}

// Upload uploads the chunks from the input
func (up *largeUpload) Upload(ctx context.Context) error {
	if up.parts >= 0 {
		fs.Debugf(up.o, "Starting upload of large file in %d chunks", up.parts)
	} else {
		fs.Debugf(up.o, "Starting streaming upload of large file")
	}
	var (
		offset        int64
		errs          = make(chan error, 1)
		wg            sync.WaitGroup
		err           error
		wholeFileHash = md5.New()
		eof           = false
	)
outer:
	for part := int64(0); !eof; part++ {
		// Check any errors
		select {
		case err = <-errs:
			break outer
		default:
		}

		// Get a block of memory
		buf := up.f.getUploadBlock()

		// Read the chunk
		var n int
		n, err = readers.ReadFill(up.in, buf)
		if err == io.EOF {
			eof = true
			buf = buf[:n]
			err = nil
		} else if err != nil {
			up.f.putUploadBlock(buf)
			break outer
		}

		// Hash it
		_, _ = io.Copy(wholeFileHash, bytes.NewBuffer(buf))

		// Get file hash if was last chunk
		fileHash := ""
		if eof {
			fileHash = hex.EncodeToString(wholeFileHash.Sum(nil))
		}

		// Transfer the chunk
		wg.Add(1)
		transferChunk := func(part, offset int64, buf []byte, fileHash string) {
			defer wg.Done()
			defer up.f.putUploadBlock(buf)
			err := up.transferChunk(ctx, part, offset, buf, fileHash)
			if err != nil {
				select {
				case errs <- err:
				default:
				}
			}
		}
		if up.streamed {
			transferChunk(part, offset, buf, fileHash) // streamed
		} else {
			go transferChunk(part, offset, buf, fileHash) // multithreaded
		}

		offset += int64(n)
	}
	wg.Wait()

	// check size read is correct
	if eof && err == nil && up.size >= 0 && up.size != offset {
		err = errors.Errorf("upload: short read: read %d bytes expected %d", up.size, offset)
	}

	// read any errors
	if err == nil {
		select {
		case err = <-errs:
		default:
		}
	}

	// finish regardless of errors
	finishErr := up.finish(ctx)
	if err == nil {
		err = finishErr
	}

	return err
}
