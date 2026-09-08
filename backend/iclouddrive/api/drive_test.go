package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDownloadURLByDriveID(t *testing.T) {
	for _, test := range []struct {
		name        string
		response    string
		wantURL     string
		wantPackage bool
	}{
		{
			name:     "data token",
			response: `{"data_token":{"url":"https://download.example/data"}}`,
			wantURL:  "https://download.example/data",
		},
		{
			name:        "package token",
			response:    `{"package_token":{"url":"https://download.example/package"}}`,
			wantURL:     "https://download.example/package",
			wantPackage: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/ws/com.apple.CloudDocs/download/by_id", r.URL.Path)
				assert.Equal(t, "document", r.URL.Query().Get("document_id"))
				_, _ = io.WriteString(w, test.response)
			}))
			defer server.Close()

			service := &DriveService{
				icloud:       &Client{Session: NewSession()},
				docsEndpoint: server.URL,
			}
			service.icloud.Session.srv = rest.NewClient(server.Client())

			url, isPackage, _, err := service.GetDownloadURLByDriveID(context.Background(), "FILE::com.apple.CloudDocs::document")
			require.NoError(t, err)
			assert.Equal(t, test.wantURL, url)
			assert.Equal(t, test.wantPackage, isPackage)
		})
	}
}

func TestZoneFromDriveID(t *testing.T) {
	for _, test := range []struct {
		name string
		id   string
		want string
	}{
		{"app container", "FOLDER::iCloud.md.obsidian::documents#o2v", "iCloud.md.obsidian"},
		{"another app container", "FOLDER::com.apple.Pages::documents#7qt", "com.apple.Pages"},
		{"ordinary folder", "FOLDER::com.apple.CloudDocs::B847FE2D", "com.apple.CloudDocs"},
		{"file in a container", "FILE::iCloud.md.obsidian::abc123", "iCloud.md.obsidian"},
		{"no zone", "root", defaultZone},
		{"empty", "", defaultZone},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, ZoneFromDriveID(test.id))
		})
	}
}

func TestZoneFromDriveIDRoundTrip(t *testing.T) {
	id := ConstructDriveID("abc123", "iCloud.md.obsidian", "FILE")
	assert.Equal(t, "FILE::iCloud.md.obsidian::abc123", id)
	assert.Equal(t, "iCloud.md.obsidian", ZoneFromDriveID(id))
}
