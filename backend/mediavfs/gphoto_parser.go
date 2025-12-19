package mediavfs

import (
	"database/sql"
	"encoding/base64"
	"fmt"

	"github.com/rclone/rclone/fs"
)

// MediaItem represents a Google Photos media item with all metadata
type MediaItem struct {
	MediaKey                 string
	Caption                  sql.NullString
	FileName                 string
	DedupKey                 string
	IsCanonical              bool
	Type                     int64
	CollectionID             string
	SizeBytes                int64
	TimezoneOffset           int64
	UTCTimestamp             int64
	ServerCreationTimestamp  int64
	UploadStatus             int64
	QuotaChargedBytes        int64
	Origin                   string
	ContentVersion           int64
	TrashTimestamp           int64
	IsArchived               bool
	IsFavorite               bool
	IsLocked                 bool
	IsOriginalQuality        bool
	Latitude                 sql.NullFloat64
	Longitude                sql.NullFloat64
	LocationName             sql.NullString
	LocationID               sql.NullString
	IsEdited                 bool
	RemoteURL                sql.NullString
	Width                    sql.NullInt64
	Height                   sql.NullInt64
	Make                     sql.NullString
	Model                    sql.NullString
	Aperture                 sql.NullFloat64
	ShutterSpeed             sql.NullFloat64
	ISO                      sql.NullInt64
	FocalLength              sql.NullFloat64
	Duration                 sql.NullInt64
	CaptureFrameRate         sql.NullFloat64
	EncodedFrameRate         sql.NullFloat64
	IsMicroVideo             bool
	MicroVideoWidth          sql.NullInt64
	MicroVideoHeight         sql.NullInt64
	ParsedName               sql.NullString
	DownloadURL              sql.NullString
	ThumbnailURL             sql.NullString
}

// Helper function to get map keys for debugging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// asMap tries to interpret a value as a map (decoding bytes if needed)
func asMap(v interface{}) (map[string]interface{}, bool) {
	// Already a map
	if m, ok := v.(map[string]interface{}); ok {
		return m, true
	}
	// Raw bytes - try to decode as protobuf message
	if b, ok := v.([]byte); ok {
		if m, err := DecodeDynamicMessage(b); err == nil && len(m) > 0 {
			return m, true
		}
	}
	return nil, false
}

// asString tries to interpret a value as a string
func asString(v interface{}) (string, bool) {
	// Already a string
	if s, ok := v.(string); ok {
		return s, true
	}
	// Raw bytes - convert to string
	if b, ok := v.([]byte); ok {
		return string(b), true
	}
	return "", false
}

// asBytes gets raw bytes from a value
func asBytes(v interface{}) ([]byte, bool) {
	if b, ok := v.([]byte); ok {
		return b, true
	}
	if s, ok := v.(string); ok {
		return []byte(s), true
	}
	return nil, false
}

// asUint64 tries to interpret a value as uint64
func asUint64(v interface{}) (uint64, bool) {
	if u, ok := v.(uint64); ok {
		return u, true
	}
	return 0, false
}

// ParseDbUpdate parses the library state response from Google Photos
func ParseDbUpdate(data map[string]interface{}) (stateToken string, nextPageToken string, mediaItems []MediaItem, mediaKeysToDelete []string, err error) {
	// Get top-level field 1
	field1, ok := asMap(data["1"])
	if !ok {
		// Field 1 might be empty string when sync is complete (no more data)
		if _, isEmpty := data["1"].(string); isEmpty {
			return "", "", nil, nil, nil
		}
		// Or empty bytes
		if b, ok := data["1"].([]byte); ok && len(b) == 0 {
			return "", "", nil, nil, nil
		}
		return "", "", nil, nil, fmt.Errorf("invalid response structure: field 1 is not a map (type: %T)", data["1"])
	}

	// Extract tokens
	if token, ok := asString(field1["6"]); ok {
		stateToken = token
	}
	if token, ok := asString(field1["1"]); ok {
		nextPageToken = token
	}

	// Parse media items (field 2)
	if field2Data, ok := field1["2"]; ok {
		mediaItems, err = parseMediaItems(field2Data)
		if err != nil {
			return "", "", nil, nil, fmt.Errorf("failed to parse media items: %w", err)
		}
	}

	// Parse deletions (field 9)
	if field9Data, ok := field1["9"]; ok {
		mediaKeysToDelete = parseDeletions(field9Data)
	}

	return stateToken, nextPageToken, mediaItems, mediaKeysToDelete, nil
}

