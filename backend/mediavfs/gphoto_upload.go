package mediavfs

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/rclone/rclone/fs"
)

// CalculateSHA1Hash calculates the SHA1 hash of a file
func CalculateSHA1Hash(filePath string) ([]byte, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := sha1.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return nil, "", fmt.Errorf("failed to hash file: %w", err)
	}

	hashBytes := hasher.Sum(nil)
	hashB64 := base64.StdEncoding.EncodeToString(hashBytes)

	return hashBytes, hashB64, nil
}

// UploadFileToGPhotos uploads a file to Google Photos
func (f *Fs) UploadFileToGPhotos(ctx context.Context, filePath string, user string) (string, error) {
	// Initialize API client if not exists
	api := NewGPhotoAPI(user, f.opt.TokenServerURL, f.httpClient)

	// Ensure we have a token
	if err := api.GetAuthToken(ctx, false); err != nil {
		return "", fmt.Errorf("failed to get auth token: %w", err)
	}

	// Calculate SHA1 hash
	fs.Infof(f, "Calculating SHA1 hash for %s", filePath)
	hashBytes, hashB64, err := CalculateSHA1Hash(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	// Check if file already exists
	fs.Infof(f, "Checking if file exists in Google Photos")
	existingMediaKey, err := api.FindRemoteMediaByHash(ctx, hashBytes)
	if err == nil && existingMediaKey != "" {
		fs.Infof(f, "File already exists in Google Photos with media key: %s", existingMediaKey)
		return existingMediaKey, nil
	}

	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := fileInfo.Size()

	// Get upload token
	fs.Infof(f, "Getting upload token for %s (size: %d bytes)", filePath, fileSize)
	uploadToken, err := api.GetUploadToken(ctx, hashB64, fileSize)
	if err != nil {
		return "", fmt.Errorf("failed to get upload token: %w", err)
	}

	// Upload file
	fs.Infof(f, "Uploading file %s", filePath)
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for upload: %w", err)
	}
	defer file.Close()

	uploadResponse, err := api.UploadFile(ctx, uploadToken, file, fileSize)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	// Commit upload
	fs.Infof(f, "Committing upload for %s", filePath)
	fileName := fileInfo.Name()
	uploadTimestamp := fileInfo.ModTime().Unix()
	model := "Pixel XL"
	quality := "original"

	mediaKey, err := api.CommitUpload(ctx, uploadResponse, fileName, hashBytes, fileSize, uploadTimestamp, model, quality)
	if err != nil {
		return "", fmt.Errorf("failed to commit upload: %w", err)
	}

	fs.Infof(f, "Successfully uploaded %s with media key: %s", filePath, mediaKey)
	return mediaKey, nil
}

// UploadWithProgress uploads a file with progress reporting
func (f *Fs) UploadWithProgress(ctx context.Context, src fs.ObjectInfo, in io.Reader, user string) (string, error) {
	// Initialize API client
	api := NewGPhotoAPI(user, f.opt.TokenServerURL, f.httpClient)

	// Ensure we have a token
	if err := api.GetAuthToken(ctx, false); err != nil {
		return "", fmt.Errorf("failed to get auth token: %w", err)
	}

	// For now, we need to read the entire file to calculate hash
	// In a production implementation, you'd want to use a temporary file or streaming hash
	fs.Infof(f, "Reading file to calculate SHA1 hash")

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "gphoto-upload-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Copy to temp file while hashing
	hasher := sha1.New()
	tee := io.TeeReader(in, hasher)

	written, err := io.Copy(tmpFile, tee)
	if err != nil {
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}

	// Reject empty files - Google Photos doesn't accept them
	if written == 0 {
		return "", fmt.Errorf("cannot upload empty file (0 bytes)")
	}

	hashBytes := hasher.Sum(nil)
	hashB64 := base64.StdEncoding.EncodeToString(hashBytes)

	fs.Infof(f, "Calculated SHA1 hash: %s", hashB64)

	// Check if file already exists
	existingMediaKey, err := api.FindRemoteMediaByHash(ctx, hashBytes)
	if err == nil && existingMediaKey != "" {
		fs.Infof(f, "File already exists in Google Photos with media key: %s", existingMediaKey)
		return existingMediaKey, nil
	}

	// Get upload token
	fs.Infof(f, "Getting upload token (size: %d bytes)", written)
	uploadToken, err := api.GetUploadToken(ctx, hashB64, written)
	if err != nil {
		return "", fmt.Errorf("failed to get upload token: %w", err)
	}

	// Rewind temp file
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return "", fmt.Errorf("failed to rewind temp file: %w", err)
	}

	// Upload file
	fs.Infof(f, "Uploading file")
	uploadResponse, err := api.UploadFile(ctx, uploadToken, tmpFile, written)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	// Commit upload
	fs.Infof(f, "Committing upload")
	fileName := path.Base(src.Remote()) // Only the filename, not full path
	uploadTimestamp := src.ModTime(ctx).Unix()
	model := "Pixel XL"
	quality := "original"

	mediaKey, err := api.CommitUpload(ctx, uploadResponse, fileName, hashBytes, written, uploadTimestamp, model, quality)
	if err != nil {
		return "", fmt.Errorf("failed to commit upload: %w", err)
	}

	fs.Infof(f, "Successfully uploaded %s with media key: %s", fileName, mediaKey)
	return mediaKey, nil
}

// BatchUpload uploads multiple files
func (f *Fs) BatchUpload(ctx context.Context, filePaths []string, user string) (map[string]string, error) {
	results := make(map[string]string)

	for _, filePath := range filePaths {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
			mediaKey, err := f.UploadFileToGPhotos(ctx, filePath, user)
			if err != nil {
				fs.Errorf(f, "Failed to upload %s: %v", filePath, err)
				continue
			}
			results[filePath] = mediaKey
		}
	}

	return results, nil
}

// MonitorAndUpload monitors a directory and uploads new files
func (f *Fs) MonitorAndUpload(ctx context.Context, uploadPath string, user string, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Scan directory for files
			entries, err := os.ReadDir(uploadPath)
			if err != nil {
				fs.Errorf(f, "Failed to read upload directory: %v", err)
				continue
			}

			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				filePath := uploadPath + "/" + entry.Name()

				// Skip empty files
				info, err := entry.Info()
				if err != nil || info.Size() == 0 {
					continue
				}

				// Upload file
				mediaKey, err := f.UploadFileToGPhotos(ctx, filePath, user)
				if err != nil {
					fs.Errorf(f, "Failed to upload %s: %v", filePath, err)
					continue
				}

				fs.Infof(f, "Uploaded %s -> %s", filePath, mediaKey)

				// Optionally remove file after successful upload
				// os.Remove(filePath)
			}
		}
	}
}
