package dosya

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every by-id call names the workspace, so the API can find an id it minted
// moments ago before its file_home index row lands.
func TestByIDCallsSendWorkspaceHint(t *testing.T) {
	seen := map[string]string{}
	f := newFakeFs(rtFunc(func(r *http.Request) (*http.Response, error) {
		seen[r.Method+" "+r.URL.Path] = r.Header.Get("X-Dosya-Workspace")
		switch r.URL.Path {
		case "/api/folders/fld_1":
			return jsonResp(http.StatusOK, `{"ok":true,"permanent":true,"complete":true}`), nil
		case "/api/files/fil_1":
			return jsonResp(http.StatusOK, `{"ok":true,"permanent":true}`), nil
		case "/api/folders/fld_1/move":
			return jsonResp(http.StatusOK, `{"ok":true,"name":"n"}`), nil
		default:
			return jsonResp(http.StatusOK, `{"ok":true}`), nil
		}
	}))
	ctx := context.Background()

	require.NoError(t, f.removeFolder(ctx, "fld_1"))
	require.NoError(t, f.renameFolder(ctx, "fld_1", "n"))
	_, err := f.moveFolder(ctx, "fld_1", "fld_2", "n")
	require.NoError(t, err)
	require.NoError(t, f.deleteFile(ctx, "fil_1"))
	require.NoError(t, f.renameFile(ctx, "fil_1", "n"))
	require.NoError(t, f.moveFile(ctx, "fil_1", "fld_2", "n"))
	_, err = f.copyFile(ctx, "fil_1", "fld_2", "n")
	require.NoError(t, err)
	_, err = f.createShareLink(ctx, "fil_1", 0)
	require.NoError(t, err)

	for call, header := range seen {
		assert.Equal(t, "ws_test", header, call)
	}
	assert.Len(t, seen, 8)
}

// The download's API request carries the hint; the presigned R2 fetch must not.
func TestDownloadSendsWorkspaceHintOnlyToTheAPI(t *testing.T) {
	var apiHint, blobHint string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/files/fil_1/download" {
			apiHint = r.Header.Get("X-Dosya-Workspace")
			http.Redirect(w, r, srv.URL+"/blob", http.StatusFound)
			return
		}
		blobHint = r.Header.Get("X-Dosya-Workspace")
		_, _ = w.Write([]byte("bytes"))
	}))
	defer srv.Close()
	f := newTestFs(srv)
	f.opt.APIURL = srv.URL

	rc, err := f.downloadFile(context.Background(), "fil_1", nil)
	require.NoError(t, err)
	_ = rc.Close()

	assert.Equal(t, "ws_test", apiHint)
	assert.Equal(t, "", blobHint)
}
