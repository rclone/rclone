package dosya

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldRetryHonoursRetryAfter(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		status int
		header string
		want   time.Duration
		ok     bool
	}{
		{"503 during a workspace move", http.StatusServiceUnavailable, "30", 30 * time.Second, true},
		{"429 from the edge", http.StatusTooManyRequests, "7", 7 * time.Second, true},
		{"capped", http.StatusServiceUnavailable, "86400", maxRetryAfter, true},
		{"missing header", http.StatusServiceUnavailable, "", 0, false},
		{"HTTP date is not seconds", http.StatusServiceUnavailable, "Wed, 21 Oct 2015 07:28:00 GMT", 0, false},
		{"502 ignores the header", http.StatusBadGateway, "30", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := jsonResp(tc.status, `{"ok":false,"error":"busy"}`)
			if tc.header != "" {
				resp.Header.Set("Retry-After", tc.header)
			}
			retry, err := shouldRetry(ctx, resp, errorHandler(resp))
			assert.True(t, retry)
			d, ok := pacer.IsRetryAfter(err)
			assert.Equal(t, tc.ok, ok)
			if tc.ok {
				assert.Equal(t, tc.want, d)
			}
		})
	}
}

func TestShouldRetryDoesNotRetryConflict(t *testing.T) {
	resp := jsonResp(http.StatusConflict, `{"ok":false,"error":"A folder with this name already exists"}`)
	resp.Header.Set("Retry-After", "30")
	retry, err := shouldRetry(context.Background(), resp, errorHandler(resp))
	assert.False(t, retry)
	_, ok := pacer.IsRetryAfter(err)
	assert.False(t, ok)
}

func TestRemoveFolderPurgesPermanently(t *testing.T) {
	// trash, then a purge that needs two bounded passes
	answers := []struct {
		status int
		body   string
	}{
		{http.StatusOK, `{"ok":true,"permanent":false,"files_affected":3,"folders_removed":2}`},
		{http.StatusAccepted, `{"ok":true,"permanent":true,"complete":false,"remaining":1}`},
		{http.StatusOK, `{"ok":true,"permanent":true,"complete":true,"remaining":0}`},
	}
	calls := 0
	f := newFakeFs(rtFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "DELETE", r.Method)
		require.Equal(t, "/api/folders/fld_1", r.URL.Path)
		require.Less(t, calls, len(answers), "DELETE sent after the purge completed")
		a := answers[calls]
		calls++
		return jsonResp(a.status, a.body), nil
	}))

	require.NoError(t, f.removeFolder(context.Background(), "fld_1"))
	assert.Equal(t, len(answers), calls)
}

func TestRemoveFolderStopsOnPurgeError(t *testing.T) {
	calls := 0
	f := newFakeFs(rtFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return jsonResp(http.StatusOK, `{"ok":true,"permanent":false}`), nil
		}
		return jsonResp(http.StatusForbidden, `{"ok":false,"error":"This folder contains a locked file and cannot be purged"}`), nil
	}))

	err := f.removeFolder(context.Background(), "fld_1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "locked file")
	assert.Equal(t, 2, calls)
}

// decodeBody reads a JSON request body into a map
func decodeBody(t *testing.T, r *http.Request) map[string]any {
	b, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	return m
}

func TestMoveFolderSendsName(t *testing.T) {
	for _, tc := range []struct {
		name        string
		newName     string
		respBody    string
		wantRenamed bool
	}{
		{"server honours name", "dst", `{"ok":true,"name":"dst"}`, true},
		{"older server ignores name", "dst", `{"ok":true}`, false},
		{"no rename requested", "", `{"ok":true,"name":"src"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeFs(rtFunc(func(r *http.Request) (*http.Response, error) {
				assert.Equal(t, "/api/folders/fld_1/move", r.URL.Path)
				body := decodeBody(t, r)
				assert.Equal(t, "fld_parent", body["parent_id"])
				if tc.newName == "" {
					assert.NotContains(t, body, "name")
				} else {
					assert.Equal(t, tc.newName, body["name"])
				}
				return jsonResp(http.StatusOK, tc.respBody), nil
			}))

			renamed, err := f.moveFolder(context.Background(), "fld_1", "fld_parent", tc.newName)
			require.NoError(t, err)
			assert.Equal(t, tc.wantRenamed, renamed)
		})
	}
}

func TestCopyFileSendsName(t *testing.T) {
	f := newFakeFs(rtFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "/api/files/fil_1/copy", r.URL.Path)
		body := decodeBody(t, r)
		assert.Equal(t, "fld_dst", body["folder_id"])
		assert.Equal(t, "final.txt", body["name"])
		return jsonResp(http.StatusCreated, `{"ok":true,"file_id":"fil_2","name":"final.txt"}`), nil
	}))

	resp, err := f.copyFile(context.Background(), "fil_1", "fld_dst", "final.txt")
	require.NoError(t, err)
	assert.Equal(t, "fil_2", resp.FileID)
	assert.Equal(t, "final.txt", resp.Name)
}