func parseMediaItems(data interface{}) ([]MediaItem, error) {
	var items []MediaItem
	var skipped int

	// Handle both single item (dict/bytes) and multiple items (list)
	switch v := data.(type) {
	case map[string]interface{}:
		item, err := parseMediaItem(v)
		if err != nil {
			fs.Infof(nil, "mediavfs: Skipping media item due to error: %v", err)
			skipped++
		} else {
			items = append(items, item)
		}
	case []byte:
		// Single item as bytes - decode first
		if itemMap, ok := asMap(v); ok {
			item, err := parseMediaItem(itemMap)
			if err != nil {
				fs.Infof(nil, "mediavfs: Skipping media item due to error: %v", err)
				skipped++
			} else {
				items = append(items, item)
			}
		}
	case []interface{}:
		for i, itemData := range v {
			// Try to get as map (handles both map and bytes)
			if itemMap, ok := asMap(itemData); ok {
				item, err := parseMediaItem(itemMap)
				if err != nil {
					fs.Infof(nil, "mediavfs: Skipping media item #%d due to error: %v", i, err)
					skipped++
				} else {
					items = append(items, item)
				}
			}
		}
	}

	if skipped > 0 {
		fs.Infof(nil, "mediavfs: Skipped %d media items with missing required fields", skipped)
	}

	return items, nil
}

