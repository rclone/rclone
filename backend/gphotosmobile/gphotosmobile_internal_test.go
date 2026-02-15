package gphotosmobile

import (
	"encoding/hex"
	"math"
	"testing"

	"github.com/rclone/rclone/backend/gphotosmobile/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- protowire_utils.go tests ---

func TestProtoBuilderVarint(t *testing.T) {
	b := NewProtoBuilder()
	b.AddVarint(1, 150)
	data := b.Bytes()

	decoded, err := DecodeRaw(data)
	require.NoError(t, err)
	assert.Equal(t, int64(150), decoded.GetVarint(1))
}

func TestProtoBuilderString(t *testing.T) {
	b := NewProtoBuilder()
	b.AddString(1, "hello")
	data := b.Bytes()

	decoded, err := DecodeRaw(data)
	require.NoError(t, err)
	assert.Equal(t, "hello", decoded.GetString(1))
}

func TestProtoBuilderBytes(t *testing.T) {
	b := NewProtoBuilder()
	b.AddBytes(1, []byte{0xDE, 0xAD, 0xBE, 0xEF})
	data := b.Bytes()

	decoded, err := DecodeRaw(data)
	require.NoError(t, err)
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, decoded.GetBytes(1))
}

func TestProtoBuilderNestedMessage(t *testing.T) {
	inner := NewProtoBuilder()
	inner.AddString(1, "nested")
	inner.AddVarint(2, 42)

	outer := NewProtoBuilder()
	outer.AddMessage(3, inner)
	data := outer.Bytes()

	decoded, err := DecodeRaw(data)
	require.NoError(t, err)

	msg, err := decoded.GetMessage(3)
	require.NoError(t, err)
	assert.Equal(t, "nested", msg.GetString(1))
	assert.Equal(t, int64(42), msg.GetVarint(2))
}

func TestProtoBuilderEmptyMessage(t *testing.T) {
	b := NewProtoBuilder()
	b.AddEmptyMessage(5)
	data := b.Bytes()

	decoded, err := DecodeRaw(data)
	require.NoError(t, err)
	assert.True(t, decoded.Has(5))
	assert.Equal(t, []byte{}, decoded.GetBytes(5))
}

func TestProtoBuilderFixed32(t *testing.T) {
	b := NewProtoBuilder()
	b.AddFixed32(1, 12345678)
	data := b.Bytes()

	decoded, err := DecodeRaw(data)
	require.NoError(t, err)
	vals := decoded[1]
	require.Len(t, vals, 1)
	assert.Equal(t, Wire32Bit, vals[0].WireType)
	assert.Equal(t, uint32(12345678), vals[0].Fixed32)
}

func TestProtoBuilderRepeatedFields(t *testing.T) {
	b := NewProtoBuilder()
	b.AddVarint(1, 10)
	b.AddVarint(1, 20)
	b.AddVarint(1, 30)
	data := b.Bytes()

	decoded, err := DecodeRaw(data)
	require.NoError(t, err)
	vals := decoded[1]
	require.Len(t, vals, 3)
	assert.Equal(t, uint64(10), vals[0].Varint)
	assert.Equal(t, uint64(20), vals[1].Varint)
	assert.Equal(t, uint64(30), vals[2].Varint)
}

func TestProtoBuilderMultipleFields(t *testing.T) {
	b := NewProtoBuilder()
	b.AddVarint(1, 100)
	b.AddString(2, "test")
	b.AddVarint(3, 200)
	data := b.Bytes()

	decoded, err := DecodeRaw(data)
	require.NoError(t, err)
	assert.Equal(t, int64(100), decoded.GetVarint(1))
	assert.Equal(t, "test", decoded.GetString(2))
	assert.Equal(t, int64(200), decoded.GetVarint(3))
}

func TestDecodeRawEmpty(t *testing.T) {
	decoded, err := DecodeRaw([]byte{})
	require.NoError(t, err)
	assert.Empty(t, decoded)
}

