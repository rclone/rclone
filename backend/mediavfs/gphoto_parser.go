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

// ParseDbUpdate parses the library state response from Google Photos
func ParseDbUpdate(data map[string]interface{}) (stateToken string, nextPageToken string, mediaItems []MediaItem, mediaKeysToDelete []string, err error) {
	// Get top-level field 1
	field1, ok := data["1"].(map[string]interface{})
	if !ok {
		// Field 1 might be empty string when sync is complete (no more data)
		if _, isEmpty := data["1"].(string); isEmpty {
			return "", "", nil, nil, nil
		}
		return "", "", nil, nil, fmt.Errorf("invalid response structure: field 1 is not a map (type: %T)", data["1"])
	}

	// Extract tokens
	if token, ok := field1["6"].(string); ok {
		stateToken = token
	}
	if token, ok := field1["1"].(string); ok {
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

	// Handle both single item (dict) and multiple items (list)
	switch v := data.(type) {
	case map[string]interface{}:
		item, err := parseMediaItem(v)
		if err != nil {
			fs.Infof(nil, "mediavfs: Skipping media item due to error: %v", err)
			skipped++
		} else {
			items = append(items, item)
		}
	case []interface{}:
		for i, itemData := range v {
			if itemMap, ok := itemData.(map[string]interface{}); ok {
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
// Field 2->4 can be either:
//  1. A string (direct filename)
//  2. A map with field 14 containing the filename (as string or byte array)
//  3. A map with other fields - try to find filename in available fields
func extractFileName(field24 interface{}, mediaKey string) (string, error) {
	// Case 1: Direct string
	if fileName, ok := field24.(string); ok {
		return fileName, nil
	}

	// Case 2: Nested map with field 14
	if field24Map, ok := field24.(map[string]interface{}); ok {
		// Try to get field 14 from the nested map
		if field14, exists := field24Map["14"]; exists {
			// Field 14 can be a string or byte array
			if fileName, ok := field14.(string); ok {
				return fileName, nil
			}

			// Field 14 might be a byte array []interface{} that needs conversion
			if byteArray, ok := field14.([]interface{}); ok {
				// Convert []interface{} to string
				bytes := make([]byte, 0, len(byteArray))
				for _, b := range byteArray {
					switch v := b.(type) {
					case uint64:
						bytes = append(bytes, byte(v))
					case int64:
						bytes = append(bytes, byte(v))
					case string:
						// If it's already a string in the array, append its bytes
						bytes = append(bytes, []byte(v)...)
					}
				}
				return string(bytes), nil
			}
		}

		// If field 14 doesn't exist, log available fields and try alternatives
		fs.Infof(nil, "mediavfs: field 2->4 is map without field 14 for %s, available fields: %v", mediaKey, getMapKeys(field24Map))

		// Try other common fields that might contain filename
		// Check all string fields in the map as potential filenames
		for k, v := range field24Map {
			if strVal, ok := v.(string); ok && strVal != "" {
				fs.Infof(nil, "mediavfs: Using field 2->4[%s] as filename for %s: %s", k, mediaKey, strVal)
				return strVal, nil
			}
		}

		// No string field found, generate a filename from media_key
		generatedName := fmt.Sprintf("%s.unknown", mediaKey)
		fs.Infof(nil, "mediavfs: Generating filename for %s: %s", mediaKey, generatedName)
		return generatedName, nil
	}

	return "", fmt.Errorf("field 2->4 has unexpected type %T for media_key %s", field24, mediaKey)
}

func parseMediaItem(d map[string]interface{}) (MediaItem, error) {
	item := MediaItem{}

	// Field 1: media_key (REQUIRED - must exist and be a string, but can be empty)
	mediaKey, ok := d["1"].(string)
	if !ok {
		return item, fmt.Errorf("missing required field: media_key (field 1)")
	}
	item.MediaKey = mediaKey

	// Field 2: metadata (REQUIRED)
	field2, ok := d["2"].(map[string]interface{})
	if !ok {
		return item, fmt.Errorf("missing required field 2 in media item %s", item.MediaKey)
	}

	// Field 2->4: file_name (REQUIRED - can be string or nested map with field 14)
	fileName, err := extractFileName(field2["4"], item.MediaKey)
	if err != nil {
		return item, err
	}
	item.FileName = fileName

	// Field 5: type info (optional - default to 0)
	if field5, ok := d["5"].(map[string]interface{}); ok {
		if typeVal, ok := field5["1"].(uint64); ok {
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
	if field21, ok := field2["21"].(map[string]interface{}); ok {
		if debugThis {
			fs.Infof(nil, "mediavfs: field2[21] exists for %s, keys=%v", item.MediaKey, getMapKeys(field21))
		}
		for key, val := range field21 {
			if debugThis {
				fs.Infof(nil, "mediavfs: field2[21][%q] = type=%T, value=%v", key, val, val)
			}
			if key[0] == '1' {
				if dedupKey, ok := val.(string); ok {
					item.DedupKey = dedupKey
					if debugThis {
						fs.Infof(nil, "mediavfs: Got dedup_key from field2[21][%q] = %q", key, dedupKey)
					}
					break
				} else if debugThis {
					fs.Infof(nil, "mediavfs: field2[21][%q] value is NOT a string, type=%T", key, val)
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
		if field13, ok := field2["13"].(map[string]interface{}); ok {
			if debugThis {
				fs.Infof(nil, "mediavfs: field2[13] exists, field2[13][1]=%v (type=%T)", field13["1"], field13["1"])
			}
			// Try as []byte first
			if hashBytes, ok := field13["1"].([]byte); ok {
				item.DedupKey = urlsafeBase64(base64.StdEncoding.EncodeToString(hashBytes))
				if debugThis {
					fs.Infof(nil, "mediavfs: Got dedup_key from field2[13][1] as []byte = %q", item.DedupKey)
				}
			} else if hashStr, ok := field13["1"].(string); ok {
				// Handle string containing binary data
				item.DedupKey = urlsafeBase64(base64.StdEncoding.EncodeToString([]byte(hashStr)))
				if debugThis {
					fs.Infof(nil, "mediavfs: Got dedup_key from field2[13][1] as string = %q", item.DedupKey)
				}
			} else if hashInterface, ok := field13["1"].([]interface{}); ok {
				// Convert []interface{} to []byte
				hashBytes := make([]byte, len(hashInterface))
				for i, v := range hashInterface {
					switch val := v.(type) {
					case uint64:
						hashBytes[i] = byte(val)
					case int64:
						hashBytes[i] = byte(val)
					}
				}
				item.DedupKey = urlsafeBase64(base64.StdEncoding.EncodeToString(hashBytes))
				if debugThis {
					fs.Infof(nil, "mediavfs: Got dedup_key from field2[13][1] as []interface{} = %q", item.DedupKey)
				}
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
	if field2_1, ok := field2["1"].(map[string]interface{}); ok {
		if val, ok := field2_1["1"].(string); ok {
			item.CollectionID = val
		}
	}

	// Field 2->10: size_bytes
	if val, ok := field2["10"].(uint64); ok {
		item.SizeBytes = int64(val)
	}

	// Field 2->8: timezone_offset
	if val, ok := field2["8"].(uint64); ok {
		item.TimezoneOffset = int64(val)
	}

	// Field 2->7: utc_timestamp
	if val, ok := field2["7"].(uint64); ok {
		item.UTCTimestamp = int64(val)
	}

	// Field 2->9: server_creation_timestamp
	if val, ok := field2["9"].(uint64); ok {
		item.ServerCreationTimestamp = int64(val)
	}

	// Field 2->11: upload_status
	if val, ok := field2["11"].(uint64); ok {
		item.UploadStatus = int64(val)
	}

	// Field 2->35: quota info
	if field35, ok := field2["35"].(map[string]interface{}); ok {
		if val, ok := field35["2"].(uint64); ok {
			item.QuotaChargedBytes = int64(val)
		}
		if val, ok := field35["3"].(uint64); ok {
			item.IsOriginalQuality = val == 2
		}
	}

	// Field 2->30->1: origin
	if field30, ok := field2["30"].(map[string]interface{}); ok {
		if val, ok := field30["1"].(uint64); ok {
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
	if val, ok := field2["26"].(uint64); ok {
		item.ContentVersion = int64(val)
	}

	// Field 2->16->3: trash_timestamp
	if field16, ok := field2["16"].(map[string]interface{}); ok {
		if val, ok := field16["3"].(uint64); ok {
			item.TrashTimestamp = int64(val)
		}
	}

	// Field 2->29->1: is_archived
	if field29, ok := field2["29"].(map[string]interface{}); ok {
		if val, ok := field29["1"].(uint64); ok {
			item.IsArchived = val == 1
		}
	}

	// Field 2->31->1: is_favorite
	if field31, ok := field2["31"].(map[string]interface{}); ok {
		if val, ok := field31["1"].(uint64); ok {
			item.IsFavorite = val == 1
		}
	}

	// Field 2->39->1: is_locked
	if field39, ok := field2["39"].(map[string]interface{}); ok {
		if val, ok := field39["1"].(uint64); ok {
			item.IsLocked = val == 1
		}
	}

	// Field 2->5: check is_canonical (property 27 means not canonical)
	item.IsCanonical = true
	if field5, ok := field2["5"].([]interface{}); ok {
		for _, prop := range field5 {
			if propMap, ok := prop.(map[string]interface{}); ok {
				if val, ok := propMap["1"].(uint64); ok && val == 27 {
					item.IsCanonical = false
					break
				}
			}
		}
	}

	// Field 5: type and media-specific data
	if field5, ok := d["5"].(map[string]interface{}); ok {
		if val, ok := field5["1"].(uint64); ok {
			item.Type = int64(val)
		}

		// Field 5->2: photo data
		if field5_2, ok := field5["2"].(map[string]interface{}); ok {
			item.IsEdited = false
			if _, hasField4 := field5_2["4"]; hasField4 {
				item.IsEdited = true
			}

			if field5_2_1, ok := field5_2["1"].(map[string]interface{}); ok {
				if url, ok := field5_2_1["1"].(string); ok {
					item.RemoteURL = sql.NullString{String: url, Valid: true}
				}

				// Field 5->2->1->9: dimensions and EXIF
				if field9, ok := field5_2_1["9"].(map[string]interface{}); ok {
					if val, ok := field9["1"].(uint64); ok {
						item.Width = sql.NullInt64{Int64: int64(val), Valid: true}
					}
					if val, ok := field9["2"].(uint64); ok {
						item.Height = sql.NullInt64{Int64: int64(val), Valid: true}
					}

					// Field 5->2->1->9->5: EXIF data
					if field9_5, ok := field9["5"].(map[string]interface{}); ok {
						if val, ok := field9_5["1"].(string); ok {
							item.Make = sql.NullString{String: val, Valid: true}
						}
						if val, ok := field9_5["2"].(string); ok {
							item.Model = sql.NullString{String: val, Valid: true}
						}
					}
				}
			}
		}

		// Field 5->3: video data
		if field5_3, ok := field5["3"].(map[string]interface{}); ok {
			if field5_3_2, ok := field5_3["2"].(map[string]interface{}); ok {
				if url, ok := field5_3_2["1"].(string); ok {
					item.RemoteURL = sql.NullString{String: url, Valid: true}
				}
			}

			if field5_3_4, ok := field5_3["4"].(map[string]interface{}); ok {
				if val, ok := field5_3_4["1"].(uint64); ok {
					item.Duration = sql.NullInt64{Int64: int64(val), Valid: true}
				}
				if val, ok := field5_3_4["4"].(uint64); ok {
					item.Width = sql.NullInt64{Int64: int64(val), Valid: true}
				}
				if val, ok := field5_3_4["5"].(uint64); ok {
					item.Height = sql.NullInt64{Int64: int64(val), Valid: true}
				}
			}
		}

		// Field 5->5: micro video
		if field5_5, ok := field5["5"].(map[string]interface{}); ok {
			if field5_5_2, ok := field5_5["2"].(map[string]interface{}); ok {
				if field5_5_2_4, ok := field5_5_2["4"].(map[string]interface{}); ok {
					item.IsMicroVideo = true
					if val, ok := field5_5_2_4["1"].(uint64); ok {
						item.Duration = sql.NullInt64{Int64: int64(val), Valid: true}
					}
					if val, ok := field5_5_2_4["4"].(uint64); ok {
						item.MicroVideoWidth = sql.NullInt64{Int64: int64(val), Valid: true}
					}
					if val, ok := field5_5_2_4["5"].(uint64); ok {
						item.MicroVideoHeight = sql.NullInt64{Int64: int64(val), Valid: true}
					}
				}
			}
		}
	}

	// Field 17: location data
	if field17, ok := d["17"].(map[string]interface{}); ok {
		if field17_1, ok := field17["1"].(map[string]interface{}); ok {
			// Note: These are fixed32 values that need conversion
			if val, ok := field17_1["1"].(uint32); ok {
				item.Latitude = sql.NullFloat64{Float64: fixed32ToFloat(val), Valid: true}
			}
			if val, ok := field17_1["2"].(uint32); ok {
				item.Longitude = sql.NullFloat64{Float64: fixed32ToFloat(val), Valid: true}
			}
		}

		if field17_5, ok := field17["5"].(map[string]interface{}); ok {
			if field17_5_2, ok := field17_5["2"].(map[string]interface{}); ok {
				if val, ok := field17_5_2["1"].(string); ok {
					item.LocationName = sql.NullString{String: val, Valid: true}
				}
			}
			if val, ok := field17_5["3"].(string); ok {
				item.LocationID = sql.NullString{String: val, Valid: true}
			}
		}
	}

	return item, nil
}

func parseDeletions(data interface{}) []string {
	var mediaKeys []string

	// Handle both single item (dict) and multiple items (list)
	switch v := data.(type) {
	case map[string]interface{}:
		if key := parseDeletionItem(v); key != "" {
			mediaKeys = append(mediaKeys, key)
		}
	case []interface{}:
		for _, itemData := range v {
			if itemMap, ok := itemData.(map[string]interface{}); ok {
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
	field1, ok := d["1"].(map[string]interface{})
	if !ok {
		return ""
	}

	// Field 1->1: type
	delType, ok := field1["1"].(uint64)
	if !ok {
		return ""
	}

	// Type 1 means media deletion
	if delType == 1 {
		if field1_2, ok := field1["2"].(map[string]interface{}); ok {
			if mediaKey, ok := field1_2["1"].(string); ok {
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