// extractFileName extracts the filename from field 2->4
// Field 2->4 should be raw bytes containing the filename string
// In rare cases it might be a nested message with field 14 containing the filename
func extractFileName(field24 interface{}, mediaKey string) (string, error) {
	// Case 1: Already a string (rare but possible)
	if fileName, ok := field24.(string); ok && fileName != "" {
		return fileName, nil
	}

	// Case 2: Raw bytes - this is the filename directly (most common case)
	// The filename bytes should NOT be decoded as protobuf - they are just UTF-8 text
	if fileBytes, ok := field24.([]byte); ok && len(fileBytes) > 0 {
		// Convert bytes directly to string - this is the filename
		return string(fileBytes), nil
	}

	// Case 3: Repeated field - field 4 might appear multiple times
	// Take the first element that looks like a filename
	if arr, ok := field24.([]interface{}); ok && len(arr) > 0 {
		for _, elem := range arr {
			// Try as bytes
			if fileBytes, ok := elem.([]byte); ok && len(fileBytes) > 0 {
				return string(fileBytes), nil
			}
			// Try as string
			if fileName, ok := elem.(string); ok && fileName != "" {
				return fileName, nil
			}
		}
	}

	// Case 4: Already decoded map (shouldn't happen normally, but handle gracefully)
	// This means field 2->4's bytes were incorrectly decoded as nested protobuf
	// Try to find a string field that could be the filename
	if field24Map, ok := field24.(map[string]interface{}); ok {
		fs.Infof(nil, "mediavfs: field2[4] for %s is a map (keys=%v)", mediaKey, getMapKeys(field24Map))

		// Try field 14 first (sometimes used for filename)
		if field14, exists := field24Map["14"]; exists {
			fs.Infof(nil, "mediavfs: field2[4][14] for %s: type=%T", mediaKey, field14)
			// Try as direct string/bytes
			if fileName, ok := asString(field14); ok && fileName != "" {
				fs.Infof(nil, "mediavfs: Found filename in field2[4][14] for %s: %s", mediaKey, fileName)
				return fileName, nil
			}
			// Try as nested map (field 14 might contain nested structure)
			if nestedMap, ok := field14.(map[string]interface{}); ok {
				fs.Infof(nil, "mediavfs: field2[4][14] is nested map with keys=%v", getMapKeys(nestedMap))
				// Try common filename field numbers in nested structure
				for _, key := range []string{"1", "2", "6", "14"} {
					if val, exists := nestedMap[key]; exists {
						if fileName, ok := asString(val); ok && fileName != "" {
							fs.Infof(nil, "mediavfs: Found filename in field2[4][14][%s]: %s", key, fileName)
							return fileName, nil
						}
					}
				}
			}
		}

		// Try field 12
		if field12, exists := field24Map["12"]; exists {
			fs.Infof(nil, "mediavfs: field2[4][12] for %s: type=%T", mediaKey, field12)
			if fileName, ok := asString(field12); ok && fileName != "" {
				fs.Infof(nil, "mediavfs: Found filename in field2[4][12] for %s: %s", mediaKey, fileName)
				return fileName, nil
			}
		}

		// Try field 6 - this is often where the filename is
		if field6, exists := field24Map["6"]; exists {
			fs.Infof(nil, "mediavfs: field2[4][6] for %s: type=%T", mediaKey, field6)
			if fileName, ok := asString(field6); ok && fileName != "" {
				fs.Infof(nil, "mediavfs: Found filename in field2[4][6] for %s: %s", mediaKey, fileName)
				return fileName, nil
			}
			// Try as array - field 6 might be repeated, look for the longest string
			if arr, ok := field6.([]interface{}); ok && len(arr) > 0 {
				fs.Infof(nil, "mediavfs: field2[4][6] is array with %d elements", len(arr))
				var longestFileName string
				for i, elem := range arr {
					if fileName, ok := asString(elem); ok && fileName != "" {
						fs.Infof(nil, "mediavfs: field2[4][6][%d] = %q (len=%d)", i, fileName, len(fileName))
						if len(fileName) > len(longestFileName) {
							longestFileName = fileName
						}
					}
				}
				if longestFileName != "" {
					fs.Infof(nil, "mediavfs: Using longest filename from field2[4][6]: %s", longestFileName)
					return longestFileName, nil
				}
			}
			// Try as nested map
			if nestedMap, ok := field6.(map[string]interface{}); ok {
				fs.Infof(nil, "mediavfs: field2[4][6] is nested map with keys=%v", getMapKeys(nestedMap))
				for _, key := range []string{"1", "2", "6", "14"} {
					if val, exists := nestedMap[key]; exists {
						if fileName, ok := asString(val); ok && fileName != "" {
							fs.Infof(nil, "mediavfs: Found filename in field2[4][6][%s]: %s", key, fileName)
							return fileName, nil
						}
					}
				}
			}
		}

		// Try any bytes field and convert to string (might be the original filename bytes)
		for k, v := range field24Map {
			if fileBytes, ok := v.([]byte); ok && len(fileBytes) > 0 {
				fileName := string(fileBytes)
				if isProbablyString(fileBytes) {
					fs.Infof(nil, "mediavfs: Using field2[4][%s] bytes as filename for %s: %s", k, mediaKey, fileName)
					return fileName, nil
				}
			}
		}
		// Try any string field
		for k, v := range field24Map {
			if strVal, ok := v.(string); ok && strVal != "" {
				fs.Infof(nil, "mediavfs: Using field2[4][%s] string as filename for %s: %s", k, mediaKey, strVal)
				return strVal, nil
			}
		}
	}

	// Case 5: nil or empty - field 4 doesn't exist
	if field24 == nil {
		generatedName := fmt.Sprintf("%s.unknown", mediaKey)
		fs.Infof(nil, "mediavfs: field2[4] is nil for %s, generating filename: %s", mediaKey, generatedName)
		return generatedName, nil
	}

	// No filename found, generate one
	generatedName := fmt.Sprintf("%s.unknown", mediaKey)
	fs.Infof(nil, "mediavfs: Generating filename for %s (field2[4] type=%T): %s", mediaKey, field24, generatedName)
	return generatedName, nil
}

