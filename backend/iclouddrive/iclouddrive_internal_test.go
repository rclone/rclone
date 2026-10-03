//go:build !plan9 && !solaris

package iclouddrive

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/rclone/rclone/backend/iclouddrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectOpenHandlesPackageTokenSize(t *testing.T) {
	for _, test := range []struct {
		name      string
		token     string
		wantSize  int64
		wantRange string
	}{
		{
			name:      "data token keeps reported size",
			token:     "data",
			wantSize:  10,
			wantRange: "bytes=2-9",
		},
		{
			name:      "package token makes size unknown",
			token:     "package",
			wantSize:  -1,
			wantRange: "bytes=2-",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var (
				mu       sync.Mutex
				gotRange string
				server   *httptest.Server
			)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/ws/com.apple.CloudDocs/download/by_id":
					assert.Equal(t, "document", r.URL.Query().Get("document_id"))
					response := map[string]any{
						test.token + "_token": map[string]string{"url": server.URL + "/payload"},
					}
					_ = json.NewEncoder(w).Encode(response)
				case "/payload":
					mu.Lock()
					gotRange = r.Header.Get("Range")
					mu.Unlock()
					_, _ = io.WriteString(w, "0123456789")
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			client, err := api.New("", "", "", "", nil, nil, "test", "")
			require.NoError(t, err)
			accountInfo := map[string]any{
				"webservices": map[string]any{
					api.WsDocs:  map[string]string{"url": server.URL},
					api.WsDrive: map[string]string{"url": server.URL},
				},
			}
			accountInfoJSON, err := json.Marshal(accountInfo)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(accountInfoJSON, &client.Session.AccountInfo))
			service, err := client.DriveService()
			require.NoError(t, err)

			o := &Object{
				fs:      &Fs{service: service, pacer: fs.NewPacer(context.Background(), pacer.NewDefault())},
				size:    10,
				driveID: "FILE::com.apple.CloudDocs::document",
			}
			rc, err := o.Open(context.Background(), &fs.SeekOption{Offset: 2})
			require.NoError(t, err)
			body, err := io.ReadAll(rc)
			require.NoError(t, err)
			require.NoError(t, rc.Close())
			assert.Equal(t, []byte("0123456789"), body)
			mu.Lock()
			assert.Equal(t, test.wantRange, gotRange)
			mu.Unlock()
			assert.Equal(t, test.wantSize, o.Size())
		})
	}
}
