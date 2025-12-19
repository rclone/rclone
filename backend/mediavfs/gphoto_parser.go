package mediavfs

import (
	"database/sql"
	"encoding/base64"
	"fmt"
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
func getKeys(m map[string]interface{}) []string {
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

	// Debug: Log field1 structure
	fmt.Printf("DEBUG ParseDbUpdate: field1 keys = %v\n", getKeys(field1))
	fmt.Printf("DEBUG ParseDbUpdate: field1 has %d keys\n", len(field1))

	// Debug: Check types of ALL fields in field1
	for _, key := range []string{"1", "2", "3", "4", "5", "6", "9", "12"} {
		if val, ok := field1[key]; ok {
			switch v := val.(type) {
			case []interface{}:
				fmt.Printf("DEBUG ParseDbUpdate: field1[\"%s\"] = ARRAY[%d]\n", key, len(v))
			case map[string]interface{}:
				fmt.Printf("DEBUG ParseDbUpdate: field1[\"%s\"] = MAP[%d]\n", key, len(v))
			case string:
				fmt.Printf("DEBUG ParseDbUpdate: field1[\"%s\"] = string(%d chars)\n", key, len(v))
			default:
				fmt.Printf("DEBUG ParseDbUpdate: field1[\"%s\"] = %T\n", key, v)
			}
		}
	}

	// Extract tokens
	if token, ok := field1["6"].(string); ok {
		stateToken = token
		fmt.Printf("DEBUG ParseDbUpdate: stateToken = %q (length=%d)\n", token, len(token))
	}
	if token, ok := field1["1"].(string); ok {
		nextPageToken = token
		fmt.Printf("DEBUG ParseDbUpdate: nextPageToken = %q (length=%d)\n", token, len(token))
	}

	// Parse media items (field 2)
	if field2Data, ok := field1["2"]; ok {
		// DETAILED DEBUG: What is field2Data exactly?
		switch v := field2Data.(type) {
		case map[string]interface{}:
			fmt.Printf("DEBUG ParseDbUpdate: field2 is a SINGLE MAP (1 media item)\n")
			fmt.Printf("DEBUG ParseDbUpdate: field2 MAP keys: %v\n", getKeys(v))
			// This is expected behavior when there's only 1 item
		case []interface{}:
			fmt.Printf("DEBUG ParseDbUpdate: field2 is an ARRAY with %d items (GOOD!)\n", len(v))
			if len(v) > 0 {
				switch first := v[0].(type) {
				case map[string]interface{}:
					fmt.Printf("DEBUG ParseDbUpdate: first item is MAP with keys: %v\n", getKeys(first))
				default:
					fmt.Printf("DEBUG ParseDbUpdate: first item type: %T\n", first)
				}
			}
		default:
			fmt.Printf("DEBUG ParseDbUpdate: field2 has unexpected type: %T\n", v)
		}

		mediaItems, err = parseMediaItems(field2Data)
		if err != nil {
			return "", "", nil, nil, fmt.Errorf("failed to parse media items: %w", err)
		}
		fmt.Printf("DEBUG ParseDbUpdate: parseMediaItems returned %d media items\n", len(mediaItems))

		// Extra verification: show first media item's key
		if len(mediaItems) > 0 {
			fmt.Printf("DEBUG ParseDbUpdate: first media_key: %q\n", mediaItems[0].MediaKey)
		}
	}

	// Check field 3 (collections?) and field 4
	if field3Data, ok := field1["3"]; ok {
		switch v := field3Data.(type) {
		case []interface{}:
			fmt.Printf("DEBUG ParseDbUpdate: field 3 has %d items (SKIPPING - collections?)\n", len(v))
		case map[string]interface{}:
			fmt.Printf("DEBUG ParseDbUpdate: field 3 is a map with %d keys (SKIPPING)\n", len(v))
		}
	}
	if field4Data, ok := field1["4"]; ok {
		switch v := field4Data.(type) {
		case []interface{}:
			fmt.Printf("DEBUG ParseDbUpdate: field 4 has %d items (WHAT IS THIS?)\n", len(v))
		default:
			fmt.Printf("DEBUG ParseDbUpdate: field 4 type: %T\n", v)
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
			fmt.Printf("WARNING: Skipping media item due to error: %v\n", err)
			skipped++
		} else {
			items = append(items, item)
		}
	case []interface{}:
		for i, itemData := range v {
			if itemMap, ok := itemData.(map[string]interface{}); ok {
				item, err := parseMediaItem(itemMap)
				if err != nil {
					fmt.Printf("WARNING: Skipping media item #%d due to error: %v\n", i, err)
					skipped++
				} else {
					items = append(items, item)
				}
			}
		}
	}

	if skipped > 0 {
		fmt.Printf("WARNING: Skipped %d media items with missing required fields\n", skipped)
	}

	return items, nil
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

	// Field 2->4: file_name (REQUIRED - must exist and be a string, but can be empty)
	fileName, ok := field2["4"].(string)
	if !ok {
		return item, fmt.Errorf("missing required field: file_name (field 2->4) for media_key %s", item.MediaKey)
	}
	item.FileName = fileName

	// Field 5: type info (REQUIRED)
	field5, ok := d["5"].(map[string]interface{})
	if !ok {
		return item, fmt.Errorf("missing required field 5 for media_key %s", item.MediaKey)
	}
	if typeVal, ok := field5["1"].(uint64); ok {
		item.Type = int64(typeVal)
	} else {
		return item, fmt.Errorf("missing required field: type (field 5->1) for media_key %s", item.MediaKey)
	}

	// Parse dedup_key from field 2->21
	if field21, ok := field2["21"].(map[string]interface{}); ok {
		for key, val := range field21 {
			if key[0] == '1' {
				if dedupKey, ok := val.(string); ok {
					item.DedupKey = dedupKey
					break
				}
			}
		}
	}

	// Fallback: try to get dedup_key from field 2->13->1
	if item.DedupKey == "" {
		if field13, ok := field2["13"].(map[string]interface{}); ok {
			if hashBytes, ok := field13["1"].([]byte); ok {
				item.DedupKey = urlsafeBase64(base64.StdEncoding.EncodeToString(hashBytes))
			}
		}
	}

	// Field 2->1->1: collection_id (REQUIRED)
	if field2_1, ok := field2["1"].(map[string]interface{}); ok {
		if val, ok := field2_1["1"].(string); ok {
			item.CollectionID = val
		} else {
			return item, fmt.Errorf("missing required field: collection_id (field 2->1->1) for media_key %s", item.MediaKey)
		}
	} else {
		return item, fmt.Errorf("missing required field 2->1 for media_key %s", item.MediaKey)
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
