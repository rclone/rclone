package mediavfs

import (
	"context"
	"fmt"

	"github.com/rclone/rclone/fs"
)

// DeleteFromGPhotos moves files to trash in Google Photos
func (f *Fs) DeleteFromGPhotos(ctx context.Context, dedupKeys []string, user string) error {
	if len(dedupKeys) == 0 {
		return nil
	}

	// Initialize API client
	api := NewGPhotoAPI(user, "https://m.alicuxi.net", f.httpClient)

	// Ensure we have a token
	if err := api.GetAuthToken(ctx, false); err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}

	// Move to trash
	fs.Infof(f, "Moving %d files to trash", len(dedupKeys))
	if err := api.MoveToTrash(ctx, dedupKeys); err != nil {
		return fmt.Errorf("failed to move files to trash: %w", err)
	}

	fs.Infof(f, "Successfully moved %d files to trash", len(dedupKeys))
	return nil
}

// DeleteByMediaKeys deletes files by their media keys
func (f *Fs) DeleteByMediaKeys(ctx context.Context, mediaKeys []string, user string) error {
	// For Google Photos, we need dedup keys, not media keys
	// In your database, you should have a mapping or query to get dedup keys from media keys
	// For now, we'll assume media keys can be used as dedup keys

	return f.DeleteFromGPhotos(ctx, mediaKeys, user)
}

// DeleteObject removes an object from Google Photos
func (o *Object) DeleteFromGPhotos(ctx context.Context) error {
	// Get the dedup key or media key from the object
	// This requires the database to store the dedup key
	// For now, we'll use the media key

	fs.Infof(o.fs, "Deleting object %s (media_key: %s)", o.remote, o.mediaKey)

	return o.fs.DeleteByMediaKeys(ctx, []string{o.mediaKey}, o.userName)
}

// BatchDelete deletes multiple files in batches
func (f *Fs) BatchDelete(ctx context.Context, dedupKeys []string, user string, batchSize int) error {
	if batchSize <= 0 {
		batchSize = 500
	}

	totalBatches := (len(dedupKeys) + batchSize - 1) / batchSize
	fs.Infof(f, "Deleting %d files in %d batches of up to %d", len(dedupKeys), totalBatches, batchSize)

	for i := 0; i < len(dedupKeys); i += batchSize {
		end := i + batchSize
		if end > len(dedupKeys) {
			end = len(dedupKeys)
		}

		batch := dedupKeys[i:end]
		batchNum := (i / batchSize) + 1

		fs.Infof(f, "Processing batch %d/%d (%d files)", batchNum, totalBatches, len(batch))

		if err := f.DeleteFromGPhotos(ctx, batch, user); err != nil {
			return fmt.Errorf("failed to delete batch %d: %w", batchNum, err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	fs.Infof(f, "Successfully deleted all %d files", len(dedupKeys))
	return nil
}

// CleanupDuplicates finds and removes duplicate files
func (f *Fs) CleanupDuplicates(ctx context.Context, user string) error {
	// Query database for duplicates
	query := fmt.Sprintf(`
		SELECT media_key, dedup_key, file_name, size_bytes, utc_timestamp
		FROM %s
		WHERE user_name = $1
		  AND (file_name, size_bytes) IN (
			SELECT file_name, size_bytes
			FROM %s
			WHERE user_name = $1
			GROUP BY file_name, size_bytes
			HAVING COUNT(*) > 1
		  )
		ORDER BY file_name, size_bytes, utc_timestamp DESC
	`, f.opt.TableName, f.opt.TableName)

	rows, err := f.db.QueryContext(ctx, query, user)
	if err != nil {
		return fmt.Errorf("failed to query duplicates: %w", err)
	}
	defer rows.Close()

	// Track duplicates
	type fileGroup struct {
		name  string
		size  int64
		items []struct {
			mediaKey  string
			dedupKey  string
			timestamp int64
		}
	}

	groups := make(map[string]*fileGroup)

	for rows.Next() {
		var mediaKey, dedupKey, fileName string
		var sizeBytes, timestamp int64

		if err := rows.Scan(&mediaKey, &dedupKey, &fileName, &sizeBytes, &timestamp); err != nil {
			return fmt.Errorf("failed to scan duplicate: %w", err)
		}

		key := fmt.Sprintf("%s:%d", fileName, sizeBytes)
		if _, ok := groups[key]; !ok {
			groups[key] = &fileGroup{
				name: fileName,
				size: sizeBytes,
			}
		}

		groups[key].items = append(groups[key].items, struct {
			mediaKey  string
			dedupKey  string
			timestamp int64
		}{mediaKey, dedupKey, timestamp})
	}

	// For each group, keep the newest and delete the rest
	var toDelete []string
	for _, group := range groups {
		if len(group.items) <= 1 {
			continue
		}

		fs.Infof(f, "Found %d duplicates of %s (size: %d)", len(group.items), group.name, group.size)

		// Keep the first one (newest due to ORDER BY timestamp DESC), delete the rest
		for i := 1; i < len(group.items); i++ {
			toDelete = append(toDelete, group.items[i].dedupKey)
		}
	}

	if len(toDelete) == 0 {
		fs.Infof(f, "No duplicates found for user %s", user)
		return nil
	}

	fs.Infof(f, "Found %d duplicate files to delete for user %s", len(toDelete), user)

	// Delete in batches
	return f.BatchDelete(ctx, toDelete, user, 500)
}
