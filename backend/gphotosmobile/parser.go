package gphotosmobile

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// ParseDbUpdate parses the library state response into tokens, media items, and deletions.
// This is a direct port of gpmc's db_update_parser.py parse_db_update function.
// Returns: (state_token, next_page_token, media_items, media_keys_to_delete)
func ParseDbUpdate(data []byte) (string, string, []MediaItem, []string) {
	root, err := DecodeRaw(data)
	if err != nil {
		return "", "", nil, nil
	}

	// The response is wrapped in field 1
	field1, err := root.GetMessage(1)
	if err != nil {
		return "", "", nil, nil
	}

	// field 1.1 = next_page_token
	nextPageToken := field1.GetString(1)

	// field 1.6 = state_token
	stateToken := field1.GetString(6)

	// Parse media items from field 1.2 (repeated)
	var mediaItems []MediaItem
	mediaEntries, _ := field1.GetRepeatedMessages(2)
	for _, entry := range mediaEntries {
		item, err := parseMediaItem(entry)
		if err != nil {
			continue // skip malformed items
		}
		mediaItems = append(mediaItems, *item)
	}

	// Parse deletions from field 1.9 (repeated)
	var deletions []string
	deletionEntries, _ := field1.GetRepeatedMessages(9)
	for _, entry := range deletionEntries {
		if mediaKey := parseDeletionItem(entry); mediaKey != "" {
			deletions = append(deletions, mediaKey)
		}
	}

	return stateToken, nextPageToken, mediaItems, deletions
}

