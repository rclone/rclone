// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"sync"

	"github.com/zeebo/errs"

	"storj.io/uplink/private/storage/streams/splitter"
	"storj.io/uplink/private/storage/streams/streamupload"
)

type uploadResult struct {
	info streamupload.Info
	err  error
}

// Upload represents an object or part upload and is returned by PutWriter or
// PutWriterPart. Data to be uploaded is written using the Write call. Either
// Commit or Abort must be called to either complete the upload or otherwise
// free up resources related to the upload.
type Upload struct {
	mu     sync.Mutex
	split  *splitter.Splitter
	done   chan uploadResult
	info   streamupload.Info
	cancel func()
}

// Write uploads the object or part data.
func (u *Upload) Write(p []byte) (int, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.done == nil {
		return 0, errs.New("upload already done")
	}

	return u.split.Write(p)
}

// Abort aborts the upload. If called more than once, or after Commit, it will
// return an error.
func (u *Upload) Abort() error {
	u.split.Finish(errs.New("aborted"))
	u.cancel()

	u.mu.Lock()
	defer u.mu.Unlock()

	if u.done == nil {
		return errs.New("upload already done")
	}
	<-u.done
	u.done = nil

	return nil
}

// Commit commits the upload and must be called for the upload to complete
// successfully. It should only be called after all of the data has been
// written.
func (u *Upload) Commit() error {
	u.split.Finish(nil)

	u.mu.Lock()
	defer u.mu.Unlock()

	if u.done == nil {
		return errs.New("upload already done")
	}
	result := <-u.done
	u.info = result.info
	u.done = nil

	u.cancel()
	return result.err
}

// Meta returns the upload metadata. It should only be called after a
// successful Commit and will return nil otherwise.
func (u *Upload) Meta() *Meta {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.done != nil {
		return nil
	}

	return &Meta{
		Modified: u.info.CreationDate,
		Size:     u.info.PlainSize,
	}
}
