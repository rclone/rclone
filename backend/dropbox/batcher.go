// This file contains the implementation of the sync batcher for uploads
//
// Dropbox rules say you can start as many batches as you want, but
// you may only have one batch being committed and must wait for the
// batch to be finished before committing another.

package dropbox

import (
	"context"
	"fmt"

	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/files"
	"github.com/rclone/rclone/fs/fserrors"
)

// finishBatch commits the batch, returning a batch status to poll or maybe complete
func (f *Fs) finishBatch(ctx context.Context, items []*files.UploadSessionFinishArg) (complete *files.UploadSessionFinishBatchResult, err error) {
	var arg = &files.UploadSessionFinishBatchArg{
		Entries: items,
	}
	err = f.pacer.Call(func() (bool, error) {
		complete, err = f.srv.UploadSessionFinishBatchV2(arg)
		// If error is insufficient space then don't retry
		if e, ok := err.(files.UploadSessionFinishAPIError); ok {
			if e.EndpointError != nil && e.EndpointError.Path != nil && e.EndpointError.Path.Tag == files.WriteErrorInsufficientSpace {
				err = fserrors.NoRetryError(err)
				return false, err
			}
		}
		// after the first chunk is uploaded, we retry everything
		return err != nil, err
	})
	if err != nil {
		return nil, fmt.Errorf("batch commit failed: %w", err)
	}
	return complete, nil
}

// Called by the batcher to commit a batch
func (f *Fs) commitBatch(ctx context.Context, items []*files.UploadSessionFinishArg, results []*files.FileMetadata, errors []error) (err error) {
	// finalise the batch getting either a result or a job id to poll
	complete, err := f.finishBatch(ctx, items)
	if err != nil {
		return err
	}

	// Check we got the right number of entries
	entries := complete.Entries
	if len(entries) != len(results) {
		return fmt.Errorf("expecting %d items in batch but got %d", len(results), len(entries))
	}

	// Format results for return
	for i := range results {
		item := entries[i]
		if item.Tag == "success" {
			results[i] = item.Success
		} else {
			errorTag := item.Tag
			if item.Failure != nil {
				errorTag = item.Failure.Tag
				if item.Failure.LookupFailed != nil {
					errorTag += "/" + item.Failure.LookupFailed.Tag
				}
				if item.Failure.Path != nil {
					errorTag += "/" + item.Failure.Path.Tag
				}
				if item.Failure.PropertiesError != nil {
					errorTag += "/" + item.Failure.PropertiesError.Tag
				}
			}
			errors[i] = fmt.Errorf("upload failed: %s", errorTag)
		}
	}

	return nil
}
