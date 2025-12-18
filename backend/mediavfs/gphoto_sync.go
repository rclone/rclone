package mediavfs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rclone/rclone/fs"
)

// InitializeDatabase creates the necessary tables if they don't exist
func (f *Fs) InitializeDatabase(ctx context.Context) error {
	// Create remote_media table
	_, err := f.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS remote_media (
			media_key TEXT PRIMARY KEY,
			file_name TEXT,
			dedup_key TEXT,
			is_canonical BOOLEAN,
			type INTEGER,
			caption TEXT,
			collection_id TEXT,
			size_bytes BIGINT,
			quota_charged_bytes BIGINT,
			origin TEXT,
			content_version INTEGER,
			utc_timestamp BIGINT,
			server_creation_timestamp BIGINT,
			timezone_offset INTEGER,
			width INTEGER,
			height INTEGER,
			remote_url TEXT,
			upload_status INTEGER,
			trash_timestamp BIGINT,
			is_archived BOOLEAN,
			is_favorite BOOLEAN,
			is_locked BOOLEAN,
			is_original_quality BOOLEAN,
			latitude DOUBLE PRECISION,
			longitude DOUBLE PRECISION,
			location_name TEXT,
			location_id TEXT,
			is_edited BOOLEAN,
			make TEXT,
			model TEXT,
			aperture DOUBLE PRECISION,
			shutter_speed DOUBLE PRECISION,
			iso INTEGER,
			focal_length DOUBLE PRECISION,
			duration INTEGER,
			capture_frame_rate DOUBLE PRECISION,
			encoded_frame_rate DOUBLE PRECISION,
			is_micro_video BOOLEAN,
			micro_video_width INTEGER,
			micro_video_height INTEGER,
			user_name TEXT,
			name TEXT,
			path TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create remote_media table: %w", err)
	}

	// Create state table for tracking sync progress
	_, err = f.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS state (
			id SERIAL PRIMARY KEY,
			state_token TEXT,
			page_token TEXT,
			init_complete BOOLEAN DEFAULT FALSE,
			user_name TEXT UNIQUE,
			last_sync_time BIGINT
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create state table: %w", err)
	}

	// Create indices for better performance
	_, err = f.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_remote_media_user_name ON remote_media(user_name);
		CREATE INDEX IF NOT EXISTS idx_remote_media_file_name ON remote_media(file_name);
		CREATE INDEX IF NOT EXISTS idx_remote_media_dedup_key ON remote_media(dedup_key);
		CREATE INDEX IF NOT EXISTS idx_remote_media_size_timestamp ON remote_media(size_bytes, utc_timestamp);
	`)
	if err != nil {
		return fmt.Errorf("failed to create indices: %w", err)
	}

	fs.Infof(f, "Database schema initialized successfully")
	return nil
}

// SyncState represents the sync state for a user
type SyncState struct {
	StateToken   string
	PageToken    string
	InitComplete bool
	LastSyncTime int64
}

// GetSyncState retrieves the sync state for a user
func (f *Fs) GetSyncState(ctx context.Context, user string) (*SyncState, error) {
	var state SyncState
	err := f.db.QueryRowContext(ctx, `
		SELECT state_token, page_token, init_complete, COALESCE(last_sync_time, 0)
		FROM state
		WHERE user_name = $1
	`, user).Scan(&state.StateToken, &state.PageToken, &state.InitComplete, &state.LastSyncTime)

	if err == sql.ErrNoRows {
		// Create initial state
		_, err = f.db.ExecContext(ctx, `
			INSERT INTO state (state_token, page_token, init_complete, user_name, last_sync_time)
			VALUES ('', '', FALSE, $1, 0)
		`, user)
		if err != nil {
			return nil, fmt.Errorf("failed to create initial state: %w", err)
		}
		return &SyncState{
			StateToken:   "",
			PageToken:    "",
			InitComplete: false,
			LastSyncTime: 0,
		}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get sync state: %w", err)
	}

	return &state, nil
}

// UpdateSyncState updates the sync state for a user
func (f *Fs) UpdateSyncState(ctx context.Context, user string, stateToken, pageToken string, initComplete bool) error {
	_, err := f.db.ExecContext(ctx, `
		UPDATE state
		SET state_token = $1, page_token = $2, init_complete = $3, last_sync_time = $4
		WHERE user_name = $5
	`, stateToken, pageToken, initComplete, time.Now().Unix(), user)

	if err != nil {
		return fmt.Errorf("failed to update sync state: %w", err)
	}

	return nil
}

// MediaItem represents a media item from Google Photos
type MediaItem struct {
	MediaKey                string
	FileName                string
	DedupKey                string
	IsCanonical             bool
	Type                    int
	Caption                 *string
	CollectionID            string
	SizeBytes               int64
	QuotaChargedBytes       int64
	Origin                  string
	ContentVersion          int
	UTCTimestamp            int64
	ServerCreationTimestamp int64
	TimezoneOffset          *int
	Width                   *int
	Height                  *int
	RemoteURL               string
	UploadStatus            *int
	TrashTimestamp          *int64
	IsArchived              bool
	IsFavorite              bool
	IsLocked                bool
	IsOriginalQuality       bool
	Latitude                *float64
	Longitude               *float64
	LocationName            *string
	LocationID              *string
	IsEdited                bool
	Make                    *string
	Model                   *string
	Aperture                *float64
	ShutterSpeed            *float64
	ISO                     *int
	FocalLength             *float64
	Duration                *int
	CaptureFrameRate        *float64
	EncodedFrameRate        *float64
	IsMicroVideo            bool
	MicroVideoWidth         *int
	MicroVideoHeight        *int
	UserName                string
}

// InsertMediaItems inserts or updates media items in the database
func (f *Fs) InsertMediaItems(ctx context.Context, items []MediaItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO remote_media (
			media_key, file_name, dedup_key, is_canonical, type, caption, collection_id,
			size_bytes, quota_charged_bytes, origin, content_version, utc_timestamp,
			server_creation_timestamp, timezone_offset, width, height, remote_url,
			upload_status, trash_timestamp, is_archived, is_favorite, is_locked,
			is_original_quality, latitude, longitude, location_name, location_id,
			is_edited, make, model, aperture, shutter_speed, iso, focal_length,
			duration, capture_frame_rate, encoded_frame_rate, is_micro_video,
			micro_video_width, micro_video_height, user_name
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30,
			$31, $32, $33, $34, $35, $36, $37, $38, $39, $40, $41
		)
		ON CONFLICT (media_key) DO UPDATE SET
			file_name = EXCLUDED.file_name,
			dedup_key = EXCLUDED.dedup_key,
			is_canonical = EXCLUDED.is_canonical,
			size_bytes = EXCLUDED.size_bytes,
			content_version = EXCLUDED.content_version,
			utc_timestamp = EXCLUDED.utc_timestamp,
			trash_timestamp = EXCLUDED.trash_timestamp,
			is_archived = EXCLUDED.is_archived,
			is_favorite = EXCLUDED.is_favorite
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, item := range items {
		_, err = stmt.ExecContext(ctx,
			item.MediaKey, item.FileName, item.DedupKey, item.IsCanonical, item.Type,
			item.Caption, item.CollectionID, item.SizeBytes, item.QuotaChargedBytes,
			item.Origin, item.ContentVersion, item.UTCTimestamp, item.ServerCreationTimestamp,
			item.TimezoneOffset, item.Width, item.Height, item.RemoteURL, item.UploadStatus,
			item.TrashTimestamp, item.IsArchived, item.IsFavorite, item.IsLocked,
			item.IsOriginalQuality, item.Latitude, item.Longitude, item.LocationName,
			item.LocationID, item.IsEdited, item.Make, item.Model, item.Aperture,
			item.ShutterSpeed, item.ISO, item.FocalLength, item.Duration,
			item.CaptureFrameRate, item.EncodedFrameRate, item.IsMicroVideo,
			item.MicroVideoWidth, item.MicroVideoHeight, item.UserName,
		)
		if err != nil {
			return fmt.Errorf("failed to insert media item %s: %w", item.MediaKey, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteMediaItems deletes media items from the database
func (f *Fs) DeleteMediaItems(ctx context.Context, mediaKeys []string) error {
	if len(mediaKeys) == 0 {
		return nil
	}

	// Convert to array format for PostgreSQL
	query := `DELETE FROM remote_media WHERE media_key = ANY($1)`

	// Create a PostgreSQL array string
	result, err := f.db.ExecContext(ctx, query, mediaKeys)
	if err != nil {
		return fmt.Errorf("failed to delete media items: %w", err)
	}

	rows, _ := result.RowsAffected()
	fs.Infof(f, "Deleted %d media items from database", rows)

	return nil
}

// SyncFromGooglePhotos syncs media from Google Photos to the database
func (f *Fs) SyncFromGooglePhotos(ctx context.Context, user string) error {
	// Initialize API client
	api := NewGPhotoAPI(user, f.opt.TokenServerURL, f.httpClient)

	// Ensure we have a token
	if err := api.GetAuthToken(ctx, false); err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}

	// Get current sync state
	state, err := f.GetSyncState(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to get sync state: %w", err)
	}

	if !state.InitComplete {
		fs.Infof(f, "Starting initial sync for user %s", user)
		if err := f.initialSync(ctx, api, user); err != nil {
			return fmt.Errorf("initial sync failed: %w", err)
		}
	} else {
		fs.Infof(f, "Starting incremental sync for user %s", user)
		if err := f.incrementalSync(ctx, api, user, state.StateToken); err != nil {
			return fmt.Errorf("incremental sync failed: %w", err)
		}
	}

	return nil
}

// initialSync performs the initial full sync from Google Photos
func (f *Fs) initialSync(ctx context.Context, api *GPhotoAPI, user string) error {
	stateToken := ""
	pageToken := ""

	for {
		// Get library state
		response, err := api.GetLibraryState(ctx, stateToken, pageToken)
		if err != nil {
			return fmt.Errorf("failed to get library state: %w", err)
		}

		// Parse response
		newStateToken, newPageToken, mediaItems, deletions, err := parseLibraryResponse(response, user)
		if err != nil {
			return fmt.Errorf("failed to parse library response: %w", err)
		}

		// Insert media items
		if len(mediaItems) > 0 {
			fs.Infof(f, "Syncing %d media items", len(mediaItems))
			if err := f.InsertMediaItems(ctx, mediaItems); err != nil {
				return fmt.Errorf("failed to insert media items: %w", err)
			}
		}

		// Delete items
		if len(deletions) > 0 {
			fs.Infof(f, "Deleting %d items", len(deletions))
			if err := f.DeleteMediaItems(ctx, deletions); err != nil {
				return fmt.Errorf("failed to delete media items: %w", err)
			}
		}

		// Update state
		stateToken = newStateToken
		pageToken = newPageToken

		// Save progress
		if err := f.UpdateSyncState(ctx, user, stateToken, pageToken, false); err != nil {
			return fmt.Errorf("failed to update sync state: %w", err)
		}

		// Check if we're done
		if pageToken == "" {
			break
		}
	}

	// Mark initial sync as complete
	if err := f.UpdateSyncState(ctx, user, stateToken, "", true); err != nil {
		return fmt.Errorf("failed to mark sync complete: %w", err)
	}

	fs.Infof(f, "Initial sync completed for user %s", user)
	return nil
}

// incrementalSync performs an incremental sync from Google Photos
func (f *Fs) incrementalSync(ctx context.Context, api *GPhotoAPI, user string, stateToken string) error {
	response, err := api.GetLibraryState(ctx, stateToken, "")
	if err != nil {
		return fmt.Errorf("failed to get library state: %w", err)
	}

	// Parse response
	newStateToken, pageToken, mediaItems, deletions, err := parseLibraryResponse(response, user)
	if err != nil {
		return fmt.Errorf("failed to parse library response: %w", err)
	}

	// Insert/update media items
	if len(mediaItems) > 0 {
		fs.Infof(f, "Syncing %d updated items", len(mediaItems))
		if err := f.InsertMediaItems(ctx, mediaItems); err != nil {
			return fmt.Errorf("failed to insert media items: %w", err)
		}
	}

	// Delete items
	if len(deletions) > 0 {
		fs.Infof(f, "Deleting %d items", len(deletions))
		if err := f.DeleteMediaItems(ctx, deletions); err != nil {
			return fmt.Errorf("failed to delete media items: %w", err)
		}
	}

	// Process paginated results if any
	for pageToken != "" {
		response, err = api.GetLibraryPage(ctx, pageToken, newStateToken)
		if err != nil {
			return fmt.Errorf("failed to get library page: %w", err)
		}

		_, pageToken, mediaItems, deletions, err = parseLibraryResponse(response, user)
		if err != nil {
			return fmt.Errorf("failed to parse library page: %w", err)
		}

		if len(mediaItems) > 0 {
			if err := f.InsertMediaItems(ctx, mediaItems); err != nil {
				return fmt.Errorf("failed to insert media items: %w", err)
			}
		}

		if len(deletions) > 0 {
			if err := f.DeleteMediaItems(ctx, deletions); err != nil {
				return fmt.Errorf("failed to delete media items: %w", err)
			}
		}
	}

	// Update state
	if err := f.UpdateSyncState(ctx, user, newStateToken, "", true); err != nil {
		return fmt.Errorf("failed to update sync state: %w", err)
	}

	fs.Infof(f, "Incremental sync completed for user %s (%d updates, %d deletions)",
		user, len(mediaItems), len(deletions))
	return nil
}

// parseLibraryResponse parses the Google Photos library response
// This is a simplified version - in production, you'd need to properly decode protobuf
func parseLibraryResponse(response []byte, user string) (stateToken, pageToken string, items []MediaItem, deletions []string, err error) {
	var data map[string]interface{}
	if err := json.Unmarshal(response, &data); err != nil {
		return "", "", nil, nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract state and page tokens
	if respData, ok := data["1"].(map[string]interface{}); ok {
		if token, ok := respData["6"].(string); ok {
			stateToken = token
		}
		if token, ok := respData["1"].(string); ok {
			pageToken = token
		}

		// Parse media items (simplified - actual implementation would need proper protobuf parsing)
		// For now, return empty to avoid errors
		// TODO: Implement proper protobuf parsing or use the actual API response format
	}

	return stateToken, pageToken, items, deletions, nil
}