func TestDecodeRawInvalid(t *testing.T) {
	// Truncated varint
	_, err := DecodeRaw([]byte{0x80})
	assert.Error(t, err)
}

func TestProtoMapGetMissing(t *testing.T) {
	m := ProtoMap{}
	assert.Equal(t, "", m.GetString(1))
	assert.Nil(t, m.GetBytes(1))
	assert.Equal(t, int64(0), m.GetVarint(1))
	assert.Equal(t, uint64(0), m.GetUint(1))
	assert.False(t, m.Has(1))

	_, err := m.GetMessage(1)
	assert.Error(t, err)
}

func TestProtoMapGetRepeatedMessagesEmpty(t *testing.T) {
	m := ProtoMap{}
	msgs, err := m.GetRepeatedMessages(1)
	require.NoError(t, err)
	assert.Nil(t, msgs)
}

func TestProtoRoundTrip(t *testing.T) {
	// Build a complex nested structure and verify round-trip
	inner1 := NewProtoBuilder()
	inner1.AddString(1, "media_key_abc")
	inner1.AddVarint(2, 1024)

	inner2 := NewProtoBuilder()
	inner2.AddString(1, "photo.jpg")

	outer := NewProtoBuilder()
	outer.AddMessage(1, inner1)
	outer.AddMessage(2, inner2)
	outer.AddVarint(3, 999)

	data := outer.Bytes()
	decoded, err := DecodeRaw(data)
	require.NoError(t, err)

	msg1, err := decoded.GetMessage(1)
	require.NoError(t, err)
	assert.Equal(t, "media_key_abc", msg1.GetString(1))
	assert.Equal(t, int64(1024), msg1.GetVarint(2))

	msg2, err := decoded.GetMessage(2)
	require.NoError(t, err)
	assert.Equal(t, "photo.jpg", msg2.GetString(1))

	assert.Equal(t, int64(999), decoded.GetVarint(3))
}

// --- Float conversion tests ---

func TestFixed32ToFloat(t *testing.T) {
	tests := []struct {
		name string
		in   uint64
		want float64
	}{
		{"zero", 0, 0},
		{"positive", 377490000, 37.749},                  // San Francisco latitude
		{"negative", 4294967296 - 1224194000, -122.4194}, // San Francisco longitude (negative)
		{"small positive", 10000000, 1.0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Fixed32ToFloat(tc.in)
			assert.InDelta(t, tc.want, got, 0.0001, "Fixed32ToFloat(%d)", tc.in)
		})
	}
}

func TestInt32ToFloat(t *testing.T) {
	// IEEE 754 float32: 1.8 = 0x3FE66666
	bits := math.Float32bits(1.8)
	got := Int32ToFloat(int32(bits))
	assert.InDelta(t, float32(1.8), got, 0.0001)
}

func TestInt64ToFloat(t *testing.T) {
	// IEEE 754 float64: 29.97 = 0x403DF851EB851EB8
	bits := math.Float64bits(29.97)
	got := Int64ToFloat(int64(bits))
	assert.InDelta(t, 29.97, got, 0.0001)
}

// --- parseEmail / parseLanguage tests ---

func TestParseEmail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"standard", "androidId=abc123&Email=user%40gmail.com&Token=xxx", "user@gmail.com"},
		{"no encoding", "androidId=abc&Email=user@example.com&Token=xxx", "user@example.com"},
		{"missing", "androidId=abc&Token=xxx", ""},
		{"empty", "", ""},
		{"email first", "Email=test%40test.com&androidId=abc", "test@test.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseEmail(tc.input))
		})
	}
}

func TestParseLanguage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"standard", "androidId=abc&lang=en_US&Token=xxx", "en_US"},
		{"missing", "androidId=abc&Token=xxx", ""},
		{"empty", "", ""},
		{"different lang", "lang=de_DE&androidId=abc", "de_DE"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseLanguage(tc.input))
		})
	}
}

// --- parser.go tests ---