func parseMediaItem(d map[string]interface{}) (MediaItem, error) {
	item := MediaItem{}

	// Field 1: media_key (REQUIRED - must exist and be a string, but can be empty)
	mediaKey, ok := asString(d["1"])
	if !ok {
		return item, fmt.Errorf("missing required field: media_key (field 1)")
	}
	item.MediaKey = mediaKey

	// Field 2: metadata (REQUIRED)
	// Log what type d["2"] is before decoding
	fs.Infof(nil, "mediavfs: parseMediaItem %s: d[2] type=%T", mediaKey, d["2"])
	field2, ok := asMap(d["2"])
	if !ok {
		return item, fmt.Errorf("missing required field 2 in media item %s", item.MediaKey)
	}
	// Log what type field2["4"] is after decoding
	fs.Infof(nil, "mediavfs: parseMediaItem %s: field2[4] type=%T", mediaKey, field2["4"])

	// Field 2->4: file_name (REQUIRED - can be string or nested map with field 14)
	fileName, err := extractFileName(field2["4"], item.MediaKey)
	if err != nil {
		return item, err
	}
	item.FileName = fileName

	// Field 5: type info (optional - default to 0)
	if field5, ok := asMap(d["5"]); ok {
		if typeVal, ok := asUint64(field5["1"]); ok {
			item.Type = int64(typeVal)
		}
	}

	// Debug: list of media_keys with known empty dedup_key issues
	debugMediaKeys := map[string]bool{
		"AF1QipNUQyoob3Aw2yZahYwG5OX7ipb_MBf_HC8g-0So": true,
		"AF1QipP4mj-s7sitRKGb2yenPIr66a3PH5QzaRnlOPpK": true,
		"AF1QipOvzOeMUrrtKPl74aTKzrLrP6xWIUKFDkhhKqd-": true,
		"AF1QipMmQLnWK9G18i19a9ZSkYye5tcUzNceh90AL4bq": true,
		"AF1QipMxFMhyRa225fsbEE1s5OSGpIIHGr5VDqwFhyBo": true,
	}
	debugThis := debugMediaKeys[item.MediaKey]

	// Parse dedup_key from field 2->21
	if field21, ok := asMap(field2["21"]); ok {
		if debugThis {
			fs.Infof(nil, "mediavfs: field2[21] exists for %s, keys=%v", item.MediaKey, getMapKeys(field21))
		}
		for key, val := range field21 {
			if debugThis {
				fs.Infof(nil, "mediavfs: field2[21][%q] = type=%T, value=%v", key, val, val)
			}
			if key[0] == '1' {
				// Try to get as bytes (most common for dedup_key)
				if dedupBytes, ok := asBytes(val); ok {
					item.DedupKey = urlsafeBase64(base64.StdEncoding.EncodeToString(dedupBytes))
					if debugThis {
						fs.Infof(nil, "mediavfs: Got dedup_key from field2[21][%q] = %q", key, item.DedupKey)
					}
					break
				}
				if debugThis {
					fs.Infof(nil, "mediavfs: field2[21][%q] value cannot be converted to bytes, type=%T", key, val)
				}
			}
		}
	} else if debugThis {
		fs.Infof(nil, "mediavfs: field2[21] does NOT exist for %s, field2[21]=%v", item.MediaKey, field2["21"])
	}

	// Fallback: try to get dedup_key from field 2->13->1
	if item.DedupKey == "" {
		if debugThis {
			fs.Infof(nil, "mediavfs: dedup_key still empty for %s, trying fallback field2[13]", item.MediaKey)
		}
		if field13, ok := asMap(field2["13"]); ok {
			if debugThis {
				fs.Infof(nil, "mediavfs: field2[13] exists, field2[13][1]=%v (type=%T)", field13["1"], field13["1"])
			}
			// Try to get as bytes
			if hashBytes, ok := asBytes(field13["1"]); ok {
				item.DedupKey = urlsafeBase64(base64.StdEncoding.EncodeToString(hashBytes))
				if debugThis {
					fs.Infof(nil, "mediavfs: Got dedup_key from field2[13][1] = %q", item.DedupKey)
				}
			} else if debugThis {
				fs.Infof(nil, "mediavfs: field2[13][1] cannot be converted to bytes, type=%T", field13["1"])
			}
		} else if debugThis {
			fs.Infof(nil, "mediavfs: field2[13] does NOT exist or not a map for %s", item.MediaKey)
		}
	}

	// Final fallback: if dedup_key is still empty, use media_key as dedup_key
	// This handles cases where field2[21] and field2[13] are nested maps instead of bytes/strings
	if item.DedupKey == "" {
		if debugThis {
			fs.Infof(nil, "mediavfs: WARNING dedup_key is EMPTY for %s (file: %s)", item.MediaKey, item.FileName)
			fs.Infof(nil, "mediavfs: WARNING field2[21]=%v", field2["21"])
			fs.Infof(nil, "mediavfs: WARNING field2[13]=%v", field2["13"])
		}
		// Use media_key as fallback dedup_key
		item.DedupKey = item.MediaKey
		fs.Infof(nil, "mediavfs: Using media_key as dedup_key for %s", item.MediaKey)
	}

	// Field 2->1->1: collection_id (optional - default to empty)
	if field2_1, ok := asMap(field2["1"]); ok {
		if val, ok := asString(field2_1["1"]); ok {
			item.CollectionID = val
		}
	}

	// Field 2->10: size_bytes
	if val, ok := asUint64(field2["10"]); ok {
		item.SizeBytes = int64(val)
	}

	// Field 2->8: timezone_offset
	if val, ok := asUint64(field2["8"]); ok {
		item.TimezoneOffset = int64(val)
	}

	// Field 2->7: utc_timestamp
	if val, ok := asUint64(field2["7"]); ok {
		item.UTCTimestamp = int64(val)
	}

	// Field 2->9: server_creation_timestamp
	if val, ok := asUint64(field2["9"]); ok {
		item.ServerCreationTimestamp = int64(val)
	}

	// Field 2->11: upload_status
	if val, ok := asUint64(field2["11"]); ok {
		item.UploadStatus = int64(val)
	}

	// Field 2->35: quota info
	if field35, ok := asMap(field2["35"]); ok {
		if val, ok := asUint64(field35["2"]); ok {
			item.QuotaChargedBytes = int64(val)
		}
		if val, ok := asUint64(field35["3"]); ok {
			item.IsOriginalQuality = val == 2
		}
	}

	// Field 2->30->1: origin
	if field30, ok := asMap(field2["30"]); ok {
		if val, ok := asUint64(field30["1"]); ok {
			originMap := map[uint64]string{
				1: "self",
				3: "partner",
				4: "shared",
			}
			if origin, exists := originMap[val]; exists {
				item.Origin = origin
			}
		}
	}

	// Field 2->26: content_version
	if val, ok := asUint64(field2["26"]); ok {
		item.ContentVersion = int64(val)
	}

	// Field 2->16->3: trash_timestamp
	if field16, ok := asMap(field2["16"]); ok {
		if val, ok := asUint64(field16["3"]); ok {
			item.TrashTimestamp = int64(val)
		}
	}

	// Field 2->29->1: is_archived
	if field29, ok := asMap(field2["29"]); ok {
		if val, ok := asUint64(field29["1"]); ok {
			item.IsArchived = val == 1
		}
	}

	// Field 2->31->1: is_favorite
	if field31, ok := asMap(field2["31"]); ok {
		if val, ok := asUint64(field31["1"]); ok {
			item.IsFavorite = val == 1
		}
	}

	// Field 2->39->1: is_locked
	if field39, ok := asMap(field2["39"]); ok {
		if val, ok := asUint64(field39["1"]); ok {
			item.IsLocked = val == 1
		}
	}

	// Field 2->5: check is_canonical (property 27 means not canonical)
	item.IsCanonical = true
	if field5List, ok := field2["5"].([]interface{}); ok {
		for _, prop := range field5List {
			if propMap, ok := asMap(prop); ok {
				if val, ok := asUint64(propMap["1"]); ok && val == 27 {
					item.IsCanonical = false
					break
				}
			}
		}
	}

	// Field 5: type and media-specific data
	if field5, ok := asMap(d["5"]); ok {
		if val, ok := asUint64(field5["1"]); ok {
			item.Type = int64(val)
		}

		// Field 5->2: photo data
		if field5_2, ok := asMap(field5["2"]); ok {
			item.IsEdited = false
			if _, hasField4 := field5_2["4"]; hasField4 {
				item.IsEdited = true
			}

			if field5_2_1, ok := asMap(field5_2["1"]); ok {
				if url, ok := asString(field5_2_1["1"]); ok {
					item.RemoteURL = sql.NullString{String: url, Valid: true}
				}

				// Field 5->2->1->9: dimensions and EXIF
				if field9, ok := asMap(field5_2_1["9"]); ok {
					if val, ok := asUint64(field9["1"]); ok {
						item.Width = sql.NullInt64{Int64: int64(val), Valid: true}
					}
					if val, ok := asUint64(field9["2"]); ok {
						item.Height = sql.NullInt64{Int64: int64(val), Valid: true}
					}

					// Field 5->2->1->9->5: EXIF data
					if field9_5, ok := asMap(field9["5"]); ok {
						if val, ok := asString(field9_5["1"]); ok {
							item.Make = sql.NullString{String: val, Valid: true}
						}
						if val, ok := asString(field9_5["2"]); ok {
							item.Model = sql.NullString{String: val, Valid: true}
						}
					}
				}
			}
		}

		// Field 5->3: video data
		if field5_3, ok := asMap(field5["3"]); ok {
			if field5_3_2, ok := asMap(field5_3["2"]); ok {
				if url, ok := asString(field5_3_2["1"]); ok {
					item.RemoteURL = sql.NullString{String: url, Valid: true}
				}
			}

			if field5_3_4, ok := asMap(field5_3["4"]); ok {
				if val, ok := asUint64(field5_3_4["1"]); ok {
					item.Duration = sql.NullInt64{Int64: int64(val), Valid: true}
				}
				if val, ok := asUint64(field5_3_4["4"]); ok {
					item.Width = sql.NullInt64{Int64: int64(val), Valid: true}
				}
				if val, ok := asUint64(field5_3_4["5"]); ok {
					item.Height = sql.NullInt64{Int64: int64(val), Valid: true}
				}
			}
		}

		// Field 5->5: micro video
		if field5_5, ok := asMap(field5["5"]); ok {
			if field5_5_2, ok := asMap(field5_5["2"]); ok {
				if field5_5_2_4, ok := asMap(field5_5_2["4"]); ok {
					item.IsMicroVideo = true
					if val, ok := asUint64(field5_5_2_4["1"]); ok {
						item.Duration = sql.NullInt64{Int64: int64(val), Valid: true}
					}
					if val, ok := asUint64(field5_5_2_4["4"]); ok {
						item.MicroVideoWidth = sql.NullInt64{Int64: int64(val), Valid: true}
					}
					if val, ok := asUint64(field5_5_2_4["5"]); ok {
						item.MicroVideoHeight = sql.NullInt64{Int64: int64(val), Valid: true}
					}
				}
			}
		}
	}

	// Field 17: location data
	if field17, ok := asMap(d["17"]); ok {
		if field17_1, ok := asMap(field17["1"]); ok {
			// Note: These are fixed32 values that need conversion
			if val, ok := field17_1["1"].(uint32); ok {
				item.Latitude = sql.NullFloat64{Float64: fixed32ToFloat(val), Valid: true}
			}
			if val, ok := field17_1["2"].(uint32); ok {
				item.Longitude = sql.NullFloat64{Float64: fixed32ToFloat(val), Valid: true}
			}
		}

		if field17_5, ok := asMap(field17["5"]); ok {
			if field17_5_2, ok := asMap(field17_5["2"]); ok {
				if val, ok := asString(field17_5_2["1"]); ok {
					item.LocationName = sql.NullString{String: val, Valid: true}
				}
			}
			if val, ok := asString(field17_5["3"]); ok {
				item.LocationID = sql.NullString{String: val, Valid: true}
			}
		}
	}

	return item, nil
}

