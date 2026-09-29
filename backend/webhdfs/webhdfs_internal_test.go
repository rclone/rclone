package webhdfs_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rclone/rclone/backend/webhdfs"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listStatusResponse is a real LISTSTATUS response as returned by a
// CDP/Knox-fronted WebHDFS gateway, including extra fields (fileId,
// childrenNum, storagePolicy) that are not modelled by api.FileStatus and
// must be ignored when decoding.
const listStatusResponse = `{
  "FileStatuses": {
    "FileStatus": [
      {
        "accessTime": 1790151429673,
        "blockSize": 134217728,
        "childrenNum": 0,
        "fileId": 3592542062,
        "group": "cdp_admins",
        "length": 0,
        "modificationTime": 1790151429676,
        "owner": "u_usertest",
        "pathSuffix": "_SUCCESS",
        "permission": "640",
        "replication": 3,
        "storagePolicy": 0,
        "type": "FILE"
      },
      {
        "accessTime": 1790151421902,
        "blockSize": 134217728,
        "childrenNum": 0,
        "fileId": 3592541931,
        "group": "cdp_admins",
        "length": 93642293,
        "modificationTime": 1790151429609,
        "owner": "u_usertest",
        "pathSuffix": "part-00000-be201fd4-c8e8-49c7-ac6a-c6afccaa9fa7-c000.snappy.parquet",
        "permission": "640",
        "replication": 3,
        "storagePolicy": 0,
        "type": "FILE"
      }
    ]
  }
}`

// TestListDecodesRealWorldResponse checks that List() correctly decodes a
// real LISTSTATUS response containing fields not present in api.FileStatus.
func TestListDecodesRealWorldResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("op") {
		case "GETFILESTATUS":
			w.Header().Set("Content-Type", "application/json")
			_, err := fmt.Fprint(w, `{"FileStatus":{"type":"DIRECTORY","length":0,"modificationTime":1790151429676,"accessTime":1790151429673}}`)
			require.NoError(t, err)
		case "LISTSTATUS":
			w.Header().Set("Content-Type", "application/json")
			_, err := fmt.Fprint(w, listStatusResponse)
			require.NoError(t, err)
		default:
			http.Error(w, "unexpected op", http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	configfile.Install()
	m := configmap.Simple{
		"type": "webhdfs",
		"url":  ts.URL,
	}
	f, err := webhdfs.NewFs(context.Background(), "TestWebHDFS", "", m)
	require.NoError(t, err)

	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, entries, 2)

	byRemote := make(map[string]fs.DirEntry, len(entries))
	for _, e := range entries {
		byRemote[e.Remote()] = e
	}

	success, ok := byRemote["_SUCCESS"]
	require.True(t, ok, "expected _SUCCESS entry")
	assert.Equal(t, int64(0), success.Size())

	parquet, ok := byRemote["part-00000-be201fd4-c8e8-49c7-ac6a-c6afccaa9fa7-c000.snappy.parquet"]
	require.True(t, ok, "expected parquet entry")
	assert.Equal(t, int64(93642293), parquet.Size())
}