// buildTestMediaItem builds a minimal valid protobuf-encoded media item
// that can be parsed by parseMediaItem. This constructs the same wire
// format that the Google Photos API would return.
func buildTestMediaItem(mediaKey, fileName, dedupKey string, sizeBytes int64, mediaType int64, sha1Hex string) []byte {
	// Build metadata block (field 2)
	meta := NewProtoBuilder()
	meta.AddString(4, fileName)           // file_name
	meta.AddVarint(7, 1700000000000)      // utc_timestamp (millis)
	meta.AddVarint(10, uint64(sizeBytes)) // size_bytes

	// dedup_key at 21.1
	if dedupKey != "" {
		dedupMsg := NewProtoBuilder()
		dedupMsg.AddString(1, dedupKey)
		meta.AddMessage(21, dedupMsg)
	}

	// sha1 hash at 13.1
	if sha1Hex != "" {
		hashBytes, _ := hex.DecodeString(sha1Hex)
		hashMsg := NewProtoBuilder()
		hashMsg.AddBytes(1, hashBytes)
		meta.AddMessage(13, hashMsg)
	}

	// Build type info block (field 5)
	typeInfo := NewProtoBuilder()
	typeInfo.AddVarint(1, uint64(mediaType))

	if mediaType == 1 {
		// Photo: add minimal photo info at 5.2
		photoInfo := NewProtoBuilder()
		mainPhoto := NewProtoBuilder()
		dimInfo := NewProtoBuilder()
		dimInfo.AddVarint(1, 4032) // width
		dimInfo.AddVarint(2, 3024) // height
		mainPhoto.AddMessage(9, dimInfo)
		photoInfo.AddMessage(1, mainPhoto)
		typeInfo.AddMessage(2, photoInfo)
	} else if mediaType == 2 {
		// Video: add minimal video info at 5.3
		videoInfo := NewProtoBuilder()
		videoDim := NewProtoBuilder()
		videoDim.AddVarint(1, 15000) // duration ms
		videoDim.AddVarint(4, 1920)  // width
		videoDim.AddVarint(5, 1080)  // height
		videoInfo.AddMessage(4, videoDim)
		typeInfo.AddMessage(3, videoInfo)
	}

	// Build the media item entry
	entry := NewProtoBuilder()
	entry.AddString(1, mediaKey)  // media_key
	entry.AddMessage(2, meta)     // metadata
	entry.AddMessage(5, typeInfo) // type info

	return entry.Bytes()
}

// buildTestSyncResponse builds a minimal library sync response containing
// the given media item entries and optional deletions.
func buildTestSyncResponse(stateToken, pageToken string, items [][]byte, deletedKeys []string) []byte {
	wrapper := NewProtoBuilder()

	if pageToken != "" {
		wrapper.AddString(1, pageToken)
	}

	for _, itemBytes := range items {
		wrapper.AddBytes(2, itemBytes)
	}

	if stateToken != "" {
		wrapper.AddString(6, stateToken)
	}

	// Deletions at field 9
	for _, key := range deletedKeys {
		delInfo := NewProtoBuilder()
		delInfo.AddVarint(1, 1) // type = media item
		keyMsg := NewProtoBuilder()
		keyMsg.AddString(1, key)
		delInfo.AddMessage(2, keyMsg)

		delEntry := NewProtoBuilder()
		delEntry.AddMessage(1, delInfo)
		wrapper.AddBytes(9, delEntry.Bytes())
	}

	outer := NewProtoBuilder()
	outer.AddMessage(1, wrapper)
	return outer.Bytes()
}