func parseDeletions(data interface{}) []string {
	var mediaKeys []string

	// Handle both single item (dict/bytes) and multiple items (list)
	switch v := data.(type) {
	case map[string]interface{}:
		if key := parseDeletionItem(v); key != "" {
			mediaKeys = append(mediaKeys, key)
		}
	case []byte:
		if itemMap, ok := asMap(v); ok {
			if key := parseDeletionItem(itemMap); key != "" {
				mediaKeys = append(mediaKeys, key)
			}
		}
	case []interface{}:
		for _, itemData := range v {
			if itemMap, ok := asMap(itemData); ok {
				if key := parseDeletionItem(itemMap); key != "" {
					mediaKeys = append(mediaKeys, key)
				}
			}
		}
	}

	return mediaKeys
}

func parseDeletionItem(d map[string]interface{}) string {
	// Field 1: deletion data
	field1, ok := asMap(d["1"])
	if !ok {
		return ""
	}

	// Field 1->1: type
	delType, ok := asUint64(field1["1"])
	if !ok {
		return ""
	}

	// Type 1 means media deletion
	if delType == 1 {
		if field1_2, ok := asMap(field1["2"]); ok {
			if mediaKey, ok := asString(field1_2["1"]); ok {
				return mediaKey
			}
		}
	}

	return ""
}

// urlsafeBase64 converts standard base64 to URL-safe base64
func urlsafeBase64(s string) string {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return s
	}
	return base64.URLEncoding.EncodeToString(decoded)
}

// fixed32ToFloat converts a fixed32 protobuf value to float64
func fixed32ToFloat(val uint32) float64 {
	// Interpret as IEEE 754 single-precision float
	bits := uint32(val)
	sign := float64(1)
	if bits&0x80000000 != 0 {
		sign = -1
	}
	exponent := int((bits >> 23) & 0xFF)
	mantissa := bits & 0x7FFFFF

	if exponent == 0 {
		return sign * float64(mantissa) * 1.1754943508222875e-38
	}
	return sign * (1 + float64(mantissa)/8388608.0) * float64(uint64(1)<<uint(exponent-127))
}
