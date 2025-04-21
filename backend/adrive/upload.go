package adrive

import (
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
		return fmt.Errorf("failed to create upload: %w", err)
	}
	o.id = resp.FileID

	preTime := time.Now()
	var offset, length int64 = 0, defaultChunkSize

	for k, p := range resp.PartInfoList {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Refresh upload URLs if 50 minutes passed
		if time.Since(preTime) > 50*time.Minute {
			refreshPartInfos := make([]api.PartInfo, len(resp.PartInfoList)-k)
			for j := 0; j < len(refreshPartInfos); j++ {
				refreshPartInfos[j] = api.PartInfo{
					PartNumber: resp.PartInfoList[k+j].PartNumber,
				}
			}

			refreshResp, refreshErr := o.fs.FileUploadGetUploadURL(ctx, &api.FileUploadGetUploadURLParam{
				DriveID:  o.fs.driveID,
				FileID:   resp.FileID,
				UploadID: resp.UploadID,
			})
			if refreshErr != nil {
				return fmt.Errorf("failed to refresh upload URLs: %w", refreshErr)
			}

			// Update remaining parts with new URLs
			for j := 0; j < len(refreshResp.PartInfoList); j++ {
				if k+j < len(resp.PartInfoList) {
					resp.PartInfoList[k+j] = refreshResp.PartInfoList[j]
				}
			}

			preTime = time.Now()
		}

		// Adjust length for the last part
		if remain := size - offset; length > remain {
			length = remain
		}

		// Skip if no data to upload
		if length <= 0 {
			continue
		}

		// Create a limited reader for this part
		limitedReader := io.LimitReader(in, length)

		// Upload the part with retries
		opts := rest.Opts{
			Method:  "PUT",
			RootURL: p.UploadURL,
			Body:    limitedReader,
		}
		_, err = o.fs.srv.Call(ctx, &opts)
		if err != nil {
			return err
		}

		offset += length
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
