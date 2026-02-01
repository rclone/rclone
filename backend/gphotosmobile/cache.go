// cache.go provides a local SQLite cache for the Google Photos media library index.
//
// # Why SQLite
//
// The Google Photos mobile sync API returns the entire library as a stream of
// protobuf-encoded items. For a library with 35K items, this is ~15MB of data
// taking ~30 seconds to download. Caching the parsed items in SQLite avoids
// re-downloading on every rclone invocation.
//
// The cache uses modernc.org/sqlite (pure Go, no CGo) with WAL mode for
// concurrent read/write safety and a 5-second busy timeout.
//
// # Schema
//
// Two tables:
//
//	remote_media — one row per media item (keyed by media_key), 41 columns
//	               covering all metadata fields from the sync response.
//
//	state — singleton row (id=1) tracking sync state:
//	  state_token    — bookmark for incremental sync (opaque string from server)
//	  page_token     — pagination cursor for interrupted init (crash recovery)
//	  init_complete  — 0/1 whether initial full sync has finished
//	  last_sync_time — unix timestamp of last incremental sync
//
// # Cache location
//
// Default: <rclone-cache-dir>/gphotosmobile/<remote-name>.db
// Override: cache_db_path config option.
//
// Deleting the .db file forces a full re-sync on next run.
package gphotosmobile

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rclone/rclone/fs/config"
	_ "modernc.org/sqlite" // sqlite driver registration
)

// Cache wraps SQLite storage for the media library index
type Cache struct {
	db *sql.DB
}