func TestParseDbUpdateBasic(t *testing.T) {
	item1 := buildTestMediaItem("AF1QipN_key1", "photo1.jpg", "dedup1", 1024, 1, "c7b2bd9a1234567890abcdef12345678deadbeef")
	item2 := buildTestMediaItem("AF1QipN_key2", "video1.mp4", "dedup2", 2048, 2, "")

	data := buildTestSyncResponse("state_abc", "", [][]byte{item1, item2}, nil)

	stateToken, pageToken, items, deletions := ParseDbUpdate(data)

	assert.Equal(t, "state_abc", stateToken)
	assert.Equal(t, "", pageToken)
	assert.Len(t, items, 2)
	assert.Empty(t, deletions)

	// Check first item (photo)
	assert.Equal(t, "AF1QipN_key1", items[0].MediaKey)
	assert.Equal(t, "photo1.jpg", items[0].FileName)
	assert.Equal(t, "dedup1", items[0].DedupKey)
	assert.Equal(t, int64(1024), items[0].SizeBytes)
	assert.Equal(t, int64(1), items[0].Type)
	assert.Equal(t, int64(4032), items[0].Width)
	assert.Equal(t, int64(3024), items[0].Height)
	assert.Equal(t, "c7b2bd9a1234567890abcdef12345678deadbeef", items[0].SHA1Hash)
	assert.Equal(t, int64(1700000000000), items[0].UTCTimestamp)

	// Check second item (video)
	assert.Equal(t, "AF1QipN_key2", items[1].MediaKey)
	assert.Equal(t, "video1.mp4", items[1].FileName)
	assert.Equal(t, int64(2), items[1].Type)
	assert.Equal(t, int64(1920), items[1].Width)
	assert.Equal(t, int64(1080), items[1].Height)
	assert.Equal(t, int64(15000), items[1].Duration)
}

func TestParseDbUpdateWithDeletions(t *testing.T) {
	data := buildTestSyncResponse("state_xyz", "", nil, []string{"deleted_key1", "deleted_key2"})

	stateToken, _, items, deletions := ParseDbUpdate(data)

	assert.Equal(t, "state_xyz", stateToken)
	assert.Empty(t, items)
	assert.Equal(t, []string{"deleted_key1", "deleted_key2"}, deletions)
}

func TestParseDbUpdateWithPageToken(t *testing.T) {
	data := buildTestSyncResponse("state_1", "page_next", nil, nil)

	stateToken, pageToken, _, _ := ParseDbUpdate(data)

	assert.Equal(t, "state_1", stateToken)
	assert.Equal(t, "page_next", pageToken)
}

func TestParseDbUpdateEmpty(t *testing.T) {
	// Empty response
	stateToken, pageToken, items, deletions := ParseDbUpdate([]byte{})

	assert.Equal(t, "", stateToken)
	assert.Equal(t, "", pageToken)
	assert.Nil(t, items)
	assert.Nil(t, deletions)
}

func TestParseDbUpdateSHA1DerivesDedupKey(t *testing.T) {
	// When dedup_key is empty but SHA1 hash is present, parser should derive dedup_key
	item := buildTestMediaItem("key1", "test.jpg", "", 512, 1, "c7b2bd9a1234567890abcdef12345678deadbeef")
	data := buildTestSyncResponse("state", "", [][]byte{item}, nil)

	_, _, items, _ := ParseDbUpdate(data)
	require.Len(t, items, 1)

	// dedup_key should be derived from SHA1 hash as URL-safe base64
	assert.NotEmpty(t, items[0].DedupKey)
	assert.Equal(t, "c7b2bd9a1234567890abcdef12345678deadbeef", items[0].SHA1Hash)
}

