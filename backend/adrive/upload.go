package adrive

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"sync"

	"github.com/rclone/rclone/backend/adrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

// upload does a single non-multipart upload
//
// This is recommended for less than 50 MiB of content
func (o *Object) upload(ctx context.Context, in io.Reader, leaf, directoryID string, size int64) (err error) {
	chunkNum := int(math.Ceil(float64(size) / float64(defaultChunkSize)))
	resp, err := o.fs.FileUploadCreate(ctx, &api.FileUploadCreateParam{
		DriveID:         o.fs.driveID,
		Name:            leaf,
		ParentFileID:    directoryID,
		Size:            size,
		CheckNameMode:   "refuse",
		ContentHashName: "none",
		ProofVersion:    "v1",
		Type:            "file",
		PartInfoList:    make([]api.PartInfo, 0),
	}, chunkNum)
	if err != nil {
		return err
	}

	if len(resp.PartInfoList) != chunkNum {
		return fmt.Errorf("failed to upload %v - not sure why", o)
	}

	o.id = resp.FileID

	err = o.uploadMultipart(ctx, resp.PartInfoList, in, size, int64(chunkNum))
	if err != nil {
		return err
	}

	return o.setMetaData(&api.FileEntity{})
}

// uploadMultipart uploads a file using multipart upload
func (o *Object) uploadMultipart(ctx context.Context, parts []api.PartInfo, in io.Reader, size int64, chunkNum int64) error {
	// Check if input reader supports seeking
	seeker, ok := in.(io.ReadSeeker)
	if !ok {
		// Create a temporary file if reader doesn't support seeking
		tempFile, err := os.CreateTemp("", "rclone-adrive-*")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		defer func() {
			_ = tempFile.Close()
			_ = os.Remove(tempFile.Name())
		}()

		// Copy the data to the temporary file
		if _, err := io.Copy(tempFile, in); err != nil {
			return fmt.Errorf("failed to copy data to temp file: %w", err)
		}

		// Rewind the file
		if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek temp file: %w", err)
		}

		seeker = tempFile
	}

	// Setup concurrency control
	concurrency := 4 // Default concurrency
	if c := fs.GetConfig(ctx).Transfers; c > 0 {
		concurrency = c
	}

	// Create a wait group and semaphore for concurrency control
	wg := sync.WaitGroup{}
	sem := make(chan struct{}, concurrency)
	partErrs := make([]error, len(parts))

	// Process each part
	for i, part := range parts {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func(i int, part api.PartInfo) {
			defer func() {
				<-sem // Release semaphore
				wg.Done()
			}()

			// Calculate offset and size for this part
			offset := int64(0)
			if i > 0 {
				// Sum up sizes of previous parts to get the offset
				for j := 0; j < i; j++ {
					offset += parts[j].PartSize
				}
			}

			// Create a limited reader for this part
			partSize := part.PartSize

			// Seek to the correct position
			if _, err := seeker.Seek(offset, io.SeekStart); err != nil {
				partErrs[i] = fmt.Errorf("failed to seek to offset %d: %w", offset, err)
				return
			}

			// Create a buffer with the exact part size
			buffer := io.LimitReader(seeker, partSize)

			// Upload the part
			opts := rest.Opts{
				Method:        "PUT",
				RootURL:       part.UploadURL,
				ContentLength: &partSize,
				Body:          buffer,
				ExtraHeaders: map[string]string{
					"Content-Type": "application/octet-stream",
				},
			}

			if err := o.fs.pacer.Call(func() (bool, error) {
				resp, err := o.fs.srv.Call(ctx, &opts)
				return shouldRetry(ctx, resp, err)
			}); err != nil {
				partErrs[i] = fmt.Errorf("part %d/%d failed: %w", i+1, len(parts), err)
			}
		}(i, part)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Check for errors
	for _, err := range partErrs {
		if err != nil {
			return err
		}
	}

	return nil
}