// NewCache opens or creates the SQLite cache database
func NewCache(dbPath string) (*Cache, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	c := &Cache{db: db}
	if err := c.createTables(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return c, nil
}

func (c *Cache) createTables() error {
	_, err := c.db.Exec(`
	CREATE TABLE IF NOT EXISTS remote_media (
		media_key TEXT PRIMARY KEY,
		file_name TEXT,
		dedup_key TEXT,
		is_canonical INTEGER,
		type INTEGER,
		caption TEXT,
		collection_id TEXT,
		size_bytes INTEGER,
		quota_charged_bytes INTEGER,
		origin TEXT,
		content_version INTEGER,
		utc_timestamp INTEGER,
		server_creation_timestamp INTEGER,
		timezone_offset INTEGER,
		width INTEGER,
		height INTEGER,
		remote_url TEXT,
		upload_status INTEGER,
		trash_timestamp INTEGER,
		is_archived INTEGER,
		is_favorite INTEGER,
		is_locked INTEGER,
		is_original_quality INTEGER,
		latitude REAL,
		longitude REAL,
		location_name TEXT,
		location_id TEXT,
		is_edited INTEGER,
		make TEXT,
		model TEXT,
		aperture REAL,
		shutter_speed REAL,
		iso INTEGER,
		focal_length REAL,
		duration INTEGER,
		capture_frame_rate REAL,
		encoded_frame_rate REAL,
		is_micro_video INTEGER,
		micro_video_width INTEGER,
		micro_video_height INTEGER,
		sha1_hash TEXT
	)`)
	if err != nil {
		return fmt.Errorf("failed to create remote_media table: %w", err)
	}

	_, err = c.db.Exec(`
	CREATE TABLE IF NOT EXISTS state (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		state_token TEXT,
		page_token TEXT,
		init_complete INTEGER,
		last_sync_time INTEGER DEFAULT 0
	)`)
	if err != nil {
		return fmt.Errorf("failed to create state table: %w", err)
	}

	// Add last_sync_time column if migrating from an older schema
	_, _ = c.db.Exec(`ALTER TABLE state ADD COLUMN last_sync_time INTEGER DEFAULT 0`)

	// Add sha1_hash column if migrating from an older schema
	_, _ = c.db.Exec(`ALTER TABLE remote_media ADD COLUMN sha1_hash TEXT`)

	_, err = c.db.Exec(`
	INSERT OR IGNORE INTO state (id, state_token, page_token, init_complete, last_sync_time)
	VALUES (1, '', '', 0, 0)`)
	if err != nil {
		return fmt.Errorf("failed to initialize state: %w", err)
	}

	// Create index on file_name for lookups
	_, err = c.db.Exec(`CREATE INDEX IF NOT EXISTS idx_file_name ON remote_media(file_name)`)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}

// Close closes the database connection
func (c *Cache) Close() error {
	return c.db.Close()
}

// UpsertItems inserts or updates media items
func (c *Cache) UpsertItems(items []MediaItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
	INSERT INTO remote_media (
		media_key, file_name, dedup_key, is_canonical, type, caption,
		collection_id, size_bytes, quota_charged_bytes, origin,
		content_version, utc_timestamp, server_creation_timestamp,
		timezone_offset, width, height, remote_url, upload_status,
		trash_timestamp, is_archived, is_favorite, is_locked,
		is_original_quality, latitude, longitude, location_name,
		location_id, is_edited, make, model, aperture, shutter_speed,
		iso, focal_length, duration, capture_frame_rate, encoded_frame_rate,
		is_micro_video, micro_video_width, micro_video_height, sha1_hash
	) VALUES (
		?, ?, ?, ?, ?, ?,
		?, ?, ?, ?,
		?, ?, ?,
		?, ?, ?, ?, ?,
		?, ?, ?, ?,
		?, ?, ?, ?,
		?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?,
		?, ?, ?, ?
	) ON CONFLICT(media_key) DO UPDATE SET
		file_name=excluded.file_name,
		dedup_key=excluded.dedup_key,
		is_canonical=excluded.is_canonical,
		type=excluded.type,
		caption=excluded.caption,
		collection_id=excluded.collection_id,
		size_bytes=excluded.size_bytes,
		quota_charged_bytes=excluded.quota_charged_bytes,
		origin=excluded.origin,
		content_version=excluded.content_version,
		utc_timestamp=excluded.utc_timestamp,
		server_creation_timestamp=excluded.server_creation_timestamp,
		timezone_offset=excluded.timezone_offset,
		width=excluded.width,
		height=excluded.height,
		remote_url=excluded.remote_url,
		upload_status=excluded.upload_status,
		trash_timestamp=excluded.trash_timestamp,
		is_archived=excluded.is_archived,
		is_favorite=excluded.is_favorite,
		is_locked=excluded.is_locked,
		is_original_quality=excluded.is_original_quality,
		latitude=excluded.latitude,
		longitude=excluded.longitude,
		location_name=excluded.location_name,
		location_id=excluded.location_id,
		is_edited=excluded.is_edited,
		make=excluded.make,
		model=excluded.model,
		aperture=excluded.aperture,
		shutter_speed=excluded.shutter_speed,
		iso=excluded.iso,
		focal_length=excluded.focal_length,
		duration=excluded.duration,
		capture_frame_rate=excluded.capture_frame_rate,
		encoded_frame_rate=excluded.encoded_frame_rate,
		is_micro_video=excluded.is_micro_video,
		micro_video_width=excluded.micro_video_width,
		micro_video_height=excluded.micro_video_height,
		sha1_hash=excluded.sha1_hash
	`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for _, item := range items {
		_, err = stmt.Exec(
			item.MediaKey, item.FileName, item.DedupKey,
			boolToInt(item.IsCanonical), item.Type, item.Caption,
			item.CollectionID, item.SizeBytes, item.QuotaChargedBytes, item.Origin,
			item.ContentVersion, item.UTCTimestamp, item.ServerCreationTimestamp,
			item.TimezoneOffset, item.Width, item.Height, item.RemoteURL, item.UploadStatus,
			item.TrashTimestamp, boolToInt(item.IsArchived), boolToInt(item.IsFavorite),
			boolToInt(item.IsLocked), boolToInt(item.IsOriginalQuality),
			item.Latitude, item.Longitude, item.LocationName, item.LocationID,
			boolToInt(item.IsEdited), item.Make, item.Model,
			item.Aperture, item.ShutterSpeed, item.ISO, item.FocalLength,
			item.Duration, item.CaptureFrameRate, item.EncodedFrameRate,
			boolToInt(item.IsMicroVideo), item.MicroVideoWidth, item.MicroVideoHeight,
			item.SHA1Hash,
		)
		if err != nil {
			return fmt.Errorf("failed to upsert item %s: %w", item.MediaKey, err)
		}
	}

	return tx.Commit()
}

// DeleteItems removes media items by media key
func (c *Cache) DeleteItems(mediaKeys []string) error {
	if len(mediaKeys) == 0 {
		return nil
	}

	placeholders := make([]string, len(mediaKeys))
	args := make([]interface{}, len(mediaKeys))
	for i, key := range mediaKeys {
		placeholders[i] = "?"
		args[i] = key
	}

	query := fmt.Sprintf("DELETE FROM remote_media WHERE media_key IN (%s)",
		strings.Join(placeholders, ","))

	_, err := c.db.Exec(query, args...)
	return err
}

// GetByFileName retrieves a media item by filename
func (c *Cache) GetByFileName(fileName string) (*MediaItem, error) {
	row := c.db.QueryRow(`SELECT
		media_key, file_name, dedup_key, is_canonical, type, caption,
		collection_id, size_bytes, quota_charged_bytes, origin,
		content_version, utc_timestamp, server_creation_timestamp,
		timezone_offset, width, height, remote_url, upload_status,
		trash_timestamp, is_archived, is_favorite, is_locked,
		is_original_quality, latitude, longitude, location_name,
		location_id, is_edited, make, model, aperture, shutter_speed,
		iso, focal_length, duration, capture_frame_rate, encoded_frame_rate,
		is_micro_video, micro_video_width, micro_video_height, sha1_hash
	FROM remote_media WHERE file_name = ? LIMIT 1`, fileName)

	return scanMediaItem(row)
}

// ListAll returns all non-trashed media items
func (c *Cache) ListAll() ([]MediaItem, error) {
	rows, err := c.db.Query(`SELECT
		media_key, file_name, dedup_key, is_canonical, type, caption,
		collection_id, size_bytes, quota_charged_bytes, origin,
		content_version, utc_timestamp, server_creation_timestamp,
		timezone_offset, width, height, remote_url, upload_status,
		trash_timestamp, is_archived, is_favorite, is_locked,
		is_original_quality, latitude, longitude, location_name,
		location_id, is_edited, make, model, aperture, shutter_speed,
		iso, focal_length, duration, capture_frame_rate, encoded_frame_rate,
		is_micro_video, micro_video_width, micro_video_height, sha1_hash
	FROM remote_media WHERE trash_timestamp = 0 OR trash_timestamp IS NULL
	ORDER BY utc_timestamp DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var items []MediaItem
	for rows.Next() {
		item, err := scanMediaItemFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

// GetStateTokens returns (state_token, page_token)
func (c *Cache) GetStateTokens() (string, string, error) {
	var stateToken, pageToken string
	err := c.db.QueryRow("SELECT state_token, page_token FROM state WHERE id = 1").
		Scan(&stateToken, &pageToken)
	if err != nil {
		return "", "", err
	}
	return stateToken, pageToken, nil
}

// UpdateStateTokens updates state tokens. Empty string means don't update.
func (c *Cache) UpdateStateTokens(stateToken, pageToken string) error {
	if stateToken != "" && pageToken != "" {
		_, err := c.db.Exec("UPDATE state SET state_token = ?, page_token = ? WHERE id = 1",
			stateToken, pageToken)
		return err
	}
	if stateToken != "" {
		_, err := c.db.Exec("UPDATE state SET state_token = ? WHERE id = 1", stateToken)
		return err
	}
	if pageToken != "" {
		_, err := c.db.Exec("UPDATE state SET page_token = ? WHERE id = 1", pageToken)
		return err
	}
	// If both empty, update page_token to empty
	_, err := c.db.Exec("UPDATE state SET page_token = ? WHERE id = 1", pageToken)
	return err
}

// GetInitState returns whether initial sync is complete
func (c *Cache) GetInitState() (bool, error) {
	var initComplete int
	err := c.db.QueryRow("SELECT init_complete FROM state WHERE id = 1").Scan(&initComplete)
	if err != nil {
		return false, err
	}
	return initComplete != 0, nil
}

// SetInitState sets the init state
func (c *Cache) SetInitState(complete bool) error {
	v := 0
	if complete {
		v = 1
	}
	_, err := c.db.Exec("UPDATE state SET init_complete = ? WHERE id = 1", v)
	return err
}

// GetLastSyncTime returns the last sync time as unix timestamp
func (c *Cache) GetLastSyncTime() (int64, error) {
	var ts int64
	err := c.db.QueryRow("SELECT last_sync_time FROM state WHERE id = 1").Scan(&ts)
	if err != nil {
		return 0, err
	}
	return ts, nil
}

// SetLastSyncTime sets the last sync time as unix timestamp
func (c *Cache) SetLastSyncTime(ts int64) error {
	_, err := c.db.Exec("UPDATE state SET last_sync_time = ? WHERE id = 1", ts)
	return err
}

// scanMediaItem scans a row into a MediaItem
func scanMediaItem(row *sql.Row) (*MediaItem, error) {
	var item MediaItem
	var isCanonical, isArchived, isFavorite, isLocked, isOriginalQuality, isEdited, isMicroVideo int
	var caption, collectionID, origin, remoteURL, locationName, locationID, cameraMake, cameraModel, sha1Hash sql.NullString
	var lat, lon, aperture, shutterSpeed, focalLength, captureFrameRate, encodedFrameRate sql.NullFloat64
	var timezoneOffset, width, height, uploadStatus, trashTimestamp, iso, duration, microW, microH sql.NullInt64

	err := row.Scan(
		&item.MediaKey, &item.FileName, &item.DedupKey,
		&isCanonical, &item.Type, &caption,
		&collectionID, &item.SizeBytes, &item.QuotaChargedBytes, &origin,
		&item.ContentVersion, &item.UTCTimestamp, &item.ServerCreationTimestamp,
		&timezoneOffset, &width, &height, &remoteURL, &uploadStatus,
		&trashTimestamp, &isArchived, &isFavorite,
		&isLocked, &isOriginalQuality,
		&lat, &lon, &locationName, &locationID,
		&isEdited, &cameraMake, &cameraModel,
		&aperture, &shutterSpeed, &iso, &focalLength,
		&duration, &captureFrameRate, &encodedFrameRate,
		&isMicroVideo, &microW, &microH, &sha1Hash,
	)
	if err != nil {
		return nil, err
	}

	item.IsCanonical = isCanonical != 0
	item.IsArchived = isArchived != 0
	item.IsFavorite = isFavorite != 0
	item.IsLocked = isLocked != 0
	item.IsOriginalQuality = isOriginalQuality != 0
	item.IsEdited = isEdited != 0
	item.IsMicroVideo = isMicroVideo != 0
	item.Caption = nullStr(caption)
	item.CollectionID = nullStr(collectionID)
	item.Origin = nullStr(origin)
	item.RemoteURL = nullStr(remoteURL)
	item.LocationName = nullStr(locationName)
	item.LocationID = nullStr(locationID)
	item.Make = nullStr(cameraMake)
	item.Model = nullStr(cameraModel)
	item.TimezoneOffset = nullInt(timezoneOffset)
	item.Width = nullInt(width)
	item.Height = nullInt(height)
	item.UploadStatus = nullInt(uploadStatus)
	item.TrashTimestamp = nullInt(trashTimestamp)
	item.ISO = nullInt(iso)
	item.Duration = nullInt(duration)
	item.MicroVideoWidth = nullInt(microW)
	item.MicroVideoHeight = nullInt(microH)
	item.Latitude = nullFloat(lat)
	item.Longitude = nullFloat(lon)
	item.Aperture = nullFloat(aperture)
	item.ShutterSpeed = nullFloat(shutterSpeed)
	item.FocalLength = nullFloat(focalLength)
	item.CaptureFrameRate = nullFloat(captureFrameRate)
	item.EncodedFrameRate = nullFloat(encodedFrameRate)
	item.SHA1Hash = nullStr(sha1Hash)

	return &item, nil
}

// scanMediaItemFromRows scans from sql.Rows
func scanMediaItemFromRows(rows *sql.Rows) (*MediaItem, error) {
	var item MediaItem
	var isCanonical, isArchived, isFavorite, isLocked, isOriginalQuality, isEdited, isMicroVideo int
	var caption, collectionID, origin, remoteURL, locationName, locationID, cameraMake, cameraModel, sha1Hash sql.NullString
	var lat, lon, aperture, shutterSpeed, focalLength, captureFrameRate, encodedFrameRate sql.NullFloat64
	var timezoneOffset, width, height, uploadStatus, trashTimestamp, iso, duration, microW, microH sql.NullInt64

	err := rows.Scan(
		&item.MediaKey, &item.FileName, &item.DedupKey,
		&isCanonical, &item.Type, &caption,
		&collectionID, &item.SizeBytes, &item.QuotaChargedBytes, &origin,
		&item.ContentVersion, &item.UTCTimestamp, &item.ServerCreationTimestamp,
		&timezoneOffset, &width, &height, &remoteURL, &uploadStatus,
		&trashTimestamp, &isArchived, &isFavorite,
		&isLocked, &isOriginalQuality,
		&lat, &lon, &locationName, &locationID,
		&isEdited, &cameraMake, &cameraModel,
		&aperture, &shutterSpeed, &iso, &focalLength,
		&duration, &captureFrameRate, &encodedFrameRate,
		&isMicroVideo, &microW, &microH, &sha1Hash,
	)
	if err != nil {
		return nil, err
	}

	item.IsCanonical = isCanonical != 0
	item.IsArchived = isArchived != 0
	item.IsFavorite = isFavorite != 0
	item.IsLocked = isLocked != 0
	item.IsOriginalQuality = isOriginalQuality != 0
	item.IsEdited = isEdited != 0
	item.IsMicroVideo = isMicroVideo != 0
	item.Caption = nullStr(caption)
	item.CollectionID = nullStr(collectionID)
	item.Origin = nullStr(origin)
	item.RemoteURL = nullStr(remoteURL)
	item.LocationName = nullStr(locationName)
	item.LocationID = nullStr(locationID)
	item.Make = nullStr(cameraMake)
	item.Model = nullStr(cameraModel)
	item.TimezoneOffset = nullInt(timezoneOffset)
	item.Width = nullInt(width)
	item.Height = nullInt(height)
	item.UploadStatus = nullInt(uploadStatus)
	item.TrashTimestamp = nullInt(trashTimestamp)
	item.ISO = nullInt(iso)
	item.Duration = nullInt(duration)
	item.MicroVideoWidth = nullInt(microW)
	item.MicroVideoHeight = nullInt(microH)
	item.Latitude = nullFloat(lat)
	item.Longitude = nullFloat(lon)
	item.Aperture = nullFloat(aperture)
	item.ShutterSpeed = nullFloat(shutterSpeed)
	item.FocalLength = nullFloat(focalLength)
	item.CaptureFrameRate = nullFloat(captureFrameRate)
	item.EncodedFrameRate = nullFloat(encodedFrameRate)
	item.SHA1Hash = nullStr(sha1Hash)

	return &item, nil
}

// Helper functions
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func nullInt(ni sql.NullInt64) int64 {
	if ni.Valid {
		return ni.Int64
	}
	return 0
}

func nullFloat(nf sql.NullFloat64) float64 {
	if nf.Valid {
		return nf.Float64
	}
	return 0
}

// defaultCachePath returns the default cache path for a given remote name.
// It uses rclone's cache directory (--cache-dir) with a backend-specific subdirectory.
// The resulting path is: <cache-dir>/gphotosmobile/<remoteName>.db
func defaultCachePath(remoteName string) string {
	dir := filepath.Join(config.GetCacheDir(), "gphotosmobile")
	return filepath.Join(dir, remoteName+".db")
}
