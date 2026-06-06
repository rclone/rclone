package googlephotos

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rclone/rclone/backend/googlephotos/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTIFFWithDescription(desc string) []byte {
	buf := new(bytes.Buffer)
	buf.Write([]byte("II"))
	_ = binary.Write(buf, binary.LittleEndian, uint16(42))
	_ = binary.Write(buf, binary.LittleEndian, uint32(8))

	// IFD0
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))

	// Entry 1: ImageDescription (0x010e)
	_ = binary.Write(buf, binary.LittleEndian, uint16(0x010e))
	_ = binary.Write(buf, binary.LittleEndian, uint16(2))
	count := uint32(len(desc) + 1)
	_ = binary.Write(buf, binary.LittleEndian, count)
	_ = binary.Write(buf, binary.LittleEndian, uint32(26))

	// Next IFD
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	// Data
	buf.WriteString(desc)
	buf.WriteByte(0)
	return buf.Bytes()
}

// TestEXIFDescriptionMapping verifies that read_exif_description causes the
// EXIF ImageDescription tag to be extracted and forwarded to the Google Photos
// API as the media item description.
func TestEXIFDescriptionMapping(t *testing.T) {
	var uploadedDescription string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/uploads", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("upload-token-123"))
	})
	mux.HandleFunc("/v1/mediaItems:batchCreate", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req api.BatchCreateRequest
		_ = json.Unmarshal(body, &req)
		if len(req.NewMediaItems) > 0 {
			uploadedDescription = req.NewMediaItems[0].Description
		}
		resp := api.BatchCreateResponse{
			NewMediaItemResults: []api.NewMediaItemResult{{
				UploadToken: "upload-token-123",
				Status:      api.Status{Message: "Success"},
				MediaItem:   api.MediaItem{ID: "exif-photo-1", Filename: "photo.jpg"},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := &Fs{
		opt: Options{
			ReadExifDescription: true,
			BatchMode:           "off",
		},
	}
	f.batcher, _ = newBatcher(context.Background(), f, f.opt.BatchMode, f.opt.BatchSize, f.opt.BatchTimeout)

	imgData := createTIFFWithDescription("My Scenic Photo Description")
	src := mockobject.New("photo.jpg").WithContent(imgData, mockobject.SeekModeNone)

	setupRemote(t, f, srv.URL)

	obj, err := f.Put(context.Background(), bytes.NewReader(imgData), src)
	require.NoError(t, err)

	o, ok := obj.(*Object)
	require.True(t, ok)

	// Assertions
	assert.Equal(t, "exif-photo-1", o.id)
	assert.Equal(t, "My Scenic Photo Description", uploadedDescription, "Should parse EXIF description and send it to the API")
}
