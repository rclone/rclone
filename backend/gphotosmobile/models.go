package gphotosmobile

// MediaItem represents a media item in Google Photos (matching Python gpmc MediaItem)
type MediaItem struct {
	MediaKey                string  `db:"media_key"`
	FileName                string  `db:"file_name"`
	DedupKey                string  `db:"dedup_key"`
	IsCanonical             bool    `db:"is_canonical"`
	Type                    int64   `db:"type"` // 1=photo, 2=video
	Caption                 string  `db:"caption"`
	CollectionID            string  `db:"collection_id"`
	SizeBytes               int64   `db:"size_bytes"`
	QuotaChargedBytes       int64   `db:"quota_charged_bytes"`
	Origin                  string  `db:"origin"` // "self", "partner", "shared"
	ContentVersion          int64   `db:"content_version"`
	UTCTimestamp            int64   `db:"utc_timestamp"`
	ServerCreationTimestamp int64   `db:"server_creation_timestamp"`
	TimezoneOffset          int64   `db:"timezone_offset"`
	Width                   int64   `db:"width"`
	Height                  int64   `db:"height"`
	RemoteURL               string  `db:"remote_url"`
	UploadStatus            int64   `db:"upload_status"`
	TrashTimestamp          int64   `db:"trash_timestamp"`
	IsArchived              bool    `db:"is_archived"`
	IsFavorite              bool    `db:"is_favorite"`
	IsLocked                bool    `db:"is_locked"`
	IsOriginalQuality       bool    `db:"is_original_quality"`
	Latitude                float64 `db:"latitude"`
	Longitude               float64 `db:"longitude"`
	LocationName            string  `db:"location_name"`
	LocationID              string  `db:"location_id"`
	IsEdited                bool    `db:"is_edited"`
	Make                    string  `db:"make"`
	Model                   string  `db:"model"`
	Aperture                float64 `db:"aperture"`
	ShutterSpeed            float64 `db:"shutter_speed"`
	ISO                     int64   `db:"iso"`
	FocalLength             float64 `db:"focal_length"`
	Duration                int64   `db:"duration"`
	CaptureFrameRate        float64 `db:"capture_frame_rate"`
	EncodedFrameRate        float64 `db:"encoded_frame_rate"`
	IsMicroVideo            bool    `db:"is_micro_video"`
	MicroVideoWidth         int64   `db:"micro_video_width"`
	MicroVideoHeight        int64   `db:"micro_video_height"`
	SHA1Hash                string  `db:"sha1_hash"` // lowercase hex SHA1 hash of original file
}
