package internetarchive

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyMetadata(t *testing.T) {
	originalTime := time.Date(2003, 2, 3, 4, 5, 6, 7, time.UTC)
	newTime := "2004-03-03T04:05:06.499999999Z"
	for _, test := range []struct {
		name      string
		metadata  bool
		set       fs.Metadata
		mapper    string
		badSource bool
		wantNote  string
		wantTime  string
		wantErr   string
	}{
		{name: "disabled", wantTime: originalTime.Format(time.RFC3339Nano)},
		{name: "source", metadata: true, wantNote: "original", wantTime: originalTime.Format(time.RFC3339Nano)},
		{name: "override", metadata: true, set: fs.Metadata{"note": "override", "mtime": newTime, "size": "999"}, wantNote: "override", wantTime: newTime},
		{name: "mapper", metadata: true, mapper: "map", wantNote: "mapped", wantTime: newTime},
		{name: "mapper error", metadata: true, mapper: "error", wantErr: "metadata mapper"},
		{name: "source error", metadata: true, badSource: true, wantErr: "unexpected end of JSON input"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, ci := fs.AddConfig(t.Context())
			ci.Metadata = test.metadata
			ci.MetadataSet = test.set
			if test.mapper != "" {
				executable, err := os.Executable()
				require.NoError(t, err)
				ci.MetadataMapper = fs.SpaceSepList{executable, "-test.run=^TestCopyMetadataMapperHelper$"}
				t.Setenv("RCLONE_COPY_METADATA_MAPPER", test.mapper)
			}
			requests := make(chan http.Header, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Empty(t, r.Header.Get("Authorization"))
				switch r.Method {
				case http.MethodPut:
					assert.Equal(t, "/destination%2Fcopied.txt", r.URL.EscapedPath())
					requests <- r.Header.Clone()
				case http.MethodGet:
					assert.Equal(t, "/metadata/destination", r.URL.Path)
					w.Header().Set("Content-Type", "application/json")
					_, err := fmt.Fprint(w, `{"files":[]}`)
					assert.NoError(t, err)
				default:
					t.Errorf("unexpected method: %s", r.Method)
					http.Error(w, "unexpected request", http.StatusBadRequest)
				}
			}))
			defer server.Close()
			f := &Fs{name: "test", srv: rest.NewClient(server.Client()).SetRoot(server.URL), front: rest.NewClient(server.Client()).SetRoot(server.URL), pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(0)))}
			f.features = (&fs.Features{ReadMetadata: true, WriteMetadata: true}).Fill(ctx, f)
			src := &Object{fs: f, remote: "source/original.txt", modTime: originalTime, size: 5,
				rawData: json.RawMessage(`{"note":"original","rclone-mtime":"2003-02-03T04:05:06.000000007Z","rclone-update-track":"old-tracker","size":"5"}`)}
			if test.badSource {
				src.rawData = json.RawMessage("{")
			}
			dst, err := f.Copy(ctx, src, "destination/copied.txt")
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				assert.Nil(t, dst)
				assert.Empty(t, requests)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, dst)
			assert.Equal(t, "destination/copied.txt", dst.Remote())
			require.Len(t, requests, 1)
			headers := <-requests
			assert.Equal(t, "/source/original.txt", headers.Get("X-Amz-Copy-Source"))
			assert.Equal(t, "COPY", headers.Get("X-Amz-Metadata-Directive"))
			assert.Equal(t, "5", headers.Get("X-Archive-Filemeta-Size"))
			assert.Equal(t, test.wantTime, headers.Get("X-Archive-Filemeta-Rclone-Mtime"))
			assert.Equal(t, test.wantNote, headers.Get("X-Archive-Filemeta-Note"))
			assert.Len(t, headers.Get("X-Archive-Filemeta-Rclone-Update-Track"), 32)
		})
	}
}

func TestCopyMetadataMapperHelper(t *testing.T) {
	mode := os.Getenv("RCLONE_COPY_METADATA_MAPPER")
	if mode == "" {
		return
	}
	if mode == "error" {
		os.Exit(1)
	}
	var request struct{ Metadata fs.Metadata }
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil || request.Metadata["note"] != "original" {
		os.Exit(2)
	}
	request.Metadata["mtime"] = "2004-03-03T04:05:06.499999999Z"
	request.Metadata["note"] = "mapped"
	if err := json.NewEncoder(os.Stdout).Encode(request); err != nil {
		os.Exit(3)
	}
	os.Exit(0)
}
