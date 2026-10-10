package googlephotos

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/googlephotos/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/rclone/rclone/lib/batcher"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
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

// TestEXIFDescriptionMapping verifies that upload_exif_description causes the
// EXIF ImageDescription tag to be extracted and forwarded to the Google Photos
// API as the media item description.
func TestEXIFDescriptionMapping(t *testing.T) {
	var uploadedDescription string

	mux := http.NewServeMux()
	mux.HandleFunc("/uploads", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("upload-token-123"))
	})
	mux.HandleFunc("/mediaItems:batchCreate", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req api.BatchCreateRequest
		_ = json.Unmarshal(body, &req)
		if len(req.NewMediaItems) > 0 {
			uploadedDescription = req.NewMediaItems[0].Description
		}

		resp := api.BatchCreateResponse{}
		result := struct {
			UploadToken string `json:"uploadToken"`
			Status      struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
			} `json:"status"`
			MediaItem api.MediaItem `json:"mediaItem"`
		}{
			UploadToken: "upload-token-123",
			MediaItem:   api.MediaItem{ID: "exif-photo-1", Filename: "photo.tiff"},
		}
		result.Status.Message = "Success"
		resp.NewMediaItemResults = append(resp.NewMediaItemResults, result)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/albums", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.ListAlbums{
			Albums: []api.Album{
				{ID: "album-123", Title: "my-album", IsWriteable: true},
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx := context.Background()
	f := &Fs{
		name:   "TestGphotos",
		root:   "album/my-album",
		unAuth: rest.NewClient(http.DefaultClient),
		srv:    rest.NewClient(http.DefaultClient).SetRoot(srv.URL),
		pacer:  fs.NewPacer(ctx, pacer.NewGoogleDrive(pacer.MinSleep(10*time.Millisecond))),
		albums: map[bool]*albums{},
		opt: Options{
			UploadExifDescription: true,
			ExifDescriptionFields: "Description,Caption-Abstract,ImageDescription,Title,ObjectName",
			BatchMode:             "off",
		},
	}
	f.srv.SetErrorHandler(errorHandler)

	_, err := f.listAlbums(ctx, false)
	require.NoError(t, err)

	batcherOpts := defaultBatcherOptions
	batcherOpts.Mode = f.opt.BatchMode
	f.batcher, err = batcher.New(ctx, f, f.commitBatch, batcherOpts)
	require.NoError(t, err)

	imgData := createTIFFWithDescription("My Scenic Photo Description")
	src := mockobject.New("photo.tiff").WithContent(imgData, mockobject.SeekModeNone)

	obj, err := f.Put(ctx, bytes.NewReader(imgData), src)
	require.NoError(t, err)

	o, ok := obj.(*Object)
	require.True(t, ok)

	// Assertions
	assert.Equal(t, "exif-photo-1", o.id)
	assert.Equal(t, "My Scenic Photo Description", uploadedDescription, "Should parse EXIF description and send it to the API")
}

func TestExtractEXIFDescriptionXMP(t *testing.T) {
	// Let's test the extractEXIFDescription function using the local test file if present
	testFile := "/Volumes/home/Photos/Lightroom Export/Dog Park/20230305-121628-Z72_3640.jpg"
	f, err := os.Open(testFile)
	if err != nil {
		t.Skip("Skipping test: test file not accessible")
	}
	defer func() { _ = f.Close() }()

	desc, _, err := extractEXIFDescription(f, "20230305-121628-Z72_3640.jpg", "Description,Caption-Abstract,ImageDescription,Title,ObjectName")
	require.NoError(t, err)
	assert.Equal(t, "Puck at the Dog Park", desc, "Should extract Title/ObjectName successfully from Lightroom export")
}
