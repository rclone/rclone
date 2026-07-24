package api

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client := NewClient()
	require.NotNil(t, client)
}

func TestChannelMarshalJSON(t *testing.T) {
	ch := &Channel{ID: "UC1", Title: "Test", Handle: "@t", URL: "http://t", Uploader: "U"}
	data, err := json.Marshal(ch)
	require.NoError(t, err)
	var decoded Channel
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	require.Equal(t, ch.ID, decoded.ID)
}

func TestVideoMarshalJSON(t *testing.T) {
	v := &Video{ID: "v1", Title: "V", Duration: 60, URL: "http://v", UploadDate: "20240101"}
	data, err := json.Marshal(v)
	require.NoError(t, err)
	var decoded Video
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	require.Equal(t, v.ID, decoded.ID)
}

func TestPlaylistMarshalJSON(t *testing.T) {
	p := &Playlist{ID: "PL1", Title: "P", URL: "http://p"}
	data, err := json.Marshal(p)
	require.NoError(t, err)
	var decoded Playlist
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	require.Equal(t, p.ID, decoded.ID)
}

func TestChannelUnmarshalJSON(t *testing.T) {
	jsonData := `{"id": "UC1", "title": "T", "webpage_url": "http://t"}`
	var ch Channel
	err := json.Unmarshal([]byte(jsonData), &ch)
	require.NoError(t, err)
	require.Equal(t, "UC1", ch.ID)
}

func TestVideoUnmarshalJSON(t *testing.T) {
	jsonStr := `{"id": "v1", "title": "V", "duration": 60, "webpage_url": "http://v"}`
	var v Video
	err := json.Unmarshal([]byte(jsonStr), &v)
	require.NoError(t, err)
	require.Equal(t, 60, v.Duration)
}

func TestPlaylistUnmarshalJSON(t *testing.T) {
	jsonStr := `{"id": "PL1", "title": "P", "webpage_url": "http://p"}`
	var p Playlist
	err := json.Unmarshal([]byte(jsonStr), &p)
	require.NoError(t, err)
	require.Equal(t, "PL1", p.ID)
}

func TestPlaylistEntryUnmarshalJSON(t *testing.T) {
	jsonStr := `{"id": "v1", "title": "V", "playlist_index": 1}`
	var pe PlaylistEntry
	err := json.Unmarshal([]byte(jsonStr), &pe)
	require.NoError(t, err)
	require.Equal(t, 1, pe.Index)
}

func TestContextCancellation(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.GetChannelInfo(ctx, "http://test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

func TestParsePlaylistJSON(t *testing.T) {
	jsonStr := `{"entries": [{"id": "v1", "title": "V1", "playlist_index": 1}]}`
	var result struct {
		Entries []PlaylistEntry `json:"entries"`
	}
	err := json.Unmarshal([]byte(jsonStr), &result)
	require.NoError(t, err)
	require.Equal(t, 1, len(result.Entries))
}

func TestParseVideoJSON(t *testing.T) {
	jsonStr := `{"entries": [{"id": "v1", "title": "V1", "duration": 120, "webpage_url": "http://test"}]}`
	var result struct {
		Entries []Video `json:"entries"`
	}
	err := json.Unmarshal([]byte(jsonStr), &result)
	require.NoError(t, err)
	require.Equal(t, 1, len(result.Entries))
}

func TestLargePlaylist(t *testing.T) {
	entries := `"entries": [`
	for i := 1; i <= 50; i++ {
		if i > 1 {
			entries += ","
		}
		entries += fmt.Sprintf(`{"id": "v%d", "title": "V%d", "playlist_index": %d}`, i, i, i)
	}
	entries += `]`
	jsonStr := `{` + entries + `}`
	var result struct {
		Entries []PlaylistEntry `json:"entries"`
	}
	err := json.Unmarshal([]byte(jsonStr), &result)
	require.NoError(t, err)
	require.Equal(t, 50, len(result.Entries))
}

func TestRoundTrip(t *testing.T) {
	ch := &Channel{ID: "UC1", Title: "T", URL: "http://t"}
	data, err := json.Marshal(ch)
	require.NoError(t, err)
	var decoded Channel
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
}

func TestEdgeCases(t *testing.T) {
	v := &Video{ID: "v1", Title: "", Duration: 0, URL: "http://v"}
	data, err := json.Marshal(v)
	require.NoError(t, err)
	var decoded Video
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
}

func TestSpecialChars(t *testing.T) {
	ch := &Channel{ID: "UC1", Title: "T / test"}
	data, err := json.Marshal(ch)
	require.NoError(t, err)
	var decoded Channel
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Contains(t, decoded.Title, "/")
}

func TestFieldNames(t *testing.T) {
	jsonStr := `{"id": "UC1", "uploader_id": "@h", "webpage_url": "http://t", "title": "T"}`
	var ch Channel
	err := json.Unmarshal([]byte(jsonStr), &ch)
	require.NoError(t, err)
	require.Equal(t, "@h", ch.Handle)
}

func TestVideoFields(t *testing.T) {
	v := &Video{ID: "v1", Title: "T", Duration: 60, URL: "http://u"}
	require.Equal(t, "v1", v.ID)
	require.Equal(t, 60, v.Duration)
}

func TestChannelFields(t *testing.T) {
	ch := &Channel{ID: "UC1", Title: "T", Handle: "@h", URL: "http://u"}
	require.Equal(t, "UC1", ch.ID)
	require.Equal(t, "@h", ch.Handle)
}

func TestPlaylistFields(t *testing.T) {
	p := &Playlist{ID: "PL1", Title: "T", URL: "http://u"}
	require.Equal(t, "PL1", p.ID)
}