// parseMediaItem parses a single media item from decoded protobuf.
// Port of _parse_media_item from db_update_parser.py
func parseMediaItem(d ProtoMap) (*MediaItem, error) {
	// d["1"] = media_key
	mediaKey := d.GetString(1)
	if mediaKey == "" {
		return nil, fmt.Errorf("no media key")
	}

	// d["2"] = media metadata
	meta, err := d.GetMessage(2)
	if err != nil {
		return nil, fmt.Errorf("no metadata: %w", err)
	}

	// d["5"] = media type info
	typeInfo, err := d.GetMessage(5)
	if err != nil {
		return nil, fmt.Errorf("no type info: %w", err)
	}

	// d["17"] = location info (optional)
	locInfo, _ := d.GetMessage(17)

	item := &MediaItem{
		MediaKey: mediaKey,
	}

	// meta["4"] = file_name
	item.FileName = meta.GetString(4)

	// meta["1"] = collection info
	if collInfo, err := meta.GetMessage(1); err == nil {
		item.CollectionID = collInfo.GetString(1)
	}

	// meta["3"] = caption (first value starting with "3")
	item.Caption = meta.GetString(3)

	// meta["7"] = utc_timestamp
	item.UTCTimestamp = meta.GetVarint(7)

	// meta["8"] = timezone_offset
	item.TimezoneOffset = meta.GetVarint(8)

	// meta["9"] = server_creation_timestamp
	item.ServerCreationTimestamp = meta.GetVarint(9)

	// meta["10"] = size_bytes
	item.SizeBytes = meta.GetVarint(10)

	// meta["11"] = upload_status
	item.UploadStatus = meta.GetVarint(11)

	// meta["26"] = content_version
	item.ContentVersion = meta.GetVarint(26)

	// meta["21"] = dedup_key info
	if dedupInfo, err := meta.GetMessage(21); err == nil {
		// First value starting with "1" in the dedup info
		dedupKey := dedupInfo.GetString(1)
		if dedupKey != "" {
			item.DedupKey = dedupKey
		}
	}

	// Extract SHA1 hash from field 13.1 (raw hash bytes)
	if hashInfo, err := meta.GetMessage(13); err == nil {
		hashBytes := hashInfo.GetBytes(1)
		if len(hashBytes) > 0 {
			item.SHA1Hash = hex.EncodeToString(hashBytes)
			// If dedup_key is still empty, derive it from the hash
			if item.DedupKey == "" {
				item.DedupKey = urlSafeBase64(base64.StdEncoding.EncodeToString(hashBytes))
			}
		}
	}

	// meta["5"] = properties (repeated) - check for is_canonical
	// is_canonical = not any(prop.get("1") == 27 for prop in d["2"]["5"])
	item.IsCanonical = true
	props, _ := meta.GetRepeatedMessages(5)
	for _, prop := range props {
		if prop.GetVarint(1) == 27 {
			item.IsCanonical = false
			break
		}
	}

	// meta["35"] = quota info
	if quotaInfo, err := meta.GetMessage(35); err == nil {
		item.QuotaChargedBytes = quotaInfo.GetVarint(2)
		if quotaInfo.GetVarint(3) == 2 {
			item.IsOriginalQuality = true
		}
	}

	// meta["30"] = origin info
	if originInfo, err := meta.GetMessage(30); err == nil {
		originVal := originInfo.GetVarint(1)
		switch originVal {
		case 1:
			item.Origin = "self"
		case 3:
			item.Origin = "partner"
		case 4:
			item.Origin = "shared"
		default:
			item.Origin = "self"
		}
	}

	// meta["16"] = trash info
	if trashInfo, err := meta.GetMessage(16); err == nil {
		item.TrashTimestamp = trashInfo.GetVarint(3)
	}

	// meta["29"] = archive info
	if archiveInfo, err := meta.GetMessage(29); err == nil {
		item.IsArchived = archiveInfo.GetVarint(1) == 1
	}

	// meta["31"] = favorite info
	if favInfo, err := meta.GetMessage(31); err == nil {
		item.IsFavorite = favInfo.GetVarint(1) == 1
	}

	// meta["39"] = lock info
	if lockInfo, err := meta.GetMessage(39); err == nil {
		item.IsLocked = lockInfo.GetVarint(1) == 1
	}

	// typeInfo["1"] = type (1=photo, 2=video, etc)
	item.Type = typeInfo.GetVarint(1)

	// typeInfo["2"] = photo info
	if photoInfo, err := typeInfo.GetMessage(2); err == nil {
		// Check if edited: "4" key present
		item.IsEdited = photoInfo.Has(4)

		// photoInfo["1"] = main photo data
		if mainPhoto, err := photoInfo.GetMessage(1); err == nil {
			item.RemoteURL = mainPhoto.GetString(1)

			// mainPhoto["9"] = dimensions/exif
			if dimInfo, err := mainPhoto.GetMessage(9); err == nil {
				item.Width = dimInfo.GetVarint(1)
				item.Height = dimInfo.GetVarint(2)

				// dimInfo["5"] = EXIF data
				if exif, err := dimInfo.GetMessage(5); err == nil {
					item.Make = exif.GetString(1)
					item.Model = exif.GetString(2)
					if v := exif.GetVarint(4); v != 0 {
						item.Aperture = float64(Int32ToFloat(int32(v)))
					}
					if v := exif.GetVarint(5); v != 0 {
						item.ShutterSpeed = float64(Int32ToFloat(int32(v)))
					}
					item.ISO = exif.GetVarint(6)
					if v := exif.GetVarint(7); v != 0 {
						item.FocalLength = float64(Int32ToFloat(int32(v)))
					}
				}
			}
		}
	}

	// typeInfo["3"] = video info
	if videoInfo, err := typeInfo.GetMessage(3); err == nil {
		// videoInfo["2"] = video data
		if videoData, err := videoInfo.GetMessage(2); err == nil {
			item.RemoteURL = videoData.GetString(1)
		}
		// videoInfo["4"] = video dimensions/duration
		if videoDim, err := videoInfo.GetMessage(4); err == nil {
			item.Duration = videoDim.GetVarint(1)
			item.Width = videoDim.GetVarint(4)
			item.Height = videoDim.GetVarint(5)
		}
		// videoInfo["6"] = frame rates
		if frameInfo, err := videoInfo.GetMessage(6); err == nil {
			if v := frameInfo.GetVarint(4); v != 0 {
				item.CaptureFrameRate = Int64ToFloat(v)
			}
			if v := frameInfo.GetVarint(5); v != 0 {
				item.EncodedFrameRate = Int64ToFloat(v)
			}
		}
	}

	// typeInfo["5"] = micro video info
	if microInfo, err := typeInfo.GetMessage(5); err == nil {
		if microData, err := microInfo.GetMessage(2); err == nil {
			if microDetail, err := microData.GetMessage(4); err == nil {
				item.IsMicroVideo = true
				item.Duration = microDetail.GetVarint(1)
				item.MicroVideoWidth = microDetail.GetVarint(4)
				item.MicroVideoHeight = microDetail.GetVarint(5)
			}
		}
	}

	// Location info from d["17"]
	if locInfo != nil {
		if gps, err := locInfo.GetMessage(1); err == nil {
			if gps.Has(1) {
				item.Latitude = Fixed32ToFloat(gps.GetUint(1))
			}
			if gps.Has(2) {
				item.Longitude = Fixed32ToFloat(gps.GetUint(2))
			}
		}
		if locName, err := locInfo.GetMessage(5); err == nil {
			if locDetail, err := locName.GetMessage(2); err == nil {
				item.LocationName = locDetail.GetString(1)
			}
			item.LocationID = locName.GetString(3)
		}
	}

	return item, nil
}

// parseDeletionItem parses a deletion entry
// Port of _parse_deletion_item from db_update_parser.py
func parseDeletionItem(d ProtoMap) string {
	// d["1"] = deletion info
	delInfo, err := d.GetMessage(1)
	if err != nil {
		return ""
	}

	// delInfo["1"] = type (1 = media item)
	delType := delInfo.GetVarint(1)
	if delType == 1 {
		// delInfo["2"]["1"] = media key
		if itemInfo, err := delInfo.GetMessage(2); err == nil {
			return itemInfo.GetString(1)
		}
	}

	return ""
}

// urlSafeBase64 converts standard base64 to URL-safe base64
func urlSafeBase64(b64 string) string {
	s := strings.ReplaceAll(b64, "+", "-")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.TrimRight(s, "=")
	return s
}