func TestParseMediaItemWithLocation(t *testing.T) {
	// Build an item with location info (field 17)
	meta := NewProtoBuilder()
	meta.AddString(4, "geotagged.jpg")
	meta.AddVarint(10, 512)

	typeInfo := NewProtoBuilder()
	typeInfo.AddVarint(1, 1) // photo

	// Location at field 17
	locInfo := NewProtoBuilder()
	gps := NewProtoBuilder()
	// Latitude: 37.7749 * 1e7 = 377749000
	gps.AddVarint(1, 377749000)
	// Longitude: -122.4194 * 1e7 = -1224194000 → as uint64: 4294967296 - 1224194000 = 3070773296
	gps.AddVarint(2, 3070773296)
	locInfo.AddMessage(1, gps)

	locName := NewProtoBuilder()
	locDetail := NewProtoBuilder()
	locDetail.AddString(1, "San Francisco, CA")
	locName.AddMessage(2, locDetail)
	locName.AddString(3, "ChIJIQBpAG2ahYAR_6128GcTUEo")
	locInfo.AddMessage(5, locName)

	entry := NewProtoBuilder()
	entry.AddString(1, "key_geo")
	entry.AddMessage(2, meta)
	entry.AddMessage(5, typeInfo)
	entry.AddMessage(17, locInfo)

	decoded, err := DecodeRaw(entry.Bytes())
	require.NoError(t, err)

	item, err := parseMediaItem(decoded)
	require.NoError(t, err)

	assert.Equal(t, "key_geo", item.MediaKey)
	assert.InDelta(t, 37.7749, item.Latitude, 0.001)
	assert.InDelta(t, -122.4194, item.Longitude, 0.001)
	assert.Equal(t, "San Francisco, CA", item.LocationName)
	assert.Equal(t, "ChIJIQBpAG2ahYAR_6128GcTUEo", item.LocationID)
}

func TestParseMediaItemWithFlags(t *testing.T) {
	meta := NewProtoBuilder()
	meta.AddString(4, "flagged.jpg")
	meta.AddVarint(10, 256)

	// Archived at 29.1
	archiveInfo := NewProtoBuilder()
	archiveInfo.AddVarint(1, 1)
	meta.AddMessage(29, archiveInfo)

	// Favorite at 31.1
	favInfo := NewProtoBuilder()
	favInfo.AddVarint(1, 1)
	meta.AddMessage(31, favInfo)

	// Origin at 30.1 (3 = partner)
	originInfo := NewProtoBuilder()
	originInfo.AddVarint(1, 3)
	meta.AddMessage(30, originInfo)

	typeInfo := NewProtoBuilder()
	typeInfo.AddVarint(1, 1)

	entry := NewProtoBuilder()
	entry.AddString(1, "key_flags")
	entry.AddMessage(2, meta)
	entry.AddMessage(5, typeInfo)

	decoded, err := DecodeRaw(entry.Bytes())
	require.NoError(t, err)

	item, err := parseMediaItem(decoded)
	require.NoError(t, err)

	assert.True(t, item.IsArchived)
	assert.True(t, item.IsFavorite)
	assert.Equal(t, "partner", item.Origin)
}

func TestParseMediaItemOriginValues(t *testing.T) {
	tests := []struct {
		originVal int64
		want      string
	}{
		{1, "self"},
		{3, "partner"},
		{4, "shared"},
		{99, "self"}, // unknown defaults to self
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			meta := NewProtoBuilder()
			meta.AddString(4, "test.jpg")
			meta.AddVarint(10, 100)
			originInfo := NewProtoBuilder()
			originInfo.AddVarint(1, uint64(tc.originVal))
			meta.AddMessage(30, originInfo)

			typeInfo := NewProtoBuilder()
			typeInfo.AddVarint(1, 1)

			entry := NewProtoBuilder()
			entry.AddString(1, "key")
			entry.AddMessage(2, meta)
			entry.AddMessage(5, typeInfo)

			decoded, err := DecodeRaw(entry.Bytes())
			require.NoError(t, err)

			item, err := parseMediaItem(decoded)
			require.NoError(t, err)
			assert.Equal(t, tc.want, item.Origin)
		})
	}
}

// --- urlSafeBase64 tests ---

func TestUrlSafeBase64(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no change", "abc123", "abc123"},
		{"plus to dash", "a+b+c", "a-b-c"},
		{"slash to underscore", "a/b/c", "a_b_c"},
		{"strip padding", "abc=", "abc"},
		{"strip double padding", "ab==", "ab"},
		{"all transforms", "a+b/c==", "a-b_c"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, urlSafeBase64(tc.input))
		})
	}
}

// --- api.Error tests ---

func TestAPIError(t *testing.T) {
	err := &api.Error{StatusCode: 404, Body: "not found"}
	assert.Equal(t, "API error 404: not found", err.Error())
	assert.Implements(t, (*error)(nil), err)
}
