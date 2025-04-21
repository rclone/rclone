package adrive

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/rclone/rclone/backend/adrive/api"
	"github.com/rclone/rclone/lib/rest"
)

// upload does a single non-multipart upload
//
// This is recommended for less than 50 MiB of content
func (o *Object) upload(ctx context.Context, in io.Reader, leaf, directoryID string,size int64) (err error) {
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
		return fmt.Errorf("failed to create upload: %w", err)
	}
	o.id = resp.FileID

	preTime := time.Now()

	// Check if the reader implements io.Seeker and if we have a valid size
	seeker, canSeek := in.(io.Seeker)
	useSeek := canSeek && size >= 0

	for k, p := range resp.PartInfoList {
		// Refresh upload URLs if 50 minutes passed
		if time.Since(preTime) > 50*time.Minute {
			refreshResp, refreshErr := o.fs.FileUploadGetUploadURL(ctx, &api.FileUploadGetUploadURLParam{
				DriveID:  o.fs.driveID,
				FileID:   resp.FileID,
				UploadID: resp.UploadID,
			})
			if refreshErr != nil {
				return fmt.Errorf("failed to refresh upload URLs: %w", refreshErr)
			}

			preTime = time.Now()
			resp.PartInfoList = refreshResp.PartInfoList
		}

		// Calculate chunk size
		chunkSize := int64(defaultChunkSize)
		if size >= 0 && k == int(chunkNum-1) {
			chunkSize = size - defaultChunkSize*int64(chunkNum-1)
		}

		// Use Seek if available and we have a valid size
		if useSeek {
			chunkPos := int64(k) * defaultChunkSize
			_, err = seeker.Seek(chunkPos, io.SeekStart)
			if err != nil {
				return fmt.Errorf("failed to seek to position %d: %w", chunkPos, err)
			}
		}

		// Read the chunk
		buf := make([]byte, chunkSize)
		var n int
		n, err = io.ReadFull(in, buf)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("failed to read chunk %d: %w", k, err)
		}

		// Upload the part with retries
		opts := rest.Opts{
			Method:  "PUT",
			RootURL: p.UploadURL,
			Body:    bytes.NewReader(buf[:n]),
		}
		_, err = o.fs.srv.Call(ctx, &opts)
		if err != nil {
			return err
		}
	}

	file, err := o.fs.FileUploadComplete(ctx, &api.FileUploadCompleteParam{
		DriveID:  o.fs.driveID,
		FileID:   resp.FileID,
		UploadID: resp.UploadID,
	})
	if err != nil {
		return fmt.Errorf("failed to complete upload: %w", err)
	}

	return o.setMetaData(file)
}
